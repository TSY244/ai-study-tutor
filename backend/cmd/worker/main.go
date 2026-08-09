// Worker 进程入口：PostgreSQL 任务队列消费者（素材解析、Claude 调用、发布）。
//
// B5：Job 领取循环 + lesson_import 素材管线（文本/文档/URL 字幕优先、
// 标准化与 raw_text 文档发布）。Claude 相关 Job kind 在 B6 注册。
//
// 用法：
//
//	worker               启动 Worker（SIGINT/SIGTERM 优雅退出）
//	worker healthcheck   容器健康检查：数据库可达且数据目录可写
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	aipkg "github.com/TSY244/ai-study-tutor/backend/internal/ai"
	"github.com/TSY244/ai-study-tutor/backend/internal/claudegateway"
	"github.com/TSY244/ai-study-tutor/backend/internal/filestore"
	"github.com/TSY244/ai-study-tutor/backend/internal/idempotency"
	"github.com/TSY244/ai-study-tutor/backend/internal/job"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/config"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/database"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/logging"
	"github.com/TSY244/ai-study-tutor/backend/internal/sitecookie"
	"github.com/TSY244/ai-study-tutor/backend/internal/system"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker/aihandlers"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker/cookiecheck"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker/lessonimport"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker/proc"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(healthcheck())
	}

	cfg, err := config.Load(config.ServiceWorker)
	if err != nil {
		fatal(err)
	}
	logger := logging.New(cfg.Service, cfg.Version, cfg.LogLevel)
	logger.Info("starting", slog.Any("config", cfg.SafeSummary()))

	// Claude 是能力降级：Key 缺失只禁用 AI Job，不影响进程启动。
	anthropicKey, claudeOK := cfg.Claude.AnthropicKey()
	providers := buildClaudeProviders(cfg, anthropicKey, claudeOK, logger)
	gateway := providers.Gateway
	gatewayKeys := providers.GatewayEnabledKeys

	switch {
	case cfg.Claude.Provider == "fake":
		logger.Warn("claude_capability", slog.String("status", "fake_local"))
	case gateway != nil && gatewayKeys > 0:
		logger.Info("claude_capability",
			slog.String("status", "gateway_configured"),
			slog.Int("enabled_keys", gatewayKeys),
		)
	case claudeOK:
		logger.Info("claude_capability", slog.String("status", "configured"))
	default:
		logger.Warn("claude_capability",
			slog.String("status", "not_configured"),
			slog.String("error_code", "claude_not_configured"),
		)
	}
	aiProvider := providers.AI
	lessonImportAI := providers.LessonImportAI

	if err := ensureDataDirWritable(cfg.DataDir); err != nil {
		logger.Warn("data_dir_not_writable",
			slog.String("data_dir", cfg.DataDir),
			slog.String("error_code", "storage_unavailable"),
		)
	}

	pool, err := database.Connect(context.Background(), cfg.DB.URL)
	if err != nil {
		fatal(err)
	}
	defer pool.Close()

	logger.Info("worker_started",
		slog.String("worker_id", cfg.Worker.ID),
		slog.Int("concurrency", cfg.Worker.Concurrency),
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	heartbeatDone := make(chan struct{})
	go runSystemHeartbeatLoop(ctx, logger, pool, workerHeartbeatSource(cfg, claudeOK, gateway), heartbeatDone)

	files, err := filestore.New(pool, cfg.DataDir, 24*time.Hour)
	if err != nil {
		fatal(err)
	}

	// B6 Job Handler registry：素材管线 + Worker-only Claude 任务。
	registry := worker.NewRegistry()
	registry.Register(job.KindLessonImport, &lessonimport.Handler{
		Pool:               pool,
		Files:              files,
		Proc:               proc.ExecRunner{},
		AI:                 lessonImportAI,
		AIAvailable:        providers.LessonImportAvailable,
		URLImportAI:        providers.URLImportAI,
		URLImportAvailable: providers.URLImportAvailable,
		URLExtractor:       providers.URLExtractor,
		URLAllowlist:       cfg.Material.URLAllowlist,
		MaxDownloadBytes:   cfg.Material.MaxDownloadBytes,
		TranscriptionMode:  cfg.Material.TranscriptionMode,
		WhisperModelPath:   cfg.Material.WhisperModelPath,
	})
	aihandlers.Register(registry, aihandlers.Deps{
		Pool:     pool,
		Provider: aiProvider,
		QuizCLI:  providers.QuizCLI,
	})
	// B 站 Cookie 可用性检测：每次执行实时读取 Cookie 文件（热加载）。
	registry.Register(job.KindBilibiliCookieTest, &cookiecheck.Handler{
		Store:   sitecookie.NewStore(cfg.SiteCookiesFile),
		Checker: sitecookie.NewChecker(nil),
	})
	runner := &worker.Runner{
		Pool:       pool,
		Registry:   registry,
		Files:      files,
		WorkerID:   cfg.Worker.ID,
		Lease:      cfg.Worker.JobLease,
		JobTimeout: cfg.Worker.JobTimeout,
		Logger:     logger,
	}

	var wg sync.WaitGroup
	for i := 0; i < cfg.Worker.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runner.Run(ctx)
		}()
	}

	// 周期 Recovery：回收过期 Job 租约（worker_lost）、过期 temporary
	// 文件对象与幂等记录。
	recoveryDone := make(chan struct{})
	go runRecoveryLoop(ctx, logger, pool, files, recoveryDone)

	<-ctx.Done()
	logger.Info("worker_stopping")
	wg.Wait()
	<-recoveryDone
	<-heartbeatDone
	logger.Info("worker_stopped")
}

type claudeProviders struct {
	Gateway               *claudegateway.Router
	GatewayEnabledKeys    int
	AI                    aipkg.Provider
	LessonImportAI        aipkg.Provider
	LessonImportAvailable func() bool
	// URLImportAI/URLImportAvailable 仅在存在网关配置时装配：URL 导入只用
	// 网关启用 Key（GatewayOnly，无 legacy 回退）；legacy 直连 Key 或
	// fake/非 Anthropic Provider 永不是 URL 导入替代品（字段保持 nil）。
	URLImportAI        aipkg.Provider
	URLImportAvailable func() bool
	// URLExtractor 是 URL 素材的 Claude Code CLI 提取器；除网关配置外还
	// 要求有效的运行时 HOME/settings 路径对。
	URLExtractor lessonimport.URLExtractor
	// QuizCLI 是测验生成的 Claude Code CLI 生成器（与 URLExtractor 同条件
	// 装配；运行时选择同样从 settings.json/网关 keys.json 解析）。
	QuizCLI aihandlers.QuizGenerator
}

// buildClaudeProviders 是 Worker 的唯一 Claude Provider 装配入口。网关
// Store 在每次调用/能力检查时热重载，因此运行期新增 Key 无需重启 Worker。
func buildClaudeProviders(cfg config.Config, anthropicKey string, claudeOK bool, logger *slog.Logger) claudeProviders {
	providers := claudeProviders{AI: aipkg.NotConfiguredProvider{}}
	if cfg.Claude.Provider == "fake" {
		fake := aipkg.NewDeterministicFakeProvider()
		providers.AI = fake
		providers.LessonImportAI = fake
		return providers
	}
	if cfg.Claude.Provider != "anthropic" {
		return providers
	}

	var legacy aipkg.Provider = aipkg.NotConfiguredProvider{}
	if claudeOK {
		legacy = aipkg.NewAnthropicProvider(aipkg.AnthropicConfig{
			APIKey:          anthropicKey,
			Model:           cfg.Claude.Model,
			Endpoint:        cfg.Claude.AnthropicMessagesEndpoint(),
			MaxOutputTokens: cfg.Claude.MaxOutputTokens,
			Timeout:         cfg.Claude.RequestTimeout,
		})
	}
	if cfg.Claude.GatewayConfigFile != "" {
		runtimeHome, runtimeSettingsFile, runtimeOK := claudeCLIRuntimePaths(cfg)
		gateway := &claudegateway.Router{
			Store:           claudegateway.NewStore(cfg.Claude.GatewayConfigFile),
			State:           claudegateway.NewStateTracker(claudegateway.StatePathFor(cfg.Claude.GatewayConfigFile), logger, cfg.Claude.AnthropicBaseURL),
			Legacy:          legacy,
			Endpoint:        cfg.Claude.AnthropicMessagesEndpoint(),
			DefaultModel:    cfg.Claude.Model,
			MaxOutputTokens: cfg.Claude.MaxOutputTokens,
			Logger:          logger,
		}
		providers.Gateway = gateway
		providers.GatewayEnabledKeys = gateway.EnabledKeyCount()
		providers.AI = gateway
		providers.LessonImportAI = gateway
		providers.LessonImportAvailable = func() bool {
			return claudeOK || gateway.EnabledKeyCount() > 0
		}
		providers.URLImportAI = gateway.GatewayOnly()
		providers.URLImportAvailable = func() bool {
			return gateway.EnabledKeyCount() > 0
		}
		// URL 提取走 worker 容器内 Claude Code CLI（WebFetch 客户端抓取）：
		// 凭证由已应用 settings.json 选择（缺失时按网关优先级回退），
		// HOME 使用进程运行时 HOME，空值才回退可写数据目录。
		if runtimeOK {
			providers.URLExtractor = &lessonimport.ClaudeURLCLIExtractor{
				Proc:         proc.ExecRunner{},
				KeysFile:     cfg.Claude.GatewayConfigFile,
				SettingsFile: runtimeSettingsFile,
				BaseURL:      cfg.Claude.AnthropicBaseURL,
				Home:         runtimeHome,
				ClaudeBin:    "claude",
			}
			// 测验生成走 worker 容器内 Claude Code CLI 文件契约（资料落盘、
			// agent 自读、quiz.json 产出），绕开单响应 max_tokens 截断。
			providers.QuizCLI = &aihandlers.ClaudeQuizCLIGenerator{
				Proc:         proc.ExecRunner{},
				KeysFile:     cfg.Claude.GatewayConfigFile,
				SettingsFile: runtimeSettingsFile,
				BaseURL:      cfg.Claude.AnthropicBaseURL,
				Home:         runtimeHome,
				ClaudeBin:    "claude",
			}
		}
		return providers
	}
	if claudeOK {
		providers.AI = legacy
		providers.LessonImportAI = legacy
	}
	return providers
}

func claudeCLIRuntimePaths(cfg config.Config) (home, settingsFile string, ok bool) {
	home = strings.TrimSpace(os.Getenv("HOME"))
	settingsFile = strings.TrimSpace(cfg.Claude.ClaudeCodeSettingsFile)
	if home == "" {
		home = strings.TrimSpace(cfg.DataDir)
	}
	if home == "" {
		return "", "", false
	}
	absoluteHome, err := filepath.Abs(home)
	if err != nil || strings.TrimSpace(absoluteHome) == "" {
		return "", "", false
	}
	if settingsFile == "" {
		settingsFile = filepath.Join(absoluteHome, ".claude", "settings.json")
	}
	return absoluteHome, settingsFile, true
}

// recoveryInterval 是周期 Recovery 的间隔（短于默认 60s Job 租约，
// 保证过期租约在一个租约周期内被回收）。
const recoveryInterval = 15 * time.Second

const workerHeartbeatInterval = 10 * time.Second

// workerHeartbeat 构造非敏感能力信号；状态词汇保持
// ok/degraded/unavailable/unknown 不变，仅在消息中区分「网关已配置」。
func workerHeartbeat(cfg config.Config, keyConfigured bool, gatewayEnabledKeys int) system.WorkerHeartbeat {
	heartbeat := system.WorkerHeartbeat{
		WorkerID:     cfg.Worker.ID,
		ClaudeStatus: "unknown",
		ClaudeModel:  cfg.Claude.Model,
	}
	switch {
	case cfg.Claude.Provider == "fake":
		heartbeat.ClaudeStatus = "degraded"
		heartbeat.ClaudeMessage = "当前部署使用 fake Provider，尚未验证真实 Claude。"
	case !keyConfigured && gatewayEnabledKeys == 0:
		heartbeat.ClaudeStatus = "unavailable"
		heartbeat.ClaudeMessage = "Claude Key 未配置；请在部署侧配置 ANTHROPIC_API_KEY_FILE 后重试。"
	case gatewayEnabledKeys > 0:
		heartbeat.ClaudeMessage = fmt.Sprintf("Claude 网关已配置（%d 个启用 Key），尚未执行连通性测试。", gatewayEnabledKeys)
	case cfg.Claude.Model == "":
		heartbeat.ClaudeStatus = "unavailable"
		heartbeat.ClaudeMessage = "Claude 模型未配置；请设置 CLAUDE_MODEL_DEFAULT 后重试。"
	default:
		heartbeat.ClaudeMessage = "Claude 已配置，尚未执行连通性测试。"
	}
	return heartbeat
}

// workerHeartbeatSource 在每次心跳前读取网关当前启用 Key 数。Store 会热重载
// keys.json，因此运行期新增、停用或删除 Key 不会让系统状态滞留在旧能力状态。
func workerHeartbeatSource(cfg config.Config, keyConfigured bool, gateway *claudegateway.Router) func() system.WorkerHeartbeat {
	return func() system.WorkerHeartbeat {
		gatewayEnabledKeys := 0
		if gateway != nil {
			gatewayEnabledKeys = gateway.EnabledKeyCount()
		}
		return workerHeartbeat(cfg, keyConfigured, gatewayEnabledKeys)
	}
}

func runSystemHeartbeatLoop(ctx context.Context, logger *slog.Logger, pool *pgxpool.Pool, heartbeat func() system.WorkerHeartbeat, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(workerHeartbeatInterval)
	defer ticker.Stop()
	for {
		writeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err := system.UpsertWorkerHeartbeat(writeCtx, pool, heartbeat())
		cancel()
		if err != nil && ctx.Err() == nil {
			logger.Warn("worker_heartbeat_failed", slog.String("error", err.Error()))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func runRecoveryLoop(ctx context.Context, logger *slog.Logger, pool *pgxpool.Pool, files *filestore.Store, done chan<- struct{}) {
	defer close(done)
	jobRepo := &job.Repository{}
	idemSvc := idempotency.NewService(pool)
	ticker := time.NewTicker(recoveryInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		recoverCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		tx, err := pool.Begin(recoverCtx)
		if err != nil {
			logger.Warn("recovery_begin_failed", slog.String("error", err.Error()))
			cancel()
			continue
		}
		requeued, failed, cancelled, err := jobRepo.RecoverExpiredLeases(recoverCtx, tx, 100)
		if err == nil {
			var filesRes filestore.RecoverResult
			filesRes, err = files.Recover(recoverCtx, tx, 100)
			if err == nil && (requeued+failed+cancelled > 0 || filesRes.Completed+filesRes.Quarantined+filesRes.Deleted+filesRes.Orphaned > 0) {
				logger.Info("lease_recovery",
					slog.Int("jobs_requeued", requeued),
					slog.Int("jobs_failed", failed),
					slog.Int("jobs_cancelled", cancelled),
					slog.Int("files_completed", filesRes.Completed),
					slog.Int("files_quarantined", filesRes.Quarantined),
					slog.Int("files_deleted", filesRes.Deleted),
					slog.Int("files_orphaned", filesRes.Orphaned))
			}
		}
		if err != nil {
			logger.Warn("lease_recovery_failed", slog.String("error", err.Error()))
			_ = tx.Rollback(recoverCtx)
		} else if err := tx.Commit(recoverCtx); err != nil {
			logger.Warn("lease_recovery_commit_failed", slog.String("error", err.Error()))
		}
		if _, err := idemSvc.RecoverInProgress(recoverCtx, time.Hour); err != nil {
			logger.Warn("idempotency_recovery_failed", slog.String("error", err.Error()))
		}
		if _, err := idemSvc.CleanupExpired(recoverCtx); err != nil {
			logger.Warn("idempotency_cleanup_failed", slog.String("error", err.Error()))
		}
		cancel()
	}
}

// healthcheck：数据库可达 + 数据目录可写，全部通过才退出 0。
func healthcheck() int {
	cfg, err := config.Load(config.ServiceWorker)
	if err != nil {
		return 1
	}
	if err := ensureDataDirWritable(cfg.DataDir); err != nil {
		return 1
	}
	pool, err := database.Connect(context.Background(), cfg.DB.URL)
	if err != nil {
		return 1
	}
	defer pool.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		return 1
	}
	return 0
}

// ensureDataDirWritable 确认数据目录存在且可写（通过临时文件探测）。
func ensureDataDirWritable(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建数据目录失败")
	}
	probe, err := os.CreateTemp(dir, ".write-probe-*")
	if err != nil {
		return fmt.Errorf("数据目录不可写")
	}
	name := probe.Name()
	_ = probe.Close()
	_ = os.Remove(name)
	return nil
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "worker 启动失败：%v\n", err)
	os.Exit(1)
}
