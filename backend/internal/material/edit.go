package material

import (
	"context"
	"encoding/json"
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

type EditInput struct {
	Version           int64   `json:"version"`
	Title             *string `json:"title,omitempty"`
	Text              *string `json:"text,omitempty"`
	SourceURL         *string `json:"source_url,omitempty"`
	PreferredLanguage *string `json:"preferred_language,omitempty"`
}
type DetailDTO struct {
	DTO
	Text              *string `json:"text"`
	Title             *string `json:"title"`
	PreferredLanguage *string `json:"preferred_language"`
}

func (s *Service) GetMaterial(ctx context.Context, id uuid.UUID) (*DetailDTO, error) {
	m, err := s.repo.GetByID(ctx, s.pool, id)
	if errors.Is(err, ErrNotFound) {
		return nil, contentwrite.NotFound()
	}
	if err != nil {
		return nil, err
	}
	var meta struct {
		Title             *string `json:"title"`
		PreferredLanguage *string `json:"preferred_language"`
	}
	_ = json.Unmarshal(m.Metadata, &meta)
	return &DetailDTO{m.toDTO(), m.InlineText, meta.Title, meta.PreferredLanguage}, nil
}
func (s *Service) EditMaterial(ctx context.Context, rec *idempotency.Record, id uuid.UUID, in EditInput) (*Accepted, error) {
	if in.Version < 1 {
		return nil, contentwrite.Invalid("version", "请提供当前版本。")
	}
	if in.Title == nil && in.Text == nil && in.SourceURL == nil && in.PreferredLanguage == nil {
		return nil, contentwrite.Invalid("body", "至少提供一个修改字段。")
	}
	if in.Title != nil && utf8.RuneCountInString(*in.Title) > 300 {
		return nil, contentwrite.Invalid("title", "标题最多 300 字。")
	}
	if in.Text != nil && (strings.TrimSpace(*in.Text) == "" || utf8.RuneCountInString(*in.Text) > 500000) {
		return nil, contentwrite.Invalid("text", "文本须为 1 至 500000 字。")
	}
	if in.PreferredLanguage != nil && utf8.RuneCountInString(*in.PreferredLanguage) > 30 {
		return nil, contentwrite.Invalid("preferred_language", "语言代码最多 30 字。")
	}
	m, err := s.repo.GetByID(ctx, s.pool, id)
	if errors.Is(err, ErrNotFound) {
		return nil, contentwrite.NotFound()
	}
	if err != nil {
		return nil, err
	}
	if in.Text != nil && m.Kind != KindText {
		return nil, contentwrite.Invalid("text", "仅文本素材可以修改 text。")
	}
	if in.SourceURL != nil && m.Kind != KindURL {
		return nil, contentwrite.Invalid("source_url", "仅链接素材可以修改 source_url。")
	}
	if in.SourceURL != nil {
		normalized, err := s.validateSourceURL(ctx, *in.SourceURL)
		if err != nil {
			return nil, err
		}
		in.SourceURL = &normalized
	}
	var out *Accepted
	err = contentwrite.Run(ctx, s.pool, m.LessonID, func(tx pgx.Tx) error {
		m, err = s.repo.GetByID(ctx, tx, id)
		if errors.Is(err, ErrNotFound) {
			return contentwrite.NotFound()
		}
		if err != nil {
			return err
		}
		if m.Version != in.Version {
			return contentwrite.Conflict()
		}
		metadata := map[string]any{}
		_ = json.Unmarshal(m.Metadata, &metadata)
		if in.Title != nil {
			metadata["title"] = *in.Title
		}
		if in.PreferredLanguage != nil {
			metadata["preferred_language"] = *in.PreferredLanguage
		}
		if in.Text != nil {
			m.InlineText = in.Text
		}
		if in.SourceURL != nil {
			m.SourceURL = in.SourceURL
		}
		m.Metadata = mustJSON(metadata)
		if _, err = tx.Exec(ctx, `UPDATE materials SET inline_text=$2,source_url=$3,metadata=$4,status='pending',version=version+1 WHERE id=$1`, id, m.InlineText, m.SourceURL, m.Metadata); err != nil {
			return err
		}
		m.Version++
		m.Status = StatusPending
		les, err := s.lessons.GetByID(ctx, tx, m.LessonID)
		if err != nil {
			return err
		}
		payload := basePayload(m)
		if m.SourceURL != nil {
			payload["source_url"] = *m.SourceURL
		}
		if m.InlineText != nil {
			payload["text"] = *m.InlineText
		}
		if m.FileObjectID != nil {
			payload["file_object_id"] = m.FileObjectID.String()
		}
		for _, k := range []string{"preferred_language", "transcription_mode"} {
			if v, ok := metadata[k]; ok {
				payload[k] = v
			}
		}
		out, err = s.finishCreation(ctx, tx, idempotency.NewService(s.pool), rec, les, m, payload)
		return err
	})
	return out, err
}
func (h *Handler) getMaterial(c *gin.Context) {
	id, ok := pathUUID(c, "material_id")
	if !ok {
		return
	}
	out, err := h.svc.GetMaterial(c.Request.Context(), id)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	c.JSON(200, out)
}
func (h *Handler) editMaterial(c *gin.Context) {
	id, ok := pathUUID(c, "material_id")
	if !ok {
		return
	}
	var in EditInput
	if !httpx.BindJSON(c, &in) {
		httpx.WriteError(c, httpx.NewProblemError(400, "bad_request", "请求无效", "请求体必须是合法 JSON 对象。"))
		return
	}
	h.idem.Run(c, "editMaterial", httpx.HashRequest(c.Request.Method, c.FullPath(), httpx.CanonicalJSON(in), id.String()), func(ctx context.Context, rec *idempotency.Record) (int, any, error) {
		out, err := h.svc.EditMaterial(ctx, rec, id, in)
		return http.StatusAccepted, out, err
	})
}
