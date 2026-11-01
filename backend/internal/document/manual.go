package document

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/TSY244/ai-study-tutor/backend/internal/contentwrite"
	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
	"github.com/TSY244/ai-study-tutor/backend/internal/idempotency"
)

type ManualInput struct {
	ContentMD        string `json:"content_md"`
	ExpectedRevision int32  `json:"expected_revision"`
}

func (s *Service) WriteManual(ctx context.Context, lessonID uuid.UUID, kind string, in ManualInput, create bool, complete func(pgx.Tx, *Document) error) (*Document, error) {
	if strings.TrimSpace(in.ContentMD) == "" || utf8.RuneCountInString(in.ContentMD) > 200000 {
		return nil, contentwrite.Invalid("content_md", "文档内容须为 1 至 200000 字。")
	}
	if !ValidKind(kind) {
		return nil, contentwrite.NotFound()
	}
	var out *Document
	err := contentwrite.Run(ctx, s.pool, lessonID, func(tx pgx.Tx) error {
		current, err := s.repo.GetActive(ctx, tx, lessonID, kind)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return err
		}
		if create && current != nil {
			return contentwrite.Conflict()
		}
		if !create && (current == nil || current.Revision != in.ExpectedRevision) {
			if current == nil {
				return contentwrite.NotFound()
			}
			return contentwrite.Conflict()
		}
		out = &Document{LessonID: lessonID, Kind: kind, ContentMD: in.ContentMD}
		if err := s.repo.CreateVersion(ctx, tx, out); err != nil {
			return err
		}
		if err := s.repo.Publish(ctx, tx, out.ID); err != nil {
			return err
		}
		out.IsActive = true
		if kind == KindRawText {
			if _, err := tx.Exec(ctx, `UPDATE course_knowledge_points SET active=false,version=version+1 WHERE lesson_id=$1 AND active`, lessonID); err != nil {
				return err
			}
		}
		if complete != nil {
			return complete(tx, out)
		}
		return nil
	})
	return out, err
}
func (s *Service) DeleteManual(ctx context.Context, id uuid.UUID) error {
	d, err := s.repo.GetByID(ctx, s.pool, id)
	if errors.Is(err, ErrNotFound) {
		return contentwrite.NotFound()
	}
	if err != nil {
		return err
	}
	return contentwrite.Run(ctx, s.pool, d.LessonID, func(tx pgx.Tx) error {
		current, err := s.repo.GetByID(ctx, tx, id)
		if errors.Is(err, ErrNotFound) {
			return contentwrite.NotFound()
		}
		if err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `UPDATE documents SET is_active=false,deleted_at=now() WHERE id=$1 AND deleted_at IS NULL`, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return contentwrite.NotFound()
		}
		if current.Kind == KindRawText && current.IsActive {
			_, err = tx.Exec(ctx, `UPDATE course_knowledge_points SET active=false,version=version+1 WHERE lesson_id=$1 AND active`, d.LessonID)
		}
		return err
	})
}
func (h *Handler) manualWrite(c *gin.Context) {
	lessonID, ok := pathUUID(c, "lesson_id")
	if !ok {
		return
	}
	kind := c.Param("kind")
	if !ValidKind(kind) {
		httpx.WriteError(c, contentwrite.NotFound())
		return
	}
	var in ManualInput
	create := c.Request.Method == http.MethodPost
	if create {
		var body struct {
			ContentMD string `json:"content_md"`
		}
		if !httpx.BindJSON(c, &body) {
			badRequest(c, "请求体必须是合法 JSON 对象。")
			return
		}
		in.ContentMD = body.ContentMD
	} else {
		if !httpx.BindJSON(c, &in) {
			badRequest(c, "请求体必须是合法 JSON 对象。")
			return
		}
		if in.ExpectedRevision < 1 {
			httpx.WriteError(c, contentwrite.Invalid("expected_revision", "请提供当前文档版本。"))
			return
		}
	}
	if create {
		hash := httpx.HashRequest(c.Request.Method, c.FullPath(), httpx.CanonicalJSON(in), lessonID.String()+":"+kind)
		h.idem.Run(c, "createManualDocument", hash, func(ctx context.Context, rec *idempotency.Record) (int, any, error) {
			d, err := h.svc.WriteManual(ctx, lessonID, kind, in, true, func(tx pgx.Tx, d *Document) error {
				dto := toDTO(d)
				return httpx.CompleteJSON(ctx, tx, h.idem.Svc, rec.ID, 201, dto, nil, nil, nil, nil)
			})
			if err != nil {
				return 0, nil, err
			}
			return 201, toDTO(d), nil
		})
		return
	}
	d, err := h.svc.WriteManual(c.Request.Context(), lessonID, kind, in, false, nil)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	c.JSON(200, toDTO(d))
}
func (h *Handler) manualDelete(c *gin.Context) {
	id, ok := pathUUID(c, "document_id")
	if !ok {
		return
	}
	if err := h.svc.DeleteManual(c.Request.Context(), id); err != nil {
		httpx.WriteError(c, err)
		return
	}
	c.Status(204)
}
