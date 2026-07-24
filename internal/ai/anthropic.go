package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const defaultAnthropicEndpoint = "https://api.anthropic.com/v1/messages"

type AnthropicConfig struct {
	APIKey          string
	Model           string
	Endpoint        string
	MaxOutputTokens int
	Timeout         time.Duration
	HTTPClient      *http.Client
}

type AnthropicProvider struct {
	cfg    AnthropicConfig
	client *http.Client
}

func NewAnthropicProvider(cfg AnthropicConfig) *AnthropicProvider {
	if cfg.Endpoint == "" {
		cfg.Endpoint = defaultAnthropicEndpoint
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
	return &AnthropicProvider{cfg: cfg, client: client}
}

func (p *AnthropicProvider) Generate(ctx context.Context, req Request) (Response, error) {
	if strings.TrimSpace(p.cfg.APIKey) == "" {
		return Response{}, ErrorForCode(ErrNotConfigured, "Claude Key 未配置；请在部署侧配置 ANTHROPIC_API_KEY_FILE 后重试。")
	}
	model := req.Model
	if model == "" {
		model = p.cfg.Model
	}
	if model == "" {
		return Response{}, ErrorForCode(ErrInvalidRequest, "Claude 模型未配置。")
	}
	maxTokens := req.MaxOutputTokens
	if maxTokens <= 0 {
		maxTokens = p.cfg.MaxOutputTokens
	}
	body, err := json.Marshal(anthropicRequest{
		Model:     model,
		MaxTokens: maxTokens,
		System:    req.System,
		Messages: []anthropicMessage{{
			Role:    "user",
			Content: []anthropicText{{Type: "text", Text: req.User}},
		}},
		Tools: []anthropicTool{{
			Name:        "structured_result",
			Description: "Return only the requested structured JSON result.",
			InputSchema: req.Schema,
		}},
		ToolChoice: anthropicToolChoice{Type: "tool", Name: "structured_result"},
	})
	if err != nil {
		return Response{}, WrapError(ErrInvalidRequest, "Claude 请求构造失败。", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.Endpoint, bytes.NewReader(body))
	if err != nil {
		return Response{}, WrapError(ErrInvalidRequest, "Claude 请求构造失败。", err)
	}
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("accept", "application/json")
	httpReq.Header.Set("x-api-key", p.cfg.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	start := time.Now()
	resp, err := p.client.Do(httpReq)
	latency := time.Since(start)
	if err != nil {
		var netErr net.Error
		partial := Response{Model: model, Latency: latency}
		if errors.Is(err, context.DeadlineExceeded) ||
			errors.Is(ctx.Err(), context.DeadlineExceeded) ||
			(errors.As(err, &netErr) && netErr.Timeout()) {
			return partial, WrapError(ErrTimeout, "Claude 请求超时，将自动重试。", err)
		}
		return partial, WrapError(ErrUnavailable, "Claude 暂时不可用，请稍后重试。", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return Response{}, WrapError(ErrUnavailable, "读取 Claude 响应失败，将自动重试。", err)
	}
	requestID := responseRequestID(resp.Header)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Response{Model: model, ProviderRequestID: requestID, Latency: latency},
			mapHTTPError(resp.StatusCode, resp.Header, raw)
	}
	var out anthropicResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return Response{Model: model, ProviderRequestID: requestID, Latency: latency},
			WrapError(ErrInvalidJSON, "Claude 返回了无效 JSON，将自动重试。", err)
	}
	if out.Model != "" {
		model = out.Model
	}
	providerResp := Response{
		Model:             model,
		ProviderRequestID: requestID,
		Usage: Usage{
			InputTokens:      out.Usage.InputTokens,
			OutputTokens:     out.Usage.OutputTokens,
			CacheReadTokens:  out.Usage.CacheReadInputTokens,
			CacheWriteTokens: out.Usage.CacheCreationInputTokens,
		},
		Latency:    latency,
		StopReason: out.StopReason,
	}
	if out.StopReason == "refusal" {
		return providerResp, ErrorForCode(ErrRefused, "Claude 拒绝生成该结果；请调整输入后重试。")
	}
	if out.StopReason == "max_tokens" {
		return providerResp, ErrorForCode(ErrOutputTruncated, "Claude 输出被截断；请减少输入或提高输出上限后重试。")
	}
	var structured json.RawMessage
	for _, c := range out.Content {
		if c.Type == "refusal" {
			return providerResp, ErrorForCode(ErrRefused, "Claude 拒绝生成该结果；请调整输入后重试。")
		}
		if c.Type == "tool_use" && c.Name == "structured_result" && len(c.Input) > 0 {
			structured = c.Input
			break
		}
	}
	if len(structured) == 0 || !json.Valid(structured) {
		return providerResp, ErrorForCode(ErrInvalidJSON, "Claude 未返回有效的结构化 JSON，将自动重试。")
	}
	providerResp.JSON = structured
	return providerResp, nil
}

type anthropicRequest struct {
	Model      string              `json:"model"`
	MaxTokens  int                 `json:"max_tokens"`
	System     string              `json:"system,omitempty"`
	Messages   []anthropicMessage  `json:"messages"`
	Tools      []anthropicTool     `json:"tools"`
	ToolChoice anthropicToolChoice `json:"tool_choice"`
}

type anthropicMessage struct {
	Role    string          `json:"role"`
	Content []anthropicText `json:"content"`
}

type anthropicText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type anthropicToolChoice struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type anthropicResponse struct {
	ID         string             `json:"id"`
	Model      string             `json:"model"`
	StopReason string             `json:"stop_reason"`
	Content    []anthropicContent `json:"content"`
	Usage      anthropicUsage     `json:"usage"`
}

type anthropicContent struct {
	Type  string          `json:"type"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
	Text  string          `json:"text"`
}

type anthropicUsage struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
}

func responseRequestID(h http.Header) string {
	for _, key := range []string{"request-id", "anthropic-request-id", "x-request-id"} {
		if v := h.Get(key); v != "" {
			return v
		}
	}
	return ""
}

func mapHTTPError(status int, h http.Header, raw []byte) *ProviderError {
	code := ErrUnavailable
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		code = ErrAuthFailed
	case http.StatusTooManyRequests:
		code = ErrRateLimited
	case http.StatusBadRequest, http.StatusNotFound, http.StatusMethodNotAllowed, http.StatusRequestEntityTooLarge:
		code = ErrInvalidRequest
	default:
		if status >= 500 {
			code = ErrOverloaded
		}
	}
	detail := "Claude 暂时不可用，请稍后重试。"
	switch code {
	case ErrAuthFailed:
		detail = "Claude Key 无效或权限不足；请在部署侧更新 secret。"
	case ErrRateLimited:
		detail = "Claude 触发限流；系统将按退避策略重试。"
	case ErrInvalidRequest:
		detail = "Claude 请求无效；请检查模型配置或输入大小。"
	case ErrOverloaded:
		detail = "Claude 服务繁忙；系统将自动重试。"
	}
	var payload struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(raw, &payload)
	if payload.Error.Type == "overloaded_error" {
		code = ErrOverloaded
		detail = "Claude 服务繁忙；系统将自动重试。"
	}
	err := ErrorForCode(code, detail)
	if ra := parseRetryAfter(h.Get("Retry-After")); ra > 0 {
		err.RetryAfter = ra
	}
	err.cause = fmt.Errorf("anthropic status %d type %s", status, payload.Error.Type)
	return err
}

func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
		return time.Duration(n) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d
		}
	}
	return 0
}
