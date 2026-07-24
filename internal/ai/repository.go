package ai

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/TSY244/ai-study-tutor/internal/platform/database"
)

// Run is the ai_runs audit row. It intentionally stores only metadata and
// hashes, never complete prompts, materials, API keys, or upstream raw bodies.
type Run struct {
	ID                    uuid.UUID
	JobID                 uuid.UUID
	Provider              string
	Purpose               string
	Model                 string
	PromptTemplateVersion string
	SchemaVersion         string
	PromptHash            string
	AnthropicRequestID    string
	InputTokens           int64
	OutputTokens          int64
	CacheReadTokens       int64
	CacheWriteTokens      int64
	Latency               time.Duration
	StopReason            string
	Redacted              bool
	Status                string
	ErrorType             *string
}

type Repository struct{}

func NewRun(jobID uuid.UUID, req Request, resp Response, callErr error) Run {
	status := "success"
	var errorType *string
	if callErr != nil {
		status = "error"
		perr := AsProviderError(callErr)
		errorType = &perr.Code
	}
	model := resp.Model
	if model == "" {
		model = req.Model
	}
	if model == "" {
		model = "not-configured"
	}
	return Run{
		ID:                    uuid.New(),
		JobID:                 jobID,
		Provider:              "anthropic",
		Purpose:               string(req.Purpose),
		Model:                 model,
		PromptTemplateVersion: req.PromptTemplateVersion,
		SchemaVersion:         req.SchemaVersion,
		PromptHash:            req.PromptHash(),
		AnthropicRequestID:    resp.ProviderRequestID,
		InputTokens:           resp.Usage.InputTokens,
		OutputTokens:          resp.Usage.OutputTokens,
		CacheReadTokens:       resp.Usage.CacheReadTokens,
		CacheWriteTokens:      resp.Usage.CacheWriteTokens,
		Latency:               resp.Latency,
		StopReason:            resp.StopReason,
		Redacted:              true,
		Status:                status,
		ErrorType:             errorType,
	}
}

func (Repository) Create(ctx context.Context, q database.Querier, r Run) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	if r.Provider == "" {
		r.Provider = "anthropic"
	}
	if r.Model == "" {
		r.Model = "not-configured"
	}
	if r.Status == "" {
		r.Status = "success"
	}
	_, err := q.Exec(ctx, `
		INSERT INTO ai_runs (
			id, job_id, provider, purpose, model, prompt_template_version,
			schema_version, prompt_hash, anthropic_request_id,
			input_tokens, output_tokens, cache_read_tokens, cache_write_tokens,
			latency_ms, stop_reason, redacted, status, error_type)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`,
		r.ID, r.JobID, r.Provider, r.Purpose, r.Model, r.PromptTemplateVersion,
		r.SchemaVersion, r.PromptHash, nullableString(r.AnthropicRequestID),
		nullableInt(r.InputTokens), nullableInt(r.OutputTokens), nullableInt(r.CacheReadTokens), nullableInt(r.CacheWriteTokens),
		r.Latency.Milliseconds(), nullableString(r.StopReason), r.Redacted, r.Status, r.ErrorType)
	return err
}

func (Repository) MarkError(ctx context.Context, q database.Querier, id uuid.UUID, errorType string) error {
	_, err := q.Exec(ctx, `
		UPDATE ai_runs
		SET status = 'error', error_type = $2
		WHERE id = $1`,
		id, nullableString(errorType))
	return err
}

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func nullableInt(n int64) *int64 {
	if n == 0 {
		return nil
	}
	return &n
}
