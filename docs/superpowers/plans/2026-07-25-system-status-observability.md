# System Status Observability Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the published system-status endpoint report non-sensitive, live API/Worker capabilities so the web application can identify actionable deployment problems.

**Architecture:** A Worker writes a bounded, non-sensitive heartbeat to PostgreSQL. The API aggregates that heartbeat with database reachability, maintenance state, queue counts, recent Claude results, and local storage capacity. Claude credentials remain Worker-only and never enter database rows, logs, or HTTP responses.

**Tech Stack:** Go, Gin, pgx/PostgreSQL migrations, Docker Compose, OpenAPI contract tests.

---

### Task 1: Define behavior with integration tests

**Files:**
- Modify: `backend/internal/system/handler_integration_test.go`

- [ ] **Step 1: Add a failing status endpoint test** that expects `GET /api/v1/system/status` to validate against OpenAPI and return database/storage/maintenance state.
- [ ] **Step 2: Add a failing heartbeat test** that expects a recent Worker heartbeat to make Worker state `ok` and a missing Claude key state `unavailable`.
- [ ] **Step 3: Run** `go test -C backend ./internal/system -run SystemStatus -count=1` and confirm the endpoint/service is missing.

### Task 2: Persist and aggregate status

**Files:**
- Create: `backend/internal/platform/migrate/migrations/0004_worker_heartbeats.up.sql`
- Create: `backend/internal/platform/migrate/migrations/0004_worker_heartbeats.down.sql`
- Create: `backend/internal/system/status.go`
- Create: `backend/internal/system/heartbeat.go`
- Modify: `backend/internal/system/handler.go`
- Modify: `backend/internal/api/router.go`
- Modify: `backend/cmd/api/main.go`
- Modify: `backend/cmd/worker/main.go`

- [ ] **Step 1: Add the heartbeat schema** with only worker id, Claude capability metadata, and heartbeat timestamp.
- [ ] **Step 2: Implement bounded heartbeat upsert/loop** and start it from the Worker process.
- [ ] **Step 3: Implement status aggregation** with database, maintenance, Worker/queue, Claude, and storage signals.
- [ ] **Step 4: Register the published GET route** and pass version/data-directory dependencies from API startup.
- [ ] **Step 5: Run** `go test -C backend ./internal/system -run SystemStatus -count=1` and confirm green.

### Task 3: Validate and deploy

**Files:**
- No source changes expected.

- [ ] **Step 1: Run** formatting, package tests, API route smoke test, and OpenAPI validation.
- [ ] **Step 2: Rebuild Compose services**, wait for migrations/health checks, and seed existing demo data only if absent.
- [ ] **Step 3: Run release smoke, doctor, and a live system-status request.**
