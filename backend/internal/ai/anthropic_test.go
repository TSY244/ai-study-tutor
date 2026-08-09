package ai

import (
	"context"
	"encoding/json"
	"fmt"
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

func TestAnthropicProviderDeepSeekStructuredRequestThinking(t *testing.T) {
	tests := []struct {
		name         string
		model        string
		wantThinking bool
	}{
		{name: "DeepSeek model", model: "deepseek-v4-flash", wantThinking: true},
		{name: "normalized DeepSeek model", model: " \tDEEPSEEK-V4-Flash \n", wantThinking: true},
		{name: "Claude model", model: "claude-sonnet-4-5", wantThinking: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var captured map[string]any
			client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
					t.Fatal(err)
				}
				return jsonResponse(http.StatusOK, `{
					"id":"msg_structured","model":"response-model","stop_reason":"tool_use",
					"content":[{"type":"tool_use","id":"toolu_1","name":"structured_result","input":{"ok":true}}]
				}`), nil
			})}
			provider := NewAnthropicProvider(AnthropicConfig{
				APIKey: "sk-ant-unit", Model: tc.model,
				Endpoint: "https://anthropic.test/v1/messages", HTTPClient: client,
			})

			if _, err := provider.Generate(context.Background(), BuildConnectivityRequest()); err != nil {
				t.Fatal(err)
			}
			if captured["model"] != tc.model {
				t.Fatalf("model = %q, want selected model preserved as %q", captured["model"], tc.model)
			}
			toolChoice, ok := captured["tool_choice"].(map[string]any)
			if !ok || toolChoice["type"] != "tool" || toolChoice["name"] != "structured_result" {
				t.Fatalf("tool_choice = %#v, want forced structured_result", captured["tool_choice"])
			}
			thinking, hasThinking := captured["thinking"]
			if !tc.wantThinking {
				if hasThinking {
					t.Fatalf("Claude request unexpectedly contains thinking: %#v", thinking)
				}
				return
			}
			thinkingObject, ok := thinking.(map[string]any)
			if !hasThinking || !ok || thinkingObject["type"] != "disabled" {
				t.Fatalf("thinking = %#v, want {type:disabled}", thinking)
			}
		})
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

const (
	sentinelAPIKey   = "sk-ant-SECRET-SENTINEL-9f8e7d6c"
	sentinelSource   = "SENTINEL-RAW-SOURCE-TEXT-abcdef"
	canonicalTestURL = "https://www.youtube.com/watch?v=abc"
)

func buildWebFetchRequest(t *testing.T) Request {
	t.Helper()
	req, err := BuildLessonURLExtractRequest(LessonURLExtractPromptInput{
		URL:            canonicalTestURL,
		AllowedDomains: []string{"www.youtube.com", "youtube.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return req
}

func webFetchSuccessResultBlock(toolUseID string) string {
	return fmt.Sprintf(`{"type":"web_fetch_tool_result","tool_use_id":%q,"content":{"type":"web_fetch_result","url":%q,"retrieved_at":"2026-07-25T00:00:00Z","content":{"type":"document","title":"示例","text":"原始正文"}}}`, toolUseID, canonicalTestURL)
}

type capturedWireRequest struct {
	Model      string            `json:"model"`
	MaxTokens  int               `json:"max_tokens"`
	System     string            `json:"system"`
	ToolChoice map[string]any    `json:"tool_choice"`
	Tools      []json.RawMessage `json:"tools"`
	Messages   []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"messages"`
}

func assertDetailExcludesSentinels(t *testing.T, perr *ProviderError) {
	t.Helper()
	if strings.Contains(perr.Detail, sentinelAPIKey) || strings.Contains(perr.Detail, sentinelSource) {
		t.Fatalf("ProviderError.Detail 泄漏了敏感内容: %q", perr.Detail)
	}
}

func TestAnthropicProviderWebFetchRequestShapeAndSuccess(t *testing.T) {
	var betaHeader string
	var captured capturedWireRequest
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		betaHeader = r.Header.Get("anthropic-beta")
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatal(err)
		}
		body := `{
			"id":"msg_wf1","model":"claude-webfetch","stop_reason":"tool_use",
			"usage":{"input_tokens":10,"output_tokens":5},
			"content":[
				{"type":"server_tool_use","id":"srvtoolu_1","name":"web_fetch","input":{"url":"` + canonicalTestURL + `"}},
				` + webFetchSuccessResultBlock("srvtoolu_1") + `,
				{"type":"tool_use","id":"toolu_9","name":"structured_result","input":{"title":"示例","raw_text_markdown":"原始正文","extraction_note":"ok"}}
			]}`
		resp := jsonResponse(http.StatusOK, body)
		resp.Header.Set("request-id", "req_wf1")
		return resp, nil
	})}
	provider := NewAnthropicProvider(AnthropicConfig{
		APIKey: "sk-ant-unit", Model: "claude-test", Endpoint: "https://anthropic.test/v1/messages", HTTPClient: client,
	})
	resp, err := provider.Generate(context.Background(), buildWebFetchRequest(t))
	if err != nil {
		t.Fatal(err)
	}
	if betaHeader != "web-fetch-2025-09-10" {
		t.Fatalf("URL 请求必须发送 web-fetch beta header，实际 %q", betaHeader)
	}
	if captured.ToolChoice["type"] != "auto" {
		t.Fatalf("URL 请求 tool_choice 必须为 auto: %v", captured.ToolChoice)
	}
	if _, hasName := captured.ToolChoice["name"]; hasName {
		t.Fatalf("auto tool_choice 不得携带 name: %v", captured.ToolChoice)
	}
	if captured.MaxTokens != 16384 {
		t.Fatalf("max_tokens 应来自请求: %v", captured.MaxTokens)
	}
	var serverTool map[string]any
	var hasResultTool bool
	for _, raw := range captured.Tools {
		var tool map[string]any
		if err := json.Unmarshal(raw, &tool); err != nil {
			t.Fatal(err)
		}
		if tool["type"] == "web_fetch_20250910" {
			serverTool = tool
		}
		if tool["name"] == "structured_result" {
			hasResultTool = true
			if tool["input_schema"] == nil {
				t.Fatal("structured_result 缺少 input_schema")
			}
		}
	}
	if serverTool == nil {
		t.Fatalf("缺少 web_fetch_20250910 server tool: %s", captured.Tools)
	}
	if serverTool["name"] != "web_fetch" || serverTool["max_uses"] != float64(3) || serverTool["max_content_tokens"] != float64(12000) {
		t.Fatalf("server tool 配置错误: %v", serverTool)
	}
	domains, _ := serverTool["allowed_domains"].([]any)
	if len(domains) != 2 || domains[0] != "www.youtube.com" || domains[1] != "youtube.com" {
		t.Fatalf("allowed_domains 错误: %v", serverTool["allowed_domains"])
	}
	if !hasResultTool {
		t.Fatal("缺少 structured_result client tool")
	}
	if !strings.Contains(string(resp.JSON), `"raw_text_markdown"`) {
		t.Fatalf("未提取 structured_result: %s", resp.JSON)
	}
	if resp.Usage.InputTokens != 10 || resp.Usage.OutputTokens != 5 {
		t.Fatalf("usage 提取错误: %+v", resp.Usage)
	}
	if resp.ProviderRequestID != "req_wf1" || resp.StopReason != "tool_use" || resp.Model != "claude-webfetch" {
		t.Fatalf("元数据错误: %+v", resp)
	}
	if resp.Latency <= 0 {
		t.Fatal("整个逻辑调用必须记录一次 latency")
	}
}

func TestAnthropicProviderWebFetchLeavesOrdinaryRequestsUnchanged(t *testing.T) {
	var betaHeader string
	var captured capturedWireRequest
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		betaHeader = r.Header.Get("anthropic-beta")
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatal(err)
		}
		return jsonResponse(http.StatusOK, `{
			"id":"msg_1","model":"claude-test","stop_reason":"tool_use",
			"usage":{"input_tokens":1,"output_tokens":1},
			"content":[{"type":"tool_use","id":"toolu_1","name":"structured_result","input":{"ok":true}}]
		}`), nil
	})}
	provider := NewAnthropicProvider(AnthropicConfig{
		APIKey: "sk-ant-unit", Model: "claude-test", Endpoint: "https://anthropic.test/v1/messages", HTTPClient: client,
	})
	if _, err := provider.Generate(context.Background(), BuildConnectivityRequest()); err != nil {
		t.Fatal(err)
	}
	if betaHeader != "" {
		t.Fatalf("普通请求不得发送 anthropic-beta header: %q", betaHeader)
	}
	if captured.ToolChoice["type"] != "tool" || captured.ToolChoice["name"] != "structured_result" {
		t.Fatalf("普通请求仍须强制 structured_result: %v", captured.ToolChoice)
	}
	if len(captured.Tools) != 1 {
		t.Fatalf("普通请求只能有 structured_result 一个工具: %s", captured.Tools)
	}
	var tool map[string]any
	if err := json.Unmarshal(captured.Tools[0], &tool); err != nil {
		t.Fatal(err)
	}
	if tool["name"] != "structured_result" || tool["type"] != nil {
		t.Fatalf("普通请求工具载荷被改变: %v", tool)
	}
}

func TestAnthropicProviderWebFetchRequiresCanonicalFetchEvidence(t *testing.T) {
	structured := `{"type":"tool_use","id":"toolu_1","name":"structured_result","input":{"title":"t","raw_text_markdown":"x","extraction_note":"n"}}`
	canonicalUse := `{"type":"server_tool_use","id":"srvtoolu_1","name":"web_fetch","input":{"url":"` + canonicalTestURL + `"}}`
	cases := []struct {
		name          string
		content       string
		wantCode      string
		wantRetryable bool
	}{
		{
			name:          "structured_without_fetch",
			content:       `[` + structured + `]`,
			wantCode:      ErrMaterialFetchFailed,
			wantRetryable: false,
		},
		{
			name: "fetch_of_different_url",
			content: `[{"type":"server_tool_use","id":"srvtoolu_1","name":"web_fetch","input":{"url":"https://evil.example/page"}},` +
				`{"type":"web_fetch_tool_result","tool_use_id":"srvtoolu_1","content":{"type":"web_fetch_result","url":"https://evil.example/page","content":{"type":"document","text":"` + sentinelSource + `"}}},` +
				structured + `]`,
			wantCode:      ErrMaterialFetchFailed,
			wantRetryable: false,
		},
		{
			name: "permanent_fetch_error",
			content: `[` + canonicalUse + `,` +
				`{"type":"web_fetch_tool_result","tool_use_id":"srvtoolu_1","content":{"type":"web_fetch_tool_error","error_code":"url_not_accessible","message":"` + sentinelSource + `"}},` +
				structured + `]`,
			wantCode:      ErrMaterialFetchFailed,
			wantRetryable: false,
		},
		{
			name: "temporary_fetch_error",
			content: `[` + canonicalUse + `,` +
				`{"type":"web_fetch_tool_result","tool_use_id":"srvtoolu_1","content":{"type":"web_fetch_tool_error","error_code":"too_many_requests"}},` +
				structured + `]`,
			wantCode:      ErrMaterialFetchFailed,
			wantRetryable: true,
		},
		{
			name: "truncated_fetch_maps_material_too_large",
			content: `[` + canonicalUse + `,` +
				`{"type":"web_fetch_tool_result","tool_use_id":"srvtoolu_1","content":{"type":"web_fetch_result","url":"` + canonicalTestURL + `","truncated":true}},` +
				structured + `]`,
			wantCode:      "material_too_large",
			wantRetryable: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusOK, `{
					"id":"msg_e","model":"claude-test","stop_reason":"tool_use",
					"usage":{"input_tokens":1,"output_tokens":1},
					"content":`+tc.content+`
				}`), nil
			})}
			provider := NewAnthropicProvider(AnthropicConfig{
				APIKey: "sk-ant-unit", Model: "claude-test", Endpoint: "https://anthropic.test/v1/messages", HTTPClient: client,
			})
			_, err := provider.Generate(context.Background(), buildWebFetchRequest(t))
			perr, ok := err.(*ProviderError)
			if !ok {
				t.Fatalf("期望 ProviderError，实际 %T %v", err, err)
			}
			if perr.Code != tc.wantCode {
				t.Fatalf("错误码 got %s want %s", perr.Code, tc.wantCode)
			}
			if perr.Retryable != tc.wantRetryable {
				t.Fatalf("retryable got %v want %v", perr.Retryable, tc.wantRetryable)
			}
			assertDetailExcludesSentinels(t, perr)
		})
	}
}

func TestAnthropicProviderWebFetchPauseTurnContinuation(t *testing.T) {
	assistantContent := `[ {"type":"server_tool_use","id":"srvtoolu_1","name":"web_fetch","input":{"url":"` + canonicalTestURL + `"}} ,` +
		webFetchSuccessResultBlock("srvtoolu_1") + ` ]`
	responses := []string{
		`{"id":"msg_p1","model":"claude-pause-1","stop_reason":"pause_turn","usage":{"input_tokens":5,"output_tokens":3,"cache_read_input_tokens":1,"cache_creation_input_tokens":1},"content":` + assistantContent + `}`,
		`{"id":"msg_p2","model":"claude-pause-2","stop_reason":"tool_use","usage":{"input_tokens":7,"output_tokens":4,"cache_read_input_tokens":2,"cache_creation_input_tokens":1},"content":[{"type":"tool_use","id":"toolu_2","name":"structured_result","input":{"title":"t","raw_text_markdown":"正文","extraction_note":"n"}}]}`,
	}
	requestIDs := []string{"req_p1", "req_p2"}
	var captured []capturedWireRequest
	var betaHeaders []string
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		idx := len(captured)
		if idx >= len(responses) {
			t.Fatalf("请求次数超出预期: %d", idx+1)
		}
		betaHeaders = append(betaHeaders, r.Header.Get("anthropic-beta"))
		var body capturedWireRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		captured = append(captured, body)
		resp := jsonResponse(http.StatusOK, responses[idx])
		resp.Header.Set("request-id", requestIDs[idx])
		return resp, nil
	})}
	provider := NewAnthropicProvider(AnthropicConfig{
		APIKey: "sk-ant-unit", Model: "claude-test", Endpoint: "https://anthropic.test/v1/messages", HTTPClient: client,
	})
	resp, err := provider.Generate(context.Background(), buildWebFetchRequest(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(captured) != 2 {
		t.Fatalf("pause_turn 应触发一次 continuation，实际请求 %d 次", len(captured))
	}
	for i, h := range betaHeaders {
		if h != "web-fetch-2025-09-10" {
			t.Fatalf("第 %d 次请求缺少 beta header: %q", i, h)
		}
	}
	cont := captured[1]
	if cont.Model != captured[0].Model || cont.System != captured[0].System {
		t.Fatal("continuation 必须保持 model/system 不变")
	}
	if len(cont.Tools) != len(captured[0].Tools) {
		t.Fatal("continuation 必须保持 tools 不变")
	}
	for i := range cont.Tools {
		if string(cont.Tools[i]) != string(captured[0].Tools[i]) {
			t.Fatalf("continuation 工具 %d 被改变:\n got %s\nwant %s", i, cont.Tools[i], captured[0].Tools[i])
		}
	}
	if len(cont.Messages) != 3 {
		t.Fatalf("continuation 应为 user+assistant+user 三条消息，实际 %d 条", len(cont.Messages))
	}
	if cont.Messages[0].Role != "user" || cont.Messages[1].Role != "assistant" || cont.Messages[2].Role != "user" {
		t.Fatalf("continuation 消息角色错误: %v", cont.Messages)
	}
	if string(cont.Messages[1].Content) != assistantContent {
		t.Fatalf("assistant content 必须逐字节保留:\n got %s\nwant %s", cont.Messages[1].Content, assistantContent)
	}
	var contBlocks []map[string]any
	if err := json.Unmarshal(cont.Messages[2].Content, &contBlocks); err != nil || len(contBlocks) == 0 || contBlocks[0]["type"] != "text" {
		t.Fatalf("continuation user 消息必须是文本块: %s", cont.Messages[2].Content)
	}
	wantUsage := Usage{InputTokens: 12, OutputTokens: 7, CacheReadTokens: 3, CacheWriteTokens: 2}
	if resp.Usage != wantUsage {
		t.Fatalf("usage 必须跨 continuation 聚合: got %+v want %+v", resp.Usage, wantUsage)
	}
	if resp.Model != "claude-pause-2" || resp.ProviderRequestID != "req_p2" || resp.StopReason != "tool_use" {
		t.Fatalf("必须保留最终非空元数据: %+v", resp)
	}
	if resp.Latency <= 0 {
		t.Fatal("整个逻辑调用必须记录一次 latency")
	}
	if !strings.Contains(string(resp.JSON), `"raw_text_markdown"`) {
		t.Fatalf("未提取 structured_result: %s", resp.JSON)
	}
}

func TestAnthropicProviderWebFetchPauseTurnExhaustsContinuations(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return jsonResponse(http.StatusOK, `{
			"id":"msg_x","model":"claude-x","stop_reason":"pause_turn",
			"usage":{"input_tokens":1,"output_tokens":1},
			"content":[{"type":"text","text":"working"}]
		}`), nil
	})}
	provider := NewAnthropicProvider(AnthropicConfig{
		APIKey: "sk-ant-unit", Model: "claude-test", Endpoint: "https://anthropic.test/v1/messages", HTTPClient: client,
	})
	_, err := provider.Generate(context.Background(), buildWebFetchRequest(t))
	perr, ok := err.(*ProviderError)
	if !ok {
		t.Fatalf("期望 ProviderError，实际 %T %v", err, err)
	}
	if perr.Code != ErrTimeout {
		t.Fatalf("第三次仍未解决的 pause_turn 必须映射 claude_timeout，实际 %s", perr.Code)
	}
	if !perr.Retryable {
		t.Fatal("claude_timeout 必须可重试")
	}
	if calls != 3 {
		t.Fatalf("最多初始请求 + 2 次 continuation，实际 %d 次", calls)
	}
}

func TestAnthropicProviderWebFetchHTTP400RemainsInvalidRequest(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusBadRequest, `{"error":{"type":"invalid_request_error","message":"tools.0 web_fetch rejected `+sentinelAPIKey+` `+sentinelSource+`"}}`), nil
	})}
	provider := NewAnthropicProvider(AnthropicConfig{
		APIKey: "sk-ant-unit", Model: "claude-test", Endpoint: "https://anthropic.test/v1/messages", HTTPClient: client,
	})
	_, err := provider.Generate(context.Background(), buildWebFetchRequest(t))
	perr, ok := err.(*ProviderError)
	if !ok {
		t.Fatalf("期望 ProviderError，实际 %T %v", err, err)
	}
	if perr.Code != ErrInvalidRequest {
		t.Fatalf("HTTP 400 工具拒绝必须保持 claude_invalid_request，实际 %s", perr.Code)
	}
	assertDetailExcludesSentinels(t, perr)
}
