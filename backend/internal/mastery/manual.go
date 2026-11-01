package mastery

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/TSY244/ai-study-tutor/backend/internal/contentwrite"
	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
	"github.com/TSY244/ai-study-tutor/backend/internal/idempotency"
)

type WeaknessCreate struct {
	LessonID     *uuid.UUID `json:"lesson_id"`
	ConceptKey   string     `json:"concept_key"`
	ConceptLabel string     `json:"concept_label"`
	Description  string     `json:"description"`
	Severity     int16      `json:"severity"`
}
type WeaknessEdit struct {
	Version      int64   `json:"version"`
	ConceptKey   *string `json:"concept_key,omitempty"`
	ConceptLabel *string `json:"concept_label,omitempty"`
	Description  *string `json:"description,omitempty"`
	Severity     *int16  `json:"severity,omitempty"`
	Mastered     *bool   `json:"mastered,omitempty"`
}

func validateWeakness(w *Weakness) error {
	if strings.TrimSpace(w.ConceptKey) == "" || utf8.RuneCountInString(w.ConceptKey) > 300 {
		return contentwrite.Invalid("concept_key", "概念标识须为 1 至 300 字。")
	}
	if strings.TrimSpace(w.ConceptLabel) == "" || utf8.RuneCountInString(w.ConceptLabel) > 300 {
		return contentwrite.Invalid("concept_label", "概念名称须为 1 至 300 字。")
	}
	if w.Severity < 1 || w.Severity > 5 {
		return contentwrite.Invalid("severity", "严重度须为 1 至 5。")
	}
	if utf8.RuneCountInString(w.Description) > 10000 {
		return contentwrite.Invalid("description", "描述最多 10000 字。")
	}
	return nil
}
func weaknessWriteErr(err error) error {
	var e *pgconn.PgError
	if errors.As(err, &e) && e.Code == "23505" {
		return contentwrite.Invalid("concept_key", "该范围内已有同名薄弱点。")
	}
	return err
}
func (s *WeaknessService) Create(ctx context.Context, courseID uuid.UUID, in WeaknessCreate, complete func(pgx.Tx, *WeaknessDTO) error) (*WeaknessDTO, error) {
	key := in.ConceptKey
	if key == "" {
		key = NormalizeConceptKey(in.ConceptLabel)
	}
	w := &Weakness{ID: uuid.New(), CourseID: courseID, LessonID: in.LessonID, ConceptKey: key, ConceptLabel: in.ConceptLabel, Description: in.Description, Severity: in.Severity}
	if err := validateWeakness(w); err != nil {
		return nil, err
	}
	var out *WeaknessDTO
	err := contentwrite.RunCourse(ctx, s.pool, courseID, in.LessonID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO weaknesses(id,course_id,lesson_id,concept_key,concept_label,description,severity) VALUES($1,$2,$3,$4,$5,$6,$7)`, w.ID, w.CourseID, w.LessonID, w.ConceptKey, w.ConceptLabel, w.Description, w.Severity)
		if err != nil {
			return err
		}
		if err = NewApplyService(s.scores).Refresh(ctx, tx, w.CourseID, w.LessonID); err != nil {
			return err
		}
		saved, err := s.repo.GetWeaknessByID(ctx, tx, w.ID)
		if err != nil {
			return err
		}
		dto := ToWeaknessDTO(saved)
		out = &dto
		if complete != nil {
			return complete(tx, out)
		}
		return nil
	})
	return out, weaknessWriteErr(err)
}
func (s *WeaknessService) Edit(ctx context.Context, id uuid.UUID, in WeaknessEdit) (*WeaknessDTO, error) {
	w, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	var out *WeaknessDTO
	err = contentwrite.RunCourse(ctx, s.pool, w.CourseID, w.LessonID, func(tx pgx.Tx) error {
		current, err := s.repo.GetWeaknessByID(ctx, tx, id)
		if errors.Is(err, ErrNotFound) {
			return contentwrite.NotFound()
		}
		if err != nil {
			return err
		}
		w = current
		if in.ConceptKey != nil {
			w.ConceptKey = *in.ConceptKey
		}
		if in.ConceptLabel != nil {
			w.ConceptLabel = *in.ConceptLabel
		}
		if in.Description != nil {
			w.Description = *in.Description
		}
		if in.Severity != nil {
			w.Severity = *in.Severity
		}
		if in.Mastered != nil {
			w.Mastered = *in.Mastered
		}
		if err := validateWeakness(w); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `UPDATE weaknesses SET concept_key=$3,concept_label=$4,description=$5,severity=$6,mastered=$7,version=version+1,last_seen_at=now() WHERE id=$1 AND version=$2`, id, in.Version, w.ConceptKey, w.ConceptLabel, w.Description, w.Severity, w.Mastered)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return contentwrite.Conflict()
		}
		if err = NewApplyService(s.scores).Refresh(ctx, tx, w.CourseID, w.LessonID); err != nil {
			return err
		}
		saved, err := s.repo.GetWeaknessByID(ctx, tx, id)
		if err != nil {
			return err
		}
		dto := ToWeaknessDTO(saved)
		out = &dto
		return nil
	})
	return out, weaknessWriteErr(err)
}
func (s *WeaknessService) Delete(ctx context.Context, id uuid.UUID) error {
	w, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	return contentwrite.RunCourse(ctx, s.pool, w.CourseID, w.LessonID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM weaknesses WHERE id=$1`, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return contentwrite.NotFound()
		}
		return NewApplyService(s.scores).Refresh(ctx, tx, w.CourseID, w.LessonID)
	})
}
func (h *Handler) createWeakness(c *gin.Context) {
	id, ok := pathUUID(c, "course_id")
	if !ok {
		return
	}
	var in WeaknessCreate
	if !httpx.BindJSON(c, &in) {
		badRequest(c, "请求体必须是合法 JSON 对象。")
		return
	}
	h.idem.Run(c, "createWeakness", httpx.HashRequest(c.Request.Method, c.FullPath(), httpx.CanonicalJSON(in), id.String()), func(ctx context.Context, rec *idempotency.Record) (int, any, error) {
		out, err := h.weakness.Create(ctx, id, in, func(tx pgx.Tx, w *WeaknessDTO) error {
			return httpx.CompleteJSON(ctx, tx, h.idem.Svc, rec.ID, 201, w, nil, nil, nil, nil)
		})
		return 201, out, err
	})
}
func (h *Handler) editWeakness(c *gin.Context) {
	id, ok := pathUUID(c, "weakness_id")
	if !ok {
		return
	}
	var in WeaknessEdit
	if !httpx.BindJSON(c, &in) {
		badRequest(c, "请求体必须是合法 JSON 对象。")
		return
	}
	if in.Version < 1 {
		httpx.WriteError(c, contentwrite.Invalid("version", "请提供当前版本。"))
		return
	}
	out, err := h.weakness.Edit(c.Request.Context(), id, in)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	c.JSON(200, out)
}
func (h *Handler) deleteWeakness(c *gin.Context) {
	id, ok := pathUUID(c, "weakness_id")
	if !ok {
		return
	}
	if err := h.weakness.Delete(c.Request.Context(), id); err != nil {
		httpx.WriteError(c, err)
		return
	}
	c.Status(204)
}
