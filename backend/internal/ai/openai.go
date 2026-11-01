package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// OpenAIBaseURL accepts a host, API base, or complete Chat Completions endpoint.
// Custom path prefixes are preserved. A bare host uses the standard /v1 base.
func OpenAIBaseURL(raw string) string {
	base := strings.TrimRight(strings.TrimSpace(raw), "/")
	if base == "" {
		return "https://api.openai.com/v1"
	}
	u, err := url.Parse(base)
	if err != nil {
		return base
	}
	u.Path = strings.TrimSuffix(strings.TrimRight(u.Path, "/"), "/chat/completions")
	if u.Path == "" {
		u.Path = "/v1"
	}
	u.RawPath = ""
	return u.String()
}

func OpenAIChatEndpoint(base string) string {
	u, err := url.Parse(OpenAIBaseURL(base))
	if err != nil {
		return base
	}
	u.Path += "/chat/completions"
	return u.String()
}

type OpenAIConfig struct {
	APIKey          string
	Model           string
	Endpoint        string
	MaxOutputTokens int
	Timeout         time.Duration
	HTTPClient      *http.Client
}

type OpenAIProvider struct {
	cfg    OpenAIConfig
	client *http.Client
}

func NewOpenAIProvider(cfg OpenAIConfig) *OpenAIProvider {
	if cfg.Endpoint == "" {
		cfg.Endpoint = OpenAIChatEndpoint("")
	}
	if cfg.MaxOutputTokens <= 0 {
		cfg.MaxOutputTokens = 8192
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 300 * time.Second
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: cfg.Timeout}
	}
	return &OpenAIProvider{cfg: cfg, client: client}
}

func (p *OpenAIProvider) Generate(ctx context.Context, req Request) (Response, error) {
	if strings.TrimSpace(p.cfg.APIKey) == "" {
		return Response{}, ErrorForCode(ErrNotConfigured, "模型 Key 未配置。")
	}
	model := req.Model
	if model == "" {
		model = p.cfg.Model
	}
	if model == "" || req.WebFetch != nil {
		return Response{}, ErrorForCode(ErrInvalidRequest, "模型配置无效，或当前协议不支持此请求。")
	}
	maxTokens := req.MaxOutputTokens
	if maxTokens <= 0 {
		maxTokens = p.cfg.MaxOutputTokens
	}
	u, _ := url.Parse(p.cfg.Endpoint)
	officialOpenAI := u != nil && strings.EqualFold(u.Hostname(), "api.openai.com")
	system := req.System
	var toolChoice any = map[string]any{"type": "function", "function": map[string]string{"name": "structured_result"}}
	if !officialOpenAI {
		// Some compatible endpoints accept tools but reject forced tool choice
		// with an upstream 503. Request the result through the prompt instead;
		// JSON and the purpose-specific schema are still validated below/caller.
		toolChoice = "auto"
		system = strings.TrimSpace(system + "\nCall structured_result with the result matching its JSON schema. Do not add explanatory text.")
	}
	messages := []map[string]string{}
	if system != "" {
		messages = append(messages, map[string]string{"role": "system", "content": system})
	}
	messages = append(messages, map[string]string{"role": "user", "content": req.User})
	payload := map[string]any{
		"model": model, "messages": messages,
		"tools": []any{map[string]any{"type": "function", "function": map[string]any{
			"name": "structured_result", "description": "Return only the requested structured JSON result.", "parameters": req.Schema,
		}}},
		"tool_choice": toolChoice,
	}
	// Official OpenAI models use the current token field. Compatible gateways
	// commonly require the original max_tokens field, as does Pi's adapter.
	if officialOpenAI {
		payload["max_completion_tokens"] = maxTokens
	} else {
		payload["max_tokens"] = maxTokens
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Response{}, WrapError(ErrInvalidRequest, "模型请求构造失败。", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.Endpoint, bytes.NewReader(body))
	if err != nil {
		return Response{}, WrapError(ErrInvalidRequest, "模型请求构造失败。", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	callCtx, cancel := context.WithTimeout(httpReq.Context(), p.cfg.Timeout)
	defer cancel()
	start := time.Now()
	resp, err := p.client.Do(httpReq.WithContext(callCtx))
	result := Response{Model: model, Latency: time.Since(start)}
	if err != nil {
		var netErr net.Error
		if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
			return result, WrapError(ErrTimeout, "模型请求超时，将自动重试。", err)
		}
		return result, WrapError(ErrUnavailable, "模型暂时不可用，请稍后重试。", err)
	}
	defer resp.Body.Close()
	result.ProviderRequestID = responseRequestID(resp.Header)
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return result, WrapError(ErrUnavailable, "读取模型响应失败。", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		mapped := mapHTTPError(resp.StatusCode, resp.Header, raw)
		mapped.Detail = strings.ReplaceAll(mapped.Detail, "Claude", "模型")
		return result, mapped
	}
	var out struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Content   string `json:"content"`
				Refusal   string `json:"refusal"`
				ToolCalls []struct {
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens         int64 `json:"prompt_tokens"`
			CompletionTokens     int64 `json:"completion_tokens"`
			PromptCacheHitTokens int64 `json:"prompt_cache_hit_tokens"`
			PromptTokensDetails  struct {
				CachedTokens int64 `json:"cached_tokens"`
			} `json:"prompt_tokens_details"`
		} `json:"usage"`
	}
	if json.Unmarshal(raw, &out) != nil || len(out.Choices) == 0 {
		return result, ErrorForCode(ErrInvalidJSON, "模型未返回有效的结构化结果。")
	}
	if out.Model != "" {
		result.Model = out.Model
	}
	if result.ProviderRequestID == "" {
		result.ProviderRequestID = out.ID
	}
	cached := max(out.Usage.PromptTokensDetails.CachedTokens, out.Usage.PromptCacheHitTokens)
	result.Usage = Usage{InputTokens: max(0, out.Usage.PromptTokens-cached), OutputTokens: out.Usage.CompletionTokens, CacheReadTokens: cached}
	choice := out.Choices[0]
	result.StopReason = choice.FinishReason
	if choice.FinishReason == "length" {
		return result, ErrorForCode(ErrOutputTruncated, "模型输出被截断；请减少输入或提高输出上限。")
	}
	if choice.FinishReason == "content_filter" || choice.Message.Refusal != "" {
		return result, ErrorForCode(ErrRefused, "模型拒绝生成该结果；请调整输入后重试。")
	}
	structured := choice.Message.Content
	for _, tool := range choice.Message.ToolCalls {
		if tool.Function.Name == "structured_result" {
			structured = tool.Function.Arguments
			break
		}
	}
	if !json.Valid([]byte(structured)) {
		return result, ErrorForCode(ErrInvalidJSON, "模型未返回有效的结构化 JSON，将自动重试。")
	}
	result.JSON = json.RawMessage(structured)
	return result, nil
}
