# Claude Code Settings Export Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Export a user-selected saved Claude gateway Key into the host Claude Code `settings.json` without returning the Key to the browser.

**Architecture:** Add a handler-local atomic writer for the fixed Claude Code settings document. The existing Key store remains the sole source of the secret; a new safe POST route selects a Key by ID. API alone receives a narrowly scoped `/claude-code` bind mount, while Settings adds an explicit confirmation action per Key card.

**Tech Stack:** Go, Gin, OpenAPI/Orval, React, Vitest/MSW, Docker Compose.

---

### Task 1: Test and Implement the Safe Settings Writer

**Files:**
- Modify: `backend/internal/claudegateway/handler.go`
- Modify: `backend/internal/claudegateway/handler_integration_test.go`
- Modify: `docs/web-rebuild/openapi.yaml`
- Regenerate: `api/openapi.yaml`

- [x] **Step 1: Write failing writer/route integration tests**

Inject a temporary `settings.json` target into the handler. Assert the selected
Key writes the exact requested document, uses `0600`, atomically replaces a
prior file, and returns only `{status,target}`. Cover no Key leakage, unknown
Key, target write failure, and `CLAUDE_GATEWAY_WRITE_ENABLED=false`.

- [x] **Step 2: Run the focused test and verify RED**

Run: `go test ./internal/claudegateway -run 'TestClaudeCodeSettings' -count=1 -v`

Expected: the route does not yet exist.

- [x] **Step 3: Add the published endpoint contract**

Declare `POST /api/v1/system/claude-keys/{key_id}/claude-code-settings` and a
safe response schema. Generate `api/openapi.yaml`; do not add a Key example.

- [x] **Step 4: Implement the writer and handler route**

Add a narrow writer dependency with `/claude-code/settings.json` supplied by
config. Serialize the fixed document, atomically replace the target with
`0600`, map filesystem errors to a stable safe Problem, and reuse the existing
write gate and Key lookup.

- [x] **Step 5: Verify GREEN**

Run: `go test ./internal/claudegateway -run 'TestClaudeCodeSettings' -count=1 -v`

Expected: all selected-Key, failure, and redaction assertions pass.

### Task 2: Generate Client and Add Explicit Settings Action

**Files:**
- Regenerate: `frontend/src/api/generated/**`
- Modify: `frontend/src/mocks/handlers.ts`
- Modify: `frontend/src/routes/-shell.test.tsx`
- Modify: `frontend/src/features/claude-gateway/claude-gateway-section.tsx`

- [x] **Step 1: Generate the typed client**

Run: `cd frontend && npm run api:gen`

- [x] **Step 2: Write a failing Settings interaction test**

Open the selected Key action, assert a confirmation is required, confirm it,
and assert a single POST reaches the selected settings-export endpoint. Build
the synthetic secret at runtime and assert it never appears in rendered text.

- [x] **Step 3: Verify RED**

Run: `cd frontend && npm run test -- src/routes/-shell.test.tsx`

Expected: the export action is absent.

- [x] **Step 4: Extend safe MSW state and UI**

Add a safe mock response with no request body or secret retention. Add a
compact Key-card action and confirmation dialog; disable it in read-only mode
and present safe server failures with `FormAlert`.

- [x] **Step 5: Verify GREEN**

Run: `cd frontend && npm run test -- src/routes/-shell.test.tsx && npm run typecheck`

Expected: confirmation and no-secret assertions pass.

### Task 3: Restrict Deployment Mount and Verify End to End

**Files:**
- Modify: `backend/internal/platform/config/config.go`
- Modify: `backend/internal/platform/config/config_test.go`
- Modify: `backend/cmd/api/main.go`
- Modify: `deploy/.env.example`
- Modify: `deploy/compose.yaml`
- Modify: `deploy/scripts/init.sh`
- Modify: `deploy/scripts/verify-secrets.sh`
- Modify: `docs/deployment.md`

- [x] **Step 1: Add a failing config test**

Verify API reads only the non-secret in-container settings target and that it
is represented by a boolean/path-safe summary; no Worker-only secret file is
read.

- [x] **Step 2: Verify RED**

Run: `go test ./internal/platform/config -run ClaudeCode -count=1 -v`

Expected: the new config field is absent.

- [x] **Step 3: Wire the API-only mount**

Initialize `CLAUDE_CODE_SETTINGS_DIR` to the invoking user's absolute
`~/.claude` path only in ignored `deploy/.env`, mount it only at
`/claude-code` in API, and set the in-container target. Do not alter an
existing host directory's mode unnecessarily.

- [x] **Step 4: Strengthen boundary checks**

Assert rendered Compose gives API the Claude Code settings mount and confirms
Worker/Web do not have it. Preserve the existing gateway and Anthropic-secret
assertions.

- [x] **Step 5: Run complete verification**

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

- [x] **Step 6: Redeploy and inspect runtime status**

Run `make docker-up`, then `docker compose --env-file deploy/.env -f deploy/compose.yaml ps`. Confirm api, worker, web, and db are healthy.
