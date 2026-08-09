// API 进程入口：Gin HTTP Server（REST/SSE/上传/查询）。
//
// 用法：
//
//	api               启动 HTTP 服务（SIGINT/SIGTERM 优雅退出）
//	api healthcheck   容器健康检查：请求本机 /healthz，成功退出码 0
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/backend/internal/api"
	"github.com/TSY244/ai-study-tutor/backend/internal/claudegateway"
	"github.com/TSY244/ai-study-tutor/backend/internal/filestore"
	"github.com/TSY244/ai-study-tutor/backend/internal/job"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/config"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/database"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/httpserver"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/id"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/logging"
	"github.com/TSY244/ai-study-tutor/backend/internal/sitecookie"
	"github.com/TSY244/ai-study-tutor/backend/internal/system"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(healthcheck())
	}

	cfg, err := config.Load(config.ServiceAPI)
	if err != nil {
		fatal(err)
	}
	logger := logging.New(cfg.Service, cfg.Version, cfg.LogLevel)
	logger.Info("starting", slog.Any("config", cfg.SafeSummary()))

	// 惰性连接池：数据库不可达不阻止启动，由 /readyz 报告依赖状态。
	pool, err := database.Connect(context.Background(), cfg.DB.URL)
	if err != nil {
		fatal(err)
	}
	defer pool.Close()

	// 两阶段发布的文件存储（temporary 24h TTL，与 worker 一致）。
	files, err := filestore.New(pool, cfg.DataDir, 24*time.Hour)
	if err != nil {
		fatal(err)
	}

	server := httpserver.New(httpserver.Deps{
		Config:     cfg.HTTP,
		Logger:     logger,
		IDGen:      id.Random{},
		Production: cfg.Env == "production",
		ReadyChecks: map[string]database.Checker{
			"database": database.PingChecker{Pool: pool},
		},
	})

	// Claude 多 Key 网关：仅在配置 CLAUDE_GATEWAY_CONFIG_FILE 时注册管理端点。
	// API 进程看不到 Anthropic secret；legacy_fallback 由 Worker 心跳推导。
	claudeGateway := buildClaudeGatewayHandler(cfg, pool, logger)

	// B 站 Cookie 管理：仅在配置 SITE_COOKIES_FILE 时注册管理端点。
	// API 写共享挂载的 Cookie 文件；可用性检测由 Worker 任务执行（API 无 egress）。
	var bilibiliCookie *sitecookie.Handler
	if cfg.SiteCookiesFile != "" {
		bilibiliCookie = &sitecookie.Handler{
			Store: sitecookie.NewStore(cfg.SiteCookiesFile),
			Jobs:  job.NewService(pool),
		}
	}

	// 注册 /api/v1 业务路由（B3 主链路：课程/课时/素材/文档/出题/导出）。
	api.RegisterRoutes(server.Engine(), api.Deps{
		Pool:                 pool,
		Files:                files,
		Version:              cfg.Version,
		URLAllowlist:         cfg.Material.URLAllowlist,
		SkipURLDNSValidation: !cfg.Material.APIDNSValidation,
		BodyLimitBytes:       cfg.HTTP.BodyLimitBytes,
		UploadMaxBytes:       cfg.HTTP.UploadMaxBytes,
		ClaudeGateway:        claudeGateway,
		BilibiliCookie:       bilibiliCookie,
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := server.Run(ctx, cfg.HTTP.Addr); err != nil {
		logger.Error("http_server_failed", slog.String("error_code", "server_error"))
		os.Exit(1)
	}
}

func buildClaudeGatewayHandler(cfg config.Config, pool *pgxpool.Pool, logger *slog.Logger) *claudegateway.Handler {
	if cfg.Claude.GatewayConfigFile == "" {
		return nil
	}
	statusSvc := system.NewStatusService(pool, cfg.DataDir, cfg.Version)
	return &claudegateway.Handler{
		Pool:                   pool,
		Store:                  claudegateway.NewStore(cfg.Claude.GatewayConfigFile),
		State:                  claudegateway.NewStateTracker(claudegateway.StatePathFor(cfg.Claude.GatewayConfigFile), logger, cfg.Claude.AnthropicBaseURL),
		Jobs:                   job.NewService(pool),
		WriteEnabled:           cfg.Claude.GatewayWriteEnabled,
		LegacyFallback:         statusSvc.WorkerClaudeConfigured,
		ClaudeCodeSettingsFile: cfg.Claude.ClaudeCodeSettingsFile,
		DefaultBaseURL:         cfg.Claude.AnthropicBaseURL,
	}
}

// healthcheck 不依赖完整配置校验，直接读 HTTP_ADDR 探测本机 /healthz。
func healthcheck() int {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + addr + "/healthz")
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "api 启动失败：%v\n", err)
	os.Exit(1)
}
