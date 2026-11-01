package course

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

type KnowledgeEdit struct {
	Version           int64   `json:"version"`
	Label             *string `json:"label,omitempty"`
	Explanation       *string `json:"explanation,omitempty"`
	LearningObjective *string `json:"learning_objective,omitempty"`
	SourceExcerpt     *string `json:"source_excerpt,omitempty"`
	Importance        *string `json:"importance,omitempty"`
}

func (s *Service) GetKnowledge(ctx context.Context, id uuid.UUID) (*KnowledgePoint, error) {
	var p KnowledgePoint
	err := s.pool.QueryRow(ctx, `SELECT k.id,k.course_id,k.lesson_id,l.title,k.label,k.explanation,k.learning_objective,k.source_excerpt,k.importance,k.updated_at,k.version FROM course_knowledge_points k JOIN lessons l ON l.id=k.lesson_id WHERE k.id=$1 AND k.active`, id).Scan(&p.ID, &p.CourseID, &p.LessonID, &p.LessonTitle, &p.Label, &p.Explanation, &p.LearningObjective, &p.SourceExcerpt, &p.Importance, &p.UpdatedAt, &p.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, contentwrite.NotFound()
	}
	return &p, err
}
func validateManualKnowledge(ctx context.Context, tx pgx.Tx, lessonID uuid.UUID, p KnowledgeDraft) error {
	for _, f := range []struct {
		name, value string
		max         int
	}{{"label", p.Label, 120}, {"explanation", p.Explanation, 2000}, {"learning_objective", p.LearningObjective, 1000}, {"source_excerpt", p.SourceExcerpt, 1000}} {
		if strings.TrimSpace(f.value) == "" || utf8.RuneCountInString(f.value) > f.max {
			return contentwrite.Invalid(f.name, "字段不能为空且不能超过长度限制。")
		}
	}
	if p.Importance != "core" && p.Importance != "supporting" {
		return contentwrite.Invalid("importance", "必须为 core 或 supporting。")
	}
	var source string
	err := tx.QueryRow(ctx, `SELECT content_md FROM documents WHERE lesson_id=$1 AND kind='raw_text' AND is_active`, lessonID).Scan(&source)
	if errors.Is(err, pgx.ErrNoRows) {
		return contentwrite.Invalid("source_excerpt", "课时需要先有原文，才能核对知识点依据。")
	}
	if err != nil {
		return err
	}
	if !strings.Contains(strings.Join(strings.Fields(source), " "), strings.Join(strings.Fields(p.SourceExcerpt), " ")) {
		return contentwrite.Invalid("source_excerpt", "原文依据必须是当前课时原文中的连续引用。")
	}
	return nil
}
func knowledgeErr(err error) error {
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe.Code == "23505" {
		return contentwrite.Invalid("label", "该课时已有同名知识点。")
	}
	return err
}
func (s *Service) CreateKnowledge(ctx context.Context, lessonID uuid.UUID, p KnowledgeDraft, complete func(pgx.Tx, *KnowledgePoint) error) (*KnowledgePoint, error) {
	var out *KnowledgePoint
	err := contentwrite.Run(ctx, s.pool, lessonID, func(tx pgx.Tx) error {
		if err := validateManualKnowledge(ctx, tx, lessonID, p); err != nil {
			return err
		}
		out = &KnowledgePoint{KnowledgeDraft: p, ID: uuid.New(), LessonID: lessonID}
		err := tx.QueryRow(ctx, `INSERT INTO course_knowledge_points(id,course_id,lesson_id,concept_key,label,explanation,learning_objective,source_excerpt,importance) SELECT $1,course_id,id,$3,$4,$5,$6,$7,$8 FROM lessons WHERE id=$2 ON CONFLICT(lesson_id,concept_key) DO UPDATE SET label=excluded.label,explanation=excluded.explanation,learning_objective=excluded.learning_objective,source_excerpt=excluded.source_excerpt,importance=excluded.importance,source_job_id=NULL,active=true,version=course_knowledge_points.version+1,updated_at=now() WHERE NOT course_knowledge_points.active RETURNING id,course_id,updated_at,version`, out.ID, lessonID, strings.ToLower(strings.TrimSpace(p.Label)), p.Label, p.Explanation, p.LearningObjective, p.SourceExcerpt, p.Importance).Scan(&out.ID, &out.CourseID, &out.UpdatedAt, &out.Version)
		if errors.Is(err, pgx.ErrNoRows) {
			return contentwrite.Invalid("label", "该课时已有同名知识点。")
		}
		if err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT title FROM lessons WHERE id=$1`, lessonID).Scan(&out.LessonTitle); err != nil {
			return err
		}
		if complete != nil {
			return complete(tx, out)
		}
		return nil
	})
	return out, knowledgeErr(err)
}
func (s *Service) EditKnowledge(ctx context.Context, id uuid.UUID, in KnowledgeEdit) (*KnowledgePoint, error) {
	p, err := s.GetKnowledge(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.Version != in.Version {
		return nil, contentwrite.Conflict()
	}
	err = contentwrite.Run(ctx, s.pool, p.LessonID, func(tx pgx.Tx) error {
		draft := p.KnowledgeDraft
		if in.Label != nil {
			draft.Label = *in.Label
		}
		if in.Explanation != nil {
			draft.Explanation = *in.Explanation
		}
		if in.LearningObjective != nil {
			draft.LearningObjective = *in.LearningObjective
		}
		if in.SourceExcerpt != nil {
			draft.SourceExcerpt = *in.SourceExcerpt
		}
		if in.Importance != nil {
			draft.Importance = *in.Importance
		}
		if err := validateManualKnowledge(ctx, tx, p.LessonID, draft); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `UPDATE course_knowledge_points SET concept_key=$3,label=$4,explanation=$5,learning_objective=$6,source_excerpt=$7,importance=$8,source_job_id=NULL,version=version+1,updated_at=now() WHERE id=$1 AND version=$2 AND active`, id, in.Version, strings.ToLower(strings.TrimSpace(draft.Label)), draft.Label, draft.Explanation, draft.LearningObjective, draft.SourceExcerpt, draft.Importance)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return contentwrite.Conflict()
		}
		return nil
	})
	if err != nil {
		return nil, knowledgeErr(err)
	}
	return s.GetKnowledge(ctx, id)
}
func (s *Service) DeleteKnowledge(ctx context.Context, id uuid.UUID) error {
	p, err := s.GetKnowledge(ctx, id)
	if err != nil {
		return err
	}
	return contentwrite.Run(ctx, s.pool, p.LessonID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE course_knowledge_points SET active=false,version=version+1,updated_at=now() WHERE id=$1 AND active`, id)
		if err == nil && tag.RowsAffected() == 0 {
			return contentwrite.NotFound()
		}
		return err
	})
}
func knowledgeID(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		httpx.WriteError(c, contentwrite.NotFound())
		return id, false
	}
	return id, true
}
func (h *Handler) createKnowledge(c *gin.Context) {
	id, ok := knowledgeID(c, "lesson_id")
	if !ok {
		return
	}
	var in KnowledgeDraft
	if !httpx.BindJSON(c, &in) {
		badRequest(c, "请求体必须是合法 JSON 对象。")
		return
	}
	h.idem.Run(c, "createKnowledgePoint", httpx.HashRequest(c.Request.Method, c.FullPath(), httpx.CanonicalJSON(in), id.String()), func(ctx context.Context, rec *idempotency.Record) (int, any, error) {
		out, err := h.svc.CreateKnowledge(ctx, id, in, func(tx pgx.Tx, p *KnowledgePoint) error {
			return httpx.CompleteJSON(ctx, tx, h.idem.Svc, rec.ID, 201, p, nil, nil, nil, nil)
		})
		return 201, out, err
	})
}
func (h *Handler) getKnowledge(c *gin.Context) {
	id, ok := knowledgeID(c, "knowledge_point_id")
	if !ok {
		return
	}
	p, err := h.svc.GetKnowledge(c.Request.Context(), id)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	c.JSON(200, p)
}
func (h *Handler) editKnowledge(c *gin.Context) {
	id, ok := knowledgeID(c, "knowledge_point_id")
	if !ok {
		return
	}
	var in KnowledgeEdit
	if !httpx.BindJSON(c, &in) {
		badRequest(c, "请求体必须是合法 JSON 对象。")
		return
	}
	if in.Version < 1 {
		httpx.WriteError(c, contentwrite.Invalid("version", "请提供当前版本。"))
		return
	}
	p, err := h.svc.EditKnowledge(c.Request.Context(), id, in)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	c.JSON(200, p)
}
func (h *Handler) deleteKnowledge(c *gin.Context) {
	id, ok := knowledgeID(c, "knowledge_point_id")
	if !ok {
		return
	}
	if err := h.svc.DeleteKnowledge(c.Request.Context(), id); err != nil {
		httpx.WriteError(c, err)
		return
	}
	c.Status(204)
}
