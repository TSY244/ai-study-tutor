package ai

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRealAnthropicConnectivitySmoke(t *testing.T) {
	if os.Getenv("AI_STUDY_TUTOR_REAL_CLAUDE_SMOKE") != "1" {
		t.Skip("真实 Claude smoke 需要显式设置 AI_STUDY_TUTOR_REAL_CLAUDE_SMOKE=1")
	}
	keyPath := os.Getenv("ANTHROPIC_API_KEY_FILE")
	if keyPath == "" {
		t.Fatal("真实 smoke 只读取 ANTHROPIC_API_KEY_FILE")
	}
	raw, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal("读取 ANTHROPIC_API_KEY_FILE 失败")
	}
	key := strings.TrimRight(string(raw), "\r\n")
	if key == "" {
		t.Fatal("ANTHROPIC_API_KEY_FILE 为空")
	}
	model := os.Getenv("CLAUDE_MODEL_DEFAULT")
	if model == "" {
		t.Fatal("真实 smoke 需要 CLAUDE_MODEL_DEFAULT")
	}
	provider := NewAnthropicProvider(AnthropicConfig{
		APIKey: key, Model: model, MaxOutputTokens: 128, Timeout: 20 * time.Second,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := provider.Generate(ctx, Request{
		Purpose:               PurposeConnectivity,
		System:                "Return only the requested structured result.",
		User:                  "Return ok=true.",
		Schema:                json.RawMessage(`{"type":"object","additionalProperties":false,"required":["ok"],"properties":{"ok":{"type":"boolean"}}}`),
		PromptTemplateVersion: "connectivity/v1",
		SchemaVersion:         "connectivity-result/v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(resp.JSON) {
		t.Fatalf("真实 smoke 返回非 JSON: %s", resp.JSON)
	}
}
