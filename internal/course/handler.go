package course

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/TSY244/ai-study-tutor/internal/httpx"
	"github.com/TSY244/ai-study-tutor/internal/idempotency"
	"github.com/TSY244/ai-study-tutor/internal/platform/database"
	"github.com/TSY244/ai-study-tutor/internal/platform/httpserver"
)

// Handler 绑定 Course REST 端点（04 §5）：只做 HTTP 绑定与响应映射，
// 业务编排在 Service。
type Handler struct {
	svc  *Service
	idem *httpx.IdemExecutor
}

// NewHandler 构造课程 Handler；idem 为 POST 创建端点的幂等执行器。
func NewHandler(svc *Service, idem *httpx.IdemExecutor) *Handler {
	return &Handler{svc: svc, idem: idem}
}

// RegisterRoutes 注册 GET/POST /courses 与 GET/PATCH/DELETE
// /courses/{course_id}；全部挂载 jsonLimit body 限制中间件。
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, jsonLimit gin.HandlerFunc) {
	g := rg.Group("/courses", jsonLimit)
	g.GET("", h.list)
	g.POST("", h.create)
	g.GET("/:course_id", h.get)
	g.PATCH("/:course_id", h.update)
	g.DELETE("/:course_id", h.delete)
}

// createRequest 对齐 CourseCreate Schema（additionalProperties:false）。
type createRequest struct {
	Title       string  `json:"title"`
	Description *string `json:"description"`
}

// updateRequest 对齐 CourseUpdate Schema（additionalProperties:false）。
type updateRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Archived    *bool   `json:"archived"`
	Version     *int64  `json:"version"`
}

func (h *Handler) create(c *gin.Context) {
	var req createRequest
	if !httpx.BindJSON(c, &req) {
		badRequest(c, "请求体不是有效的 JSON。")
		return
	}
	var errs []httpserver.FieldError
	errs = append(errs, titleFieldErrs(&req.Title, true)...)
	errs = append(errs, descriptionFieldErrs(req.Description)...)
	if len(errs) > 0 {
		httpx.WriteValidation(c, errs)
		return
	}
	hash := httpx.HashRequest(c.Request.Method, c.FullPath(), httpx.CanonicalJSON(req), "")
	h.idem.Run(c, "createCourse", hash, func(ctx context.Context, rec *idempotency.Record) (int, any, error) {
		dto, err := h.svc.Create(ctx, CreateInput{Title: req.Title, Description: req.Description},
			func(ctx context.Context, tx database.Querier, dto CourseDTO) error {
				resType := "course"
				resID, _ := uuid.Parse(dto.ID)
				return httpx.CompleteJSON(ctx, tx, h.idem.Svc, rec.ID,
					http.StatusCreated, dto,
					map[string]string{"Location": "/api/v1/courses/" + dto.ID}, &resType, &resID, nil)
			})
		if err != nil {
			return 0, nil, err
		}
		c.Header("Location", "/api/v1/courses/"+dto.ID)
		return http.StatusCreated, dto, nil
	})
}

func (h *Handler) list(c *gin.Context) {
	limit, ok := httpx.ParseLimit(c)
	if !ok {
		badRequest(c, "limit 必须是 1-100 的整数。")
		return
	}
	f := ListFilter{Limit: limit}
	if raw := c.Query("archived"); raw != "" {
		b, err := strconv.ParseBool(raw)
		if err != nil {
			badRequest(c, "archived 必须是布尔值。")
			return
		}
		f.Archived = &b
	}
	if q := c.Query("q"); q != "" {
		if utf8.RuneCountInString(q) > 200 {
			badRequest(c, "q 不能超过 200 字符。")
			return
		}
		f.Query = q
	}
	if raw := c.Query("cursor"); raw != "" {
		var m map[string]any
		if !httpx.DecodeCursor(raw, &m) {
			badRequest(c, "cursor 无效。")
			return
		}
		createdAt, id, ok := parseCourseCursor(m)
		if !ok {
			badRequest(c, "cursor 无效。")
			return
		}
		f.AfterCreatedAt = &createdAt
		f.AfterID = &id
	}
	rows, hasMore, err := h.svc.List(c.Request.Context(), f)
	if err != nil {
		writeError(c, err)
		return
	}
	items := make([]CourseDTO, 0, len(rows))
	for _, r := range rows {
		items = append(items, toDTO(r))
	}
	var next *string
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		// 游标使用全精度时间，避免同秒创建的课程翻页时漏行。
		s := httpx.EncodeCursor(map[string]any{
			"created_at": last.CreatedAt.UTC().Format(time.RFC3339Nano),
			"id":         last.ID.String(),
		})
		next = &s
	}
	c.JSON(http.StatusOK, httpx.Collection[CourseDTO]{
		Items:    items,
		PageInfo: httpx.PageInfo{NextCursor: next, HasMore: hasMore},
	})
}

func (h *Handler) get(c *gin.Context) {
	id, ok := parseUUID(c, "course_id")
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
	id, ok := parseUUID(c, "course_id")
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
	errs = append(errs, titleFieldErrs(req.Title, false)...)
	errs = append(errs, descriptionFieldErrs(req.Description)...)
	if len(errs) > 0 {
		httpx.WriteValidation(c, errs)
		return
	}
	dto, err := h.svc.Update(c.Request.Context(), id, UpdateInput{
		Title: req.Title, Description: req.Description,
		Archived: req.Archived, Version: *req.Version,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto)
}

func (h *Handler) delete(c *gin.Context) {
	id, ok := parseUUID(c, "course_id")
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// writeError 映射本模块哨兵错误；其余交给 httpx.WriteError。
// 本映射必须与 httpx.WriteError 中其他模块的响应形状保持一致
// （httpx 不能 import 本包，故映射留在本模块，见 httpx 包注释）。
func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpserver.AbortWithProblem(c, http.StatusNotFound, "not_found",
			"资源不存在", "请求的资源不存在或已删除。")
	case errors.Is(err, ErrVersionConflict):
		httpserver.AbortWithProblem(c, http.StatusConflict, "version_conflict",
			"版本冲突", "资源已被修改，请刷新后重试。")
	default:
		httpx.WriteError(c, err)
	}
}

func badRequest(c *gin.Context, detail string) {
	httpserver.AbortWithProblem(c, http.StatusBadRequest, "bad_request", "请求无效", detail)
}

func parseUUID(c *gin.Context, param string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(param))
	if err != nil {
		badRequest(c, "路径参数 "+param+" 必须是 UUID。")
		return uuid.Nil, false
	}
	return id, true
}

// parseCourseCursor 提取 created_at/id 排序键；缺键或类型不符返回 false。
func parseCourseCursor(m map[string]any) (time.Time, uuid.UUID, bool) {
	s, ok := m["created_at"].(string)
	if !ok {
		return time.Time{}, uuid.Nil, false
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, uuid.Nil, false
	}
	ids, ok := m["id"].(string)
	if !ok {
		return time.Time{}, uuid.Nil, false
	}
	id, err := uuid.Parse(ids)
	if err != nil {
		return time.Time{}, uuid.Nil, false
	}
	return t, id, true
}

// titleFieldErrs 校验标题（required 表示缺失也算错误）。
func titleFieldErrs(v *string, required bool) []httpserver.FieldError {
	if v == nil {
		if required {
			return []httpserver.FieldError{httpx.FieldErr("title", "required", "课程名称不能为空")}
		}
		return nil
	}
	switch {
	case strings.TrimSpace(*v) == "":
		return []httpserver.FieldError{httpx.FieldErr("title", "too_short", "课程名称不能为空")}
	case utf8.RuneCountInString(*v) > 200:
		return []httpserver.FieldError{httpx.FieldErr("title", "too_long", "课程名称不能超过 200 字符")}
	}
	return nil
}

// descriptionFieldErrs 校验描述长度。
func descriptionFieldErrs(v *string) []httpserver.FieldError {
	if v != nil && utf8.RuneCountInString(*v) > 5000 {
		return []httpserver.FieldError{httpx.FieldErr("description", "too_long", "课程描述不能超过 5000 字符")}
	}
	return nil
}
