package main

import (
	"testing"

	"github.com/TSY244/ai-study-tutor/backend/internal/claudegateway"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/config"
)

func TestClaudeGatewayHandlerWiring(t *testing.T) {
	cfg := config.Config{}
	cfg.Claude.GatewayConfigFile = t.TempDir() + "/keys.json"
	cfg.Claude.AnthropicBaseURL = "https://configured.deepseek.example/anthropic"

	h := buildClaudeGatewayHandler(cfg, nil, nil)
	if h == nil {
		t.Fatal("configured Claude gateway must build a handler")
	}
	if got, want := h.DefaultBaseURL, cfg.Claude.AnthropicBaseURL; got != want {
		t.Fatalf("Handler.DefaultBaseURL=%q want configured ANTHROPIC_BASE_URL %q", got, want)
	}

	key := claudegateway.Key{
		ID: "ck_api_wiring", APIKey: "runtime-api-wiring-secret", Model: "deepseek-chat", Enabled: true,
	}
	h.State.MarkAuthFailedForKey(key, "claude_auth_failed", 0)
	cfg.Claude.AnthropicBaseURL = "https://replacement.deepseek.example/anthropic"
	restarted := buildClaudeGatewayHandler(cfg, nil, nil)
	if restarted == nil || !restarted.State.Eligible(key, restarted.State.Now()) {
		t.Fatal("API StateTracker did not receive the replacement deployment BaseURL")
	}
}
