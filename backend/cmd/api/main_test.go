package main

import (
	"testing"

	"github.com/TSY244/ai-study-tutor/backend/internal/platform/config"
)

func TestClaudeGatewayHandlerWiring(t *testing.T) {
	cfg := config.Config{}
	cfg.Claude.GatewayConfigFile = "/runtime/claude-gateway/keys.json"
	cfg.Claude.AnthropicBaseURL = "https://configured.deepseek.example/anthropic"

	h := buildClaudeGatewayHandler(cfg, nil, nil)
	if h == nil {
		t.Fatal("configured Claude gateway must build a handler")
	}
	if got, want := h.DefaultBaseURL, cfg.Claude.AnthropicBaseURL; got != want {
		t.Fatalf("Handler.DefaultBaseURL=%q want configured ANTHROPIC_BASE_URL %q", got, want)
	}
}
