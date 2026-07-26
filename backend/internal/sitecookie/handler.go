package sitecookie

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
	"github.com/TSY244/ai-study-tutor/backend/internal/idempotency"
	"github.com/TSY244/ai-study-tutor/backend/internal/job"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/httpserver"
)

// Handler 暴露 /api/v1/system/bilibili-cookie 管理端点。
// 所有响应只含安全视图（masked_sessdata/过期时间），绝不含完整 Cookie。
type Handler struct {
	Store *Store
	Jobs  *job.Service

	idem *httpx.IdemExecutor
}

// RegisterRoutes 注册 B 站 Cookie 管理路由（启动期调用一次）。
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, jsonLimit gin.HandlerFunc, idem *httpx.IdemExecutor) {
	h.idem = idem
	rg.GET("/system/bilibili-cookie", h.getStatus)
	rg.PUT("/system/bilibili-cookie", jsonLimit, h.saveCookie)
	rg.DELETE("/system/bilibili-cookie", h.deleteCookie)
	rg.POST("/system/bilibili-cookie/tests", jsonLimit, h.createCookieTest)
}

func (h *Handler) getStatus(c *gin.Context) {
	status, err := h.Store.Status()
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, status)
}

type saveCookieRequest struct {
	Content string `json:"content"`
}

// saveCookie 保存 Cookie。PUT 天然幂等（相同内容重复保存结果一致，
// 写盘原子），因此不要求 Idempotency-Key。
func (h *Handler) saveCookie(c *gin.Context) {
	var req saveCookieRequest
	if !httpx.BindJSON(c, &req) {
		badRequest(c, "请求体不是有效的 JSON。")
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		badRequest(c, "content 不能为空或全为空白。")
		return
	}
	status, err := h.Store.Save(req.Content)
	if err != nil {
		if errors.Is(err, ErrInvalid) {
			badRequest(c, "内容无法识别：请粘贴 Netscape Cookie 导出内容、Cookie 头串或 SESSDATA 值，且必须包含 bilibili.com 的 SESSDATA。")
			return
		}
		httpx.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, status)
}

func (h *Handler) deleteCookie(c *gin.Context) {
	if err := h.Store.Delete(); err != nil {
		if errors.Is(err, ErrNotFound) {
			httpserver.AbortWithProblem(c, http.StatusNotFound, "not_found",
				"资源不存在", "尚未保存 B 站 Cookie。")
			return
		}
		httpx.WriteError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) createCookieTest(c *gin.Context) {
	hash := httpx.HashRequest(c.Request.Method, c.FullPath(), nil, "")
	h.idem.Run(c, "createBilibiliCookieTest", hash,
		func(ctx context.Context, rec *idempotency.Record) (int, any, error) {
			status, body, err := h.Jobs.CreateBilibiliCookieTest(ctx, rec)
			if err == nil {
				c.Header("Location", "/api/v1/jobs/"+body.Job.ID.String())
			}
			return status, body, err
		})
}

func badRequest(c *gin.Context, detail string) {
	httpserver.AbortWithProblem(c, http.StatusBadRequest, "bad_request", "请求无效", detail)
}
