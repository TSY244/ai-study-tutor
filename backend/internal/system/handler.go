// Package system exposes non-sensitive process capabilities. It only maps
// HTTP requests to application services; Worker owns provider execution.
package system

import (
	"context"

	"github.com/gin-gonic/gin"

	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
	"github.com/TSY244/ai-study-tutor/backend/internal/idempotency"
	"github.com/TSY244/ai-study-tutor/backend/internal/job"
)

type Handler struct {
	jobs   *job.Service
	idem   *httpx.IdemExecutor
	status *StatusService
}

func NewHandler(jobs *job.Service, idem *httpx.IdemExecutor, status *StatusService) *Handler {
	return &Handler{jobs: jobs, idem: idem, status: status}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, jsonLimit gin.HandlerFunc) {
	rg.GET("/system/status", h.getStatus)
	rg.POST("/system/claude-tests", jsonLimit, h.createClaudeTest)
}

func (h *Handler) getStatus(c *gin.Context) {
	c.JSON(200, h.status.Get(c.Request.Context()))
}

func (h *Handler) createClaudeTest(c *gin.Context) {
	hash := httpx.HashRequest(c.Request.Method, c.FullPath(), nil, "")
	h.idem.Run(c, "createClaudeConnectivityTest", hash,
		func(ctx context.Context, rec *idempotency.Record) (int, any, error) {
			status, body, err := h.jobs.CreateConnectivity(ctx, rec)
			if err == nil {
				c.Header("Location", "/api/v1/jobs/"+body.Job.ID.String())
			}
			return status, body, err
		})
}
