# Claude Gateway Config Center Implementation Plan

**Status:** Completed 2026-07-25. All implementation and verification steps below passed; the temporary browser verification servers were stopped.

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the partially implemented Claude-only multi-key gateway configuration center without exposing full API keys or regressing the existing Claude connectivity job.

**Architecture:** Keep `docs/web-rebuild/openapi.yaml` as the API source of truth, use the existing file-backed `internal/claudegateway` package for secret storage and runtime state, and route Worker calls through an `ai.Provider` implementation that reloads the shared config file. Extend the existing Settings route with generated API calls and local, ephemeral form state only; API and Worker share the config mount while Web remains outside the secret boundary.

**Tech Stack:** Go 1.24, Gin, PostgreSQL job queue, OpenAPI/Orval, React 19, TanStack Query/Router, Vitest/MSW, Docker Compose.

**Worktree note:** The requested feature is already partially implemented as uncommitted work on `main`. Preserve all existing and unrelated changes, work in place, and do not create commits unless the user explicitly requests them.

---

### Task 1: Lock Down Store, State, and Routing Semantics

**Files:**
- Create: `backend/internal/claudegateway/store_test.go`
- Create: `backend/internal/claudegateway/router_test.go`
- Modify: `backend/internal/claudegateway/store.go`
- Modify: `backend/internal/claudegateway/state.go`
- Modify: `backend/internal/claudegateway/router.go`

- [ ] **Step 1: Write failing store security and persistence tests**

Cover safe JSON serialization without `api_key`, masking/fingerprinting, `0600` atomic writes, reload after replacement, PATCH without `api_key` preserving the exact stored secret, and blank replacement rejection. Use synthetic tokens assembled at runtime and assert that neither responses nor error text contains them.

- [ ] **Step 2: Run store tests and verify the intended failures**

Run: `go test ./internal/claudegateway -run 'TestStore|TestSafe' -count=1 -v`

Expected: new assertions fail only where the partial implementation is incomplete.

- [ ] **Step 3: Write failing routing/state tests**

Use an `httptest.Server` to simulate Anthropic responses. Cover disabled-key skipping, ascending priority, deterministic weighted same-priority distribution, one attempt per eligible key, 401/403 `auth_failed`, 429/5xx/timeout/network cooldown, failover to the next eligible key, non-failover for refusal/truncation/invalid JSON, cooldown expiry, replacement-key reset, successful manual-test recovery from `auth_failed`, and legacy fallback only when there are no enabled configured keys.

- [ ] **Step 4: Run routing tests and verify RED**

Run: `go test ./internal/claudegateway -run 'TestRouter|TestState' -count=1 -v`

Expected: failures expose the current fallback and state-transition gaps rather than test setup errors.

- [ ] **Step 5: Implement the minimum store/router/state corrections**

Keep full keys confined to `Key.APIKey`, config JSON, and request headers. Never add key material to logging or returned errors. Distinguish "no enabled configured key" from "enabled keys exist but are temporarily/auth unavailable" before invoking `Legacy`.

- [ ] **Step 6: Verify Task 1 GREEN**

Run: `go test ./internal/claudegateway -count=1 -v`

Expected: all package tests pass with no secret values in output.

### Task 2: Make Single-Key Connectivity Jobs Target the Selected Key

**Files:**
- Modify: `backend/cmd/worker/main.go`
- Create or modify: `backend/cmd/worker/main_test.go`
- Modify: `backend/internal/worker/aihandlers/handler.go`
- Modify: `backend/internal/worker/aihandlers/handler_integration_test.go`
- Modify: `backend/internal/job/service.go`
- Modify: `backend/internal/system/handler_integration_test.go`

- [ ] **Step 1: Write a failing Worker test for `payload.key_id`**

Provide a test provider implementing both normal `Generate` and targeted `GenerateForKey`, enqueue a `claude_connectivity_test` job containing a non-secret key ID, and assert only the targeted method receives that ID while the normal global test still calls `Generate`.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/worker/aihandlers -run 'TestConnectivity' -count=1 -v`

Expected: the targeted test fails because the current handler ignores `key_id`; the existing global job test remains green.

- [ ] **Step 3: Add a narrow optional targeted-provider interface and audit-safe call path**

Parse an optional `key_id` from the connectivity job payload. If present, require a provider that supports `GenerateForKey`; otherwise preserve the current `Provider.Generate` behavior and the same AI-run/job result shape. Invalid payloads must produce a sanitized stable error without echoing data.

- [ ] **Step 4: Strengthen the existing API job-shape test**

Assert `POST /api/v1/system/claude-tests` still stores `{}` as payload and returns the existing `claude_connectivity_test` accepted response. Add coverage that the single-key service path stores only `{"key_id":"..."}`.

- [ ] **Step 5: Verify the production Worker provider assembly**

Extract only the minimal provider-construction seam needed for a unit test, or otherwise test the existing assembly directly. Prove that Anthropic mode plus `CLAUDE_GATEWAY_CONFIG_FILE` constructs a `claudegateway.Router`, that the router sees config-file updates without restart, and that an empty/no-enabled gateway uses the legacy provider while configured-but-unavailable enabled keys do not. Keep fake-provider and no-key degradation behavior unchanged.

- [ ] **Step 6: Run integration-level 401 and 429 failover cases through the production gateway type**

Use a temporary `keys.json` and an `httptest.Server` to prove the provider selected by Worker assembly fails over from the first configured key to the second for both authentication failure and rate limiting. Assert the first key state changes to `auth_failed` or `cooling_down` respectively and no request/error/log output contains either synthetic secret.

- [ ] **Step 7: Verify Task 2 GREEN**

Run: `go test ./cmd/worker ./internal/worker/aihandlers ./internal/system -count=1 -v`

Expected: global and targeted connectivity paths both pass and retain the established job contract.

### Task 3: Verify the Management API and Published Contract

**Files:**
- Create: `backend/internal/claudegateway/handler_integration_test.go`
- Modify: `backend/internal/claudegateway/handler.go`
- Modify: `backend/internal/api/router_test.go`
- Modify: `backend/internal/platform/config/config_test.go`
- Modify: `docs/web-rebuild/openapi.yaml`
- Regenerate: `api/openapi.yaml`

- [ ] **Step 1: Write failing handler/contract tests**

Register the handler against the project test engine and database. Cover list/create/update/delete/test routes, `400` for blank keys, omitted update key preservation, write-disabled `403`, not-found behavior, safe response DTOs, and contract validation. Check response bodies and serialized fixtures for forbidden `api_key` fields and synthetic secret values.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/claudegateway ./internal/api ./internal/platform/config -count=1 -v`

Expected: tests fail only for missing or incorrect partial API behavior.

- [ ] **Step 3: Implement minimal handler/config/router fixes**

Follow existing `httpx`, idempotency, body-limit, and Problem Details conventions. Keep `POST` and single-key test idempotent; do not add full keys to OpenAPI examples, test logs, or errors.

- [ ] **Step 4: Regenerate and verify OpenAPI**

Run: `make generate-openapi && make verify-openapi && make check-openapi-breaking`

Expected: published contract is generated from the design source, references resolve, and only additive changes are reported.

- [ ] **Step 5: Verify Task 3 GREEN**

Run: `go test ./internal/claudegateway ./internal/api ./internal/platform/config -count=1 -v`

Expected: all targeted tests pass.

### Task 4: Build the Settings Configuration Center Test First

**Files:**
- Regenerate: `frontend/src/api/generated/**`
- Modify: `frontend/src/mocks/fixtures.ts`
- Modify: `frontend/src/mocks/handlers.ts`
- Modify: `frontend/src/routes/-shell.test.tsx`
- Modify: `frontend/src/routes/_shell/settings.tsx`

- [ ] **Step 1: Generate the typed client from the published contract**

Run: `cd frontend && npm run api:gen`

Expected: Claude key models and System endpoint functions are generated; generated files are not hand-edited.

- [ ] **Step 2: Add safe, stateful Claude gateway MSW data**

Use only masked keys and fingerprints in response fixtures. Request handlers may receive a full key transiently to simulate create/update, but must discard it immediately and never copy it into response state, diagnostics, snapshots, or logs.

- [ ] **Step 3: Write failing Settings interaction tests**

Cover safe key listing, summary counts/latest success, add submission followed by password-field clearing, edit without `api_key`, optional replacement, enable/disable confirmation, delete confirmation, correct single-key test endpoint plus navigation, write-disabled controls, and the existing global gateway test navigation.

- [ ] **Step 4: Verify frontend RED**

Run: `cd frontend && npm run test -- src/routes/-shell.test.tsx`

Expected: new configuration-center tests fail because the UI is not yet implemented while the existing global connectivity test remains green.

- [ ] **Step 5: Implement the configuration center**

Extend the existing Settings route using `Button`, `Badge`, `Dialog`, `ConfirmDialog`, `Input`, `Label`, `FormAlert`, `Skeleton`, and `ErrorState`. Keep the operational layout compact and responsive; use icon buttons with accessible labels/tooltips where the action is familiar. The password field is ephemeral React form state, cleared on success and dialog close, and never persisted.

- [ ] **Step 6: Verify Task 4 GREEN and static checks**

Run: `cd frontend && npm run typecheck && npm run test -- src/routes/-shell.test.tsx && npm run build`

Expected: typecheck, focused tests, and production build pass.

### Task 5: Deployment Boundary and End-to-End Verification

**Files:**
- Modify: `deploy/compose.yaml`
- Modify: `deploy/scripts/init.sh`
- Modify: `deploy/scripts/verify-secrets.sh`
- Modify: `deploy/.env.example`
- Modify: `docs/deployment.md`

- [ ] **Step 1: Audit deployment changes against the secret boundary**

Confirm the host config directory is initialized as `0700`, files are written `0600`, API and Worker mount `/config/claude-gateway`, Worker is not write-enabled, and Web has no gateway mount or Claude/Anthropic secret environment.

- [ ] **Step 2: Run deployment checks and make only evidence-driven fixes**

Run: `make docker-config && make docker-verify-secrets`

Expected: Compose renders and the boundary script verifies API/Worker/Web independently.

- [ ] **Step 3: Run the complete acceptance command set**

Run in repository root unless noted:

```bash
make generate-openapi
make verify-openapi
make check-openapi-breaking
make go-fmt
make go-vet
make go-test
cd frontend && npm run api:gen
cd frontend && npm run typecheck
cd frontend && npm run test
cd frontend && npm run build
make docker-config
make docker-verify-secrets
```

Expected: all available commands pass; any infrastructure or sandbox blocker is reported with the exact command and failure.

- [ ] **Step 4: Inspect the running Settings page at desktop and mobile widths**

Start the existing Vite development server, open `/settings` with MSW enabled, verify dialogs/actions/statuses visually, check for overlap/overflow, and confirm browser-visible data contains only masked keys/fingerprints.

- [ ] **Step 5: Re-read the prompt and spec as a final security checklist**

Search tracked and changed frontend/API/test/deployment files for accidental full-key-shaped literals, `localStorage`, `sessionStorage`, build env key exposure, `api_key` response fields, and Web mounts. Review `git diff` to ensure unrelated local changes were preserved.
