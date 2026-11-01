// handler.go 是薄弱点与进度端点的 HTTP 层：只做绑定、校验与响应映射。
package mastery

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
	"github.com/TSY244/ai-study-tutor/backend/internal/idempotency"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/httpserver"
)

// Handler 只做 HTTP 绑定与响应映射；业务在 WeaknessService/ProgressService。
type Handler struct {
	weakness *WeaknessService
	progress *ProgressService
	idem     *httpx.IdemExecutor
}

func NewHandler(weakness *WeaknessService, progress *ProgressService, idem *httpx.IdemExecutor) *Handler {
	return &Handler{weakness: weakness, progress: progress, idem: idem}
}

// RegisterRoutes 注册薄弱点与进度端点；resolve/reopen 要求 Idempotency-Key。
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, jsonLimit gin.HandlerFunc) {
	rg.GET("/weaknesses", h.listWeaknesses)
	rg.POST("/courses/:course_id/weaknesses", jsonLimit, h.createWeakness)
	rg.PATCH("/weaknesses/:weakness_id", jsonLimit, h.editWeakness)
	rg.DELETE("/weaknesses/:weakness_id", h.deleteWeakness)
	rg.GET("/weaknesses/:weakness_id", h.getWeakness)
	rg.POST("/weaknesses/:weakness_id/resolve", jsonLimit, h.resolve)
	rg.POST("/weaknesses/:weakness_id/reopen", jsonLimit, h.reopen)
	rg.GET("/courses/:course_id/progress", h.courseProgress)
	rg.GET("/lessons/:lesson_id/progress", h.lessonProgress)
}

// GET /api/v1/weaknesses —— 薄弱点列表（course/lesson/mastered/severity 过滤）。
func (h *Handler) listWeaknesses(c *gin.Context) {
	limit, ok := httpx.ParseLimit(c)
	if !ok {
		badRequest(c, "limit 必须是 1-100 的整数。")
		return
	}
	var f WeaknessFilter
	if raw := c.Query("course_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			badRequest(c, "course_id 必须是 UUID。")
			return
		}
		f.CourseID = &id
	}
	if raw := c.Query("lesson_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			badRequest(c, "lesson_id 必须是 UUID。")
			return
		}
		f.LessonID = &id
	}
	if raw := c.Query("mastered"); raw != "" {
		switch raw {
		case "true":
			v := true
			f.Mastered = &v
		case "false":
			v := false
			f.Mastered = &v
		default:
			badRequest(c, "mastered 必须是布尔值。")
			return
		}
	}
	if raw := c.Query("severity"); raw != "" {
		var n int16
		if _, err := fmt.Sscanf(raw, "%d", &n); err != nil || n < 1 || n > 5 {
			badRequest(c, "severity 必须是 1-5 的整数。")
			return
		}
		f.Severity = &n
	}
	if raw := c.Query("cursor"); raw != "" {
		var cur map[string]any
		if !httpx.DecodeCursor(raw, &cur) {
			badRequest(c, "cursor 无法解析。")
			return
		}
		ts, ok1 := cur["first_seen_at"].(string)
		id, ok2 := cur["id"].(string)
		t, err1 := time.Parse(time.RFC3339, ts)
		u, err2 := uuid.Parse(id)
		if !ok1 || !ok2 || err1 != nil || err2 != nil {
			badRequest(c, "cursor 无法解析。")
			return
		}
		f.AfterFirstSeenAt = &t
		f.AfterID = &u
	}
	rows, err := h.weakness.List(c.Request.Context(), f, limit+1)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	items := make([]WeaknessDTO, 0, limit)
	for _, w := range rows {
		if len(items) == limit {
			break
		}
		w := w
		items = append(items, ToWeaknessDTO(&w))
	}
	page := httpx.PageInfo{HasMore: len(rows) > limit}
	if page.HasMore && len(items) > 0 {
		last := rows[len(items)-1]
		next := httpx.EncodeCursor(map[string]any{
			"first_seen_at": last.FirstSeenAt.UTC().Format(time.RFC3339Nano),
			"id":            last.ID.String(),
		})
		page.NextCursor = &next
	}
	c.JSON(http.StatusOK, httpx.Collection[WeaknessDTO]{Items: items, PageInfo: page})
}

// GET /api/v1/weaknesses/:weakness_id —— 薄弱点详情与证据。
func (h *Handler) getWeakness(c *gin.Context) {
	id, ok := pathUUID(c, "weakness_id")
	if !ok {
		return
	}
	w, err := h.weakness.Get(c.Request.Context(), id)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, ToWeaknessDTO(w))
}

// versionRequest 是 resolve/reopen 请求体（契约 ResourceVersion）。
type versionRequest struct {
	Version int64 `json:"version"`
}

// POST /api/v1/weaknesses/:weakness_id/resolve —— 手动标记已掌握。
func (h *Handler) resolve(c *gin.Context) {
	h.setMastered(c, true, "resolveWeakness")
}

// POST /api/v1/weaknesses/:weakness_id/reopen —— 重新激活。
func (h *Handler) reopen(c *gin.Context) {
	h.setMastered(c, false, "reopenWeakness")
}

func (h *Handler) setMastered(c *gin.Context, mastered bool, scope string) {
	id, ok := pathUUID(c, "weakness_id")
	if !ok {
		return
	}
	var req versionRequest
	if !httpx.BindJSON(c, &req) {
		badRequest(c, "请求体必须是合法 JSON 对象。")
		return
	}
	if req.Version < 1 {
		httpx.WriteValidation(c, []httpserver.FieldError{
			httpx.FieldErr("version", "out_of_range", "version 必须大于等于 1。")})
		return
	}
	hash := httpx.HashRequest(c.Request.Method, c.FullPath(), httpx.CanonicalJSON(req), "")
	h.idem.Run(c, scope, hash,
		func(ctx context.Context, rec *idempotency.Record) (int, any, error) {
			return h.weakness.SetMastered(ctx, rec, id, req.Version, mastered)
		})
}

// GET /api/v1/courses/:course_id/progress —— 课程进度聚合。
func (h *Handler) courseProgress(c *gin.Context) {
	id, ok := pathUUID(c, "course_id")
	if !ok {
		return
	}
	dto, err := h.progress.GetProgress(c.Request.Context(), "course", id)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto)
}

// GET /api/v1/lessons/:lesson_id/progress —— 课时掌握度与趋势。
func (h *Handler) lessonProgress(c *gin.Context) {
	id, ok := pathUUID(c, "lesson_id")
	if !ok {
		return
	}
	dto, err := h.progress.GetProgress(c.Request.Context(), "lesson", id)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto)
}

// pathUUID 解析路径参数 UUID；非法值映射 404。
func pathUUID(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		httpserver.AbortWithProblem(c, http.StatusNotFound, "not_found",
			"资源不存在", "请求的资源不存在或已删除。")
		return uuid.Nil, false
	}
	return id, true
}

func badRequest(c *gin.Context, detail string) {
	httpserver.AbortWithProblem(c, http.StatusBadRequest, "bad_request", "请求无效", detail)
}
