package apitest

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/TSY244/ai-study-tutor/internal/platform/config"
	"github.com/TSY244/ai-study-tutor/internal/platform/httpserver"
	"github.com/TSY244/ai-study-tutor/internal/platform/id"
)

// NewEngine 构建与生产一致的基础引擎（全部中间件 + healthz/readyz），
// 业务模块在其上注册 /api/v1 路由。Host 白名单仅 localhost。
func NewEngine(t *testing.T) *gin.Engine {
	t.Helper()
	s := httpserver.New(httpserver.Deps{
		Config: config.HTTPConfig{
			AllowedHosts:   []string{"localhost", "127.0.0.1"},
			BodyLimitBytes: 1 << 20,
			RequestTimeout: 30 * time.Second,
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		IDGen:  id.Random{},
	})
	return s.Engine()
}

// Do 发起测试请求（Host=localhost 满足 HostOrigin 校验）。
func Do(t *testing.T, engine http.Handler, method, target string,
	headers map[string]string, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, "http://localhost"+target, reader)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

// DoRaw 与 Do 相同，但接受任意 body reader（multipart 构造用）。
func DoRaw(t *testing.T, engine http.Handler, method, target string,
	headers map[string]string, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "http://localhost"+target, body)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}
