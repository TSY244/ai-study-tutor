package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestOpenAIBaseURLs(t *testing.T) {
	if got := OpenAIChatEndpoint("https://proxy.example/api/v1?version=1"); got != "https://proxy.example/api/v1/chat/completions?version=1" {
		t.Fatalf("query lost or appended into path: %q", got)
	}
	for _, tc := range []struct{ in, want string }{
		{"", "https://api.openai.com/v1"},
		{"https://api.openai.com/", "https://api.openai.com/v1"},
		{"https://proxy.example/v1/", "https://proxy.example/v1"},
		{"https://proxy.example/api/v1/chat/completions/", "https://proxy.example/api/v1"},
		{"https://proxy.example/custom", "https://proxy.example/custom"},
	} {
		if got := OpenAIBaseURL(tc.in); got != tc.want {
			t.Fatalf("base %q: %q != %q", tc.in, got, tc.want)
		}
		if got := OpenAIChatEndpoint(tc.in); got != tc.want+"/chat/completions" {
			t.Fatalf("endpoint: %q", got)
		}
	}
}

func TestOpenAIStructuredRequestAndUsage(t *testing.T) {
	for _, host := range []string{"api.openai.com", "proxy.example"} {
		t.Run(host, func(t *testing.T) {
			client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.String() != "https://"+host+"/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer test-secret" || r.Header.Get("x-api-key") != "" {
					t.Fatal("incorrect protocol endpoint or auth headers")
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				field := "max_tokens"
				if host == "api.openai.com" {
					field = "max_completion_tokens"
				}
				if body[field] != float64(256) || body["model"] != "selected-model" {
					t.Fatalf("incorrect model/token settings: %v", body)
				}
				if host == "api.openai.com" {
					choice, ok := body["tool_choice"].(map[string]any)
					if !ok || choice["function"].(map[string]any)["name"] != "structured_result" {
						t.Fatal("official OpenAI endpoint must retain named tool choice")
					}
				} else if body["tool_choice"] != "auto" {
					// Some compatible gateways return 503 for both required and
					// named tool choices while accepting the same tools with auto.
					return jsonResponse(503, `{"error":{"code":"upstream_error"}}`), nil
				}
				tool := body["tools"].([]any)[0].(map[string]any)["function"].(map[string]any)
				if tool["name"] != "structured_result" || tool["parameters"].(map[string]any)["type"] != "object" {
					t.Fatal("missing structured tool schema")
				}
				messages := body["messages"].([]any)
				if len(messages) != 2 || messages[0].(map[string]any)["role"] != "system" {
					t.Fatal("system/user messages missing")
				}
				if host != "api.openai.com" && !strings.Contains(messages[0].(map[string]any)["content"].(string), "Call structured_result") {
					t.Fatal("auto tool choice must instruct the model to return the structured result")
				}
				resp := jsonResponse(200, `{"id":"chat-123","model":"resolved-model","choices":[{"finish_reason":"tool_calls","message":{"tool_calls":[{"function":{"name":"structured_result","arguments":"{\"ok\":true}"}}]}}],"usage":{"prompt_tokens":11,"completion_tokens":7,"prompt_tokens_details":{"cached_tokens":3}}}`)
				resp.Header.Set("x-request-id", "request-123")
				return resp, nil
			})}
			p := NewOpenAIProvider(OpenAIConfig{APIKey: "test-secret", Model: "default", Endpoint: "https://" + host + "/v1/chat/completions", HTTPClient: client})
			resp, err := p.Generate(context.Background(), Request{Model: "selected-model", MaxOutputTokens: 256, System: "system", User: "user", Schema: json.RawMessage(`{"type":"object"}`)})
			if err != nil {
				t.Fatal(err)
			}
			if string(resp.JSON) != `{"ok":true}` || resp.Model != "resolved-model" || resp.ProviderRequestID != "request-123" || resp.Usage.InputTokens != 8 || resp.Usage.OutputTokens != 7 || resp.Usage.CacheReadTokens != 3 {
				t.Fatalf("incorrect result/metadata: %+v", resp)
			}
		})
	}
}

func TestOpenAIErrorClassificationAndRedaction(t *testing.T) {
	for _, tc := range []struct {
		name, body, code string
		status           int
	}{
		{"auth", `{"error":{"message":"echo-secret"}}`, ErrAuthFailed, 401},
		{"forbidden", `echo-secret`, ErrAuthFailed, 403},
		{"account quota", `{"error":{"code":"insufficient_user_quota","message":"echo-secret balance -0.13"}}`, ErrQuotaExhausted, 403},
		{"token quota", `{"error":{"code":"insufficient_token_quota","message":"echo-secret"}}`, ErrQuotaExhausted, 403},
		{"OpenAI quota", `{"error":{"code":"insufficient_quota","type":"insufficient_quota","message":"echo-secret"}}`, ErrQuotaExhausted, 429},
		{"limit", `echo-secret`, ErrRateLimited, 429},
		{"server", `echo-secret`, ErrOverloaded, 503},
		{"bad request", `echo-secret`, ErrInvalidRequest, 400},
		{"malformed", `echo-secret`, ErrInvalidJSON, 200},
		{"empty", `{"choices":[]}`, ErrInvalidJSON, 200},
		{"truncated", `{"choices":[{"finish_reason":"length","message":{"content":"{}"}}]}`, ErrOutputTruncated, 200},
		{"refused", `{"choices":[{"finish_reason":"stop","message":{"refusal":"echo-secret"}}]}`, ErrRefused, 200},
		{"bad arguments", `{"choices":[{"finish_reason":"tool_calls","message":{"tool_calls":[{"function":{"name":"structured_result","arguments":"echo-secret"}}]}}]}`, ErrInvalidJSON, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				resp := jsonResponse(tc.status, tc.body)
				resp.Header.Set("Retry-After", "5")
				return resp, nil
			})}
			_, err := NewOpenAIProvider(OpenAIConfig{APIKey: "echo-secret", Model: "model", HTTPClient: client}).Generate(context.Background(), Request{})
			if err == nil || AsProviderError(err).Code != tc.code || strings.Contains(err.Error(), "echo-secret") {
				t.Fatalf("incorrect or unsafe error: %v", err)
			}
			if tc.code == ErrQuotaExhausted && (AsProviderError(err).Retryable || strings.Contains(err.Error(), "-0.13")) {
				t.Fatal("quota errors must be sanitized and must not retry automatically")
			}
			if tc.code == ErrRateLimited && AsProviderError(err).RetryAfter != 5*time.Second {
				t.Fatal("Retry-After lost")
			}
		})
	}
}

func TestOpenAIJSONTextFallbackAndTimeout(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(200, `{"id":"chat-id","choices":[{"finish_reason":"stop","message":{"content":"{\"ok\":true}"}}]}`), nil
	})}
	resp, err := NewOpenAIProvider(OpenAIConfig{APIKey: "key", Model: "model", HTTPClient: client}).Generate(context.Background(), Request{})
	if err != nil || string(resp.JSON) != `{"ok":true}` || resp.ProviderRequestID != "chat-id" {
		t.Fatalf("text fallback: %+v %v", resp, err)
	}
	client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) { <-r.Context().Done(); return nil, r.Context().Err() })
	_, err = NewOpenAIProvider(OpenAIConfig{APIKey: "key", Model: "model", HTTPClient: client, Timeout: time.Millisecond}).Generate(context.Background(), Request{})
	if AsProviderError(err).Code != ErrTimeout {
		t.Fatalf("timeout: %v", err)
	}
}
