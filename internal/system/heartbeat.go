package system

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// WorkerHeartbeat is the non-sensitive capability signal written by a Worker.
// It intentionally does not carry secrets, prompts, or learning content.
type WorkerHeartbeat struct {
	WorkerID      string
	ClaudeStatus  string
	ClaudeMessage string
	ClaudeModel   string
}

// UpsertWorkerHeartbeat records that a Worker is alive and exposes only its
// Claude capability state to the API status endpoint.
func UpsertWorkerHeartbeat(ctx context.Context, pool *pgxpool.Pool, h WorkerHeartbeat) error {
	if pool == nil {
		return errors.New("worker heartbeat requires a database pool")
	}
	h.WorkerID = strings.TrimSpace(h.WorkerID)
	if h.WorkerID == "" {
		return errors.New("worker heartbeat requires a worker id")
	}
	switch h.ClaudeStatus {
	case "ok", "degraded", "unavailable", "unknown":
	default:
		return errors.New("worker heartbeat has an invalid Claude status")
	}
	_, err := pool.Exec(ctx, `
		INSERT INTO worker_heartbeats (
			worker_id, claude_status, claude_message, claude_model, last_heartbeat_at
		) VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), now())
		ON CONFLICT (worker_id) DO UPDATE
		SET claude_status = EXCLUDED.claude_status,
			claude_message = EXCLUDED.claude_message,
			claude_model = EXCLUDED.claude_model,
			last_heartbeat_at = EXCLUDED.last_heartbeat_at`,
		h.WorkerID, h.ClaudeStatus, h.ClaudeMessage, h.ClaudeModel)
	return err
}
