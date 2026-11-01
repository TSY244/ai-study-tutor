// Package summary 实现学习总结评估（B3/B5）：
//
//   - POST /lessons/{lesson_id}/summary-evaluations：事务内创建
//     summary_evaluate Job 与评估占位行（202），评估由 Worker 异步完成；
//   - GET /summary-evaluations/{evaluation_id}：查询评估状态与结果。
//
// 事务边界（pool.Begin）在 Service 层，SQL 全部下沉到 Repository。
package summary

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
	"github.com/TSY244/ai-study-tutor/backend/internal/idempotency"
	"github.com/TSY244/ai-study-tutor/backend/internal/job"
	"github.com/TSY244/ai-study-tutor/backend/internal/lesson"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/gate"
)

// Accepted 是 202 响应体（契约 SummaryEvaluationAccepted：
// {"evaluation": SummaryEvaluation, "job": Job}）。
type Accepted struct {
	Evaluation DTO     `json:"evaluation"`
	Job        job.DTO `json:"job"`
}

// Service 编排总结评估的创建与查询。
type Service struct {
	pool    *pgxpool.Pool
	lessons lesson.Repository
	jobs    *job.Repository
	repo    Repository
	idem    *idempotency.Service
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{
		pool: pool,
		jobs: &job.Repository{},
		idem: idempotency.NewService(pool),
	}
}

// evaluatePayload 是 summary_evaluate Job 的 payload（Worker 输入快照：
// evaluation_id/lesson_id/summary 三件套全传）。
type evaluatePayload struct {
	EvaluationID uuid.UUID `json:"evaluation_id"`
	LessonID     uuid.UUID `json:"lesson_id"`
	Summary      string    `json:"summary"`
}

// CreateEvaluation 在单个事务内创建总结评估：业务门禁 → 课时存在 →
// 创建 summary_evaluate Job（aggregate 指向评估，无互斥约束故不设
// generation_family）→ 插入评估占位行（外键要求先建 Job）→ 同事务完成
// 幂等记录 → 提交。
func (s *Service) CreateEvaluation(ctx context.Context, rec *idempotency.Record,
	lessonID uuid.UUID, summaryText string) (int, *Accepted, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := gate.AssertBusinessOpen(ctx, tx); err != nil {
		return 0, nil, err
	}
	if _, err := s.lessons.GetByIDForGeneration(ctx, tx, lessonID); err != nil {
		if errors.Is(err, lesson.ErrNotFound) {
			// httpx 不能反向依赖 lesson 包，404 映射在本模块完成。
			return 0, nil, httpx.NewProblemError(http.StatusNotFound, "not_found",
				"资源不存在", "请求的资源不存在或已删除。")
		}
		return 0, nil, err
	}
	body, err := s.createInTx(ctx, tx, rec, lessonID, summaryText)
	if err != nil {
		return 0, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, nil, err
	}
	return http.StatusAccepted, body, nil
}

// GetEvaluation 读取单个评估；不存在返回 404 not_found。
func (s *Service) GetEvaluation(ctx context.Context, id uuid.UUID) (*DTO, error) {
	e, err := s.repo.GetByID(ctx, s.pool, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, httpx.NewProblemError(http.StatusNotFound, "not_found",
				"资源不存在", "请求的资源不存在或已删除。")
		}
		return nil, err
	}
	dto := ToDTO(e)
	return &dto, nil
}

func (s *Service) createInTx(ctx context.Context, tx pgx.Tx, rec *idempotency.Record, lessonID uuid.UUID, summaryText string) (*Accepted, error) {
	evaluationID := uuid.New()
	payload, err := json.Marshal(evaluatePayload{
		EvaluationID: evaluationID,
		LessonID:     lessonID,
		Summary:      summaryText,
	})
	if err != nil {
		return nil, err
	}
	j := &job.Job{
		Kind:             job.KindSummaryEvaluate,
		AggregateType:    "summary_evaluation",
		AggregateID:      evaluationID,
		Payload:          payload,
		IdempotencyScope: &rec.Scope,
		IdempotencyKey:   &rec.Key,
	}
	if err := s.jobs.Create(ctx, tx, j); err != nil {
		return nil, err
	}
	e := &Evaluation{
		ID:          evaluationID,
		LessonID:    lessonID,
		SummaryText: summaryText,
		JobID:       &j.ID,
	}
	if err := s.repo.Create(ctx, tx, e); err != nil {
		return nil, err
	}
	body := &Accepted{Evaluation: ToDTO(e), Job: job.ToDTO(j, nil)}
	if err := httpx.CompleteJSON(ctx, tx, s.idem, rec.ID, http.StatusAccepted, body, nil, nil, nil, &j.ID); err != nil {
		return nil, err
	}

	return body, nil
}
