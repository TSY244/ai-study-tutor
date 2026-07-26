package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	aipkg "github.com/TSY244/ai-study-tutor/backend/internal/ai"
	"github.com/TSY244/ai-study-tutor/backend/internal/claudegateway"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/config"
)

func TestBuildClaudeProvidersKeepsGatewayHotReloadAvailableToLessonImport(t *testing.T) {
	cfg := gatewayTestConfig(filepath.Join(t.TempDir(), "keys.json"))
	providers := buildClaudeProviders(cfg, "", false, nil)

	if providers.Gateway == nil || providers.AI != providers.Gateway || providers.LessonImportAI != providers.Gateway {
		t.Fatal("网关应同时装配到普通 AI Job 与 lesson_import")
	}
	if providers.LessonImportAvailable == nil || providers.LessonImportAvailable() {
		t.Fatal("没有启用 Key 时 lesson_import 应保持 raw_text 降级")
	}
	if _, err := providers.Gateway.Store.Create(workerGatewayTestKey("hot-added", 10)); err != nil {
		t.Fatal(err)
	}
	if !providers.LessonImportAvailable() {
		t.Fatal("运行期新增 Key 后 lesson_import 应无需重启即可启用 Claude")
	}
}

func TestWorkerHeartbeatSourceReloadsGatewayKeyCount(t *testing.T) {
	cfg := gatewayTestConfig(filepath.Join(t.TempDir(), "keys.json"))
	providers := buildClaudeProviders(cfg, "", false, nil)
	source := workerHeartbeatSource(cfg, false, providers.Gateway)

	if heartbeat := source(); heartbeat.ClaudeStatus != "unavailable" {
		t.Fatalf("没有启用 Key 时 status=%q want unavailable", heartbeat.ClaudeStatus)
	}
	if _, err := providers.Gateway.Store.Create(workerGatewayTestKey("heartbeat-hot-reload", 10)); err != nil {
		t.Fatal(err)
	}
	heartbeat := source()
	if heartbeat.ClaudeStatus == "unavailable" {
		t.Fatalf("新增 Key 后 status=%q，心跳未热重载网关配置", heartbeat.ClaudeStatus)
	}
	if !strings.Contains(heartbeat.ClaudeMessage, "1 个启用 Key") {
		t.Fatalf("新增 Key 后心跳未报告启用数量: %q", heartbeat.ClaudeMessage)
	}
}

func TestBuildClaudeProvidersRoutesFailoverThroughAssembledGateway(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusTooManyRequests} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			cfg := gatewayTestConfig(filepath.Join(t.TempDir(), "keys.json"))
			providers := buildClaudeProviders(cfg, "", false, nil)
			first, err := providers.Gateway.Store.Create(workerGatewayTestKey("first", 1))
			if err != nil {
				t.Fatal(err)
			}
			second, err := providers.Gateway.Store.Create(workerGatewayTestKey("second", 2))
			if err != nil {
				t.Fatal(err)
			}

			calls := []string{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.Header.Get("x-api-key") {
				case first.APIKey:
					calls = append(calls, first.ID)
					w.WriteHeader(status)
					_, _ = io.WriteString(w, `{"type":"error","error":{"type":"api_error","message":"redacted"}}`)
				case second.APIKey:
					calls = append(calls, second.ID)
					w.Header().Set("content-type", "application/json")
					_, _ = io.WriteString(w, `{"model":"claude-test","stop_reason":"tool_use","content":[{"type":"tool_use","name":"structured_result","input":{"ok":true}}]}`)
				default:
					t.Fatal("收到未知 Key")
				}
			}))
			defer server.Close()
			providers.Gateway.Endpoint = server.URL
			providers.Gateway.HTTPClient = server.Client()

			if _, err := providers.AI.Generate(context.Background(), aipkg.BuildConnectivityRequest()); err != nil {
				t.Fatal(err)
			}
			if got, want := strings.Join(calls, ","), strings.Join([]string{first.ID, second.ID}, ","); got != want {
				t.Fatalf("failover calls=%s want=%s", got, want)
			}
		})
	}
}

func TestBuildClaudeProvidersUsesConfiguredBaseURLForGateway(t *testing.T) {
	cfg := gatewayTestConfig(filepath.Join(t.TempDir(), "keys.json"))
	cfg.Claude.AnthropicBaseURL = "https://gateway.example/"

	providers := buildClaudeProviders(cfg, "", false, nil)
	if providers.Gateway == nil {
		t.Fatal("expected gateway provider")
	}
	if got, want := providers.Gateway.Endpoint, "https://gateway.example/v1/messages"; got != want {
		t.Fatalf("gateway endpoint=%q want %q", got, want)
	}
}

func gatewayTestConfig(path string) config.Config {
	return config.Config{Claude: config.ClaudeConfig{
		Provider:          "anthropic",
		Model:             "claude-test",
		RequestTimeout:    time.Second,
		MaxOutputTokens:   64,
		GatewayConfigFile: path,
	}}
}

func workerGatewayTestKey(name string, priority int) claudegateway.Key {
	return claudegateway.Key{
		Name: name, APIKey: strings.Join([]string{"runtime", "worker", name}, "-"), Model: "claude-test",
		Enabled: true, Priority: priority, Weight: 1, TimeoutSeconds: 1,
		CooldownSeconds: 60, MaxConsecutiveFailures: 1,
	}
}

// TestBuildClaudeProvidersURLImportRequiresGatewayKey 证明 URL 导入只认网关
// 启用 Key：legacy ANTHROPIC_API_KEY_FILE 直连 Key 不是替代品；Key 热增删
// 立即反映到 URLImportAvailable，且移除后绝不回退 legacy 发 HTTP。
func TestBuildClaudeProvidersURLImportRequiresGatewayKey(t *testing.T) {
	cfg := gatewayTestConfig(filepath.Join(t.TempDir(), "keys.json"))
	providers := buildClaudeProviders(cfg, "legacy-direct-key", true, nil)

	if providers.URLImportAI == nil {
		t.Fatal("存在网关配置时必须装配 URL 专用 Provider")
	}
	if providers.URLExtractor == nil {
		t.Fatal("存在网关配置时必须装配 URL 专用 CLI 提取器")
	}
	if providers.QuizCLI == nil {
		t.Fatal("存在网关配置时必须装配测验 CLI 生成器")
	}
	if providers.URLImportAvailable == nil || providers.URLImportAvailable() {
		t.Fatal("零网关启用 Key 时 URL 导入必须不可用（legacy 直连 Key 不是替代品）")
	}
	// 既有非 URL 接线保持不变。
	if providers.AI == nil || providers.LessonImportAI == nil || providers.LessonImportAvailable == nil {
		t.Fatal("URL 字段的装配不得影响既有 AI/LessonImportAI 接线")
	}

	httpCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		httpCalls++
		w.Header().Set("content-type", "application/json")
		_, _ = io.WriteString(w, `{"model":"claude-test","stop_reason":"tool_use","content":[{"type":"tool_use","name":"structured_result","input":{"ok":true}}]}`)
	}))
	defer server.Close()
	providers.Gateway.Endpoint = server.URL
	providers.Gateway.HTTPClient = server.Client()

	// 零启用 Key：claude_not_configured，且不发 HTTP。
	if _, err := providers.URLImportAI.Generate(context.Background(), aipkg.BuildConnectivityRequest()); aipkg.AsProviderError(err).Code != aipkg.ErrNotConfigured {
		t.Fatalf("零启用 Key 时 URL 调用应 claude_not_configured: %v", err)
	}
	if httpCalls != 0 {
		t.Fatalf("零启用 Key 时 URL 调用不得发起 HTTP: %d", httpCalls)
	}

	// 运行期新增网关 Key：URL 调用用该 Key。
	key, err := providers.Gateway.Store.Create(workerGatewayTestKey("url-import", 10))
	if err != nil {
		t.Fatal(err)
	}
	if !providers.URLImportAvailable() {
		t.Fatal("运行期新增启用 Key 后 URL 导入应无需重启即可用")
	}
	if _, err := providers.URLImportAI.Generate(context.Background(), aipkg.BuildConnectivityRequest()); err != nil {
		t.Fatal(err)
	}
	if httpCalls != 1 {
		t.Fatalf("URL 调用应恰好一次 HTTP: %d", httpCalls)
	}

	// 请求前移除 Key：再次失败且无 legacy 回退（HTTP 计数不变）。
	if err := providers.Gateway.Store.Delete(key.ID); err != nil {
		t.Fatal(err)
	}
	if providers.URLImportAvailable() {
		t.Fatal("移除 Key 后 URL 导入应立即不可用")
	}
	if _, err := providers.URLImportAI.Generate(context.Background(), aipkg.BuildConnectivityRequest()); aipkg.AsProviderError(err).Code != aipkg.ErrNotConfigured {
		t.Fatalf("移除 Key 后 URL 调用应 claude_not_configured: %v", err)
	}
	if httpCalls != 1 {
		t.Fatalf("移除 Key 后 URL 调用不得回退 legacy 发 HTTP: %d", httpCalls)
	}
}

// TestBuildClaudeProvidersURLImportHasNoLegacyOrFakeSubstitute 证明无网关
// 配置时（legacy 直连 Key 或 fake Provider）绝不装配 URL 导入替代品。
func TestBuildClaudeProvidersURLImportHasNoLegacyOrFakeSubstitute(t *testing.T) {
	t.Run("legacy direct key only", func(t *testing.T) {
		cfg := config.Config{Claude: config.ClaudeConfig{
			Provider: "anthropic", Model: "claude-test",
			RequestTimeout: time.Second, MaxOutputTokens: 64,
		}}
		providers := buildClaudeProviders(cfg, "legacy-direct-key", true, nil)
		if providers.Gateway != nil {
			t.Fatal("无网关配置时不应装配 Gateway")
		}
		if providers.URLImportAI != nil || providers.URLImportAvailable != nil || providers.URLExtractor != nil || providers.QuizCLI != nil {
			t.Fatal("legacy 直连 Key 不得成为 URL 导入/测验 CLI 替代品")
		}
		if providers.AI == nil || providers.LessonImportAI == nil {
			t.Fatal("legacy 直连 Key 仍应装配既有非 URL 接线")
		}
	})

	t.Run("fake provider", func(t *testing.T) {
		cfg := config.Config{Claude: config.ClaudeConfig{Provider: "fake", Model: "claude-test"}}
		providers := buildClaudeProviders(cfg, "", false, nil)
		if providers.URLImportAI != nil || providers.URLImportAvailable != nil || providers.URLExtractor != nil || providers.QuizCLI != nil {
			t.Fatal("fake Provider 不得成为 URL 导入/测验 CLI 替代品")
		}
	})
}
