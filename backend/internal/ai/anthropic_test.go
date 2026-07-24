package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"content-type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestAnthropicProviderExtractsStructuredToolUseAndUsage(t *testing.T) {
	var capturedAuth string
	var capturedBody map[string]any
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		capturedAuth = r.Header.Get("x-api-key")
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Fatal(err)
		}
		resp := jsonResponse(http.StatusOK, `{
			"id":"msg_123",
			"model":"claude-test",
			"stop_reason":"tool_use",
			"usage":{"input_tokens":11,"output_tokens":7,"cache_read_input_tokens":2,"cache_creation_input_tokens":3},
			"content":[{"type":"tool_use","id":"toolu_1","name":"structured_result","input":{"ok":true}}]
		}`)
		resp.Header.Set("request-id", "req_123")
		return resp, nil
	})}

	provider := NewAnthropicProvider(AnthropicConfig{
		APIKey:          "sk-ant-unit",
		Model:           "claude-test",
		Endpoint:        "https://anthropic.test/v1/messages",
		MaxOutputTokens: 256,
		HTTPClient:      client,
	})
	resp, err := provider.Generate(context.Background(), Request{
		Purpose:               PurposeConnectivity,
		System:                "只返回 JSON",
		User:                  "ping",
		Schema:                json.RawMessage(`{"type":"object","properties":{"ok":{"type":"boolean"}},"required":["ok"]}`),
		PromptTemplateVersion: "connectivity/v1",
		SchemaVersion:         "connectivity-result/v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if capturedAuth != "sk-ant-unit" {
		t.Fatal("Anthropic adapter 必须使用 x-api-key 发送 Worker 侧 secret")
	}
	if capturedBody["model"] != "claude-test" {
		t.Fatalf("model 未写入请求: %v", capturedBody)
	}
	if len(resp.JSON) == 0 || !strings.Contains(string(resp.JSON), `"ok":true`) {
		t.Fatalf("未提取 tool_use input: %s", resp.JSON)
	}
	if resp.ProviderRequestID != "req_123" || resp.Usage.InputTokens != 11 ||
		resp.Usage.CacheReadTokens != 2 || resp.Usage.CacheWriteTokens != 3 {
		t.Fatalf("元数据提取错误: %+v", resp)
	}
}

func TestAnthropicProviderMapsHTTPAndModelFailures(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		body       string
		wantCode   string
		retryAfter bool
	}{
		{"auth", http.StatusUnauthorized, `{"error":{"type":"authentication_error","message":"bad key"}}`, ErrAuthFailed, false},
		{"rate_limit", http.StatusTooManyRequests, `{"error":{"type":"rate_limit_error","message":"slow down"}}`, ErrRateLimited, true},
		{"overloaded", http.StatusServiceUnavailable, `{"error":{"type":"overloaded_error","message":"busy"}}`, ErrOverloaded, false},
		{"invalid_request", http.StatusBadRequest, `{"error":{"type":"invalid_request_error","message":"bad"}}`, ErrInvalidRequest, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				resp := jsonResponse(tc.status, tc.body)
				if tc.retryAfter {
					resp.Header.Set("Retry-After", "2")
				}
				return resp, nil
			})}
			provider := NewAnthropicProvider(AnthropicConfig{
				APIKey: "sk-ant-unit", Model: "claude-test", Endpoint: "https://anthropic.test/v1/messages",
				MaxOutputTokens: 128, HTTPClient: client,
			})
			_, err := provider.Generate(context.Background(), Request{Purpose: PurposeConnectivity})
			perr, ok := err.(*ProviderError)
			if !ok {
				t.Fatalf("期望 ProviderError，实际 %T %v", err, err)
			}
			if perr.Code != tc.wantCode {
				t.Fatalf("错误码 got %s want %s", perr.Code, tc.wantCode)
			}
			if tc.retryAfter && perr.RetryAfter != 2*time.Second {
				t.Fatalf("Retry-After 未解析: %+v", perr)
			}
		})
	}
}

func TestAnthropicProviderMapsRefusalTruncationAndMissingStructuredJSON(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantCode string
	}{
		{"refusal_stop", `{"model":"claude-test","stop_reason":"refusal","content":[]}`, ErrRefused},
		{"max_tokens", `{"model":"claude-test","stop_reason":"max_tokens","content":[]}`, ErrOutputTruncated},
		{"text_only", `{"model":"claude-test","stop_reason":"end_turn","content":[{"type":"text","text":"not json"}]}`, ErrInvalidJSON},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusOK, tc.body), nil
			})}
			provider := NewAnthropicProvider(AnthropicConfig{
				APIKey: "sk-ant-unit", Model: "claude-test", Endpoint: "https://anthropic.test/v1/messages",
				MaxOutputTokens: 128, HTTPClient: client,
			})
			_, err := provider.Generate(context.Background(), Request{Purpose: PurposeQuizGenerate})
			perr, ok := err.(*ProviderError)
			if !ok || perr.Code != tc.wantCode {
				t.Fatalf("错误映射 got %T %[1]v want %s", err, tc.wantCode)
			}
		})
	}
}
