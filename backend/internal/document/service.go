package document

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
	"github.com/TSY244/ai-study-tutor/backend/internal/idempotency"
	"github.com/TSY244/ai-study-tutor/backend/internal/job"
	"github.com/TSY244/ai-study-tutor/backend/internal/lesson"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/gate"
)

// Kind 枚举（契约 DocumentKind）。
const (
	KindRawText        = "raw_text"
	KindStudentNotes   = "student_notes"
	KindTeacherOutline = "teacher_outline"
)

// ValidKind 判定 kind 是否为契约 DocumentKind 枚举值；非法值在 HTTP 层
// 映射 404（路径参数不合法视为资源不存在）。
func ValidKind(kind string) bool {
	switch kind {
	case KindRawText, KindStudentNotes, KindTeacherOutline:
		return true
	}
	return false
}

// RegeneratableKind 判定 kind 是否可被重生成（契约
// RegenerationCreate.kinds 只允许 student_notes/teacher_outline）。
func RegeneratableKind(kind string) bool {
	return kind == KindStudentNotes || kind == KindTeacherOutline
}

// JobAccepted 是 202 响应体（契约 AsyncAccepted：{"job": Job}）。
type JobAccepted struct {
	Job job.DTO `json:"job"`
}

// Service 编排文档只读查询与重生成任务创建；事务边界在本层（pool.Begin），
// SQL 全部下沉到 Repository。
type Service struct {
	pool    *pgxpool.Pool
	repo    Repository
	lessons lesson.Repository
	jobs    *job.Repository
	idem    *idempotency.Service
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{
		pool: pool,
		jobs: &job.Repository{},
		idem: idempotency.NewService(pool),
	}
}

// ListCurrent 返回课时的全部当前发布版本（每 kind 至多一条，按 kind 字典序）。
// 课时不存在返回 lesson.ErrNotFound。
func (s *Service) ListCurrent(ctx context.Context, lessonID uuid.UUID) ([]Document, error) {
	if _, err := s.lessons.GetByID(ctx, s.pool, lessonID); err != nil {
		return nil, err
	}
	return s.repo.ListActiveByLesson(ctx, s.pool, lessonID)
}

// GetCurrent 返回指定 kind 的当前发布版本；无 active 版本返回 ErrNotFound。
func (s *Service) GetCurrent(ctx context.Context, lessonID uuid.UUID, kind string) (*Document, error) {
	if _, err := s.lessons.GetByID(ctx, s.pool, lessonID); err != nil {
		return nil, err
	}
	return s.repo.GetActive(ctx, s.pool, lessonID, kind)
}

// ListVersions 按 revision DESC 返回历史版本；limit 为抓取上限（调用方传
// 页面大小 +1 以判定 has_more）。
func (s *Service) ListVersions(ctx context.Context, lessonID uuid.UUID, kind string,
	beforeRevision *int32, limit int) ([]Document, error) {
	if _, err := s.lessons.GetByID(ctx, s.pool, lessonID); err != nil {
		return nil, err
	}
	return s.repo.ListVersions(ctx, s.pool, lessonID, kind, beforeRevision, limit)
}

// GetVersion 按 ID 读取任意版本；不存在返回 ErrNotFound。
func (s *Service) GetVersion(ctx context.Context, id uuid.UUID) (*Document, error) {
	return s.repo.GetByID(ctx, s.pool, id)
}

// regenerationPayload 是 lesson_regenerate Job 的 payload（Worker 输入）。
type regenerationPayload struct {
	LessonID uuid.UUID `json:"lesson_id"`
	Kinds    []string  `json:"kinds"`
	Reason   string    `json:"reason"`
}

// CreateRegeneration 在单个事务内创建笔记重生成任务：业务门禁 → 课时存在 →
// 创建 lesson_regenerate Job（lesson_generation 族唯一）→ 同事务完成幂等
// 记录 → 提交。kinds 须已经 Handler 校验去重。
func (s *Service) CreateRegeneration(ctx context.Context, rec *idempotency.Record,
	lessonID uuid.UUID, kinds []string, reason string) (int, *JobAccepted, error) {
	payload, err := json.Marshal(regenerationPayload{LessonID: lessonID, Kinds: kinds, Reason: reason})
	if err != nil {
		return 0, nil, err
	}
	family := job.GenerationFamilyLessonGeneration
	j := &job.Job{
		Kind:             job.KindLessonRegenerate,
		AggregateType:    "lesson",
		AggregateID:      lessonID,
		GenerationFamily: &family,
		Payload:          payload,
		IdempotencyScope: &rec.Scope,
		IdempotencyKey:   &rec.Key,
	}
	return s.createGenerationJob(ctx, rec, lessonID, j)
}

// createGenerationJob 是创建类生成任务共用的提交路径；族唯一冲突时映射为
// 带 active_job_id 扩展的 409 resource_busy。
func (s *Service) createGenerationJob(ctx context.Context, rec *idempotency.Record,
	lessonID uuid.UUID, j *job.Job) (int, *JobAccepted, error) {
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
func (s *Service) activeJobConflict(ctx context.Context, lessonID uuid.UUID) error {
	active, err := s.jobs.GetActiveGenerationJob(ctx, s.pool, lessonID)
	if err != nil || active == nil {
		return nil
	}
	return job.ResourceBusyError(active.ID)
}
