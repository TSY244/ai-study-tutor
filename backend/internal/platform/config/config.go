// Package config loads process configuration from environment variables and
// *_FILE secrets into an immutable Config at startup.
//
// Rules (docs/web-rebuild/03-backend-architecture.md §12):
//   - Non-sensitive values come from environment variables with defaults.
//   - Secrets are only read through *_FILE paths, never from plain env vars.
//   - Missing required entries fail fast at startup (Load returns an error).
//   - A missing/unreadable/empty ANTHROPIC_API_KEY_FILE is a capability
//     degradation (Claude not configured), never a startup failure.
//   - Secret values are never logged or included in error messages.
package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Service names accepted by Load.
const (
	ServiceAPI          = "api"
	ServiceWorker       = "worker"
	ServiceMigrate      = "migrate"
	ServiceLegacyImport = "legacy-import"
	ServiceDoctor       = "doctor"
	ServiceSeed         = "seed"
	ServiceBackup       = "backup"
	ServiceRestore      = "restore"
)

// Config is the immutable runtime configuration. Build it once via Load and
// pass it (or narrow sub-values) to consumers; tests construct it directly.
type Config struct {
	Service  string
	Env      string
	Version  string
	LogLevel string

	HTTP        HTTPConfig
	DB          DBConfig
	Worker      WorkerConfig
	Maintenance MaintenanceConfig
	Claude      ClaudeConfig
	Material    MaterialConfig

	DataDir   string
	BackupDir string

	// SiteCookiesFile 是 B 站 Cookie（Netscape 格式）的容器内路径；api 提供
	// 管理端点（写），worker 的 URL 提取与可用性检测读取（热加载，无需重启）。
	// 为空时不注册管理端点，检测任务报 bilibili_cookie_not_configured。
	SiteCookiesFile string
}

type HTTPConfig struct {
	Addr           string
	AllowedHosts   []string
	BodyLimitBytes int64
	UploadMaxBytes int64
	RequestTimeout time.Duration
	// SSEPaths are route paths exempt from the request timeout middleware.
	SSEPaths []string
}

type DBConfig struct {
	// URL is the effective PostgreSQL DSN. It is built either from
	// DATABASE_URL(_FILE) or from DB_HOST/DB_PORT/DB_NAME/DB_USER plus
	// DB_PASSWORD_FILE. It may contain a password: never log it.
	URL string
}

type WorkerConfig struct {
	ID          string
	Concurrency int
	JobLease    time.Duration
	// JobTimeout 是单个 Job 的总时限（03 §8.2 限制总任务时间）。
	JobTimeout time.Duration
}

type MaintenanceConfig struct {
	LeaseDuration     time.Duration
	HeartbeatInterval time.Duration
}

type ClaudeConfig struct {
	// apiKey is the Anthropic key read from ANTHROPIC_API_KEY_FILE.
	// Unexported so it cannot leak through %+v / slog by accident.
	apiKey         string
	KeyFile        string
	KeyFilePresent bool // file existed and was readable
	Provider       string
	// AnthropicBaseURL 是 Claude 兼容网关的非敏感基础地址；Worker 在其后
	// 追加 /v1/messages。默认指向 Anthropic 官方 API。
	AnthropicBaseURL string
	Model            string
	RequestTimeout   time.Duration
	MaxOutputTokens  int
	// GatewayConfigFile 是 Claude 多 Key 网关配置（keys.json）路径；
	// api 与 worker 都读取。为空表示未启用网关，回退到单 Key 行为。
	GatewayConfigFile string
	// GatewayWriteEnabled 仅 api 使用：开启 /system/claude-keys 写端点；
	// 默认关闭（只读）。
	GatewayWriteEnabled bool
	// ClaudeCodeSettingsFile 供 api 与 worker 共用：显式配置优先，否则解析
	// 为运行时 HOME 下的 .claude/settings.json。它不包含密钥。
	ClaudeCodeSettingsFile string
}

const defaultAnthropicBaseURL = "https://api.anthropic.com"

// AnthropicMessagesEndpoint returns the complete Messages API URL. Operators
// may provide either a base URL or the full /v1/messages endpoint.
func (c ClaudeConfig) AnthropicMessagesEndpoint() string {
	base := strings.TrimRight(strings.TrimSpace(c.AnthropicBaseURL), "/")
	if strings.HasSuffix(base, "/v1/messages") {
		return base
	}
	return base + "/v1/messages"
}

// AnthropicKey returns the configured key and whether AI calls are possible.
// Callers must never log the returned key.
func (c ClaudeConfig) AnthropicKey() (string, bool) {
	return c.apiKey, c.apiKey != ""
}

// Configured reports whether AI capability is available. When false the
// system stays up and AI-dependent operations return claude_not_configured.
func (c ClaudeConfig) Configured() bool { return c.apiKey != "" }

type MaterialConfig struct {
	MaxDownloadBytes  int64
	URLAllowlist      []string
	APIDNSValidation  bool
	TranscriptionMode string
	WhisperModelPath  string
}

// Load reads and validates configuration for the given service.
// getEnv/file reading are injectable for tests via LoadFrom.
func Load(service string) (Config, error) {
	return LoadFrom(service, os.Getenv)
}

// LoadFrom is Load with an injectable environment for deterministic tests.
func LoadFrom(service string, getEnv func(string) string) (Config, error) {
	switch service {
	case ServiceAPI, ServiceWorker, ServiceMigrate, ServiceLegacyImport, ServiceDoctor, ServiceSeed, ServiceBackup, ServiceRestore:
	default:
		return Config{}, fmt.Errorf("未知 service：%q", service)
	}

	cfg := Config{
		Service:  service,
		Env:      envOr(getEnv, "APP_ENV", "development"),
		Version:  envOr(getEnv, "APP_VERSION", "dev"),
		LogLevel: envOr(getEnv, "LOG_LEVEL", "info"),
	}

	if err := cfg.loadHTTP(getEnv); err != nil {
		return Config{}, err
	}
	if err := cfg.loadDB(getEnv); err != nil {
		return Config{}, err
	}
	if err := cfg.loadWorker(getEnv); err != nil {
		return Config{}, err
	}
	if err := cfg.loadMaintenance(getEnv); err != nil {
		return Config{}, err
	}
	if err := cfg.loadClaude(getEnv, service); err != nil {
		return Config{}, err
	}
	if err := cfg.loadMaterial(getEnv); err != nil {
		return Config{}, err
	}

	cfg.DataDir = envOr(getEnv, "APP_DATA_DIR", "./data")
	cfg.BackupDir = envOr(getEnv, "APP_BACKUP_DIR", "./backups")
	cfg.SiteCookiesFile = strings.TrimSpace(getEnv("SITE_COOKIES_FILE"))
	return cfg, nil
}

func (c *Config) loadHTTP(getEnv func(string) string) error {
	timeout, err := durationOr(getEnv, "HTTP_REQUEST_TIMEOUT", 30*time.Second)
	if err != nil {
		return err
	}
	bodyLimit, err := intOr(getEnv, "HTTP_BODY_LIMIT_BYTES", 1<<20)
	if err != nil {
		return err
	}
	uploadMax, err := intOr(getEnv, "MAX_UPLOAD_BYTES", 2<<30)
	if err != nil {
		return err
	}
	c.HTTP = HTTPConfig{
		Addr:           envOr(getEnv, "HTTP_ADDR", ":8080"),
		AllowedHosts:   csvOr(getEnv, "HTTP_ALLOWED_HOSTS", []string{"localhost", "127.0.0.1", "::1"}),
		BodyLimitBytes: bodyLimit,
		UploadMaxBytes: uploadMax,
		RequestTimeout: timeout,
		SSEPaths:       csvOr(getEnv, "HTTP_SSE_PATHS", []string{"/api/v1/jobs/stream"}),
	}
	if len(c.HTTP.AllowedHosts) == 0 {
		return fmt.Errorf("HTTP_ALLOWED_HOSTS 不能为空")
	}
	if c.HTTP.BodyLimitBytes <= 0 || c.HTTP.UploadMaxBytes <= 0 {
		return fmt.Errorf("HTTP 体积限制必须为正数")
	}
	return nil
}

func (c *Config) loadDB(getEnv func(string) string) error {
	// 优先 DATABASE_URL / DATABASE_URL_FILE；否则由 DB_* 部件拼装。
	if url, err := secretFromFileOrEnv(getEnv, "DATABASE_URL_FILE", "DATABASE_URL"); err != nil {
		return err
	} else if url != "" {
		c.DB.URL = url
		return nil
	}

	host := getEnv("DB_HOST")
	name := getEnv("DB_NAME")
	user := getEnv("DB_USER")
	if host == "" || name == "" || user == "" {
		return fmt.Errorf("数据库配置缺失：需要 DATABASE_URL(_FILE) 或 DB_HOST/DB_NAME/DB_USER（密码经 DB_PASSWORD_FILE）")
	}
	port := envOr(getEnv, "DB_PORT", "5432")
	password, err := readSecretFile(getEnv("DB_PASSWORD_FILE"))
	if err != nil {
		return fmt.Errorf("读取 DB_PASSWORD_FILE 失败")
	}
	c.DB.URL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		percentEncode(user), percentEncode(password), host, port, name)
	return nil
}

func (c *Config) loadWorker(getEnv func(string) string) error {
	concurrency, err := intOr(getEnv, "WORKER_CONCURRENCY", 1)
	if err != nil {
		return err
	}
	lease, err := durationOr(getEnv, "JOB_LEASE_DURATION", 60*time.Second)
	if err != nil {
		return err
	}
	jobTimeout, err := durationOr(getEnv, "WORKER_JOB_TIMEOUT", 30*time.Minute)
	if err != nil {
		return err
	}
	if concurrency < 1 {
		return fmt.Errorf("WORKER_CONCURRENCY 必须 >= 1")
	}
	id := getEnv("WORKER_ID")
	if id == "" {
		hostname, _ := os.Hostname()
		id = fmt.Sprintf("worker-%s-%d", hostname, os.Getpid())
	}
	c.Worker = WorkerConfig{ID: id, Concurrency: int(concurrency), JobLease: lease, JobTimeout: jobTimeout}
	return nil
}

func (c *Config) loadMaintenance(getEnv func(string) string) error {
	lease, err := durationOr(getEnv, "MAINTENANCE_LEASE_DURATION", 45*time.Second)
	if err != nil {
		return err
	}
	heartbeat, err := durationOr(getEnv, "MAINTENANCE_HEARTBEAT_INTERVAL", 10*time.Second)
	if err != nil {
		return err
	}
	if lease <= 0 || heartbeat <= 0 || heartbeat >= lease {
		return fmt.Errorf("维护租约配置不合法：heartbeat 必须小于 lease 且均为正")
	}
	c.Maintenance = MaintenanceConfig{LeaseDuration: lease, HeartbeatInterval: heartbeat}
	return nil
}

func (c *Config) loadClaude(getEnv func(string) string, service string) error {
	c.Claude.Provider = envOr(getEnv, "AI_PROVIDER", "anthropic")
	switch c.Claude.Provider {
	case "anthropic", "fake":
	default:
		return fmt.Errorf("AI_PROVIDER 只支持 anthropic/fake")
	}
	if service == ServiceWorker && c.Claude.Provider == "anthropic" {
		c.Claude.KeyFile = getEnv("ANTHROPIC_API_KEY_FILE")
		key, err := readSecretFile(c.Claude.KeyFile)
		if err == nil && key != "" {
			c.Claude.apiKey = key
			c.Claude.KeyFilePresent = true
		}
	}
	baseURL := strings.TrimRight(envOr(getEnv, "ANTHROPIC_BASE_URL", defaultAnthropicBaseURL), "/")
	parsedBaseURL, err := url.ParseRequestURI(baseURL)
	if err != nil || parsedBaseURL.Scheme != "https" || parsedBaseURL.Host == "" {
		return fmt.Errorf("ANTHROPIC_BASE_URL 必须是有效的 https URL")
	}
	c.Claude.AnthropicBaseURL = baseURL
	c.Claude.Model = getEnv("CLAUDE_MODEL_DEFAULT")
	if timeout, err := durationOr(getEnv, "CLAUDE_REQUEST_TIMEOUT", 300*time.Second); err == nil {
		c.Claude.RequestTimeout = timeout
	}
	if tokens, err := intOr(getEnv, "CLAUDE_MAX_OUTPUT_TOKENS", 8192); err == nil {
		c.Claude.MaxOutputTokens = int(tokens)
	}
	c.Claude.GatewayConfigFile = strings.TrimSpace(getEnv("CLAUDE_GATEWAY_CONFIG_FILE"))
	writeEnabled, err := boolOr(getEnv, "CLAUDE_GATEWAY_WRITE_ENABLED", false)
	if err != nil {
		return err
	}
	c.Claude.GatewayWriteEnabled = writeEnabled
	settingsFile := strings.TrimSpace(getEnv("CLAUDE_CODE_SETTINGS_FILE"))
	if settingsFile == "" && (service == ServiceAPI || service == ServiceWorker) {
		if runtimeHome := strings.TrimSpace(getEnv("HOME")); runtimeHome != "" {
			settingsFile = filepath.Join(runtimeHome, ".claude", "settings.json")
		}
	}
	c.Claude.ClaudeCodeSettingsFile = settingsFile
	return nil
}

func (c *Config) loadMaterial(getEnv func(string) string) error {
	maxDownload, err := intOr(getEnv, "MAX_DOWNLOAD_BYTES", 2<<30)
	if err != nil {
		return err
	}
	apiDNSValidation, err := boolOr(getEnv, "URL_API_DNS_VALIDATION", true)
	if err != nil {
		return err
	}
	c.Material = MaterialConfig{
		MaxDownloadBytes:  maxDownload,
		URLAllowlist:      csvOr(getEnv, "URL_ALLOWLIST", nil),
		APIDNSValidation:  apiDNSValidation,
		TranscriptionMode: envOr(getEnv, "TRANSCRIPTION_MODE", "subtitle_only"),
		WhisperModelPath:  getEnv("WHISPER_MODEL_PATH"),
	}
	switch c.Material.TranscriptionMode {
	case "subtitle_only", "whisper_local":
	default:
		return fmt.Errorf("TRANSCRIPTION_MODE 只支持 subtitle_only/whisper_local")
	}
	return nil
}

// SafeSummary returns non-secret fields for startup logging. Secret values
// are represented only by a configured/not_configured boolean.
func (c Config) SafeSummary() map[string]any {
	_, claudeConfigured := c.Claude.AnthropicKey()
	return map[string]any{
		"service":                         c.Service,
		"env":                             c.Env,
		"version":                         c.Version,
		"http_addr":                       c.HTTP.Addr,
		"db_configured":                   c.DB.URL != "",
		"data_dir":                        c.DataDir,
		"worker_id":                       c.Worker.ID,
		"worker_concurrency":              c.Worker.Concurrency,
		"claude_configured":               claudeConfigured,
		"claude_provider":                 c.Claude.Provider,
		"claude_model":                    c.Claude.Model,
		"claude_gateway_configured":       c.Claude.GatewayConfigFile != "",
		"claude_gateway_write_enabled":    c.Claude.GatewayWriteEnabled,
		"claude_code_settings_configured": c.Claude.ClaudeCodeSettingsFile != "",
		"site_cookies_configured":         c.SiteCookiesFile != "",
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func envOr(getEnv func(string) string, key, fallback string) string {
	if v := strings.TrimSpace(getEnv(key)); v != "" {
		return v
	}
	return fallback
}

func csvOr(getEnv func(string) string, key string, fallback []string) []string {
	v := strings.TrimSpace(getEnv(key))
	if v == "" {
		return fallback
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func intOr(getEnv func(string) string, key string, fallback int64) (int64, error) {
	v := strings.TrimSpace(getEnv(key))
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s 不是合法整数", key)
	}
	return n, nil
}

func durationOr(getEnv func(string) string, key string, fallback time.Duration) (time.Duration, error) {
	v := strings.TrimSpace(getEnv(key))
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s 不是合法时长（如 60s/5m）", key)
	}
	return d, nil
}

func boolOr(getEnv func(string) string, key string, fallback bool) (bool, error) {
	v := strings.TrimSpace(getEnv(key))
	if v == "" {
		return fallback, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s 不是合法布尔值（true/false）", key)
	}
	return b, nil
}

// secretFromFileOrEnv prefers the *_FILE variant; the plain env var is only
// accepted for DATABASE_URL because Compose/本地开发都把它当作连接串载体。
// 返回空串表示未配置。
func secretFromFileOrEnv(getEnv func(string) string, fileKey, envKey string) (string, error) {
	if path := strings.TrimSpace(getEnv(fileKey)); path != "" {
		secret, err := readSecretFile(path)
		if err != nil {
			return "", fmt.Errorf("读取 %s 失败", fileKey)
		}
		return secret, nil
	}
	return strings.TrimSpace(getEnv(envKey)), nil
}

// readSecretFile reads a secret file, trimming a single trailing newline.
// 错误信息不含文件内容。空路径/空文件返回空串（能力未配置）。
func readSecretFile(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(data), "\r\n"), nil
}

func percentEncode(s string) string {
	r := strings.NewReplacer(
		"%", "%25", "@", "%40", ":", "%3A", "/", "%2F",
		"?", "%3F", "#", "%23", "&", "%26", "=", "%3D", "+", "%2B",
	)
	return r.Replace(s)
}
