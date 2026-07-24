package document

import (
	"context"
	"errors"
	"net/http"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/TSY244/ai-study-tutor/internal/httpx"
	"github.com/TSY244/ai-study-tutor/internal/idempotency"
	"github.com/TSY244/ai-study-tutor/internal/lesson"
	"github.com/TSY244/ai-study-tutor/internal/platform/httpserver"
)

// SummaryDTO 是文档摘要的 API 表示（契约 DocumentSummary，不含 content_md）。
type SummaryDTO struct {
	ID          uuid.UUID  `json:"id"`
	LessonID    uuid.UUID  `json:"lesson_id"`
	Kind        string     `json:"kind"`
	Revision    int32      `json:"revision"`
	SourceJobID *uuid.UUID `json:"source_job_id"`
	IsActive    bool       `json:"is_active"`
	CreatedAt   string     `json:"created_at"`
}

// DTO 是文档完整版本的 API 表示（契约 Document = Summary + content_md）。
type DTO struct {
	ID          uuid.UUID  `json:"id"`
	LessonID    uuid.UUID  `json:"lesson_id"`
	Kind        string     `json:"kind"`
	Revision    int32      `json:"revision"`
	ContentMD   string     `json:"content_md"`
	SourceJobID *uuid.UUID `json:"source_job_id"`
	IsActive    bool       `json:"is_active"`
	CreatedAt   string     `json:"created_at"`
}

func toSummaryDTO(d *Document) SummaryDTO {
	return SummaryDTO{
		ID:          d.ID,
		LessonID:    d.LessonID,
		Kind:        d.Kind,
		Revision:    d.Revision,
		SourceJobID: d.SourceJobID,
		IsActive:    d.IsActive,
		CreatedAt:   httpx.Time(d.CreatedAt),
	}
}

func toDTO(d *Document) DTO {
	return DTO{
		ID:          d.ID,
		LessonID:    d.LessonID,
		Kind:        d.Kind,
		Revision:    d.Revision,
		ContentMD:   d.ContentMD,
		SourceJobID: d.SourceJobID,
		IsActive:    d.IsActive,
		CreatedAt:   httpx.Time(d.CreatedAt),
	}
}

// summaryCollection 是 listCurrentDocuments 的非分页响应信封（契约
// DocumentSummaryCollection：仅 items，无 page_info）。
type summaryCollection struct {
	Items []SummaryDTO `json:"items"`
}

// Handler 只做 HTTP 绑定与响应映射；业务在 Service。
type Handler struct {
	svc  *Service
	idem *httpx.IdemExecutor
}

func NewHandler(svc *Service, idem *httpx.IdemExecutor) *Handler {
	return &Handler{svc: svc, idem: idem}
}

// RegisterRoutes 注册文档查询（4 个 GET）与笔记重生成（POST）路由。
// jsonLimit 只挂在 POST 上（请求体大小限制由装配层给定）。
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, jsonLimit gin.HandlerFunc) {
	rg.GET("/lessons/:lesson_id/documents", h.listCurrent)
	rg.GET("/lessons/:lesson_id/documents/:kind", h.getCurrent)
	rg.GET("/lessons/:lesson_id/documents/:kind/versions", h.listVersions)
	rg.GET("/documents/:document_id", h.getVersion)
	rg.POST("/lessons/:lesson_id/regenerations", jsonLimit, h.createRegeneration)
}

// GET /api/v1/lessons/:lesson_id/documents —— 当前发布文档集合（非分页）。
func (h *Handler) listCurrent(c *gin.Context) {
	lessonID, ok := pathUUID(c, "lesson_id")
	if !ok {
		return
	}
	docs, err := h.svc.ListCurrent(c.Request.Context(), lessonID)
	if err != nil {
		writeError(c, err)
		return
	}
	items := make([]SummaryDTO, 0, len(docs))
	for i := range docs {
		items = append(items, toSummaryDTO(&docs[i]))
	}
	c.JSON(http.StatusOK, summaryCollection{Items: items})
}

// GET /api/v1/lessons/:lesson_id/documents/:kind —— 指定类型当前版本（含正文）。
func (h *Handler) getCurrent(c *gin.Context) {
	lessonID, ok := pathUUID(c, "lesson_id")
	if !ok {
		return
	}
	kind := c.Param("kind")
	if !ValidKind(kind) {
		notFound(c)
		return
	}
	d, err := h.svc.GetCurrent(c.Request.Context(), lessonID, kind)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, toDTO(d))
}

// GET /api/v1/lessons/:lesson_id/documents/:kind/versions —— 历史版本
// （游标分页，revision DESC；游标 {"revision": n} 取严格小于 n 的页）。
func (h *Handler) listVersions(c *gin.Context) {
	lessonID, ok := pathUUID(c, "lesson_id")
	if !ok {
		return
	}
	kind := c.Param("kind")
	if !ValidKind(kind) {
		notFound(c)
		return
	}
	limit, ok := httpx.ParseLimit(c)
	if !ok {
		badRequest(c, "limit 必须是 1 到 100 的整数。")
		return
	}
	var before *int32
	if raw := c.Query("cursor"); raw != "" {
		var m map[string]any
		if !httpx.DecodeCursor(raw, &m) {
			badRequest(c, "cursor 无法解析。")
			return
		}
		rev, ok := m["revision"].(float64)
		if !ok {
			badRequest(c, "cursor 无法解析。")
			return
		}
		r := int32(rev)
		before = &r
	}
	// 多抓一行判定 has_more。
	docs, err := h.svc.ListVersions(c.Request.Context(), lessonID, kind, before, limit+1)
	if err != nil {
		writeError(c, err)
		return
	}
	hasMore := len(docs) > limit
	if hasMore {
		docs = docs[:limit]
	}
	items := make([]SummaryDTO, 0, len(docs))
	for i := range docs {
		items = append(items, toSummaryDTO(&docs[i]))
	}
	var next *string
	if hasMore && len(docs) > 0 {
		s := httpx.EncodeCursor(map[string]any{"revision": docs[len(docs)-1].Revision})
		next = &s
	}
	c.JSON(http.StatusOK, httpx.Collection[SummaryDTO]{
		Items:    items,
		PageInfo: httpx.PageInfo{NextCursor: next, HasMore: hasMore},
	})
}

// GET /api/v1/documents/:document_id —— 任意版本（含正文）。
func (h *Handler) getVersion(c *gin.Context) {
	id, ok := pathUUID(c, "document_id")
	if !ok {
		return
	}
	d, err := h.svc.GetVersion(c.Request.Context(), id)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, toDTO(d))
}

// regenerationRequest 是 createLessonRegeneration 的请求体（契约
// RegenerationCreate：kinds 必填非空，reason 可选 ≤1000）。
type regenerationRequest struct {
	Kinds  []string `json:"kinds"`
	Reason string   `json:"reason"`
}

// POST /api/v1/lessons/:lesson_id/regenerations —— 创建笔记重生成任务。
func (h *Handler) createRegeneration(c *gin.Context) {
	lessonID, ok := pathUUID(c, "lesson_id")
	if !ok {
		return
	}
	var req regenerationRequest
	if !httpx.BindJSON(c, &req) {
		badRequest(c, "请求体必须是合法 JSON 对象。")
		return
	}
	kinds, errs := validateRegenerationRequest(&req)
	if len(errs) > 0 {
		httpx.WriteValidation(c, errs)
		return
	}
	hash := httpx.HashRequest(c.Request.Method, c.FullPath(), httpx.CanonicalJSON(req), "")
	h.idem.Run(c, "createLessonRegeneration", hash,
		func(ctx context.Context, rec *idempotency.Record) (int, any, error) {
			return h.svc.CreateRegeneration(ctx, rec, lessonID, kinds, req.Reason)
		})
}

// validateRegenerationRequest 校验并去重 kinds（保持首次出现顺序）。
func validateRegenerationRequest(req *regenerationRequest) ([]string, []httpserver.FieldError) {
	var errs []httpserver.FieldError
	if len(req.Kinds) == 0 {
		errs = append(errs, httpx.FieldErr("kinds", "required",
			"kinds 不能为空，至少选择一种文档类型。"))
	}
	seen := make(map[string]bool, len(req.Kinds))
	reported := make(map[string]bool, len(req.Kinds))
	kinds := make([]string, 0, len(req.Kinds))
	for _, k := range req.Kinds {
		if !RegeneratableKind(k) {
			if !reported[k] {
				reported[k] = true
				errs = append(errs, httpx.FieldErr("kinds", "invalid_enum",
					"kinds 只支持 student_notes/teacher_outline。"))
			}
			continue
		}
		if !seen[k] {
			seen[k] = true
			kinds = append(kinds, k)
		}
	}
	if utf8.RuneCountInString(req.Reason) > 1000 {
		errs = append(errs, httpx.FieldErr("reason", "too_long", "reason 不能超过 1000 字符。"))
	}
	if len(errs) > 0 {
		return nil, errs
	}
	return kinds, nil
}

// pathUUID 解析路径参数 UUID；非法值映射 404（路径标识的资源不可能存在）。
func pathUUID(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		notFound(c)
		return uuid.Nil, false
	}
	return id, true
}

// writeError 把 service 错误映射为 Problem；document.ErrNotFound 与
// lesson.ErrNotFound 由本包自行映射（httpx 不能反向依赖这些包），
// 形状与 httpx.WriteError 的 404 一致。
func writeError(c *gin.Context, err error) {
	if errors.Is(err, ErrNotFound) || errors.Is(err, lesson.ErrNotFound) {
		notFound(c)
		return
	}
	httpx.WriteError(c, err)
}

func notFound(c *gin.Context) {
	httpserver.AbortWithProblem(c, http.StatusNotFound, "not_found",
		"资源不存在", "请求的资源不存在或已删除。")
}

func badRequest(c *gin.Context, detail string) {
	httpserver.AbortWithProblem(c, http.StatusBadRequest, "bad_request", "请求无效", detail)
}
