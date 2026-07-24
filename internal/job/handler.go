package job

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
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

// RegisterRoutes 注册 Job 端点。SSE 静态路由必须先于 :job_id。
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, jsonLimit gin.HandlerFunc) {
	rg.GET("/jobs", h.list)
	rg.GET("/jobs/stream", h.stream)
	rg.GET("/jobs/:job_id/events", h.events)
	rg.GET("/jobs/:job_id", h.get)
	rg.POST("/jobs/:job_id/cancel", jsonLimit, h.cancel)
	rg.POST("/jobs/:job_id/retry", jsonLimit, h.retry)
}

func (h *Handler) list(c *gin.Context) {
	limit, ok := httpx.ParseLimit(c)
	if !ok {
		badRequest(c, "limit 必须是 1-100 的整数。")
		return
	}
	f := ListFilter{Limit: limit}
	if raw := c.Query("status"); raw != "" {
		if !validStatus(raw) {
			badRequest(c, "status 无效。")
			return
		}
		f.Status = &raw
	}
	if raw := c.Query("active"); raw != "" {
		b, err := strconv.ParseBool(raw)
		if err != nil {
			badRequest(c, "active 必须是布尔值。")
			return
		}
		f.Active = &b
	}
	if raw := c.Query("kind"); raw != "" {
		if !validKind(raw) {
			badRequest(c, "kind 无效。")
			return
		}
		f.Kind = &raw
	}
	if raw := c.Query("aggregate_type"); raw != "" {
		f.AggregateType = &raw
	}
	if raw := c.Query("aggregate_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			badRequest(c, "aggregate_id 必须是 UUID。")
			return
		}
		f.AggregateID = &id
	}
	if raw := c.Query("cursor"); raw != "" {
		createdAt, id, ok := parseJobCursor(raw)
		if !ok {
			badRequest(c, "cursor 无效。")
			return
		}
		f.BeforeCreatedAt = &createdAt
		f.BeforeID = &id
	}
	rows, hasMore, err := h.svc.List(c.Request.Context(), f)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	items := make([]DTO, 0, len(rows))
	for i := range rows {
		items = append(items, ToDTO(&rows[i].Job, rows[i].Message))
	}
	var next *string
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		s := httpx.EncodeCursor(map[string]any{
			"created_at": last.CreatedAt.UTC().Format(time.RFC3339Nano),
			"id":         last.ID.String(),
		})
		next = &s
	}
	c.JSON(http.StatusOK, httpx.Collection[DTO]{
		Items:    items,
		PageInfo: httpx.PageInfo{NextCursor: next, HasMore: hasMore},
	})
}

func (h *Handler) get(c *gin.Context) {
	jobID, ok := pathJobID(c)
	if !ok {
		return
	}
	dto, err := h.svc.Get(c.Request.Context(), jobID)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto)
}

func (h *Handler) events(c *gin.Context) {
	jobID, ok := pathJobID(c)
	if !ok {
		return
	}
	limit, ok := httpx.ParseLimit(c)
	if !ok {
		badRequest(c, "limit 必须是 1-100 的整数。")
		return
	}
	var afterID int64
	if raw := c.Query("cursor"); raw != "" {
		id, ok := parseEventCursor(raw)
		if !ok {
			badRequest(c, "cursor 无效。")
			return
		}
		afterID = id
	}
	events, hasMore, err := h.svc.Events(c.Request.Context(), jobID, afterID, limit)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	items := make([]EventDTO, 0, len(events))
	for _, ev := range events {
		items = append(items, ToEventDTO(ev))
	}
	var next *string
	if hasMore && len(events) > 0 {
		s := httpx.EncodeCursor(map[string]any{"id": events[len(events)-1].ID})
		next = &s
	}
	c.JSON(http.StatusOK, httpx.Collection[EventDTO]{
		Items:    items,
		PageInfo: httpx.PageInfo{NextCursor: next, HasMore: hasMore},
	})
}

func (h *Handler) cancel(c *gin.Context) {
	jobID, ok := pathJobID(c)
	if !ok {
		return
	}
	hash := httpx.HashRequest(c.Request.Method, c.FullPath(), nil, "")
	h.idem.Run(c, "cancelJob", hash,
		func(ctx context.Context, rec *idempotency.Record) (int, any, error) {
			status, dto, err := h.svc.Cancel(ctx, rec, jobID)
			return status, dto, err
		})
}

// POST /api/v1/jobs/:job_id/retry —— 为终态失败任务创建手动重试 Job。
func (h *Handler) retry(c *gin.Context) {
	jobID, ok := pathJobID(c)
	if !ok {
		return
	}
	hash := httpx.HashRequest(c.Request.Method, c.FullPath(), nil, "")
	h.idem.Run(c, "retryJob", hash,
		func(ctx context.Context, rec *idempotency.Record) (int, any, error) {
			return h.svc.Retry(ctx, rec, jobID)
		})
}

func (h *Handler) stream(c *gin.Context) {
	lastID, ok := parseLastEventID(c)
	if !ok {
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	c.Writer.WriteHeaderNow()
	_ = h.svc.Stream(c.Request.Context(), ginStreamSink{w: c.Writer}, lastID)
}

type ginStreamSink struct {
	w gin.ResponseWriter
}

func (s ginStreamSink) WriteFrame(frame StreamFrame) error {
	if frame.ID != nil {
		if _, err := fmt.Fprintf(s.w, "id: %d\n", *frame.ID); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(s.w, "event: %s\n", frame.Event); err != nil {
		return err
	}
	data, err := json.Marshal(frame.Data)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(s.w, "data: %s\n\n", data); err != nil {
		return err
	}
	s.w.Flush()
	return nil
}

func pathJobID(c *gin.Context) (uuid.UUID, bool) {
	jobID, err := uuid.Parse(c.Param("job_id"))
	if err != nil {
		httpserver.AbortWithProblem(c, http.StatusNotFound, "not_found",
			"资源不存在", "请求的资源不存在或已删除。")
		return uuid.Nil, false
	}
	return jobID, true
}

func parseJobCursor(raw string) (time.Time, uuid.UUID, bool) {
	var m map[string]any
	if !httpx.DecodeCursor(raw, &m) {
		return time.Time{}, uuid.Nil, false
	}
	ts, ok := m["created_at"].(string)
	if !ok {
		return time.Time{}, uuid.Nil, false
	}
	createdAt, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return time.Time{}, uuid.Nil, false
	}
	rawID, ok := m["id"].(string)
	if !ok {
		return time.Time{}, uuid.Nil, false
	}
	id, err := uuid.Parse(rawID)
	return createdAt, id, err == nil
}

func parseEventCursor(raw string) (int64, bool) {
	var m map[string]any
	if !httpx.DecodeCursor(raw, &m) {
		return 0, false
	}
	f, ok := m["id"].(float64)
	if !ok || f < 0 || f != float64(int64(f)) {
		return 0, false
	}
	return int64(f), true
}

func parseLastEventID(c *gin.Context) (int64, bool) {
	raw := c.GetHeader("Last-Event-ID")
	if raw == "" {
		return 0, true
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n < 0 {
		badRequest(c, "Last-Event-ID 必须是非负整数。")
		return 0, false
	}
	return n, true
}

func badRequest(c *gin.Context, detail string) {
	httpserver.AbortWithProblem(c, http.StatusBadRequest, "bad_request", "请求无效", detail)
}

func validStatus(s string) bool {
	switch s {
	case StatusQueued, StatusRunning, StatusValidating, StatusSucceeded,
		StatusFailed, StatusCancelRequested, StatusCancelled:
		return true
	default:
		return false
	}
}

func validKind(s string) bool {
	switch s {
	case KindClaudeConnectivityTest, KindLessonImport, KindLessonRegenerate,
		KindQuizGenerate, KindReviewQuizGenerate, KindAttemptGrade,
		KindWrongAnswerAnalyze, KindSummaryEvaluate, KindLessonExport:
		return true
	default:
		return false
	}
}
