// Package logging configures the process-wide structured logger.
//
// Output is JSON on stdout with the field set defined in
// docs/web-rebuild/03-backend-architecture.md §13:
// time, level, service, version, request_id, job_id, module, event,
// duration_ms, error_code. Secrets must never be logged; the config
// package only exposes SafeSummary for startup logs.
package logging

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

type ctxKey string

const requestIDKey ctxKey = "request_id"

// New builds the JSON slog logger for a service and installs it as default.
func New(service, version, level string) *slog.Logger {
	lvl := parseLevel(level)
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	logger := slog.New(handler).With(
		slog.String("service", service),
		slog.String("version", version),
	)
	slog.SetDefault(logger)
	return logger
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// WithRequestID returns a context carrying the request ID; the request
// logger picks it up via FromContext.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// RequestID extracts the request ID from context ("" when absent).
func RequestID(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

// FromContext returns the default logger enriched with request_id when the
// context carries one.
func FromContext(ctx context.Context) *slog.Logger {
	logger := slog.Default()
	if id := RequestID(ctx); id != "" {
		logger = logger.With(slog.String("request_id", id))
	}
	return logger
}
