package summary

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/TSY244/ai-study-tutor/internal/platform/database"
)

var ErrNotFound = errors.New("总结评估不存在")

type Repository struct{}

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
