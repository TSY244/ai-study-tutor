// handler.go 是作答端点的 HTTP 层：只做绑定、校验与响应映射。
package attempt

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/TSY244/ai-study-tutor/internal/httpx"
	"github.com/TSY244/ai-study-tutor/internal/idempotency"
	"github.com/TSY244/ai-study-tutor/internal/platform/httpserver"
)

// Handler 只做 HTTP 绑定与响应映射；业务在 Service。
type Handler struct {
	svc  *Service
	idem *httpx.IdemExecutor
}

func NewHandler(svc *Service, idem *httpx.IdemExecutor) *Handler {
	return &Handler{svc: svc, idem: idem}
}

// RegisterRoutes 注册作答端点；创建/提交/重建要求 Idempotency-Key，
// 自动保存以 answer_version 为乐观锁（PATCH 语义天然幂等）。
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, jsonLimit gin.HandlerFunc) {
	rg.POST("/quizzes/:quiz_id/attempts", jsonLimit, h.createAttempt)
	rg.GET("/attempts/:attempt_id", h.getAttempt)
	rg.PATCH("/attempts/:attempt_id/answers/:question_id", jsonLimit, h.saveAnswer)
	rg.POST("/attempts/:attempt_id/submit", jsonLimit, h.submit)
	rg.POST("/attempts/:attempt_id/grading-jobs", jsonLimit, h.createGradingJob)
}

// POST /api/v1/quizzes/:quiz_id/attempts —— 开始作答（201 + Location）。
func (h *Handler) createAttempt(c *gin.Context) {
	quizID, ok := pathUUID(c, "quiz_id")
	if !ok {
		return
	}
	hash := httpx.HashRequest(c.Request.Method, c.FullPath(), nil, "")
	h.idem.Run(c, "createAttempt", hash,
		func(ctx context.Context, rec *idempotency.Record) (int, any, error) {
			status, dto, err := h.svc.CreateAttempt(ctx, rec, quizID)
			if err != nil {
				return 0, nil, err
			}
			c.Header("Location", "/api/v1/attempts/"+dto.ID)
			return status, dto, nil
		})
}

// GET /api/v1/attempts/:attempt_id —— 查询作答与批阅状态。
func (h *Handler) getAttempt(c *gin.Context) {
	attemptID, ok := pathUUID(c, "attempt_id")
	if !ok {
		return
	}
	dto, err := h.svc.GetAttempt(c.Request.Context(), attemptID)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto)
}

// answerSaveRequest 是自动保存请求体（契约 AnswerSave）。
type answerSaveRequest struct {
	Answer          json.RawMessage `json:"answer"`
	AnswerVersion   int64           `json:"answer_version"`
	ClientUpdatedAt string          `json:"client_updated_at"`
}

// PATCH /api/v1/attempts/:attempt_id/answers/:question_id —— 自动保存单题答案。
func (h *Handler) saveAnswer(c *gin.Context) {
	attemptID, ok := pathUUID(c, "attempt_id")
	if !ok {
		return
	}
	questionID, ok := pathUUID(c, "question_id")
	if !ok {
		return
	}
	var req answerSaveRequest
	if !httpx.BindJSON(c, &req) {
		badRequest(c, "请求体必须是合法 JSON 对象。")
		return
	}
	var errs []httpserver.FieldError
	if req.AnswerVersion < 0 {
		errs = append(errs, httpx.FieldErr("answer_version", "out_of_range", "answer_version 不能小于 0。"))
	}
	if !validAnswerValue(req.Answer) {
		errs = append(errs, httpx.FieldErr("answer", "invalid", "answer 必须是包含 value 键的 JSON 对象。"))
	}
	if _, err := time.Parse(time.RFC3339, req.ClientUpdatedAt); err != nil {
		errs = append(errs, httpx.FieldErr("client_updated_at", "invalid", "client_updated_at 必须是 RFC 3339 时间。"))
	}
	if len(errs) > 0 {
		httpx.WriteValidation(c, errs)
		return
	}
	dto, err := h.svc.SaveAnswer(c.Request.Context(), attemptID, questionID, req.Answer, req.AnswerVersion)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto)
}

// validAnswerValue 校验 answer 为含 value 键的 JSON 对象（AnswerValue 契约）。
func validAnswerValue(raw json.RawMessage) bool {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return false
	}
	_, ok := obj["value"]
	return ok
}

// submitRequest 是提交请求体（契约 submitAttempt 请求）。
type submitRequest struct {
	Version int64 `json:"version"`
}

// POST /api/v1/attempts/:attempt_id/submit —— 提交作答（200/202 拆分）。
func (h *Handler) submit(c *gin.Context) {
	attemptID, ok := pathUUID(c, "attempt_id")
	if !ok {
		return
	}
	var req submitRequest
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
	h.idem.Run(c, "submitAttempt", hash,
		func(ctx context.Context, rec *idempotency.Record) (int, any, error) {
			return h.svc.Submit(ctx, rec, attemptID, req.Version)
		})
}

// POST /api/v1/attempts/:attempt_id/grading-jobs —— 为批阅失败的作答重建批阅任务。
func (h *Handler) createGradingJob(c *gin.Context) {
	attemptID, ok := pathUUID(c, "attempt_id")
	if !ok {
		return
	}
	hash := httpx.HashRequest(c.Request.Method, c.FullPath(), nil, "")
	h.idem.Run(c, "createAttemptGradingJob", hash,
		func(ctx context.Context, rec *idempotency.Record) (int, any, error) {
			return h.svc.CreateGradingJob(ctx, rec, attemptID)
		})
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
