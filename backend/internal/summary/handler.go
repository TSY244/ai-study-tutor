package summary

import (
	"context"
	"net/http"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
	"github.com/TSY244/ai-study-tutor/backend/internal/idempotency"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/httpserver"
)

// createEvaluationRequest 是 createSummaryEvaluation 的请求体
// （契约：summary 必填，1-50000 字符，additionalProperties:false 对齐）。
type createEvaluationRequest struct {
	Summary string `json:"summary"`
}

// Handler 只做 HTTP 绑定与响应映射；业务在 Service。
type Handler struct {
	svc  *Service
	idem *httpx.IdemExecutor
}

func NewHandler(svc *Service, idem *httpx.IdemExecutor) *Handler {
	return &Handler{svc: svc, idem: idem}
}

// RegisterRoutes 注册总结评估端点（POST 要求 Idempotency-Key；GET 只读）。
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, jsonLimit gin.HandlerFunc) {
	rg.POST("/lessons/:lesson_id/summary-evaluations", jsonLimit, h.createEvaluation)
	rg.GET("/summary-evaluations/:evaluation_id", h.getEvaluation)
	rg.GET("/lessons/:lesson_id/summary-evaluations", h.listEvaluations)
	rg.PATCH("/summary-evaluations/:evaluation_id", jsonLimit, h.editEvaluation)
	rg.DELETE("/summary-evaluations/:evaluation_id", h.deleteEvaluation)
}

// POST /api/v1/lessons/:lesson_id/summary-evaluations —— 创建学习总结评估任务。
func (h *Handler) createEvaluation(c *gin.Context) {
	lessonID, ok := pathUUID(c, "lesson_id")
	if !ok {
		return
	}
	var req createEvaluationRequest
	if !httpx.BindJSON(c, &req) {
		badRequest(c, "请求体必须是合法 JSON 对象。")
		return
	}
	var errs []httpserver.FieldError
	if req.Summary == "" {
		errs = append(errs, httpx.FieldErr("summary", "required", "summary 不能为空。"))
	} else if utf8.RuneCountInString(req.Summary) > 50000 {
		errs = append(errs, httpx.FieldErr("summary", "too_long", "summary 不能超过 50000 字符。"))
	}
	if len(errs) > 0 {
		httpx.WriteValidation(c, errs)
		return
	}
	hash := httpx.HashRequest(c.Request.Method, c.FullPath(), httpx.CanonicalJSON(req), "")
	h.idem.Run(c, "createSummaryEvaluation", hash,
		func(ctx context.Context, rec *idempotency.Record) (int, any, error) {
			return h.svc.CreateEvaluation(ctx, rec, lessonID, req.Summary)
		})
}

// GET /api/v1/summary-evaluations/:evaluation_id —— 查询学习总结评估。
func (h *Handler) getEvaluation(c *gin.Context) {
	evaluationID, ok := pathUUID(c, "evaluation_id")
	if !ok {
		return
	}
	dto, err := h.svc.GetEvaluation(c.Request.Context(), evaluationID)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto)
}

// pathUUID 解析路径参数 UUID；非法值映射 404（路径标识的资源不可能存在）。
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
