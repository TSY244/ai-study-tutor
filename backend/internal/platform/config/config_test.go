package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func env(pairs ...string) func(string) string {
	m := map[string]string{}
	for i := 0; i+1 < len(pairs); i += 2 {
		m[pairs[i]] = pairs[i+1]
	}
	return func(k string) string { return m[k] }
}

func writeSecret(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadRequiresDatabaseConfig(t *testing.T) {
	_, err := LoadFrom(ServiceAPI, env())
	if err == nil || !strings.Contains(err.Error(), "数据库配置缺失") {
		t.Fatalf("期望数据库配置缺失错误，实际：%v", err)
	}
}

func TestLoadRejectsUnknownService(t *testing.T) {
	if _, err := LoadFrom("web", env()); err == nil {
		t.Fatal("未知 service 应失败")
	}
}

func TestLoadDatabaseFromParts(t *testing.T) {
	cfg, err := LoadFrom(ServiceAPI, env(
		"DB_HOST", "db", "DB_NAME", "tutor", "DB_USER", "tutor",
		"DB_PASSWORD_FILE", writeSecret(t, "p@ss/w:rd\n"),
	))
	if err != nil {
		t.Fatal(err)
	}
	want := "postgres://tutor:p%40ss%2Fw%3Ard@db:5432/tutor"
	if cfg.DB.URL != want {
		t.Fatalf("DSN 拼装错误：got %q want %q", cfg.DB.URL, want)
	}
}

func TestDatabaseURLFileTakesPrecedence(t *testing.T) {
	cfg, err := LoadFrom(ServiceAPI, env(
		"DATABASE_URL_FILE", writeSecret(t, "postgres://u:x@db:5432/t\n"),
		"DB_HOST", "ignored", "DB_NAME", "ignored", "DB_USER", "ignored",
	))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DB.URL != "postgres://u:x@db:5432/t" {
		t.Fatalf("DATABASE_URL_FILE 未生效：%q", cfg.DB.URL)
	}
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := LoadFrom(ServiceWorker, env("DATABASE_URL", "postgres://u:x@db/t"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTP.Addr != ":8080" || cfg.HTTP.RequestTimeout.Seconds() != 30 {
		t.Fatalf("HTTP 默认值错误：%+v", cfg.HTTP)
	}
	if cfg.Worker.Concurrency != 1 || cfg.Worker.JobLease.Seconds() != 60 {
		t.Fatalf("Worker 默认值错误：%+v", cfg.Worker)
	}
	if cfg.Maintenance.LeaseDuration.Seconds() != 45 || cfg.Maintenance.HeartbeatInterval.Seconds() != 10 {
		t.Fatalf("Maintenance 默认值错误：%+v", cfg.Maintenance)
	}
	if !strings.HasPrefix(cfg.Worker.ID, "worker-") {
		t.Fatalf("WORKER_ID 默认应自动生成：%q", cfg.Worker.ID)
	}
	if cfg.Material.TranscriptionMode != "subtitle_only" {
		t.Fatalf("转写模式默认值错误：%q", cfg.Material.TranscriptionMode)
	}
	if !cfg.Material.APIDNSValidation {
		t.Fatal("API URL DNS validation should default to enabled")
	}
}

func TestLoadMaterialURLPolicy(t *testing.T) {
	cfg, err := LoadFrom(ServiceAPI, env(
		"DATABASE_URL", "postgres://u:x@db/t",
		"URL_ALLOWLIST", "youtube.com, bilibili.com",
		"URL_API_DNS_VALIDATION", "false",
	))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(cfg.Material.URLAllowlist, ","); got != "youtube.com,bilibili.com" {
		t.Fatalf("URL allowlist = %q", got)
	}
	if cfg.Material.APIDNSValidation {
		t.Fatal("URL_API_DNS_VALIDATION=false was ignored")
	}
}

func TestClaudeKeyMissingIsDegradationNotError(t *testing.T) {
	// 文件不存在
	cfg, err := LoadFrom(ServiceWorker, env(
		"DATABASE_URL", "postgres://u:x@db/t",
		"ANTHROPIC_API_KEY_FILE", "/nonexistent/key.txt",
	))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Claude.Configured() {
		t.Fatal("Key 文件不存在时应未配置")
	}

	// 空文件
	cfg, err = LoadFrom(ServiceWorker, env(
		"DATABASE_URL", "postgres://u:x@db/t",
		"ANTHROPIC_API_KEY_FILE", writeSecret(t, ""),
	))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Claude.Configured() {
		t.Fatal("空 Key 文件应视为未配置")
	}

	// 有效文件（含尾换行）
	cfg, err = LoadFrom(ServiceWorker, env(
		"DATABASE_URL", "postgres://u:x@db/t",
		"ANTHROPIC_API_KEY_FILE", writeSecret(t, "test-key-value\n"),
	))
	if err != nil {
		t.Fatal(err)
	}
	key, ok := cfg.Claude.AnthropicKey()
	if !ok || key != "test-key-value" {
		t.Fatalf("Key 读取错误：ok=%v", ok)
	}
}

func TestAPIServiceDoesNotReadAnthropicKeyFile(t *testing.T) {
	cfg, err := LoadFrom(ServiceAPI, env(
		"DATABASE_URL", "postgres://u:x@db/t",
		"ANTHROPIC_API_KEY_FILE", writeSecret(t, "api-must-not-read-this"),
		"CLAUDE_MODEL_DEFAULT", "configured-model",
	))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Claude.Configured() {
		t.Fatal("API 进程不得读取 ANTHROPIC_API_KEY_FILE")
	}
	if cfg.Claude.KeyFilePresent {
		t.Fatal("API 进程不得探测 Anthropic secret 文件是否存在")
	}
	if cfg.Claude.Model != "configured-model" {
		t.Fatalf("非敏感模型配置应仍可读取: %q", cfg.Claude.Model)
	}
}

func TestLoadClaudeGatewayConfigWithoutReadingWorkerSecret(t *testing.T) {
	cfg, err := LoadFrom(ServiceAPI, env(
		"DATABASE_URL", "postgres://u:x@db/t",
		"ANTHROPIC_API_KEY_FILE", writeSecret(t, "api-must-not-read-this"),
		"CLAUDE_GATEWAY_CONFIG_FILE", "/config/claude-gateway/keys.json",
		"CLAUDE_GATEWAY_WRITE_ENABLED", "true",
	))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Claude.Configured() || cfg.Claude.KeyFilePresent {
		t.Fatal("API 进程不得读取或探测 Worker-only Anthropic secret")
	}
	if cfg.Claude.GatewayConfigFile != "/config/claude-gateway/keys.json" || !cfg.Claude.GatewayWriteEnabled {
		t.Fatalf("Claude 网关配置加载错误: %+v", cfg.Claude)
	}
}

func TestLoadClaudeCodeSettingsFileWithoutReadingWorkerSecret(t *testing.T) {
	cfg, err := LoadFrom(ServiceAPI, env(
		"DATABASE_URL", "postgres://u:x@db/t",
		"ANTHROPIC_API_KEY_FILE", writeSecret(t, "api-must-not-read-this"),
		"CLAUDE_CODE_SETTINGS_FILE", "/claude-code/settings.json",
	))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Claude.Configured() || cfg.Claude.KeyFilePresent {
		t.Fatal("API 进程不得读取或探测 Worker-only Anthropic secret")
	}
	if cfg.Claude.ClaudeCodeSettingsFile != "/claude-code/settings.json" {
		t.Fatalf("Claude Code 设置文件配置加载错误: %q", cfg.Claude.ClaudeCodeSettingsFile)
	}
	if cfg.SafeSummary()["claude_code_settings_configured"] != true {
		t.Fatalf("SafeSummary 未报告 Claude Code 设置能力: %v", cfg.SafeSummary())
	}
}

func TestLoadClaudeCodeSettingsRuntimeHomeDefault(t *testing.T) {
	cfg, err := LoadFrom(ServiceAPI, env(
		"DATABASE_URL", "postgres://u:x@db/t",
		"HOME", "/runtime/home",
	))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := cfg.Claude.ClaudeCodeSettingsFile, "/runtime/home/.claude/settings.json"; got != want {
		t.Fatalf("Claude Code settings path=%q want runtime HOME default %q", got, want)
	}
}

func TestLoadClaudeCodeSettingsExplicitFileOverridesRuntimeHome(t *testing.T) {
	cfg, err := LoadFrom(ServiceAPI, env(
		"DATABASE_URL", "postgres://u:x@db/t",
		"HOME", "/runtime/home",
		"CLAUDE_CODE_SETTINGS_FILE", "/explicit/claude/settings.json",
	))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := cfg.Claude.ClaudeCodeSettingsFile, "/explicit/claude/settings.json"; got != want {
		t.Fatalf("Claude Code settings path=%q want explicit override %q", got, want)
	}
}

func TestLoadClaudeCodeSettingsMissingRuntimeHomeIsUnavailable(t *testing.T) {
	cfg, err := LoadFrom(ServiceAPI, env("DATABASE_URL", "postgres://u:x@db/t"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Claude.ClaudeCodeSettingsFile != "" {
		t.Fatalf("Claude Code settings capability must be unavailable without HOME: %q", cfg.Claude.ClaudeCodeSettingsFile)
	}
	if cfg.SafeSummary()["claude_code_settings_configured"] != false {
		t.Fatalf("SafeSummary must report unavailable Claude Code settings: %v", cfg.SafeSummary())
	}
}

func TestLoadClaudeBaseURLBuildsMessagesEndpoint(t *testing.T) {
	cfg, err := LoadFrom(ServiceWorker, env(
		"DATABASE_URL", "postgres://u:x@db/t",
		"ANTHROPIC_BASE_URL", "https://gateway.example/",
	))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Claude.AnthropicBaseURL != "https://gateway.example" {
		t.Fatalf("Claude Base URL=%q want canonical gateway base", cfg.Claude.AnthropicBaseURL)
	}
	if got, want := cfg.Claude.AnthropicMessagesEndpoint(), "https://gateway.example/v1/messages"; got != want {
		t.Fatalf("Claude messages endpoint=%q want %q", got, want)
	}
}

func TestToolsServiceDoesNotReadAnthropicKeyFile(t *testing.T) {
	cfg, err := LoadFrom(ServiceBackup, env(
		"DATABASE_URL", "postgres://u:x@db/t",
		"ANTHROPIC_API_KEY_FILE", writeSecret(t, "tools-must-not-read-this"),
	))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Claude.Configured() || cfg.Claude.KeyFilePresent {
		t.Fatal("Tools 进程不得读取或探测 Anthropic secret")
	}
}

func TestWorkerFakeProviderDoesNotReadAnthropicKeyFile(t *testing.T) {
	cfg, err := LoadFrom(ServiceWorker, env(
		"DATABASE_URL", "postgres://u:x@db/t",
		"AI_PROVIDER", "fake",
		"ANTHROPIC_API_KEY_FILE", writeSecret(t, "must-not-be-read"),
	))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Claude.Provider != "fake" || cfg.Claude.Configured() || cfg.Claude.KeyFilePresent {
		t.Fatalf("fake provider must not read secret: %+v", cfg.Claude)
	}
}

func TestInvalidValuesFailFast(t *testing.T) {
	base := []string{"DATABASE_URL", "postgres://u:x@db/t"}
	for _, tc := range [][2]string{
		{"HTTP_REQUEST_TIMEOUT", "not-a-duration"},
		{"WORKER_CONCURRENCY", "0"},
		{"MAINTENANCE_HEARTBEAT_INTERVAL", "60s"}, // >= lease 45s
		{"TRANSCRIPTION_MODE", "cloud"},
		{"MAX_UPLOAD_BYTES", "-1"},
		{"CLAUDE_GATEWAY_WRITE_ENABLED", "not-a-bool"},
	} {
		_, err := LoadFrom(ServiceAPI, env(append(base, tc[0], tc[1])...))
		if err == nil {
			t.Fatalf("%s=%s 应启动失败", tc[0], tc[1])
		}
	}
}

func TestSafeSummaryContainsNoSecrets(t *testing.T) {
	cfg, err := LoadFrom(ServiceAPI, env(
		"DATABASE_URL", "postgres://u:supersecret@db/t",
		"ANTHROPIC_API_KEY_FILE", writeSecret(t, "topsecret-key"),
	))
	if err != nil {
		t.Fatal(err)
	}
	summary := cfg.SafeSummary()
	text := fmt.Sprintf("%v", summary)
	if strings.Contains(text, "supersecret") || strings.Contains(text, "topsecret-key") {
		t.Fatalf("SafeSummary 泄露 secret：%v", summary)
	}
	if summary["claude_configured"] != false || summary["db_configured"] != true {
		t.Fatalf("SafeSummary 配置状态错误：%v", summary)
	}
}
