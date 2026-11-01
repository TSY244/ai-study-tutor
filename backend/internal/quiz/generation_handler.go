package quiz

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

// quizGenerationRequest 是 createQuizGeneration 的请求体（契约
// QuizGenerationCreate：question_count 5..15 可空，title ≤300 可空）。
type quizGenerationRequest struct {
	QuestionCount   *int32  `json:"question_count"`
	Title           *string `json:"title"`
	GenerationNotes string  `json:"generation_notes"`
}

// reviewQuizGenerationRequest 是 createReviewQuizGeneration 的请求体
// （契约 ReviewQuizGenerationCreate）。
type reviewQuizGenerationRequest struct {
	QuestionCount    *int32      `json:"question_count"`
	IncludeLessonIDs []uuid.UUID `json:"include_lesson_ids"`
	WeaknessIDs      []uuid.UUID `json:"weakness_ids"`
	GenerationNotes  string      `json:"generation_notes"`
}

// GenerationHandler 只做 HTTP 绑定与响应映射；业务在 GenerationService。
type GenerationHandler struct {
	svc  *GenerationService
	idem *httpx.IdemExecutor
}

func NewGenerationHandler(svc *GenerationService, idem *httpx.IdemExecutor) *GenerationHandler {
	return &GenerationHandler{svc: svc, idem: idem}
}

// RegisterRoutes 注册两个出题端点（均要求 Idempotency-Key）。
func (h *GenerationHandler) RegisterRoutes(rg *gin.RouterGroup, jsonLimit gin.HandlerFunc) {
	rg.POST("/lessons/:lesson_id/quiz-generations", jsonLimit, h.createQuizGeneration)
	rg.POST("/lessons/:lesson_id/review-quiz-generations", jsonLimit, h.createReviewQuizGeneration)
}

// POST /api/v1/lessons/:lesson_id/quiz-generations —— 创建常规试卷生成任务。
func (h *GenerationHandler) createQuizGeneration(c *gin.Context) {
	lessonID, ok := pathUUID(c, "lesson_id")
	if !ok {
		return
	}
	var req quizGenerationRequest
	if !httpx.BindJSON(c, &req) {
		badRequest(c, "请求体必须是合法 JSON 对象。")
		return
	}
	var errs []httpserver.FieldError
	if e := validateQuestionCount(req.QuestionCount); e != nil {
		errs = append(errs, *e)
	}
	if req.Title != nil && utf8.RuneCountInString(*req.Title) > 300 {
		errs = append(errs, httpx.FieldErr("title", "too_long", "title 不能超过 300 字符。"))
	}
	if utf8.RuneCountInString(req.GenerationNotes) > 3000 {
		errs = append(errs, httpx.FieldErr("generation_notes", "too_long", "出题备注不能超过 3000 字符。"))
	}
	if len(errs) > 0 {
		httpx.WriteValidation(c, errs)
		return
	}
	hash := httpx.HashRequest(c.Request.Method, c.FullPath(), httpx.CanonicalJSON(req), "")
	h.idem.Run(c, "createQuizGeneration", hash,
		func(ctx context.Context, rec *idempotency.Record) (int, any, error) {
			return h.svc.CreateQuizGeneration(ctx, rec, lessonID, req.QuestionCount, req.Title, req.GenerationNotes)
		})
}

// POST /api/v1/lessons/:lesson_id/review-quiz-generations —— 创建复习试卷生成任务。
func (h *GenerationHandler) createReviewQuizGeneration(c *gin.Context) {
	lessonID, ok := pathUUID(c, "lesson_id")
	if !ok {
		return
	}
	var req reviewQuizGenerationRequest
	if !httpx.BindJSON(c, &req) {
		badRequest(c, "请求体必须是合法 JSON 对象。")
		return
	}
	if e := validateQuestionCount(req.QuestionCount); e != nil {
		httpx.WriteValidation(c, []httpserver.FieldError{*e})
		return
	}
	if utf8.RuneCountInString(req.GenerationNotes) > 3000 {
		httpx.WriteValidation(c, []httpserver.FieldError{httpx.FieldErr("generation_notes", "too_long", "出题备注不能超过 3000 字符。")})
		return
	}
	hash := httpx.HashRequest(c.Request.Method, c.FullPath(), httpx.CanonicalJSON(req), "")
	h.idem.Run(c, "createReviewQuizGeneration", hash,
		func(ctx context.Context, rec *idempotency.Record) (int, any, error) {
			return h.svc.CreateReviewQuizGeneration(ctx, rec, lessonID,
				req.QuestionCount, req.IncludeLessonIDs, req.WeaknessIDs, req.GenerationNotes)
		})
}

// validateQuestionCount 校验 question_count（可空；非空时必须在 5 到 15）。
func validateQuestionCount(qc *int32) *httpserver.FieldError {
	if qc != nil && (*qc < 5 || *qc > 15) {
		e := httpx.FieldErr("question_count", "out_of_range", "question_count 必须在 5 到 15 之间。")
		return &e
	}
	return nil
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
