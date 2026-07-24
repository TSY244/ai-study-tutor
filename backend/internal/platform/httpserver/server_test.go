package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/TSY244/ai-study-tutor/backend/internal/platform/config"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/database"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/id"
)

type fakeChecker struct{ err error }

func (f fakeChecker) Check(context.Context) error { return f.err }

func newTestServer(t *testing.T, checks map[string]database.Checker) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	return New(Deps{
		Config: config.HTTPConfig{
			AllowedHosts:   []string{"localhost", "127.0.0.1", "::1", "example.test"},
			BodyLimitBytes: 16,
			RequestTimeout: 50 * time.Millisecond,
			SSEPaths:       []string{"/api/v1/jobs/stream"},
		},
		Logger:      logger,
		IDGen:       &id.Fixed{Values: []string{"00000000-0000-4000-8000-000000000001"}},
		ReadyChecks: checks,
	})
}

func do(t *testing.T, s *Server, method, path, host string, headers map[string]string, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Host = host
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func decodeProblem(t *testing.T, rec *httptest.ResponseRecorder) Problem {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("错误响应 Content-Type 错误：%q", ct)
	}
	var p Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("Problem JSON 解析失败：%v", err)
	}
	return p
}

// ---------------------------------------------------------------------------
// healthz / readyz
// ---------------------------------------------------------------------------

func TestHealthzAlwaysOKEvenWhenDatabaseDown(t *testing.T) {
	s := newTestServer(t, map[string]database.Checker{
		"database": fakeChecker{err: errors.New("connection refused")},
	})
	rec := do(t, s, http.MethodGet, "/healthz", "localhost", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("healthz 应为 200，实际 %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["status"] != "ok" {
		t.Fatalf("healthz 响应体错误：%s", rec.Body.String())
	}
}

func TestReadyzFailsWhenDependencyDown(t *testing.T) {
	s := newTestServer(t, map[string]database.Checker{
		"database": fakeChecker{err: errors.New("connection refused")},
	})
	rec := do(t, s, http.MethodGet, "/readyz", "localhost", nil, "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("readyz 应为 503，实际 %d", rec.Code)
	}
	p := decodeProblem(t, rec)
	if p.Code != "service_unavailable" || p.Status != 503 {
		t.Fatalf("readyz Problem 错误：%+v", p)
	}
	if !strings.Contains(p.Detail, "database") {
		t.Fatalf("readyz 应指出失败依赖：%+v", p)
	}
}

func TestReadyzOKWhenDependenciesUp(t *testing.T) {
	s := newTestServer(t, map[string]database.Checker{"database": fakeChecker{}})
	rec := do(t, s, http.MethodGet, "/readyz", "localhost", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("readyz 应为 200，实际 %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// Request ID
// ---------------------------------------------------------------------------

func TestRequestIDEchoesValidClientValue(t *testing.T) {
	s := newTestServer(t, nil)
	rec := do(t, s, http.MethodGet, "/healthz", "localhost", map[string]string{
		"X-Request-ID": "01J-Client-Trace",
	}, "")
	if got := rec.Header().Get("X-Request-ID"); got != "01J-Client-Trace" {
		t.Fatalf("合法客户端 request id 应原样回写，实际 %q", got)
	}
}

func TestRequestIDReplacesInvalidClientValue(t *testing.T) {
	s := newTestServer(t, nil)
	rec := do(t, s, http.MethodGet, "/healthz", "localhost", map[string]string{
		"X-Request-ID": "bad value with spaces",
	}, "")
	if got := rec.Header().Get("X-Request-ID"); got != "00000000-0000-4000-8000-000000000001" {
		t.Fatalf("非法 request id 应被服务端生成值替换，实际 %q", got)
	}
}

func TestRequestIDPresentOnErrorResponses(t *testing.T) {
	s := newTestServer(t, nil)
	rec := do(t, s, http.MethodGet, "/no-such-route", "localhost", nil, "")
	p := decodeProblem(t, rec)
	if p.RequestID == "" || rec.Header().Get("X-Request-ID") == "" {
		t.Fatalf("错误响应必须带 request_id 与响应头：%+v", p)
	}
	if p.RequestID != rec.Header().Get("X-Request-ID") {
		t.Fatal("Problem.request_id 与 X-Request-ID 响应头应一致")
	}
}

// ---------------------------------------------------------------------------
// Problem Details / 路由兜底 / panic recovery
// ---------------------------------------------------------------------------

func TestNotFoundIsProblem(t *testing.T) {
	s := newTestServer(t, nil)
	rec := do(t, s, http.MethodGet, "/nope", "localhost", nil, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("应为 404，实际 %d", rec.Code)
	}
	p := decodeProblem(t, rec)
	if p.Code != "not_found" || p.Type != "https://ai-study-tutor.local/problems/not-found" {
		t.Fatalf("Problem 形状错误：%+v", p)
	}
}

func TestMethodNotAllowedIsProblem(t *testing.T) {
	s := newTestServer(t, nil)
	rec := do(t, s, http.MethodPost, "/healthz", "localhost", nil, "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("应为 405，实际 %d", rec.Code)
	}
	if p := decodeProblem(t, rec); p.Code != "method_not_allowed" {
		t.Fatalf("Problem code 错误：%+v", p)
	}
}

func TestPanicBecomesInternalErrorProblem(t *testing.T) {
	s := newTestServer(t, nil)
	s.Engine().GET("/boom", func(c *gin.Context) { panic("kaboom") })
	rec := do(t, s, http.MethodGet, "/boom", "localhost", nil, "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("panic 应为 500，实际 %d", rec.Code)
	}
	p := decodeProblem(t, rec)
	if p.Code != "internal_error" || strings.Contains(p.Detail, "kaboom") {
		t.Fatalf("panic 细节不得泄露到响应：%+v", p)
	}
}

// ---------------------------------------------------------------------------
// Host / Origin
// ---------------------------------------------------------------------------

func TestHostNotAllowedRejected(t *testing.T) {
	s := newTestServer(t, nil)
	rec := do(t, s, http.MethodGet, "/healthz", "evil.example.com", nil, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("非法 Host 应为 400，实际 %d", rec.Code)
	}
}

func TestHostAllowedWithPort(t *testing.T) {
	s := newTestServer(t, nil)
	rec := do(t, s, http.MethodGet, "/healthz", "127.0.0.1:8080", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("本机 Host:port 应放行，实际 %d", rec.Code)
	}
}

func TestCrossOriginWriteRejected(t *testing.T) {
	s := newTestServer(t, nil)
	rec := do(t, s, http.MethodPost, "/api/v1/courses", "localhost", map[string]string{
		"Origin": "https://evil.example.com",
	}, "{}")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("跨来源写请求应为 403，实际 %d", rec.Code)
	}
}

func TestSameOriginWriteAllowedThrough(t *testing.T) {
	s := newTestServer(t, nil)
	rec := do(t, s, http.MethodPost, "/api/v1/courses", "localhost", map[string]string{
		"Origin": "http://localhost:8080",
	}, "{}")
	// 路由不存在 → 404，说明通过了 Host/Origin 门禁
	if rec.Code != http.StatusNotFound {
		t.Fatalf("同源写请求应通过门禁，实际 %d", rec.Code)
	}
}

func TestCORSPreflightRejected(t *testing.T) {
	s := newTestServer(t, nil)
	rec := do(t, s, http.MethodOptions, "/api/v1/courses", "localhost", map[string]string{
		"Origin":                        "http://localhost:8080",
		"Access-Control-Request-Method": "POST",
	}, "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("CORS 预检应默认拒绝，实际 %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// BodyLimit / Timeout / 安全响应头
// ---------------------------------------------------------------------------

func TestBodyLimitExceeded(t *testing.T) {
	s := newTestServer(t, nil) // 限制 16 字节（BodyLimitBytes 配置）
	// BodyLimit 自 B3 起按路由注册（上传路由允许更大负载）。
	s.Engine().POST("/echo", BodyLimit(16, http.StatusBadRequest, "bad_request"), func(c *gin.Context) {
		if _, err := io.ReadAll(c.Request.Body); err != nil {
			_ = c.Error(err)
			return
		}
		c.Status(http.StatusOK)
	})
	rec := do(t, s, http.MethodPost, "/echo", "localhost", nil, strings.Repeat("x", 64))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("超限请求体应为 400，实际 %d", rec.Code)
	}
	if p := decodeProblem(t, rec); p.Code != "bad_request" {
		t.Fatalf("Problem code 错误：%+v", p)
	}
}

func TestTimeoutDeadlineSetForNormalRoutes(t *testing.T) {
	s := newTestServer(t, nil)
	var deadline time.Time
	var hasDeadline bool
	s.Engine().GET("/slow", func(c *gin.Context) {
		deadline, hasDeadline = c.Request.Context().Deadline()
		c.Status(http.StatusOK)
	})
	do(t, s, http.MethodGet, "/slow", "localhost", nil, "")
	if !hasDeadline || time.Until(deadline) > 50*time.Millisecond {
		t.Fatalf("普通路由应注入请求截止时间：has=%v deadline=%v", hasDeadline, deadline)
	}
}

func TestTimeoutSkippedForSSEPath(t *testing.T) {
	s := newTestServer(t, nil)
	var hasDeadline = true
	s.Engine().GET("/api/v1/jobs/stream", func(c *gin.Context) {
		_, hasDeadline = c.Request.Context().Deadline()
		c.Status(http.StatusOK)
	})
	do(t, s, http.MethodGet, "/api/v1/jobs/stream", "localhost", nil, "")
	if hasDeadline {
		t.Fatal("SSE 路由不应注入请求截止时间")
	}
}

func TestSecurityHeadersPresent(t *testing.T) {
	s := newTestServer(t, nil)
	rec := do(t, s, http.MethodGet, "/healthz", "localhost", nil, "")
	for k, v := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "no-referrer",
		"X-Frame-Options":        "DENY",
	} {
		if got := rec.Header().Get(k); got != v {
			t.Fatalf("安全响应头 %s 应为 %q，实际 %q", k, v, got)
		}
	}
}
