// service.go 是作答用例的应用服务：创建、查询、单题自动保存、提交
// （200/202 拆分）与 grading-jobs 重建。事务边界在本层（pool.Begin）；
// SQL 在 Repository；判分为纯函数（grading.go）。
package attempt

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
	"github.com/TSY244/ai-study-tutor/backend/internal/idempotency"
	"github.com/TSY244/ai-study-tutor/backend/internal/job"
	"github.com/TSY244/ai-study-tutor/backend/internal/lesson"
	"github.com/TSY244/ai-study-tutor/backend/internal/mastery"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/database"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/gate"
	"github.com/TSY244/ai-study-tutor/backend/internal/quiz"
)

// Service 编排作答用例。
type Service struct {
	pool    *pgxpool.Pool
	repo    Repository
	quizzes quiz.Repository
	lessons lesson.Repository
	jobs    *job.Repository
	idem    *idempotency.Service
	apply   *mastery.ApplyService
}

func NewService(pool *pgxpool.Pool) *Service {
	repo := Repository{}
	return &Service{
		pool:    pool,
		repo:    repo,
		quizzes: quiz.Repository{},
		lessons: lesson.Repository{},
		jobs:    &job.Repository{},
		idem:    idempotency.NewService(pool),
		apply:   mastery.NewApplyService(repo),
	}
}

// notFound 映射稳定的 404。
func notFound(detail string) *httpx.ProblemError {
	return httpx.NewProblemError(http.StatusNotFound, "not_found", "资源不存在", detail)
}

// ListAttempts 按 started_at DESC, id DESC 游标分页返回课时作答历史；
// 课时不存在返回 404。调用方按 limit+1 判定 hasMore。
func (s *Service) ListAttempts(ctx context.Context, lessonID uuid.UUID,
	afterStartedAt *time.Time, afterID *uuid.UUID, limit int) ([]SummaryRow, error) {
	if _, err := s.lessons.GetByID(ctx, s.pool, lessonID); err != nil {
		if errors.Is(err, lesson.ErrNotFound) {
			return nil, notFound("请求的资源不存在或已删除。")
		}
		return nil, err
	}
	return s.repo.ListByLesson(ctx, s.pool, lessonID, afterStartedAt, afterID, limit+1)
}

// CreateAttempt 开始一次作答（201 + Location）。试卷必须已发布（ready）；
// draft/archived 返回 422。幂等由调用方（IdemExecutor）保证。
func (s *Service) CreateAttempt(ctx context.Context, rec *idempotency.Record, quizID uuid.UUID) (int, *AttemptDTO, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := gate.AssertBusinessOpen(ctx, tx); err != nil {
		return 0, nil, err
	}
	var lockedQuiz uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT id FROM quizzes WHERE id=$1 AND deleted_at IS NULL FOR KEY SHARE`, quizID).Scan(&lockedQuiz); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil, notFound("试卷不存在或已删除。")
		}
		return 0, nil, err
	}
	z, questions, err := s.quizzes.GetWithQuestions(ctx, tx, quizID)
	if err != nil {
		if errors.Is(err, quiz.ErrNotFound) {
			return 0, nil, notFound("请求的资源不存在或已删除。")
		}
		return 0, nil, err
	}
	if z.Status != "ready" {
		return 0, nil, httpx.NewProblemError(http.StatusUnprocessableEntity, "validation_error",
			"试卷不可作答", "试卷未发布或已归档，不能开始新的作答。")
	}
	a := &Attempt{QuizID: quizID}
	if err := s.repo.Create(ctx, tx, a); err != nil {
		return 0, nil, err
	}
	dto := BuildAttemptDTO(a, z, questions, nil)
	resourceType := "attempt"
	headers := map[string]string{"Location": "/api/v1/attempts/" + a.ID.String()}
	if err := httpx.CompleteJSON(ctx, tx, s.idem, rec.ID, http.StatusCreated, dto,
		headers, &resourceType, &a.ID, nil); err != nil {
		return 0, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, nil, err
	}
	return http.StatusCreated, &dto, nil
}

// GetAttempt 查询作答与当前批阅状态（反馈可见性见 BuildAttemptDTO）。
func (s *Service) GetAttempt(ctx context.Context, attemptID uuid.UUID) (*AttemptDTO, error) {
	a, err := s.repo.GetByID(ctx, s.pool, attemptID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, notFound("请求的资源不存在或已删除。")
		}
		return nil, err
	}
	z, questions, err := s.quizzes.GetWithQuestions(ctx, s.pool, a.QuizID)
	if err != nil {
		if errors.Is(err, quiz.ErrNotFound) {
			return nil, notFound("作答关联的试卷不存在。")
		}
		return nil, err
	}
	answers, err := s.repo.ListAnswers(ctx, s.pool, attemptID)
	if err != nil {
		return nil, err
	}
	dto := BuildAttemptDTO(a, z, questions, answers)
	return &dto, nil
}

// SaveAnswer 单题自动保存（PATCH，非幂等键端点——answer_version 即乐观锁）。
// 版本冲突返回 409 version_conflict，扩展携带服务端当前状态。
func (s *Service) SaveAnswer(ctx context.Context, attemptID, questionID uuid.UUID,
	answer json.RawMessage, answerVersion int64) (*SavedAnswerDTO, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := gate.AssertBusinessOpen(ctx, tx); err != nil {
		return nil, err
	}
	// 题目必须属于该作答的试卷（否则 404——对该作答而言不存在）。
	a, err := s.repo.GetByID(ctx, tx, attemptID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, notFound("请求的资源不存在或已删除。")
		}
		return nil, err
	}
	z, questions, err := s.quizzes.GetWithQuestions(ctx, tx, a.QuizID)
	if err != nil {
		return nil, err
	}
	if !questionInQuiz(questions, questionID) {
		return nil, notFound("该题不属于本作答的试卷。")
	}
	res, err := s.repo.SaveAnswer(ctx, tx, attemptID, questionID, answer, answerVersion)
	if err != nil {
		if errors.Is(err, ErrAnswerVersionConflict) {
			return nil, s.answerConflict(ctx, a, z.ID, questionID)
		}
		if errors.Is(err, ErrNotInProgress) {
			return nil, httpx.NewProblemError(http.StatusConflict, "attempt_already_submitted",
				"作答已提交", "作答已提交，不能再修改。")
		}
		if errors.Is(err, ErrNotFound) {
			return nil, notFound("请求的资源不存在或已删除。")
		}
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &SavedAnswerDTO{
		AttemptID:      attemptID.String(),
		QuestionID:     questionID.String(),
		Answer:         answer,
		AnswerVersion:  res.AnswerVersion,
		AttemptVersion: res.AttemptVersion,
		SavedAt:        httpx.Time(res.SavedAt),
	}, nil
}

// answerConflict 构造带服务端当前状态的 409 version_conflict（04 §10.3 示例）。
// 原事务已因冲突放弃，经连接池重新读取当前状态。
func (s *Service) answerConflict(ctx context.Context, a *Attempt, quizID, questionID uuid.UUID) error {
	p := httpx.NewProblemError(http.StatusConflict, "version_conflict",
		"答案版本冲突", "服务端已有更新的答案。")
	ext := map[string]any{}
	current, err := s.repo.GetByID(ctx, s.pool, a.ID)
	if err == nil {
		ext["current_attempt_version"] = current.Version
	}
	ans, err := s.repo.GetAnswer(ctx, s.pool, a.ID, questionID)
	if err == nil {
		ext["current_answer_version"] = ans.AnswerVersion
		var value json.RawMessage
		if len(ans.Answer) > 0 && string(ans.Answer) != "null" {
			value = ans.Answer
		} else {
			value = json.RawMessage(`{"value":null}`)
		}
		ext["current_answer"] = value
	} else {
		ext["current_answer_version"] = 0
		ext["current_answer"] = json.RawMessage(`{"value":null}`)
	}
	p.Problem.Extensions = ext
	return p
}

func questionInQuiz(questions []quiz.QuizQuestion, questionID uuid.UUID) bool {
	for _, qq := range questions {
		if qq.Question.ID == questionID {
			return true
		}
	}
	return false
}

// Submit 提交作答（04 §10.4）：
//   - 锁定 Attempt、校验全局 version（不一致 → 409 version_conflict）；
//   - 补齐整卷答案行、客观题同步判分（auto_score 为暂定分）；
//   - 全客观题：同步完成（graded，final_score=auto_score）并在同事务应用
//     weakness evidence 与 mastery snapshots，返回 200 AttemptSubmitCompleted；
//   - 含主观题：创建 attempt_grade Job 并进入 grading，返回
//     202 AttemptSubmitAccepted（必含 Job；final_score 保持 null，
//     主观题未批阅前不得显示伪造总分）。
func (s *Service) Submit(ctx context.Context, rec *idempotency.Record,
	attemptID uuid.UUID, version int64) (int, any, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := gate.AssertBusinessOpen(ctx, tx); err != nil {
		return 0, nil, err
	}
	if err := s.repo.LockSubmissionParents(ctx, tx, attemptID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return 0, nil, notFound("请求的资源不存在或已删除。")
		}
		return 0, nil, err
	}
	a, err := s.repo.LockForUpdate(ctx, tx, attemptID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return 0, nil, notFound("请求的资源不存在或已删除。")
		}
		return 0, nil, err
	}
	if a.Status != StatusInProgress {
		return 0, nil, httpx.NewProblemError(http.StatusConflict, "attempt_already_submitted",
			"作答已提交", "作答已提交，不能重复提交。")
	}
	if a.Version != version {
		return 0, nil, versionConflict(a.Version)
	}
	z, questions, err := s.quizzes.GetWithQuestions(ctx, tx, a.QuizID)
	if err != nil {
		return 0, nil, err
	}
	if err := s.repo.EnsureAnswerRows(ctx, tx, a.ID, z.ID); err != nil {
		return 0, nil, err
	}
	answers, err := s.repo.ListAnswers(ctx, tx, a.ID)
	if err != nil {
		return 0, nil, err
	}
	byQuestion := make(map[uuid.UUID]Answer, len(answers))
	for _, ans := range answers {
		byQuestion[ans.QuestionID] = ans
	}

	hasSubjective := false
	autoSum := 0.0
	outcomes := make([]mastery.OutcomeItem, 0, len(questions))
	for _, qq := range questions {
		ans := byQuestion[qq.Question.ID]
		if !ObjectiveType(qq.Question.Type) {
			hasSubjective = true
			continue
		}
		isCorrect, awarded := GradeObjective(qq.Question.Type, qq.Question.CorrectAnswer, ans.Answer, qq.Score)
		feedback, err := buildObjectiveFeedback(qq, isCorrect, awarded)
		if err != nil {
			return 0, nil, err
		}
		if err := s.repo.ApplyObjectiveResult(ctx, tx, a.ID, qq.Question.ID, isCorrect, awarded, feedback); err != nil {
			return 0, nil, err
		}
		autoSum += awarded
		outcomes = append(outcomes, mastery.OutcomeItem{
			QuestionID: qq.Question.ID,
			Concepts:   conceptsOf(qq.Question.TestedConcepts),
			Correct:    isCorrect,
			Note:       "客观题作答判分",
		})
	}

	var status int
	var body any
	if hasSubjective {
		status, body, err = s.submitGrading(ctx, tx, rec, a, z, questions, autoSum)
	} else {
		status, body, err = s.submitObjectiveComplete(ctx, tx, rec, a, z, questions, autoSum, outcomes)
	}
	if err != nil {
		return 0, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, nil, err
	}
	return status, body, nil
}

// submitObjectiveComplete 全客观题路径：同事务完成判分、记忆应用与幂等记录。
func (s *Service) submitObjectiveComplete(ctx context.Context, tx database.Querier, rec *idempotency.Record,
	a *Attempt, z *quiz.Quiz, questions []quiz.QuizQuestion, autoSum float64,
	outcomes []mastery.OutcomeItem) (int, any, error) {
	if err := s.repo.CompleteObjectiveSubmit(ctx, tx, a.ID, autoSum); err != nil {
		return 0, nil, err
	}
	// 记忆应用（同事务）：课程解析自课时。
	les, err := s.lessons.GetByID(ctx, tx, z.LessonID)
	if err != nil {
		return 0, nil, err
	}
	if _, err := s.apply.ApplyAttemptOutcome(ctx, tx, mastery.ApplyInput{
		CourseID: les.CourseID, LessonID: z.LessonID,
		AttemptID: a.ID, QuizID: z.ID, Items: outcomes,
	}); err != nil {
		return 0, nil, err
	}
	return s.completeSubmit(ctx, tx, rec, a, z, questions, http.StatusOK, nil)
}

// submitGrading 含主观题路径：创建 attempt_grade Job（同一 Attempt 活动
// 批阅唯一，uq_jobs_active_attempt_grade 兜底），attempt 进入 grading。
func (s *Service) submitGrading(ctx context.Context, tx database.Querier, rec *idempotency.Record,
	a *Attempt, z *quiz.Quiz, questions []quiz.QuizQuestion, autoSum float64) (int, any, error) {
	payload, err := json.Marshal(map[string]any{
		"attempt_id": a.ID.String(), "quiz_id": z.ID.String(),
	})
	if err != nil {
		return 0, nil, err
	}
	j := &job.Job{
		Kind:             job.KindAttemptGrade,
		AggregateType:    "attempt",
		AggregateID:      a.ID,
		Payload:          payload,
		IdempotencyScope: &rec.Scope,
		IdempotencyKey:   &rec.Key,
	}
	if err := s.jobs.Create(ctx, tx, j); err != nil {
		if errors.Is(err, job.ErrActiveAttemptGradeExists) {
			return 0, nil, s.activeGradingJobConflict(ctx, a.ID)
		}
		return 0, nil, err
	}
	if err := s.repo.StartGrading(ctx, tx, a.ID, j.ID, autoSum); err != nil {
		return 0, nil, err
	}
	return s.completeSubmit(ctx, tx, rec, a, z, questions, http.StatusAccepted, j)
}

// completeSubmit 重读更新后的作答与答案，构造响应并完成幂等记录。
func (s *Service) completeSubmit(ctx context.Context, tx database.Querier, rec *idempotency.Record,
	a *Attempt, z *quiz.Quiz, questions []quiz.QuizQuestion, status int, j *job.Job) (int, any, error) {
	updated, err := s.repo.GetByID(ctx, tx, a.ID)
	if err != nil {
		return 0, nil, err
	}
	answers, err := s.repo.ListAnswers(ctx, tx, a.ID)
	if err != nil {
		return 0, nil, err
	}
	dto := BuildAttemptDTO(updated, z, questions, answers)
	var body any
	if j != nil {
		body = SubmitAcceptedBody{Attempt: dto, Job: job.ToDTO(j, nil)}
	} else {
		body = SubmitCompletedBody{Attempt: dto}
	}
	if err := httpx.CompleteJSON(ctx, tx, s.idem, rec.ID, status, body, nil, nil, nil, nil); err != nil {
		return 0, nil, err
	}
	return status, body, nil
}

// CreateGradingJob 为 grading_failed 的作答重建批阅任务（04 §10.5）：
// 要求答案与不可变试卷快照完整、当前无活动批阅 Job；新 Job 的
// retry_of_job_id 指向原 grading_job_id（若存在）。成功返回 202 AsyncAccepted。
func (s *Service) CreateGradingJob(ctx context.Context, rec *idempotency.Record, attemptID uuid.UUID) (int, any, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := gate.AssertBusinessOpen(ctx, tx); err != nil {
		return 0, nil, err
	}
	if err := s.repo.LockSubmissionParents(ctx, tx, attemptID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return 0, nil, notFound("请求的资源不存在或已删除。")
		}
		return 0, nil, err
	}
	a, err := s.repo.LockForUpdate(ctx, tx, attemptID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return 0, nil, notFound("请求的资源不存在或已删除。")
		}
		return 0, nil, err
	}
	switch a.Status {
	case StatusGradingFailed:
		// 唯一可重建状态（活动任务由下方唯一索引/预检查拦截）。
	case StatusInProgress:
		return 0, nil, httpx.NewProblemError(http.StatusUnprocessableEntity, "validation_error",
			"作答尚未提交", "作答尚未提交，不能创建批阅任务。")
	case StatusGrading:
		// grading 中正常情形是已有活动批阅任务 → 409 resource_busy。
		if active, err := s.jobs.GetActiveAttemptGradeJob(ctx, tx, a.ID); err != nil {
			return 0, nil, err
		} else if active != nil {
			p := httpx.NewProblemError(http.StatusConflict, "resource_busy",
				"资源繁忙", "该作答已有进行中的批阅任务，请关联到现有任务。")
			p.Problem.Extensions = map[string]any{"active_job_id": active.ID.String()}
			return 0, nil, p
		}
		return 0, nil, httpx.NewProblemError(http.StatusConflict, "attempt_already_submitted",
			"作答状态不允许重建", "只有批阅失败的作答可以重建批阅任务。")
	default:
		return 0, nil, httpx.NewProblemError(http.StatusConflict, "attempt_already_submitted",
			"作答状态不允许重建", "只有批阅失败的作答可以重建批阅任务。")
	}
	// 旧隔离数据不完整（无总分快照）→ 稳定、可行动错误，不创建 Job。
	if a.MaxScore == nil {
		return 0, nil, httpx.NewProblemError(http.StatusUnprocessableEntity, "validation_error",
			"作答数据不完整", "该作答缺少分数快照（旧数据已隔离），不能重建批阅任务。")
	}
	_, questions, err := s.quizzes.GetWithQuestions(ctx, tx, a.QuizID)
	if err != nil || len(questions) == 0 {
		return 0, nil, httpx.NewProblemError(http.StatusUnprocessableEntity, "validation_error",
			"作答数据不完整", "该作答的试卷快照不完整（旧数据已隔离），不能重建批阅任务。")
	}
	payload, err := json.Marshal(map[string]any{
		"attempt_id": a.ID.String(), "quiz_id": a.QuizID.String(),
	})
	if err != nil {
		return 0, nil, err
	}
	j := &job.Job{
		Kind:             job.KindAttemptGrade,
		AggregateType:    "attempt",
		AggregateID:      a.ID,
		Payload:          payload,
		RetryOfJobID:     a.GradingJobID,
		IdempotencyScope: &rec.Scope,
		IdempotencyKey:   &rec.Key,
	}
	if err := s.jobs.Create(ctx, tx, j); err != nil {
		if errors.Is(err, job.ErrActiveAttemptGradeExists) {
			return 0, nil, s.activeGradingJobConflict(ctx, a.ID)
		}
		return 0, nil, err
	}
	if err := s.repo.RestartGrading(ctx, tx, a.ID, j.ID); err != nil {
		return 0, nil, err
	}
	body := JobAcceptedBody{Job: job.ToDTO(j, nil)}
	if err := httpx.CompleteJSON(ctx, tx, s.idem, rec.ID, http.StatusAccepted, body, nil, nil, nil, &j.ID); err != nil {
		return 0, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, nil, err
	}
	return http.StatusAccepted, body, nil
}

// activeGradingJobConflict 在活动批阅 Job 唯一冲突后查询活动任务，构造带
// active_job_id 扩展的 409 resource_busy（原事务已被唯一冲突中止，经连接池重查）。
func (s *Service) activeGradingJobConflict(ctx context.Context, attemptID uuid.UUID) error {
	active, err := s.jobs.GetActiveAttemptGradeJob(ctx, s.pool, attemptID)
	if err != nil || active == nil {
		return err
	}
	p := httpx.NewProblemError(http.StatusConflict, "resource_busy",
		"资源繁忙", "该作答已有进行中的批阅任务，请关联到现有任务。")
	p.Problem.Extensions = map[string]any{"active_job_id": active.ID.String()}
	return p
}

// versionConflict 构造带 current_version 扩展的 409 version_conflict。
func versionConflict(current int64) *httpx.ProblemError {
	p := httpx.NewProblemError(http.StatusConflict, "version_conflict",
		"作答版本冲突", "服务端已有更新的作答状态，请刷新后重试。")
	p.Problem.Extensions = map[string]any{"current_version": current}
	return p
}

// conceptsOf 解析 tested_concepts JSONB 为字符串数组（缺省空数组）。
func conceptsOf(raw []byte) []string {
	out := []string{}
	if len(raw) == 0 {
		return out
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return []string{}
	}
	return out
}

// buildObjectiveFeedback 构造客观题的确定性 QuestionFeedback 快照
// （提交时写入；correct_answer/explanation 只在 graded 状态对学生可见）。
func buildObjectiveFeedback(qq quiz.QuizQuestion, isCorrect *bool, awarded float64) ([]byte, error) {
	concepts := conceptsOf(qq.Question.TestedConcepts)
	weak := []string{}
	text := "回答正确"
	switch {
	case isCorrect == nil:
		text = "未作答"
	case !*isCorrect:
		text = "回答错误"
		weak = concepts
	}
	var correctValue json.RawMessage
	if len(qq.Question.CorrectAnswer) > 0 {
		correctValue = qq.Question.CorrectAnswer
	} else {
		correctValue = json.RawMessage(`null`)
	}
	correctAnswer, err := json.Marshal(map[string]json.RawMessage{"value": correctValue})
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"question_id":           qq.Question.ID.String(),
		"awarded_score":         awarded,
		"max_score":             qq.Score,
		"is_correct":            isCorrect,
		"feedback":              text,
		"correct_understanding": "",
		"correct_answer":        json.RawMessage(correctAnswer),
		"explanation":           qq.Question.Explanation,
		"weak_concepts":         weak,
	})
}
