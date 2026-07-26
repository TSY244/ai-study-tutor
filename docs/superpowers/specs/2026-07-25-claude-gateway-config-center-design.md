# Claude Gateway Config Center Design

**Date:** 2026-07-25

**Status:** Proposed

**Scope:** Anthropic Claude only. This design does not include OpenAI-compatible providers, OpenRouter, DeepSeek, Gemini, or generic provider protocol translation.

## Goal

Add a Claude configuration center to the existing Settings page so a local user can manage multiple Anthropic Claude API keys and let the Worker automatically fail over when the selected key is unavailable.

The result should feel like a small, local "Claude gateway" inside this application: the user configures several Claude keys, the Worker routes Claude calls through the healthiest available key, and the UI shows non-sensitive status for each key.

## Existing Context

The current application is a Go API/Worker plus Vite React frontend. Claude calls are Worker-owned. The frontend currently has a Settings page at `frontend/src/routes/_shell/settings.tsx` that displays Claude status and starts a Claude connectivity-test job through `POST /api/v1/system/claude-tests`.

Important existing boundaries:

- The browser must not call Anthropic directly.
- The browser must not store Claude keys in localStorage, sessionStorage, build env, logs, or test snapshots.
- Full keys must not be returned by API responses.
- Full keys must not be written to application logs or job errors.
- Existing AI task semantics should remain compatible with `claude_not_configured`, `claude_unavailable`, `claude_auth_failed`, `claude_rate_limited`, `claude_timeout`, and related provider errors.

## Non-Goals

- No non-Anthropic provider support.
- No provider marketplace.
- No OpenAI-compatible or OpenRouter-compatible protocol adapter.
- No usage billing dashboard.
- No per-user authentication model; the deployment is still local/single-user.
- No browser-side secret storage.

## Product Behavior

The Settings page gains a "Claude 配置中心" section.

The user can:

- Add a Claude key.
- Edit a key's display name, model, priority, weight, enabled state, timeout, cooldown, and failure threshold.
- Replace a key value without seeing the old full value.
- Delete or disable a key with confirmation.
- Test a single key.
- Run the existing global Claude connectivity test, which now uses gateway routing.
- See each key's status, masked key, fingerprint, last success, last failure, last error code, cooldown-until time, and recent latency.

The user cannot:

- Read a complete saved key back from the UI or API.
- Configure a provider other than Anthropic Claude.

## Data Model

Each saved key is represented as a `ClaudeKey` record in the gateway config file.

```json
{
  "id": "ck_01J4EXAMPLE",
  "name": "Claude primary",
  "api_key": "sk-ant-...",
  "model": "claude-sonnet-4-5",
  "enabled": true,
  "priority": 10,
  "weight": 100,
  "timeout_seconds": 300,
  "cooldown_seconds": 300,
  "max_consecutive_failures": 2,
  "created_at": "2026-07-25T10:00:00Z",
  "updated_at": "2026-07-25T10:00:00Z"
}
```

The API response shape must replace `api_key` with safe presentation fields:

```json
{
  "id": "ck_01J4EXAMPLE",
  "name": "Claude primary",
  "masked_key": "sk-ant-...WXYZ",
  "fingerprint": "sha256:ab12cd",
  "model": "claude-sonnet-4-5",
  "enabled": true,
  "priority": 10,
  "weight": 100,
  "status": "healthy",
  "last_error_code": null,
  "last_success_at": "2026-07-25T10:10:00Z",
  "last_failure_at": null,
  "cooldown_until": null,
  "last_latency_ms": 900
}
```

Runtime state is separate from the secret-bearing config. It may live in memory with a persisted non-sensitive snapshot if useful.

Runtime state fields:

- `status`: `unknown | healthy | cooling_down | auth_failed | disabled | testing`
- `consecutive_failures`
- `last_success_at`
- `last_failure_at`
- `last_error_code`
- `cooldown_until`
- `last_latency_ms`

## Routing Policy

The first version supports one routing strategy: priority failover with weighted selection inside the same priority.

Selection algorithm:

1. Load enabled keys from the current gateway config.
2. Exclude keys in `disabled`, `auth_failed`, or active `cooling_down` state.
3. Sort by ascending `priority`.
4. Within the lowest available priority group, pick by weighted round-robin.
5. Attempt the Claude request through that key.
6. If the error is failover-eligible, mark key state and retry with the next eligible key.
7. If no eligible keys remain, return a mapped provider error.

Failover-eligible errors:

- Anthropic `401` or `403`: mark `auth_failed`; do not auto-recover without user update or manual test.
- Anthropic `429`: mark `cooling_down` until cooldown expires.
- Anthropic `5xx`: mark `cooling_down`.
- Request timeout: mark `cooling_down`.
- Network error: mark `cooling_down`.

Non-failover default behavior:

- `refusal`: do not fail over by default.
- `max_tokens` / output truncated: do not fail over by default.
- `invalid_json` / schema validation drift: record the failed run, but do not assume another key will fix the response unless the implementation has a narrow reason to retry.

The gateway should cap attempts to one attempt per eligible key per logical request.

## Backend API

Add OpenAPI-backed endpoints under the existing System area:

```http
GET    /api/v1/system/claude-keys
POST   /api/v1/system/claude-keys
PATCH  /api/v1/system/claude-keys/{id}
DELETE /api/v1/system/claude-keys/{id}
POST   /api/v1/system/claude-keys/{id}/test
```

Keep the existing endpoint:

```http
POST /api/v1/system/claude-tests
```

`POST /api/v1/system/claude-tests` remains a background job, but the Worker implementation should use the gateway instead of a single startup key.

Request rules:

- Creating a key requires `api_key`, `name`, and `model`.
- Updating a key may omit `api_key`; omitted means keep the existing secret.
- Updating a key with `api_key` replaces the existing secret.
- API key strings are trimmed before validation.
- Empty keys are rejected with `400 bad_request`.
- Error responses must not include the key value.

Response rules:

- Never include full `api_key`.
- Return masked key and fingerprint only.
- Include runtime state and last non-sensitive diagnostics.

## Storage

Recommended local storage:

```text
/config/claude-gateway/
  keys.json
```

Permissions:

```text
/config/claude-gateway/ 0700
keys.json               0600
```

The config file should be written atomically:

1. Serialize the new config to a temp file in the same directory.
2. Apply `0600`.
3. `fsync` when practical.
4. Rename over `keys.json`.

The file must be mounted only into API and Worker containers. The Web container must not mount it.

The first implementation may rely on OS file permissions. Optional field-level encryption can be added later, but is not part of this scope.

## Runtime Reloading

The Worker must not require a restart after a key is added, removed, disabled, or updated.

Recommended approach:

- Introduce a `ClaudeGateway` provider that implements the existing AI `Provider` interface.
- The gateway loads `keys.json` through a small store abstraction.
- The store caches parsed config briefly or reloads when file mtime changes.
- The gateway creates an Anthropic provider per selected key for each attempt, or uses a small provider cache keyed by key fingerprint plus model.

## Compatibility

The existing `ANTHROPIC_API_KEY_FILE` path remains as a bootstrap fallback.

Compatibility behavior:

- If `keys.json` has at least one enabled key, use gateway config.
- If `keys.json` is missing or contains no enabled keys, fall back to the existing `ANTHROPIC_API_KEY_FILE` behavior.
- The Settings page may show a non-sensitive notice when only legacy single-key configuration is detected.
- A later implementation may add "import legacy key" as a convenience, but direct import is optional for the first pass.

## Frontend Design

Extend `frontend/src/routes/_shell/settings.tsx` instead of adding a new primary route.

The "Claude 配置中心" section contains:

- Summary row: enabled count, healthy count, current routing mode, latest success time.
- Key table/list: name, masked key, model, priority, weight, status, last error, last checked time.
- Actions: add, edit, disable/enable, delete, test.
- Existing global "测试 Claude 连接" button, now labeled clearly as testing the gateway route.

Use existing UI primitives where possible:

- `Button`
- `Badge`
- `Dialog` / `ConfirmDialog`
- `Input`
- `Label`
- `FormAlert`
- `Skeleton`
- `ErrorState`

Accessibility expectations:

- Key inputs use password fields.
- Show/hide key value is a button with an icon and accessible label.
- Destructive delete uses a confirmation dialog.
- Status is never color-only; text labels must be present.

## Security Requirements

- Full keys never appear in API responses.
- Full keys never appear in logs.
- Full keys never appear in job errors.
- Full keys never appear in frontend tests, fixtures, diagnostics, or snapshots.
- Config write endpoints must use the same same-origin API path as the rest of the app.
- The Web container must not receive gateway config mounts or secret env vars.
- `SafeSummary` must continue to report only booleans/counts/fixed metadata.

## Implementation Units

Likely backend additions:

- `backend/internal/claudegateway/store.go`: load/save config and safe views.
- `backend/internal/claudegateway/router.go`: select keys and perform failover.
- `backend/internal/claudegateway/state.go`: runtime status transitions.
- `backend/internal/claudegateway/handler.go`: HTTP handlers for settings APIs.
- `backend/internal/claudegateway/*_test.go`: unit and integration tests.

Likely backend modifications:

- `backend/internal/api/router.go`: register Claude key management routes.
- `backend/cmd/worker/main.go`: use gateway provider when config path is present.
- `backend/internal/platform/config/config.go`: add gateway config path and write-enabled settings.
- `docs/web-rebuild/openapi.yaml`: add contract definitions and paths.
- `api/openapi.yaml`: regenerate only through `make generate-openapi`.
- `deploy/compose.yaml`: mount `/config/claude-gateway` into API and Worker only.
- `deploy/scripts/init.sh`: initialize config directory with safe permissions.
- `deploy/scripts/verify-secrets.sh`: ensure Web has no gateway config mount or Claude secrets.

Likely frontend additions/modifications:

- `frontend/src/routes/_shell/settings.tsx`: add Claude config center UI.
- `frontend/src/routes/-shell.test.tsx`: cover list/add/update/delete/test interactions.
- Generated API client files after OpenAPI generation.
- MSW handlers/fixtures for new endpoints.

## Verification

Backend:

- `go test -C backend ./internal/claudegateway -count=1`
- `go test -C backend ./internal/system -count=1`
- `make go-fmt`
- `make go-vet`
- `make go-test`

Contract:

- `make generate-openapi`
- `make verify-openapi`
- `make check-openapi-breaking`

Frontend:

- `cd frontend && npm run api:gen`
- `cd frontend && npm run routes:gen` if route metadata changes
- `cd frontend && npm run typecheck`
- `cd frontend && npm run test`
- `cd frontend && npm run build`

Deployment:

- `make docker-config`
- `make docker-verify-secrets`

Acceptance tests:

- Configure two Claude keys; force the first key to return `401`, then verify the request uses the second key.
- Configure two Claude keys; force the first key to return `429`, then verify it enters cooling down and the second key is used.
- Disable a key in Settings; verify the Worker does not select it on the next call.
- Delete a key; verify the API response no longer lists it and no full key is leaked.
- Run the global Claude connectivity test; verify it creates the existing background job and routes through the gateway.
