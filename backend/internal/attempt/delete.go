package attempt

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/TSY244/ai-study-tutor/backend/internal/contentwrite"
	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
	"github.com/TSY244/ai-study-tutor/backend/internal/mastery"
)

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	var lessonID, courseID uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT z.lesson_id,l.course_id FROM attempts a JOIN quizzes z ON z.id=a.quiz_id JOIN lessons l ON l.id=z.lesson_id WHERE a.id=$1`, id).Scan(&lessonID, &courseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return contentwrite.NotFound()
	}
	if err != nil {
		return err
	}
	return contentwrite.Run(ctx, s.pool, lessonID, func(tx pgx.Tx) error {
		if _, err := s.repo.LockForUpdate(ctx, tx, id); err != nil {
			if errors.Is(err, ErrNotFound) {
				return contentwrite.NotFound()
			}
			return err
		}
		// A submit may have created a grading job while we were waiting on the attempt.
		if err := contentwrite.Lesson(ctx, tx, lessonID); err != nil {
			return err
		}
		for _, query := range []string{
			`UPDATE weaknesses w SET evidence=(SELECT COALESCE(jsonb_agg(e),'[]'::jsonb) FROM jsonb_array_elements(w.evidence) e WHERE e->>'attempt_id' IS DISTINCT FROM $1::uuid::text),version=version+1 WHERE w.evidence @> jsonb_build_array(jsonb_build_object('attempt_id',$1::uuid::text))`,
			`DELETE FROM mastery_snapshots WHERE source_attempt_id=$1`,
			`DELETE FROM attempt_answers WHERE attempt_id=$1`,
			`DELETE FROM attempts WHERE id=$1`,
		} {
			if _, err := tx.Exec(ctx, query, id); err != nil {
				return err
			}
		}
		return mastery.NewApplyService(Repository{}).Refresh(ctx, tx, courseID, &lessonID)
	})
}
func (h *Handler) deleteAttempt(c *gin.Context) {
	id, ok := pathUUID(c, "attempt_id")
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		httpx.WriteError(c, err)
		return
	}
	c.Status(204)
}
