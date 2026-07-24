package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/TSY244/ai-study-tutor/backend/internal/platform/id"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/logging"
)

// 中间件顺序（docs/web-rebuild/03-backend-architecture.md §11）：
// RequestID → AccessLog → Recovery → SecurityHeaders → HostOrigin →
// BodyLimit → Timeout（SSE 路由例外）→ 路由 Handler。

type ctxKey string

const requestIDCtxKey ctxKey = "request_id"

// requestID 返回当前请求的 request_id（由 RequestID 中间件保证存在）。
func requestID(c *gin.Context) string {
	if v, ok := c.Get(string(requestIDCtxKey)); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// RequestID 中间件：接受合法的客户端 X-Request-ID，否则生成新值；
// 始终在响应上回写 X-Request-ID，并把 ID 放入 gin 上下文与请求 context。
func RequestID(gen id.Generator) gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := id.SanitizeRequestID(c.GetHeader("X-Request-ID"), gen)
		c.Set(string(requestIDCtxKey), rid)
		c.Header("X-Request-ID", rid)
		c.Request = c.Request.WithContext(logging.WithRequestID(c.Request.Context(), rid))
		c.Next()
	}
}

// AccessLog 输出结构化访问日志；不记录请求体、查询串敏感值或 Header。
func AccessLog(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		logger.Info("http_request",
			slog.String("request_id", requestID(c)),
			slog.String("method", c.Request.Method),
			slog.String("path", c.FullPath()),
			slog.Int("status", c.Writer.Status()),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
			slog.String("client_ip", c.ClientIP()),
		)
	}
}

// Recovery 把 panic 转换为 500 Problem，不向前端泄露堆栈；
// 堆栈只进服务端日志，且不包含请求体。
func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				logger.Error("http_panic",
					slog.String("request_id", requestID(c)),
					slog.Any("panic", rec),
					slog.String("path", c.Request.URL.Path),
				)
				AbortWithProblem(c, 500, "internal_error", "服务器内部错误",
					"处理请求时发生未预期错误，请携带 request_id 查看服务日志。")
			}
		}()
		c.Next()
	}
}

// SecurityHeaders 与 Caddy 层保持一致的基础安全响应头。
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("X-Frame-Options", "DENY")
		c.Next()
	}
}

// HostOrigin 校验本机部署边界（防 localhost DNS rebinding / 跨站写请求）：
//   - Host 必须属于允许列表（比较时忽略端口）；
//   - 携带 Origin 的写请求（POST/PUT/PATCH/DELETE）必须同源；
//   - CORS 预检（OPTIONS + Access-Control-Request-Method）默认拒绝。
func HostOrigin(allowedHosts []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedHosts))
	for _, h := range allowedHosts {
		allowed[strings.ToLower(h)] = struct{}{}
	}
	isAllowed := func(hostport string) bool {
		host := hostport
		if i := strings.LastIndex(host, ":"); i >= 0 {
			host = host[:i]
		}
		host = strings.Trim(strings.ToLower(host), "[]")
		_, ok := allowed[host]
		return ok
	}
	return func(c *gin.Context) {
		if !isAllowed(c.Request.Host) {
			AbortWithProblem(c, 400, "bad_request", "请求无效", "Host 不在允许列表内。")
			return
		}
		if origin := c.GetHeader("Origin"); origin != "" {
			switch c.Request.Method {
			case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
				u, err := url.Parse(origin)
				if err != nil || !isAllowed(u.Host) {
					AbortWithProblem(c, 403, "forbidden", "请求被拒绝", "跨来源写请求被拒绝。")
					return
				}
			case http.MethodOptions:
				if c.GetHeader("Access-Control-Request-Method") != "" {
					AbortWithProblem(c, 403, "forbidden", "请求被拒绝", "不启用跨域预检。")
					return
				}
			}
		}
		c.Next()
	}
}

// BodyLimit 限制请求体大小；超限映射为稳定 Problem：
// 上传类路由用 413 upload_too_large，普通 JSON 用 400 bad_request。
func BodyLimit(limitBytes int64, status int, code string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limitBytes)
		}
		c.Next()
		for _, err := range c.Errors {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				switch {
				case status == 413:
					AbortWithProblem(c, 413, "upload_too_large", "上传超过限制", "请求体超过大小限制。")
				default:
					AbortWithProblem(c, 400, "bad_request", "请求无效", "请求体超过大小限制。")
				}
				return
			}
		}
	}
}

// Timeout 为非 SSE 请求设置处理截止时间（注入 request context），
// SSE 路由路径保持无超时以支持长连接。
func Timeout(d time.Duration, ssePaths []string) gin.HandlerFunc {
	exempt := make(map[string]struct{}, len(ssePaths))
	for _, p := range ssePaths {
		exempt[p] = struct{}{}
	}
	return func(c *gin.Context) {
		if _, ok := exempt[c.Request.URL.Path]; ok {
			c.Next()
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), d)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
