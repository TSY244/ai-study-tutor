package summary

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/TSY244/ai-study-tutor/backend/internal/platform/database"
)

var ErrNotFound = errors.New("总结评估不存在")

// 表 status 取值（CHECK 约束；契约 SummaryEvaluationStatus 的映射见 dto.go）。
// 表无 queued：创建即 evaluating。
const (
	StatusEvaluating = "evaluating"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)

// Evaluation 是 summary_evaluations 表的领域表示。Result 为可空 JSONB；
// Error* 字段来自 jobs 左连接（失败任务的稳定错误信息，可为空）。
type Evaluation struct {
	ID             uuid.UUID
	LessonID       uuid.UUID
	SummaryText    string
	Result         []byte
	Status         string
	JobID          *uuid.UUID
	ErrorCode      *string
	ErrorDetail    *string
	ErrorRetryable *bool
	CreatedAt      time.Time
	CompletedAt    *time.Time
}

type Repository struct{}

// Create 插入评估占位行（初始 status 只能是 evaluating）并回读 created_at。
// JobID 引用同事务中已创建的 Job（外键约束要求先建 Job）。
func (Repository) Create(ctx context.Context, q database.Querier, e *Evaluation) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	e.Status = StatusEvaluating
	return q.QueryRow(ctx, `
		INSERT INTO summary_evaluations (id, lesson_id, summary_text, status, job_id)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING created_at`,
		e.ID, e.LessonID, e.SummaryText, e.Status, e.JobID).Scan(&e.CreatedAt)
}

// GetByID 读取单个评估（左连接 jobs 取失败错误信息）；不存在返回 ErrNotFound。
func (Repository) GetByID(ctx context.Context, q database.Querier, id uuid.UUID) (*Evaluation, error) {
	var e Evaluation
	err := q.QueryRow(ctx, `
		SELECT se.id, se.lesson_id, se.summary_text, se.result, se.status, se.job_id,
		       j.error_code, j.error_detail, j.error_retryable,
		       se.created_at, se.completed_at
		FROM summary_evaluations se
		LEFT JOIN jobs j ON j.id = se.job_id
		WHERE se.id = $1`, id).Scan(
		&e.ID, &e.LessonID, &e.SummaryText, &e.Result, &e.Status, &e.JobID,
		&e.ErrorCode, &e.ErrorDetail, &e.ErrorRetryable,
		&e.CreatedAt, &e.CompletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (Repository) Complete(ctx context.Context, q database.Querier, id uuid.UUID, result []byte) error {
	tag, err := q.Exec(ctx, `
		UPDATE summary_evaluations
		SET status = 'completed', result = $2, completed_at = now()
		WHERE id = $1`, id, result)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (Repository) Fail(ctx context.Context, q database.Querier, id uuid.UUID, code string) error {
	payload := []byte(`{"code":"` + code + `"}`)
	tag, err := q.Exec(ctx, `
		UPDATE summary_evaluations
		SET status = 'failed', result = $2, completed_at = now()
		WHERE id = $1`, id, payload)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (Repository) GetSummaryText(ctx context.Context, q database.Querier, id uuid.UUID) (lessonID uuid.UUID, summary string, err error) {
	err = q.QueryRow(ctx, `
		SELECT lesson_id, summary_text FROM summary_evaluations WHERE id = $1`, id).Scan(&lessonID, &summary)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, "", ErrNotFound
	}
	return lessonID, summary, err
}
