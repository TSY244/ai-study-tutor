// Package httpserver builds the Gin HTTP server for the API process:
// health/readiness probes, the middleware chain and Problem Details error
// mapping. Business routes are registered by feature modules (B3+) via
// Server.Engine; this package only owns cross-cutting HTTP concerns.
package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/TSY244/ai-study-tutor/backend/internal/platform/config"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/database"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/id"
)

// Server wraps the Gin engine and the underlying http.Server.
type Server struct {
	engine     *gin.Engine
	httpServer *http.Server
	logger     *slog.Logger
}

// Deps are the cross-cutting dependencies of the HTTP layer. Everything is
// injected so tests can substitute fakes (no real database, fixed IDs).
type Deps struct {
	Config     config.HTTPConfig
	Logger     *slog.Logger
	IDGen      id.Generator
	Production bool
	// ReadyChecks maps dependency name ("database", "data_dir") to its
	// probe; /readyz fails when any probe errors. An empty map means the
	// process has no hard dependencies and is always ready.
	ReadyChecks map[string]database.Checker
}

// New builds the engine with the full middleware chain and core routes.
func New(d Deps) *Server {
	if d.Production {
		gin.SetMode(gin.ReleaseMode)
	}
	engine := gin.New()
	engine.HandleMethodNotAllowed = true
	// 本机部署只信任回环代理（Caddy）。
	_ = engine.SetTrustedProxies([]string{"127.0.0.1", "::1"})

	engine.Use(RequestID(d.IDGen))
	engine.Use(AccessLog(d.Logger))
	engine.Use(Recovery(d.Logger))
	engine.Use(SecurityHeaders())
	engine.Use(HostOrigin(d.Config.AllowedHosts))
	// Body 大小限制按路由注册（上传路由允许更大负载），
	// 由功能模块在注册路由时附加 BodyLimit 中间件（B3+）。
	engine.Use(Timeout(d.Config.RequestTimeout, d.Config.SSEPaths))

	s := &Server{engine: engine, logger: d.Logger}
	s.registerCoreRoutes(d.ReadyChecks)
	return s
}

// Engine exposes the Gin engine so feature modules can register routes.
// Gin 类型只允许出现在 handler 层；domain/application 不得引用。
func (s *Server) Engine() *gin.Engine { return s.engine }

// Handler returns the root handler for httptest.
func (s *Server) Handler() http.Handler { return s.engine }

func (s *Server) registerCoreRoutes(checks map[string]database.Checker) {
	// /healthz：进程存活，不检查任何依赖（契约：始终 200）。
	s.engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// /readyz：必需依赖可用；任一失败返回 503 Problem。
	s.engine.GET("/readyz", func(c *gin.Context) {
		failed := make([]string, 0, len(checks))
		for name, check := range checks {
			if err := check.Check(c.Request.Context()); err != nil {
				failed = append(failed, name)
				s.logger.WarnContext(c.Request.Context(), "readiness_check_failed",
					slog.String("dependency", name),
					slog.String("error_code", "dependency_unavailable"),
				)
			}
		}
		if len(failed) > 0 {
			AbortWithProblem(c, http.StatusServiceUnavailable, "service_unavailable",
				"依赖不可用", "必需依赖暂不可用："+joinNames(failed))
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	s.engine.NoRoute(func(c *gin.Context) {
		AbortWithProblem(c, http.StatusNotFound, "not_found", "资源不存在", "请求的资源不存在。")
	})
	s.engine.NoMethod(func(c *gin.Context) {
		AbortWithProblem(c, http.StatusMethodNotAllowed, "method_not_allowed", "方法不允许", "该资源不支持此 HTTP 方法。")
	})
}

func joinNames(names []string) string {
	out := ""
	for i, n := range names {
		if i > 0 {
			out += ", "
		}
		out += n
	}
	return out
}

// Run serves until the context is canceled (SIGINT/SIGTERM from the caller)
// and then shuts down gracefully, waiting for in-flight requests.
func (s *Server) Run(ctx context.Context, addr string) error {
	s.httpServer = &http.Server{
		Addr:              addr,
		Handler:           s.engine,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("http_listen", slog.String("addr", addr))
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		return err
	}
	s.logger.Info("http_stopped")
	return nil
}
