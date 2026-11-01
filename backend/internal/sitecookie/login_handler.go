package sitecookie

import (
	"context"
	"errors"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
	"github.com/TSY244/ai-study-tutor/backend/internal/idempotency"
	"github.com/TSY244/ai-study-tutor/backend/internal/job"
)

func (h *Handler) createLogin(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	if err := h.Sessions.Prune(); err != nil {
		httpx.WriteError(c, errors.New("清理过期登录状态失败"))
		return
	}
	hash := httpx.HashRequest(c.Request.Method, c.FullPath(), nil, "")
	h.idem.Run(c, "createBilibiliLogin", hash, func(ctx context.Context, rec *idempotency.Record) (int, any, error) {
		status, body, err := h.Jobs.CreateBilibiliLogin(ctx, rec)
		if err == nil {
			c.Header("Location", "/api/v1/jobs/"+body.Job.ID.String())
		}
		return status, body, err
	})
}

func (h *Handler) pollLogin(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	id, err := uuid.Parse(c.Param("login_id"))
	if err != nil {
		badRequest(c, "登录会话 ID 无效。")
		return
	}
	j, err := h.Jobs.Get(c.Request.Context(), id)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	if j.Kind != job.KindBilibiliLogin {
		badRequest(c, "该任务不是扫码登录会话。")
		return
	}
	var st LoginStatus
	switch j.Status {
	case job.StatusSucceeded:
		st, err = h.Sessions.Complete(id, h.Store)
	case job.StatusFailed:
		st = LoginStatus{State: "failed", Message: "扫码登录失败，请重新生成二维码。"}
		if j.Error != nil && j.Error.Code == "bilibili_login_expired" {
			st = LoginStatus{State: "expired", Message: "二维码已过期，请重新生成。"}
		}
	case job.StatusCancelled, job.StatusCancelRequested:
		st = LoginStatus{State: "cancelled", Message: "登录已取消。"}
	default:
		st, err = h.Sessions.Read(id)
		if errors.Is(err, os.ErrNotExist) {
			st, err = LoginStatus{State: "generating", Message: "正在生成登录二维码…"}, nil
		}
	}
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, st)
}
