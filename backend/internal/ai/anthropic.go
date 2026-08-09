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
	if req.WebFetch != nil {
		return p.generateWebFetch(ctx, req, model, maxTokens)
	}
	payload := anthropicRequest{
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
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "deepseek-") {
		payload.Thinking = &anthropicThinking{Type: "disabled"}
	}
	body, err := json.Marshal(payload)
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
	Thinking   *anthropicThinking  `json:"thinking,omitempty"`
}

type anthropicThinking struct {
	Type string `json:"type"`
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
	Name string `json:"name,omitempty"`
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

const (
	// anthropicWebFetchBeta enables the server-side Web Fetch tool on the
	// Messages API. Only URL extraction requests send this header.
	anthropicWebFetchBeta = "web-fetch-2025-09-10"
	// errMaterialTooLarge mirrors worker.ErrCodeTooLarge; the provider cannot
	// import the worker package, so the stable code string is repeated here.
	errMaterialTooLarge = "material_too_large"
	// webFetchContinuationPrompt is the single user message appended after a
	// pause_turn so the model resumes and finishes the structured result.
	webFetchContinuationPrompt = "Continue the paused turn. Finish any remaining web_fetch calls for the canonical URL, then return structured_result."
)

// generateWebFetch runs the URL-extraction Messages loop: a controlled
// server-tool/web_fetch exchange with at most MaxContinuations pause_turn
// continuations. It never logs or surfaces raw request bodies, raw content
// blocks, upstream messages, or keys.
func (p *AnthropicProvider) generateWebFetch(ctx context.Context, req Request, model string, maxTokens int) (Response, error) {
	wf := req.WebFetch
	serverTool, err := json.Marshal(map[string]any{
		"type":               "web_fetch_20250910",
		"name":               "web_fetch",
		"max_uses":           wf.MaxUses,
		"allowed_domains":    wf.AllowedDomains,
		"max_content_tokens": wf.MaxContentTokens,
	})
	if err != nil {
		return Response{}, WrapError(ErrInvalidRequest, "Claude 请求构造失败。", err)
	}
	resultTool, err := json.Marshal(anthropicTool{
		Name:        "structured_result",
		Description: "Return only the requested structured JSON result.",
		InputSchema: req.Schema,
	})
	if err != nil {
		return Response{}, WrapError(ErrInvalidRequest, "Claude 请求构造失败。", err)
	}
	tools := []json.RawMessage{serverTool, resultTool}

	firstUser, err := json.Marshal([]anthropicText{{Type: "text", Text: req.User}})
	if err != nil {
		return Response{}, WrapError(ErrInvalidRequest, "Claude 请求构造失败。", err)
	}
	messages := []anthropicRawMessage{{Role: "user", Content: firstUser}}

	maxContinuations := wf.MaxContinuations
	if maxContinuations < 0 {
		maxContinuations = 0
	}

	providerResp := Response{Model: model}
	var (
		total                   Usage
		structured              json.RawMessage
		canonicalFetchSucceeded bool
		fetchTruncated          bool
		fetchTemporary          bool
		fetchPermanent          bool
	)
	canonicalUses := map[string]bool{}
	start := time.Now()
	for continuation := 0; ; continuation++ {
		body, err := marshalAnthropicURLRequest(anthropicURLRequest{
			Model: model, MaxTokens: maxTokens, System: req.System,
			Messages: messages, Tools: tools,
			ToolChoice: anthropicToolChoice{Type: "auto"},
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
		httpReq.Header.Set("anthropic-beta", anthropicWebFetchBeta)
		resp, err := p.client.Do(httpReq)
		if err != nil {
			providerResp.Latency = time.Since(start)
			var netErr net.Error
			if errors.Is(err, context.DeadlineExceeded) ||
				errors.Is(ctx.Err(), context.DeadlineExceeded) ||
				(errors.As(err, &netErr) && netErr.Timeout()) {
				return providerResp, WrapError(ErrTimeout, "Claude 请求超时，将自动重试。", err)
			}
			return providerResp, WrapError(ErrUnavailable, "Claude 暂时不可用，请稍后重试。", err)
		}
		raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		resp.Body.Close()
		if err != nil {
			providerResp.Latency = time.Since(start)
			return providerResp, WrapError(ErrUnavailable, "读取 Claude 响应失败，将自动重试。", err)
		}
		if requestID := responseRequestID(resp.Header); requestID != "" {
			providerResp.ProviderRequestID = requestID
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			providerResp.Latency = time.Since(start)
			return providerResp, mapHTTPError(resp.StatusCode, resp.Header, raw)
		}
		var out anthropicURLResponse
		if err := json.Unmarshal(raw, &out); err != nil {
			providerResp.Latency = time.Since(start)
			return providerResp, WrapError(ErrInvalidJSON, "Claude 返回了无效 JSON，将自动重试。", err)
		}
		if out.Model != "" {
			providerResp.Model = out.Model
		}
		if out.StopReason != "" {
			providerResp.StopReason = out.StopReason
		}
		total.InputTokens += out.Usage.InputTokens
		total.OutputTokens += out.Usage.OutputTokens
		total.CacheReadTokens += out.Usage.CacheReadInputTokens
		total.CacheWriteTokens += out.Usage.CacheCreationInputTokens

		if out.StopReason == "refusal" {
			providerResp.Usage = total
			providerResp.Latency = time.Since(start)
			return providerResp, ErrorForCode(ErrRefused, "Claude 拒绝生成该结果；请调整输入后重试。")
		}
		if out.StopReason == "max_tokens" {
			providerResp.Usage = total
			providerResp.Latency = time.Since(start)
			return providerResp, ErrorForCode(ErrOutputTruncated, "Claude 输出被截断；请减少输入或提高输出上限后重试。")
		}

		var blocks []anthropicURLBlock
		if len(out.Content) > 0 {
			if err := json.Unmarshal(out.Content, &blocks); err != nil {
				providerResp.Usage = total
				providerResp.Latency = time.Since(start)
				return providerResp, WrapError(ErrInvalidJSON, "Claude 返回了无效 JSON，将自动重试。", err)
			}
		}
		refused := false
		for _, block := range blocks {
			switch block.Type {
			case "refusal":
				refused = true
			case "tool_use":
				if block.Name == "structured_result" && len(block.Input) > 0 {
					structured = block.Input
				}
			case "server_tool_use":
				if block.Name == "web_fetch" && block.ID != "" {
					var input struct {
						URL string `json:"url"`
					}
					if err := json.Unmarshal(block.Input, &input); err == nil && input.URL == wf.URL {
						canonicalUses[block.ID] = true
					}
				}
			case "web_fetch_tool_result":
				if !canonicalUses[block.ToolUseID] {
					continue
				}
				result := parseWebFetchToolResult(block.Content)
				switch {
				case result.errorCode != "":
					if webFetchErrorRetryable(result.errorCode) {
						fetchTemporary = true
					} else {
						fetchPermanent = true
					}
				case result.truncated:
					fetchTruncated = true
				default:
					canonicalFetchSucceeded = true
				}
			}
		}
		if refused {
			providerResp.Usage = total
			providerResp.Latency = time.Since(start)
			return providerResp, ErrorForCode(ErrRefused, "Claude 拒绝生成该结果；请调整输入后重试。")
		}

		if out.StopReason == "pause_turn" {
			if continuation >= maxContinuations {
				providerResp.Usage = total
				providerResp.Latency = time.Since(start)
				return providerResp, ErrorForCode(ErrTimeout, "Claude 请求超时，将自动重试。")
			}
			// Preserve the assistant content byte-for-byte so the paused turn
			// can be resumed exactly as the API requires.
			messages = append(messages, anthropicRawMessage{Role: "assistant", Content: out.Content})
			cont, err := json.Marshal([]anthropicText{{Type: "text", Text: webFetchContinuationPrompt}})
			if err != nil {
				return Response{}, WrapError(ErrInvalidRequest, "Claude 请求构造失败。", err)
			}
			messages = append(messages, anthropicRawMessage{Role: "user", Content: cont})
			continue
		}
		break
	}
	providerResp.Usage = total
	providerResp.Latency = time.Since(start)

	if len(structured) > 0 && !json.Valid(structured) {
		return providerResp, ErrorForCode(ErrInvalidJSON, "Claude 未返回有效的结构化 JSON，将自动重试。")
	}
	if len(structured) > 0 && canonicalFetchSucceeded {
		providerResp.JSON = structured
		return providerResp, nil
	}
	if fetchTruncated {
		return providerResp, ErrorForCode(errMaterialTooLarge, "链接内容超出 Claude 可提取的大小上限。")
	}
	if fetchTemporary {
		return providerResp, MaterialFetchError(true)
	}
	if fetchPermanent {
		return providerResp, MaterialFetchError(false)
	}
	if len(structured) == 0 {
		return providerResp, ErrorForCode(ErrInvalidJSON, "Claude 未返回有效的结构化 JSON，将自动重试。")
	}
	// structured_result without evidence of a successful canonical fetch.
	return providerResp, MaterialFetchError(false)
}

// anthropicURLRequest keeps message content as raw JSON so assistant content
// blocks survive continuations byte-for-byte.
type anthropicURLRequest struct {
	Model      string                `json:"model"`
	MaxTokens  int                   `json:"max_tokens"`
	System     string                `json:"system,omitempty"`
	Messages   []anthropicRawMessage `json:"messages"`
	Tools      []json.RawMessage     `json:"tools"`
	ToolChoice anthropicToolChoice   `json:"tool_choice"`
}

// marshalAnthropicURLRequest assembles the request body manually. The
// standard encoder compacts Marshaler output, which would rewrite raw
// assistant content blocks; writing them verbatim preserves them byte-for-byte.
func marshalAnthropicURLRequest(req anthropicURLRequest) ([]byte, error) {
	model, err := json.Marshal(req.Model)
	if err != nil {
		return nil, err
	}
	system, err := json.Marshal(req.System)
	if err != nil {
		return nil, err
	}
	choice, err := json.Marshal(req.ToolChoice)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.WriteString(`{"model":`)
	buf.Write(model)
	fmt.Fprintf(&buf, `,"max_tokens":%d`, req.MaxTokens)
	if req.System != "" {
		buf.WriteString(`,"system":`)
		buf.Write(system)
	}
	buf.WriteString(`,"messages":[`)
	for i, m := range req.Messages {
		role, err := json.Marshal(m.Role)
		if err != nil {
			return nil, err
		}
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.WriteString(`{"role":`)
		buf.Write(role)
		buf.WriteString(`,"content":`)
		buf.Write(m.Content)
		buf.WriteByte('}')
	}
	buf.WriteString(`],"tools":[`)
	for i, tool := range req.Tools {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.Write(tool)
	}
	buf.WriteString(`],"tool_choice":`)
	buf.Write(choice)
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

type anthropicRawMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type anthropicURLResponse struct {
	ID         string          `json:"id"`
	Model      string          `json:"model"`
	StopReason string          `json:"stop_reason"`
	Content    json.RawMessage `json:"content"`
	Usage      anthropicUsage  `json:"usage"`
}

// anthropicURLBlock covers client tool_use, server_tool_use, and
// web_fetch_tool_result content blocks.
type anthropicURLBlock struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
}

type webFetchToolResultParse struct {
	errorCode string
	truncated bool
}

func parseWebFetchToolResult(raw json.RawMessage) webFetchToolResultParse {
	var content struct {
		Type      string `json:"type"`
		ErrorCode string `json:"error_code"`
		Truncated bool   `json:"truncated"`
	}
	if err := json.Unmarshal(raw, &content); err != nil {
		// A malformed result is treated as a temporary upstream failure; its
		// raw payload is never surfaced.
		return webFetchToolResultParse{errorCode: "unavailable"}
	}
	if content.Type == "web_fetch_tool_error" {
		return webFetchToolResultParse{errorCode: content.ErrorCode}
	}
	return webFetchToolResultParse{truncated: content.Truncated}
}

// webFetchErrorRetryable classifies web_fetch_tool_error codes. Only
// transient source-access failures are retryable; access/policy rejections
// are permanent.
func webFetchErrorRetryable(code string) bool {
	switch code {
	case "too_many_requests", "unavailable", "max_uses_exceeded":
		return true
	default:
		return false
	}
}
