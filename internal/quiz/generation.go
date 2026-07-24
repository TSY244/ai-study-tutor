package quiz

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/internal/httpx"
	"github.com/TSY244/ai-study-tutor/internal/idempotency"
	"github.com/TSY244/ai-study-tutor/internal/job"
	"github.com/TSY244/ai-study-tutor/internal/lesson"
	"github.com/TSY244/ai-study-tutor/internal/platform/database"
	"github.com/TSY244/ai-study-tutor/internal/platform/gate"
)

// JobAccepted 是 202 响应体（契约 AsyncAccepted：{"job": Job}）。
type JobAccepted struct {
	Job job.DTO `json:"job"`
}

// GenerationService 编排常规/复习试卷生成任务的创建；事务边界在本层
// （pool.Begin），SQL 全部下沉到 Repository。
type GenerationService struct {
	pool    *pgxpool.Pool
	lessons lesson.Repository
	jobs    *job.Repository
	idem    *idempotency.Service
	// QuizPolicy 是常规练习出题的策略钩子：返回 false 表示当前不需要继续
	// 生成普通练习（映射 422 practice_not_needed，不创建 Job）。
	// B4 将替换为基于掌握度/薄弱点的真实策略实现；当前默认放行。
	QuizPolicy func(ctx context.Context, q database.Querier, lessonID uuid.UUID) (needed bool, err error)
}

func NewGenerationService(pool *pgxpool.Pool) *GenerationService {
	return &GenerationService{
		pool:       pool,
		jobs:       &job.Repository{},
		idem:       idempotency.NewService(pool),
		QuizPolicy: defaultQuizPolicy,
	}
}

// defaultQuizPolicy 是默认策略：始终判定需要练习（B4 替换为真实策略）。
func defaultQuizPolicy(context.Context, database.Querier, uuid.UUID) (bool, error) {
	return true, nil
}

// quizGenerationPayload 是 quiz_generate Job 的 payload（Worker 输入）；
// question_count/title 缺省时序列化为 null。
type quizGenerationPayload struct {
	LessonID      uuid.UUID `json:"lesson_id"`
	QuestionCount *int32    `json:"question_count"`
	Title         *string   `json:"title"`
}

// reviewQuizGenerationPayload 是 review_quiz_generate Job 的 payload；
// 数组字段缺省时序列化为 []（调用方保证非 nil）。
type reviewQuizGenerationPayload struct {
	LessonID         uuid.UUID   `json:"lesson_id"`
	QuestionCount    *int32      `json:"question_count"`
	IncludeLessonIDs []uuid.UUID `json:"include_lesson_ids"`
	WeaknessIDs      []uuid.UUID `json:"weakness_ids"`
}

// CreateQuizGeneration 在单个事务内创建常规试卷生成任务：业务门禁 → 课时
// 存在 → 练习策略钩子 → 创建 quiz_generate Job（lesson_generation 族唯一）→
// 同事务完成幂等记录 → 提交。
// 显式 question_count 是用户覆盖（legacy：user_override 短路提前停止评估），
// 不再调用策略钩子；question_count 为空时才由策略决定是否继续练习。
func (s *GenerationService) CreateQuizGeneration(ctx context.Context, rec *idempotency.Record,
	lessonID uuid.UUID, questionCount *int32, title *string) (int, *JobAccepted, error) {
	payload, err := json.Marshal(quizGenerationPayload{
		LessonID:      lessonID,
		QuestionCount: questionCount,
		Title:         title,
	})
	if err != nil {
		return 0, nil, err
	}
	family := job.GenerationFamilyLessonGeneration
	j := &job.Job{
		Kind:             job.KindQuizGenerate,
		AggregateType:    "lesson",
		AggregateID:      lessonID,
		GenerationFamily: &family,
		Payload:          payload,
		IdempotencyScope: &rec.Scope,
		IdempotencyKey:   &rec.Key,
	}
	return s.runGenerationTx(ctx, rec, lessonID, j, func(txCtx context.Context, q database.Querier) error {
		if s.QuizPolicy == nil || questionCount != nil {
			return nil // 无钩子或用户显式覆盖题量：不评估提前停止
		}
		needed, err := s.QuizPolicy(txCtx, q, lessonID)
		if err != nil {
			return err
		}
		if !needed {
			return httpx.NewProblemError(http.StatusUnprocessableEntity, "practice_not_needed",
				"暂不需要练习", "当前策略判断不需要继续生成普通练习。")
		}
		return nil
	})
}

// CreateReviewQuizGeneration 在单个事务内创建复习试卷生成任务（无练习策略
// 限制：复习场景不受 practice_not_needed 约束）。缺省数组字段归一化为 []。
func (s *GenerationService) CreateReviewQuizGeneration(ctx context.Context, rec *idempotency.Record,
	lessonID uuid.UUID, questionCount *int32, includeLessonIDs, weaknessIDs []uuid.UUID) (int, *JobAccepted, error) {
	if includeLessonIDs == nil {
		includeLessonIDs = []uuid.UUID{}
	}
	if weaknessIDs == nil {
		weaknessIDs = []uuid.UUID{}
	}
	payload, err := json.Marshal(reviewQuizGenerationPayload{
		LessonID:         lessonID,
		QuestionCount:    questionCount,
		IncludeLessonIDs: includeLessonIDs,
		WeaknessIDs:      weaknessIDs,
	})
	if err != nil {
		return 0, nil, err
	}
	family := job.GenerationFamilyLessonGeneration
	j := &job.Job{
		Kind:             job.KindReviewQuizGenerate,
		AggregateType:    "lesson",
		AggregateID:      lessonID,
		GenerationFamily: &family,
		Payload:          payload,
		IdempotencyScope: &rec.Scope,
		IdempotencyKey:   &rec.Key,
	}
	return s.runGenerationTx(ctx, rec, lessonID, j, nil)
}

// runGenerationTx 是生成任务创建的共用提交路径；beforeCreate 在门禁与课时
// 检查之后、Job 创建之前执行（如练习策略钩子），返回非 nil 错误即回滚。
// 族唯一冲突映射为带 active_job_id 扩展的 409 resource_busy。
func (s *GenerationService) runGenerationTx(ctx context.Context, rec *idempotency.Record,
	lessonID uuid.UUID, j *job.Job,
	beforeCreate func(txCtx context.Context, q database.Querier) error) (int, *JobAccepted, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := gate.AssertBusinessOpen(ctx, tx); err != nil {
		return 0, nil, err
	}
	if _, err := s.lessons.GetByID(ctx, tx, lessonID); err != nil {
		if errors.Is(err, lesson.ErrNotFound) {
			// httpx 不能反向依赖 lesson 包，404 映射在本模块完成。
			return 0, nil, httpx.NewProblemError(http.StatusNotFound, "not_found",
				"资源不存在", "请求的资源不存在或已删除。")
		}
		return 0, nil, err
	}
	if beforeCreate != nil {
		if err := beforeCreate(ctx, tx); err != nil {
			return 0, nil, err
		}
	}
	if err := s.jobs.Create(ctx, tx, j); err != nil {
		if errors.Is(err, job.ErrActiveGenerationJobExists) {
			if busy := s.activeJobConflict(ctx, lessonID); busy != nil {
				return 0, nil, busy
			}
		}
		return 0, nil, err
	}
	body := &JobAccepted{Job: job.ToDTO(j, nil)}
	if err := httpx.CompleteJSON(ctx, tx, s.idem, rec.ID, http.StatusAccepted, body, nil, nil, nil, &j.ID); err != nil {
		return 0, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, nil, err
	}
	return http.StatusAccepted, body, nil
}

// activeJobConflict 在族唯一冲突后查询活动任务，构造带 active_job_id 扩展的
// 409 resource_busy；查询失败或无活动任务时返回 nil（回退为普通错误映射）。
// 原事务已因唯一冲突被 PostgreSQL 中止，必须经由连接池重新查询。
func (s *GenerationService) activeJobConflict(ctx context.Context, lessonID uuid.UUID) error {
	active, err := s.jobs.GetActiveGenerationJob(ctx, s.pool, lessonID)
	if err != nil || active == nil {
		return nil
	}
	return job.ResourceBusyError(active.ID)
}
