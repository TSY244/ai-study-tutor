// Package ai defines model-provider independent request/result types for
// Worker-only Claude usage. Domain packages consume validated drafts; they do
// not depend on Anthropic SDK or HTTP response types.
package ai

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

// Purpose is the stable ai_runs.purpose value and prompt routing key.
type Purpose string

const (
	PurposeConnectivity       Purpose = "claude_connectivity_test"
	PurposeTeacherOutline     Purpose = "teacher_outline"
	PurposeStudentNotes       Purpose = "student_notes"
	PurposeQuizGenerate       Purpose = "quiz_generate"
	PurposeReviewQuizGenerate Purpose = "review_quiz_generate"
	PurposeAttemptGrade       Purpose = "attempt_grade"
	PurposeWrongAnswerAnalyze Purpose = "wrong_answer_analyze"
	PurposeSummaryEvaluate    Purpose = "summary_evaluate"
)

// Provider performs a single structured generation. It returns JSON conforming
// to req.Schema or a classified ProviderError. Implementations must not log
// raw prompts, raw materials, API keys, or upstream raw responses.
type Provider interface {
	Generate(ctx context.Context, req Request) (Response, error)
}

// Request describes one structured model call. System/User may contain
// learning material and must never be persisted verbatim.
type Request struct {
	Purpose               Purpose
	System                string
	User                  string
	Schema                json.RawMessage
	PromptTemplateVersion string
	SchemaVersion         string
	Model                 string
	MaxOutputTokens       int
}

// PromptHash returns a SHA-256 over the complete prompt/schema payload. The
// hash is safe for ai_runs; it allows auditing without persisting the prompt.
func (r Request) PromptHash() string {
	h := sha256.New()
	h.Write([]byte(r.Purpose))
	h.Write([]byte{0})
	h.Write([]byte(r.PromptTemplateVersion))
	h.Write([]byte{0})
	h.Write([]byte(r.SchemaVersion))
	h.Write([]byte{0})
	h.Write([]byte(r.System))
	h.Write([]byte{0})
	h.Write([]byte(r.User))
	h.Write([]byte{0})
	h.Write(r.Schema)
	return hex.EncodeToString(h.Sum(nil))
}

// RedactedLogFields exposes only non-sensitive prompt metadata.
func (r Request) RedactedLogFields() map[string]any {
	return map[string]any{
		"purpose":                 r.Purpose,
		"prompt_template_version": r.PromptTemplateVersion,
		"schema_version":          r.SchemaVersion,
		"prompt_hash":             r.PromptHash(),
		"redacted":                true,
	}
}

// Usage is provider-independent token accounting.
type Usage struct {
	InputTokens      int64
	OutputTokens     int64
	CacheReadTokens  int64
	CacheWriteTokens int64
}

// Response is the structured JSON plus non-sensitive provider metadata.
type Response struct {
	JSON              json.RawMessage
	Model             string
	ProviderRequestID string
	Usage             Usage
	Latency           time.Duration
	StopReason        string
}

const (
	ErrNotConfigured   = "claude_not_configured"
	ErrAuthFailed      = "claude_auth_failed"
	ErrRateLimited     = "claude_rate_limited"
	ErrTimeout         = "claude_timeout"
	ErrOverloaded      = "claude_overloaded"
	ErrInvalidRequest  = "claude_invalid_request"
	ErrRefused         = "claude_refused"
	ErrOutputTruncated = "claude_output_truncated"
	ErrInvalidJSON     = "claude_invalid_json"
	ErrSchemaInvalid   = "claude_schema_invalid"
	ErrDomainInvalid   = "claude_domain_invalid"
	ErrUnavailable     = "claude_unavailable"
)

// ProviderError is the only error shape Worker handlers need to understand.
// Detail is a sanitized, front-end-safe summary.
type ProviderError struct {
	Code       string
	Detail     string
	Retryable  bool
	RetryAfter time.Duration
	cause      error
}

func (e *ProviderError) Error() string {
	if e == nil {
		return ""
	}
	return e.Code + ": " + e.Detail
}

func (e *ProviderError) Unwrap() error { return e.cause }

// ErrorForCode returns the stable retry classification for a Claude failure.
func ErrorForCode(code, detail string) *ProviderError {
	return &ProviderError{Code: code, Detail: detail, Retryable: retryableCode(code)}
}

func WrapError(code, detail string, cause error) *ProviderError {
	e := ErrorForCode(code, detail)
	e.cause = cause
	return e
}

func retryableCode(code string) bool {
	switch code {
	case ErrRateLimited, ErrTimeout, ErrOverloaded, ErrInvalidJSON, ErrSchemaInvalid, ErrDomainInvalid, ErrUnavailable:
		return true
	default:
		return false
	}
}

// AsProviderError normalizes arbitrary errors into a ProviderError.
func AsProviderError(err error) *ProviderError {
	if err == nil {
		return nil
	}
	var perr *ProviderError
	if errors.As(err, &perr) {
		return perr
	}
	return WrapError(ErrUnavailable, "Claude 暂时不可用，请稍后重试。", err)
}

// NotConfiguredProvider lets Worker register AI handlers even when the secret
// is absent. Jobs fail with a clear capability-degraded code instead of
// unsupported_job_kind.
type NotConfiguredProvider struct{}

func (NotConfiguredProvider) Generate(context.Context, Request) (Response, error) {
	return Response{}, ErrorForCode(ErrNotConfigured, "Claude Key 未配置；请在部署侧配置 ANTHROPIC_API_KEY_FILE 后重试。")
}
