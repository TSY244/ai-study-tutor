package lesson

import (
	"context"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
	"github.com/TSY244/ai-study-tutor/backend/internal/idempotency"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/database"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/httpserver"
)

// Handler 绑定 Lesson REST 端点（04 §6）：只做 HTTP 绑定与响应映射，
// 业务编排在 Service。
type Handler struct {
	svc  *Service
	idem *httpx.IdemExecutor
}

// NewHandler 构造课时 Handler；idem 为 POST 创建端点的幂等执行器。
func NewHandler(svc *Service, idem *httpx.IdemExecutor) *Handler {
	return &Handler{svc: svc, idem: idem}
}

// RegisterRoutes 注册 GET/POST /courses/{course_id}/lessons 与
// GET/PATCH/DELETE /lessons/{lesson_id}；写端点挂 jsonLimit。
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, jsonLimit gin.HandlerFunc) {
	rg.GET("/courses/:course_id/lessons", h.list)
	rg.POST("/courses/:course_id/lessons", jsonLimit, h.create)
	rg.GET("/lessons/:lesson_id", h.get)
	rg.PATCH("/lessons/:lesson_id", jsonLimit, h.update)
	rg.DELETE("/lessons/:lesson_id", h.delete)
}

// createRequest 对齐 LessonCreate Schema（additionalProperties:false）。
type createRequest struct {
	Ordinal     *int32  `json:"ordinal"`
	Title       *string `json:"title"`
	Description *string `json:"description"`
}

// updateRequest 对齐 LessonUpdate Schema（additionalProperties:false）。
type updateRequest struct {
	Ordinal     *int32  `json:"ordinal"`
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Version     *int64  `json:"version"`
}

func (h *Handler) create(c *gin.Context) {
	courseID, ok := courseUUID(c)
	if !ok {
		return
	}
	var req createRequest
	if !httpx.BindJSON(c, &req) {
		badRequest(c, "请求体不是有效的 JSON。")
		return
	}
	var errs []httpserver.FieldError
	errs = append(errs, ordinalFieldErrs(req.Ordinal, true)...)
	errs = append(errs, titleFieldErrs(req.Title, true)...)
	errs = append(errs, descriptionFieldErrs(req.Description)...)
	if len(errs) > 0 {
		httpx.WriteValidation(c, errs)
		return
	}
	hash := httpx.HashRequest(c.Request.Method, c.FullPath(), httpx.CanonicalJSON(req), "")
	h.idem.Run(c, "createLesson", hash, func(ctx context.Context, rec *idempotency.Record) (int, any, error) {
		dto, err := h.svc.Create(ctx, courseID, CreateInput{
			Ordinal: *req.Ordinal, Title: *req.Title, Description: req.Description,
		}, func(ctx context.Context, tx database.Querier, dto LessonDTO) error {
			resType := "lesson"
			resID, _ := uuid.Parse(dto.ID)
			return httpx.CompleteJSON(ctx, tx, h.idem.Svc, rec.ID,
				http.StatusCreated, dto,
				map[string]string{"Location": "/api/v1/lessons/" + dto.ID}, &resType, &resID, nil)
		})
		if err != nil {
			return 0, nil, err
		}
		c.Header("Location", "/api/v1/lessons/"+dto.ID)
		return http.StatusCreated, dto, nil
	})
}

func (h *Handler) list(c *gin.Context) {
	courseID, ok := courseUUID(c)
	if !ok {
		return
	}
	limit, ok := httpx.ParseLimit(c)
	if !ok {
		badRequest(c, "limit 必须是 1-100 的整数。")
		return
	}
	var afterOrdinal *int32
	var afterID *uuid.UUID
	if raw := c.Query("cursor"); raw != "" {
		var m map[string]any
		if !httpx.DecodeCursor(raw, &m) {
			badRequest(c, "cursor 无效。")
			return
		}
		ord, id, ok := parseLessonCursor(m)
		if !ok {
			badRequest(c, "cursor 无效。")
			return
		}
		afterOrdinal, afterID = &ord, &id
	}
	items, hasMore, err := h.svc.List(c.Request.Context(), courseID, afterOrdinal, afterID, limit)
	if err != nil {
		writeError(c, err)
		return
	}
	var next *string
	if hasMore && len(items) > 0 {
		last := items[len(items)-1]
		s := httpx.EncodeCursor(map[string]any{"ordinal": last.Ordinal, "id": last.ID})
		next = &s
	}
	c.JSON(http.StatusOK, httpx.Collection[LessonDTO]{
		Items:    items,
		PageInfo: httpx.PageInfo{NextCursor: next, HasMore: hasMore},
	})
}

func (h *Handler) get(c *gin.Context) {
	id, ok := lessonUUID(c)
	if !ok {
		return
	}
	dto, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto)
}

func (h *Handler) update(c *gin.Context) {
	id, ok := lessonUUID(c)
	if !ok {
		return
	}
	var req updateRequest
	if !httpx.BindJSON(c, &req) {
		badRequest(c, "请求体不是有效的 JSON。")
		return
	}
	var errs []httpserver.FieldError
	if req.Version == nil {
		errs = append(errs, httpx.FieldErr("version", "required", "缺少资源版本号"))
	} else if *req.Version < 1 {
		errs = append(errs, httpx.FieldErr("version", "too_small", "版本号必须大于等于 1"))
	}
	errs = append(errs, ordinalFieldErrs(req.Ordinal, false)...)
	errs = append(errs, titleFieldErrs(req.Title, false)...)
	errs = append(errs, descriptionFieldErrs(req.Description)...)
	if len(errs) > 0 {
		httpx.WriteValidation(c, errs)
		return
	}
	dto, err := h.svc.Update(c.Request.Context(), id, UpdateInput{
		Ordinal: req.Ordinal, Title: req.Title,
		Description: req.Description, Version: *req.Version,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto)
}

func (h *Handler) delete(c *gin.Context) {
	id, ok := lessonUUID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// writeError 统一委托 httpx.WriteError：service 层已把模块哨兵错误
// （404/409/422 ordinal 冲突）转换为 ProblemError，与 httpx 映射形状一致。
func writeError(c *gin.Context, err error) {
	httpx.WriteError(c, err)
}

func badRequest(c *gin.Context, detail string) {
	httpserver.AbortWithProblem(c, http.StatusBadRequest, "bad_request", "请求无效", detail)
}

// courseUUID 解析 course_id 路径参数；非法值映射 400（与 course 模块一致）。
func courseUUID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("course_id"))
	if err != nil {
		badRequest(c, "路径参数 course_id 必须是 UUID。")
		return uuid.Nil, false
	}
	return id, true
}

// lessonUUID 解析 lesson_id 路径参数；非法值映射 404（路径标识的资源
// 不可能存在，与 material/quiz 模块一致）。
func lessonUUID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("lesson_id"))
	if err != nil {
		httpserver.AbortWithProblem(c, http.StatusNotFound, "not_found",
			"资源不存在", "请求的资源不存在或已删除。")
		return uuid.Nil, false
	}
	return id, true
}

// parseLessonCursor 提取 ordinal/id 排序键；缺键或类型不符返回 false。
func parseLessonCursor(m map[string]any) (int32, uuid.UUID, bool) {
	ord, ok := m["ordinal"].(float64)
	if !ok {
		return 0, uuid.Nil, false
	}
	ids, ok := m["id"].(string)
	if !ok {
		return 0, uuid.Nil, false
	}
	id, err := uuid.Parse(ids)
	if err != nil {
		return 0, uuid.Nil, false
	}
	return int32(ord), id, true
}

// ordinalFieldErrs 校验序号（required 表示缺失也算错误）。
func ordinalFieldErrs(v *int32, required bool) []httpserver.FieldError {
	if v == nil {
		if required {
			return []httpserver.FieldError{httpx.FieldErr("ordinal", "required", "缺少课时序号")}
		}
		return nil
	}
	if *v < 1 {
		return []httpserver.FieldError{httpx.FieldErr("ordinal", "too_small", "课时序号必须大于等于 1")}
	}
	return nil
}

// titleFieldErrs 校验标题（required 表示缺失也算错误）。
func titleFieldErrs(v *string, required bool) []httpserver.FieldError {
	if v == nil {
		if required {
			return []httpserver.FieldError{httpx.FieldErr("title", "required", "课时标题不能为空")}
		}
		return nil
	}
	switch {
	case strings.TrimSpace(*v) == "":
		return []httpserver.FieldError{httpx.FieldErr("title", "too_short", "课时标题不能为空")}
	case utf8.RuneCountInString(*v) > 200:
		return []httpserver.FieldError{httpx.FieldErr("title", "too_long", "课时标题不能超过 200 字符")}
	}
	return nil
}

// descriptionFieldErrs 校验描述长度。
func descriptionFieldErrs(v *string) []httpserver.FieldError {
	if v != nil && utf8.RuneCountInString(*v) > 5000 {
		return []httpserver.FieldError{httpx.FieldErr("description", "too_long", "课时描述不能超过 5000 字符")}
	}
	return nil
}
