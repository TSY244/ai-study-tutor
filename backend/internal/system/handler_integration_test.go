package system_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/TSY244/ai-study-tutor/backend/internal/apitest"
	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
	"github.com/TSY244/ai-study-tutor/backend/internal/idempotency"
	"github.com/TSY244/ai-study-tutor/backend/internal/job"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/databasetest"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/httpserver"
	system "github.com/TSY244/ai-study-tutor/backend/internal/system"
)

func TestClaudeConnectivityCreatesIdempotentWorkerJob(t *testing.T) {
	pool := databasetest.New(t)
	engine := apitest.NewEngine(t)
	system.NewHandler(job.NewService(pool), &httpx.IdemExecutor{Svc: idempotency.NewService(pool)},
		system.NewStatusService(pool, t.TempDir(), "test-version")).
		RegisterRoutes(engine.Group("/api/v1"), httpserver.BodyLimit(1024, http.StatusBadRequest, "bad_request"))
	key := uuid.NewString()
	rec := apitest.Do(t, engine, http.MethodPost, "/api/v1/system/claude-tests", map[string]string{"Idempotency-Key": key}, "")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	contract := apitest.Load(t)
	contract.ValidateJSONResponse(t, http.MethodPost, "/api/v1/system/claude-tests", http.StatusAccepted, rec.Body.Bytes())
	var body struct {
		Job struct {
			ID   string `json:"id"`
			Kind string `json:"kind"`
		} `json:"job"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Job.Kind != job.KindClaudeConnectivityTest || body.Job.ID == "" {
		t.Fatalf("job=%+v", body.Job)
	}
	jobID, err := uuid.Parse(body.Job.ID)
	if err != nil {
		t.Fatal(err)
	}
	var kind string
	if err := pool.QueryRow(context.Background(), `SELECT kind FROM jobs WHERE id=$1`, jobID).Scan(&kind); err != nil {
		t.Fatal(err)
	}
	if kind != job.KindClaudeConnectivityTest {
		t.Fatalf("stored kind=%s", kind)
	}

	replay := apitest.Do(t, engine, http.MethodPost, "/api/v1/system/claude-tests", map[string]string{"Idempotency-Key": key}, "")
	var replayBody struct {
		Job struct {
			ID string `json:"id"`
		} `json:"job"`
	}
	if err := json.Unmarshal(replay.Body.Bytes(), &replayBody); err != nil {
		t.Fatal(err)
	}
	if replay.Code != http.StatusAccepted || replayBody.Job.ID != body.Job.ID {
		t.Fatalf("idempotency replay=%d %+v", replay.Code, replayBody)
	}
}

func TestSystemStatusReportsRuntimeCapabilities(t *testing.T) {
	pool := databasetest.New(t)
	engine := apitest.NewEngine(t)
	system.NewHandler(
		job.NewService(pool),
		&httpx.IdemExecutor{Svc: idempotency.NewService(pool)},
		system.NewStatusService(pool, t.TempDir(), "test-version"),
	).RegisterRoutes(engine.Group("/api/v1"), httpserver.BodyLimit(1024, http.StatusBadRequest, "bad_request"))

	rec := apitest.Do(t, engine, http.MethodGet, "/api/v1/system/status", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	apitest.Load(t).ValidateJSONResponse(t, http.MethodGet, "/api/v1/system/status", http.StatusOK, rec.Body.Bytes())

	var body struct {
		Status      string `json:"status"`
		Version     string `json:"version"`
		Maintenance struct {
			Enabled bool `json:"enabled"`
		} `json:"maintenance"`
		Database struct {
			Status string `json:"status"`
		} `json:"database"`
		Worker struct {
			Status string `json:"status"`
		} `json:"worker"`
		Claude struct {
			Status string `json:"status"`
		} `json:"claude"`
		Storage struct {
			Status string `json:"status"`
		} `json:"storage"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "degraded" || body.Version != "test-version" || body.Maintenance.Enabled {
		t.Fatalf("system status=%+v", body)
	}
	if body.Database.Status != "ok" || body.Worker.Status != "unknown" || body.Claude.Status != "unknown" || body.Storage.Status != "ok" {
		t.Fatalf("components=%+v", body)
	}
}

func TestSystemStatusUsesWorkerHeartbeatForClaudeCapability(t *testing.T) {
	pool := databasetest.New(t)
	if err := system.UpsertWorkerHeartbeat(context.Background(), pool, system.WorkerHeartbeat{
		WorkerID:      "status-test-worker",
		ClaudeStatus:  "unavailable",
		ClaudeMessage: "Claude Key 未配置；请在部署侧配置 ANTHROPIC_API_KEY_FILE 后重试。",
	}); err != nil {
		t.Fatal(err)
	}
	engine := apitest.NewEngine(t)
	system.NewHandler(
		job.NewService(pool),
		&httpx.IdemExecutor{Svc: idempotency.NewService(pool)},
		system.NewStatusService(pool, t.TempDir(), "test-version"),
	).RegisterRoutes(engine.Group("/api/v1"), httpserver.BodyLimit(1024, http.StatusBadRequest, "bad_request"))

	rec := apitest.Do(t, engine, http.MethodGet, "/api/v1/system/status", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Status string `json:"status"`
		Worker struct {
			Status          string  `json:"status"`
			LastHeartbeatAt *string `json:"last_heartbeat_at"`
		} `json:"worker"`
		Claude struct {
			Status  string  `json:"status"`
			Message *string `json:"message"`
		} `json:"claude"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "degraded" || body.Worker.Status != "ok" || body.Worker.LastHeartbeatAt == nil {
		t.Fatalf("worker status=%+v", body)
	}
	if body.Claude.Status != "unavailable" || body.Claude.Message == nil {
		t.Fatalf("claude status=%+v", body)
	}
}
