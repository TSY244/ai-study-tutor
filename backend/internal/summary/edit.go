package summary

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/TSY244/ai-study-tutor/backend/internal/contentwrite"
	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
	"github.com/TSY244/ai-study-tutor/backend/internal/idempotency"
)

func (s *Service) EditEvaluation(ctx context.Context, rec *idempotency.Record, id uuid.UUID, text string) (*Accepted, error) {
	if strings.TrimSpace(text) == "" || utf8.RuneCountInString(text) > 50000 {
		return nil, contentwrite.Invalid("summary", "总结须为 1 至 50000 字。")
	}
	old, err := s.repo.GetByID(ctx, s.pool, id)
	if errors.Is(err, ErrNotFound) {
		return nil, contentwrite.NotFound()
	}
	if err != nil {
		return nil, err
	}
	var out *Accepted
	err = contentwrite.Run(ctx, s.pool, old.LessonID, func(tx pgx.Tx) error {
		if _, err := s.repo.GetByID(ctx, tx, id); err != nil {
			if errors.Is(err, ErrNotFound) {
				return contentwrite.NotFound()
			}
			return err
		}
		var err error
		out, err = s.createInTx(ctx, tx, rec, old.LessonID, text)
		return err
	})
	return out, err
}
func (s *Service) DeleteEvaluation(ctx context.Context, id uuid.UUID) error {
	e, err := s.repo.GetByID(ctx, s.pool, id)
	if errors.Is(err, ErrNotFound) {
		return contentwrite.NotFound()
	}
	if err != nil {
		return err
	}
	return contentwrite.Run(ctx, s.pool, e.LessonID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM summary_evaluations WHERE id=$1`, id)
		if err == nil && tag.RowsAffected() == 0 {
			return contentwrite.NotFound()
		}
		return err
	})
}
func (s *Service) ListEvaluations(ctx context.Context, lessonID uuid.UUID, before *time.Time, beforeID *uuid.UUID, limit int) ([]Evaluation, error) {
	if _, err := s.lessons.GetByID(ctx, s.pool, lessonID); err != nil {
		return nil, contentwrite.NotFound()
	}
	return s.repo.List(ctx, s.pool, lessonID, before, beforeID, limit)
}
func (h *Handler) editEvaluation(c *gin.Context) {
	id, ok := pathUUID(c, "evaluation_id")
	if !ok {
		return
	}
	var in createEvaluationRequest
	if !httpx.BindJSON(c, &in) {
		badRequest(c, "请求体必须是合法 JSON 对象。")
		return
	}
	h.idem.Run(c, "editSummaryEvaluation", httpx.HashRequest(c.Request.Method, c.FullPath(), httpx.CanonicalJSON(in), id.String()), func(ctx context.Context, rec *idempotency.Record) (int, any, error) {
		out, err := h.svc.EditEvaluation(ctx, rec, id, in.Summary)
		return 202, out, err
	})
}
func (h *Handler) deleteEvaluation(c *gin.Context) {
	id, ok := pathUUID(c, "evaluation_id")
	if !ok {
		return
	}
	if err := h.svc.DeleteEvaluation(c.Request.Context(), id); err != nil {
		httpx.WriteError(c, err)
		return
	}
	c.Status(204)
}
func (h *Handler) listEvaluations(c *gin.Context) {
	id, ok := pathUUID(c, "lesson_id")
	if !ok {
		return
	}
	limit, ok := httpx.ParseLimit(c)
	if !ok {
		badRequest(c, "limit 必须是 1 至 100 的整数。")
		return
	}
	var before *time.Time
	var beforeID *uuid.UUID
	if raw := c.Query("cursor"); raw != "" {
		var cur map[string]any
		if !httpx.DecodeCursor(raw, &cur) {
			badRequest(c, "cursor 无法解析。")
			return
		}
		ts, ok1 := cur["created_at"].(string)
		ident, ok2 := cur["id"].(string)
		tm, e1 := time.Parse(time.RFC3339Nano, ts)
		u, e2 := uuid.Parse(ident)
		if !ok1 || !ok2 || e1 != nil || e2 != nil {
			badRequest(c, "cursor 无法解析。")
			return
		}
		before = &tm
		beforeID = &u
	}
	rows, err := h.svc.ListEvaluations(c.Request.Context(), id, before, beforeID, limit+1)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	page := httpx.PageInfo{HasMore: len(rows) > limit}
	if page.HasMore {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		cur := httpx.EncodeCursor(map[string]any{"created_at": last.CreatedAt.UTC().Format(time.RFC3339Nano), "id": last.ID.String()})
		page.NextCursor = &cur
	}
	items := make([]DTO, 0, len(rows))
	for i := range rows {
		items = append(items, ToDTO(&rows[i]))
	}
	c.JSON(200, httpx.Collection[DTO]{Items: items, PageInfo: page})
}
