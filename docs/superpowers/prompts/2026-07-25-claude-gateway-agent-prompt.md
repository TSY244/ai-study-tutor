# Agent Prompt: Implement Claude Gateway Config Center

You are working in `/Users/august/code/vsc/python/learning/ai-study-tutor`.

Implement the Claude-only gateway configuration center described in:

`docs/superpowers/specs/2026-07-25-claude-gateway-config-center-design.md`

## Mission

Add Settings-page management for multiple Anthropic Claude API keys, and route Worker Claude calls through a local gateway that automatically fails over when one key is unavailable.

This is Anthropic Claude only. Do not add OpenAI-compatible providers, OpenRouter, Gemini, DeepSeek, or generic provider protocol translation.

## Hard Constraints

- The browser must never receive a full API key.
- The browser must never save keys in localStorage, sessionStorage, diagnostics, snapshots, or build env.
- Full keys must never appear in logs, job errors, API error responses, OpenAPI examples, MSW fixtures, or tests.
- Web container must not mount gateway config files.
- API and Worker may share a local config mount for the Claude gateway.
- Existing endpoint `POST /api/v1/system/claude-tests` must keep working and should route through the gateway.
- Existing non-AI features must keep working when no Claude key is configured.
- Preserve unrelated local changes in the worktree.

## Relevant Existing Files

Read these before editing:

- `frontend/src/routes/_shell/settings.tsx`
- `frontend/src/routes/-shell.test.tsx`
- `backend/internal/ai/anthropic.go`
- `backend/internal/ai/provider.go`
- `backend/cmd/worker/main.go`
- `backend/internal/system/handler.go`
- `backend/internal/system/status.go`
- `backend/internal/api/router.go`
- `backend/internal/platform/config/config.go`
- `docs/web-rebuild/openapi.yaml`
- `tools/openapi/openapi_contract.py`
- `deploy/compose.yaml`
- `deploy/scripts/init.sh`
- `deploy/scripts/verify-secrets.sh`

## Suggested Architecture

Create a new backend package:

- `backend/internal/claudegateway/store.go`
- `backend/internal/claudegateway/router.go`
- `backend/internal/claudegateway/state.go`
- `backend/internal/claudegateway/handler.go`

The package should provide:

- A file-backed store for `keys.json`.
- Safe response DTOs with `masked_key` and `fingerprint`, never `api_key`.
- Atomic writes with `0600` file permissions.
- A gateway provider that implements the existing AI provider interface.
- Priority failover with weighted selection within the same priority.
- Runtime status transitions for `healthy`, `cooling_down`, `auth_failed`, `disabled`, `testing`, and `unknown`.

Use the existing Anthropic provider for actual API calls. The gateway chooses a key, builds an Anthropic provider config for that attempt, and retries failover-eligible errors against the next candidate.

## API Contract

Add OpenAPI-backed endpoints:

```http
GET    /api/v1/system/claude-keys
POST   /api/v1/system/claude-keys
PATCH  /api/v1/system/claude-keys/{id}
DELETE /api/v1/system/claude-keys/{id}
POST   /api/v1/system/claude-keys/{id}/test
```

Keep:

```http
POST /api/v1/system/claude-tests
```

Rules:

- `POST` requires `api_key`, `name`, and `model`.
- `PATCH` may omit `api_key`; omitted means keep the existing key.
- `PATCH` with `api_key` replaces the existing key.
- Empty or whitespace-only keys are invalid.
- Responses return only masked/fingerprint fields.

Regenerate the published contract with `make generate-openapi`; do not edit `api/openapi.yaml` by hand.

## Frontend Work

Extend `frontend/src/routes/_shell/settings.tsx`.

Add a "Claude 配置中心" section with:

- Summary: enabled count, healthy count, routing strategy, latest success.
- Key list: name, masked key, model, priority, weight, status, last error, latest check.
- Add/edit dialog.
- Enable/disable action.
- Delete confirmation.
- Single-key test action.
- Existing global connectivity test, clarified as gateway-level test.

Use existing UI components and conventions:

- `Button`
- `Badge`
- `Dialog`
- `ConfirmDialog`
- `Input`
- `Label`
- `FormAlert`
- `Skeleton`
- `ErrorState`

## Compose and Config

Add a shared local config directory for API and Worker, not Web:

```text
/config/claude-gateway/keys.json
```

Recommended env/config fields:

- `CLAUDE_GATEWAY_CONFIG_FILE=/config/claude-gateway/keys.json`
- `CLAUDE_GATEWAY_WRITE_ENABLED=true`

Keep `ANTHROPIC_API_KEY_FILE` as a fallback when gateway config has no enabled key.

Update `deploy/scripts/init.sh` to create the host-side config directory with safe permissions.

Update `deploy/scripts/verify-secrets.sh` so it verifies:

- Worker has access to Claude config.
- API has access to Claude config only when write-enabled config is expected.
- Web has no Claude key secret and no gateway config mount.

## Testing Expectations

Write tests before implementation where practical.

Backend tests:

- Store never exposes full keys in safe views.
- Store writes atomically and preserves existing keys when `PATCH` omits `api_key`.
- Router skips disabled keys.
- Router marks `401/403` as `auth_failed`.
- Router cools down `429`, timeout, network error, and Anthropic `5xx`.
- Router fails over to the next enabled healthy key.
- Existing `POST /api/v1/system/claude-tests` still creates the same job shape.

Frontend tests:

- Settings page lists Claude keys without full key values.
- Add key submits the full key to the API but clears the input after success.
- Edit without entering a new key preserves the old key.
- Delete requires confirmation.
- Single-key test calls the correct endpoint.
- Global Claude test still creates the existing job and navigates to `/jobs/:jobId`.

Verification commands:

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

If a command cannot run because of local services, dependencies, or sandbox limits, record exactly what was attempted and what failed.

## Definition of Done

- Multiple Claude keys can be configured from Settings.
- Worker Claude calls use the gateway and fail over on key/provider errors according to the spec.
- No full key is returned or logged.
- Existing Claude connectivity test still works.
- OpenAPI generated client, backend tests, frontend tests, and deployment secret checks are updated.
- The final response lists changed files and verification results.
