# Runtime Claude Code And DeepSeek Compatibility Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Docker and native deployments write and consume their own real `~/.claude/settings.json`, make the selected runtime Key control Worker Claude Code tasks, and restore DeepSeek V4 structured-request connectivity with accurate UI state and errors.

**Architecture:** Docker gives API and Worker the same persistent `HOME=/var/lib/ai-study-tutor`; API atomically writes `$HOME/.claude/settings.json`, while a shared Worker CLI resolver uses the non-secret saved Key ID to select the exact gateway record and construct a minimal subprocess environment. Direct DeepSeek structured calls disable thinking while retaining forced `structured_result`; native Anthropic behavior stays unchanged.

**Tech Stack:** Go 1.24, Gin, React 19/TypeScript/Vitest, Docker Compose, OpenAPI tooling.

**Design:** `docs/superpowers/specs/2026-08-09-runtime-claude-deepseek-compat-design.md`

---

## File Map

- `backend/internal/claudegateway/handler.go`: write the runtime Claude settings document and safe response.
- `backend/internal/claudegateway/handler_integration_test.go`: prove exact runtime settings and secret-safe API behavior.
- `backend/internal/platform/config/config.go`: default native target to `$HOME/.claude/settings.json`.
- `backend/internal/platform/config/config_test.go`: prove native default and explicit override.
- `backend/cmd/api/main.go`, `backend/cmd/api/main_test.go`: wire the configured default Base URL into the settings writer.
- `backend/internal/worker/claudecli/runtime.go`: shared settings-aware Key resolution and subprocess environment.
- `backend/internal/worker/claudecli/runtime_test.go`: resolver and environment contract.
- `backend/internal/worker/lessonimport/claude_url_cli.go`: consume the shared runtime resolver.
- `backend/internal/worker/lessonimport/claude_url_cli_test.go`: URL CLI integration with applied settings.
- `backend/internal/worker/aihandlers/quiz_cli.go`: consume the shared runtime resolver.
- `backend/internal/worker/aihandlers/quiz_cli_test.go`: quiz CLI integration with applied settings.
- `backend/cmd/worker/main.go`, `backend/cmd/worker/claude_providers_test.go`: wire runtime settings and HOME.
- `backend/internal/ai/anthropic.go`, `backend/internal/ai/anthropic_test.go`: DeepSeek thinking compatibility.
- `backend/internal/claudegateway/router.go`, `backend/internal/claudegateway/router_test.go`: targeted-test state transition.
- `frontend/src/features/claude-gateway/claude-gateway-section.tsx`, `frontend/src/routes/-shell.test.tsx`: runtime action wording.
- `frontend/src/api/problem-messages.ts`, `frontend/src/api/problem-messages.test.ts`: accurate invalid-request and settings errors.
- `api/openapi.yaml`, `docs/web-rebuild/openapi.yaml`, generated frontend API files: runtime OpenAPI wording.
- `deploy/compose.yaml`, `deploy/scripts/init.sh`, `deploy/scripts/verify-secrets.sh`, `deploy/.env.example`: runtime HOME boundary and removal of host mount.
- `docs/deployment.md`: deployment behavior and troubleshooting.

---

### Task 1: Runtime Settings Writer And Native Target

**Files:**
- Modify: `backend/internal/claudegateway/handler_integration_test.go`
- Modify: `backend/internal/platform/config/config_test.go`
- Modify: `backend/internal/claudegateway/handler.go`
- Modify: `backend/internal/platform/config/config.go`
- Modify: `backend/cmd/api/main.go`
- Create or modify: `backend/cmd/api/main_test.go`

- [ ] **Step 1: Write failing writer tests**

Extend `TestClaudeCodeSettingsRouteOverwritesSelectedKeyWithoutResponseSecret` to require:

```go
if body.Target != "~/.claude/settings.json" { ... }
for key, want := range map[string]string{
    "ANTHROPIC_MODEL": "deepseek-v4-flash",
    "ANTHROPIC_DEFAULT_OPUS_MODEL": "deepseek-v4-flash",
    "ANTHROPIC_DEFAULT_SONNET_MODEL": "deepseek-v4-flash",
    "ANTHROPIC_DEFAULT_HAIKU_MODEL": "deepseek-v4-flash",
    "CLAUDE_CODE_SUBAGENT_MODEL": "deepseek-v4-flash",
    "AI_STUDY_TUTOR_CLAUDE_KEY_ID": key.ID,
} {
    if saved.Env[key] != want { ... }
}
```

Use a DeepSeek Base URL/model fixture and assert file mode `0600`, newly created
`.claude` directory mode `0700`, atomic replacement, the runtime-oriented safe
backend error text, and no secret in the response. Add a Key with an empty
per-Key Base URL and a configured process default; assert the writer saves that
default rather than the old hardcoded gateway.

- [ ] **Step 2: Write failing native default tests**

Add config tests proving `HOME=/runtime/home` plus no explicit settings path resolves to `/runtime/home/.claude/settings.json`, while `CLAUDE_CODE_SETTINGS_FILE` overrides it and a missing HOME leaves the capability unavailable.

Add a focused API construction test around a small `buildClaudeGatewayHandler`
helper (or equivalent test seam) asserting
`cfg.Claude.AnthropicBaseURL` reaches `Handler.DefaultBaseURL`. This prevents
production wiring from silently diverging from unit-constructed handlers.

- [ ] **Step 3: Run tests and verify RED**

Run:

```bash
cd backend && go test ./internal/claudegateway ./internal/platform/config ./cmd/api -run 'ClaudeCodeSettings|RuntimeHome|ClaudeGatewayHandlerWiring' -count=1
```

Expected: FAIL because model fields/native default/effective Base URL do not exist.

- [ ] **Step 4: Implement the minimal writer/config changes**

Expand the settings env document with the five model variables and
`AI_STUDY_TUTOR_CLAUDE_KEY_ID`. Keep the safe API target as the runtime-relative
`~/.claude/settings.json`. Add a shared `claudegateway.EffectiveBaseURL` helper
using per-Key value, configured deployment default, then the Anthropic default;
add `Handler.DefaultBaseURL`, wire `cfg.Claude.AnthropicBaseURL` from
`backend/cmd/api/main.go`, and cover that wiring through the focused helper
test. Update the sanitized backend
failure text to runtime HOME/permissions. In `loadClaude`, resolve an empty
settings path from a non-empty `HOME` using
`filepath.Join(home, ".claude", "settings.json")`.

- [ ] **Step 5: Run tests and verify GREEN**

Run the Task 1 test command; expected PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/claudegateway/handler.go backend/internal/claudegateway/handler_integration_test.go backend/internal/platform/config/config.go backend/internal/platform/config/config_test.go backend/cmd/api/main.go backend/cmd/api/main_test.go
git commit -m "fix: target runtime Claude settings"
```

### Task 2: Shared Worker Runtime Resolver

**Files:**
- Create: `backend/internal/worker/claudecli/runtime.go`
- Create: `backend/internal/worker/claudecli/runtime_test.go`

- [ ] **Step 1: Write resolver tests**

Define the wished-for API:

```go
runtime := claudecli.Runtime{
    KeysFile: keysFile,
    SettingsFile: filepath.Join(home, ".claude", "settings.json"),
    DefaultBaseURL: "https://fallback.example",
    Home: home,
}
selection, err := runtime.Resolve()
env := runtime.Env(selection)
```

Cover selected ID, duplicate token with distinct model/Base URL,
empty per-Key Base URL with a non-default process Base URL,
missing/pre-existing settings fallback, priority/weight fallback,
malformed/unreadable file, unknown/disabled ID, and token/Base URL/model
mismatch. Assert all errors omit sentinel secrets.

- [ ] **Step 2: Run tests and verify RED**

```bash
cd backend && go test ./internal/worker/claudecli -count=1
```

Expected: FAIL because the package/API does not exist.

- [ ] **Step 3: Implement minimal resolver**

`Runtime.Resolve` loads gateway Keys once. Missing settings or a valid JSON
document without the app selector uses deterministic priority/weight fallback.
An app selector requires the exact enabled ID and matching token, shared
effective Base URL, and model; otherwise return a sanitized
`claude_not_configured`/`claude_unavailable` provider error. `Runtime.Env`
returns only HOME, PATH, SHELL, CI, NO_COLOR, auth/base/model variables, app
selector, and existing traffic controls.

- [ ] **Step 4: Run tests and verify GREEN**

Run Task 2 tests; expected PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/worker/claudecli
git commit -m "feat: resolve Worker Claude runtime settings"
```

### Task 3: Wire URL And Quiz Claude Code Tasks

**Files:**
- Modify: `backend/internal/worker/lessonimport/claude_url_cli_test.go`
- Modify: `backend/internal/worker/aihandlers/quiz_cli_test.go`
- Modify: `backend/cmd/worker/claude_providers_test.go`
- Modify: `backend/internal/worker/lessonimport/claude_url_cli.go`
- Modify: `backend/internal/worker/aihandlers/quiz_cli.go`
- Modify: `backend/cmd/worker/main.go`

- [ ] **Step 1: Write failing CLI integration tests**

Create runtime settings selecting a lower-priority Key and assert both URL and quiz fake process calls contain its token, Base URL, model variables, selector ID, and runtime HOME. Assert missing settings retains priority fallback. Add worker wiring assertions for `cfg.Claude.ClaudeCodeSettingsFile` and runtime HOME.

- [ ] **Step 2: Run tests and verify RED**

```bash
cd backend && go test ./internal/worker/lessonimport ./internal/worker/aihandlers ./cmd/worker -run 'ClaudeURLCLI|QuizCLI|RuntimeSettings' -count=1
```

Expected: FAIL because both generators still choose only by priority and omit model env.

- [ ] **Step 3: Replace duplicated selection/env code**

Add `SettingsFile` to both generator structs and delegate selection/environment to `claudecli.Runtime`. Preserve each generator's existing error mapping and response envelope logic. Wire `SettingsFile` and the process runtime HOME in `buildClaudeProviders`.

- [ ] **Step 4: Run tests and verify GREEN**

Run Task 3 tests; expected PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/worker/lessonimport backend/internal/worker/aihandlers backend/cmd/worker
git commit -m "fix: use applied Claude settings in Worker CLI"
```

### Task 4: DeepSeek Structured Calls And Connectivity State

**Files:**
- Modify: `backend/internal/ai/anthropic_test.go`
- Modify: `backend/internal/claudegateway/router_test.go`
- Modify: `backend/internal/ai/anthropic.go`
- Modify: `backend/internal/claudegateway/router.go`
- Modify: `backend/internal/claudegateway/state.go` if a targeted transition helper is required.

- [ ] **Step 1: Write failing DeepSeek request tests**

Capture request JSON and assert a `deepseek-v4-flash` request includes:

```json
{"thinking":{"type":"disabled"},"tool_choice":{"type":"tool","name":"structured_result"}}
```

Add a Claude model case asserting `thinking` is absent.

- [ ] **Step 2: Write failing targeted-state test**

Make `GenerateForKey` receive a 400 invalid request and assert final Key status is `unknown`, with `last_error_code=claude_invalid_request`, rather than `testing`.

- [ ] **Step 3: Run tests and verify RED**

```bash
cd backend && go test ./internal/ai ./internal/claudegateway -run 'DeepSeek|GenerateForKey.*Invalid' -count=1
```

Expected: FAIL because thinking is absent and testing is sticky.

- [ ] **Step 4: Implement minimal compatibility/state fixes**

Add an optional `thinking` request field and set disabled only when the normalized model begins with `deepseek-`. Add a targeted-test outcome path that preserves auth/cooling semantics but turns a non-failover testing failure into `unknown` with diagnostics.

- [ ] **Step 5: Run tests and verify GREEN**

Run Task 4 tests; expected PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/ai/anthropic.go backend/internal/ai/anthropic_test.go backend/internal/claudegateway/router.go backend/internal/claudegateway/router_test.go backend/internal/claudegateway/state.go
git commit -m "fix: support DeepSeek structured requests"
```

### Task 5: Frontend, OpenAPI, Deployment, And Documentation

**Files:**
- Modify: `frontend/src/routes/-shell.test.tsx`
- Modify: `frontend/src/api/problem-messages.test.ts`
- Modify: `frontend/src/features/claude-gateway/claude-gateway-section.tsx`
- Modify: `frontend/src/api/problem-messages.ts`
- Modify: `api/openapi.yaml`
- Modify: `docs/web-rebuild/openapi.yaml`
- Modify/generated: `frontend/src/api/generated/**`
- Modify: `deploy/compose.yaml`
- Modify: `deploy/scripts/init.sh`
- Modify: `deploy/scripts/verify-secrets.sh`
- Modify: `deploy/.env.example`
- Modify: `docs/deployment.md`

- [ ] **Step 1: Write failing frontend wording tests**

Require button/dialog/success text to say `应用到运行服务的 Claude Code` and `运行环境 ~/.claude/settings.json`. Change invalid-request expectations to contain model/Base URL compatibility guidance and not `Web Fetch`. Change settings-unavailable guidance to runtime HOME permissions.

- [ ] **Step 2: Write failing deployment assertions**

Update `verify-secrets.sh` expectations first: API and Worker both have `HOME=/var/lib/ai-study-tutor` and settings path under that HOME; no `/claude-code` target and no `CLAUDE_CODE_SETTINGS_DIR` remain; Web stays clean.

- [ ] **Step 3: Run tests and verify RED**

```bash
cd frontend && npm test -- src/routes/-shell.test.tsx src/api/problem-messages.test.ts
cd .. && ./deploy/scripts/verify-secrets.sh
```

Expected: FAIL on old local wording and old host mount.

- [ ] **Step 4: Implement frontend/deployment/docs changes**

Update UI copy and problem messages. In Compose, set API and Worker
HOME/settings path and the same `ANTHROPIC_BASE_URL` default on the shared
app-data volume, then remove the host settings bind. Remove host `.claude`
initialization and `.env.example` variable. Update OpenAPI source plus
deployment docs; keep the response target enum as the runtime-relative
`~/.claude/settings.json`.

- [ ] **Step 5: Regenerate OpenAPI clients**

```bash
make generate-openapi
cd frontend && npm run api:gen
```

- [ ] **Step 6: Run focused tests and verify GREEN**

Run Task 5 tests plus:

```bash
make verify-openapi
./deploy/scripts/verify-secrets.sh
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/features/claude-gateway/claude-gateway-section.tsx frontend/src/api/problem-messages.ts frontend/src/api/problem-messages.test.ts frontend/src/api/generated/endpoints/system/system.ts frontend/src/api/generated/endpoints/system/system.msw.ts deploy/compose.yaml deploy/scripts/init.sh deploy/scripts/verify-secrets.sh deploy/.env.example docs/deployment.md
git add -p frontend/src/routes/-shell.test.tsx api/openapi.yaml docs/web-rebuild/openapi.yaml
git diff --cached --check
git diff --cached --stat
git commit -m "fix: configure Claude in the runtime environment"
```

For the three already-dirty files, accept only this feature's runtime-Claude
wording hunks during `git add -p`; reject theme, batch-delete, course, and lesson
hunks. If a generated hunk combines unrelated edits, leave that file unstaged
and report it rather than committing user work.

### Task 6: Full Verification, Review, And Live Docker Acceptance

**Files:** all files above.

- [ ] **Step 1: Format and inspect**

```bash
gofmt -w backend/internal/ai backend/internal/claudegateway backend/internal/platform/config backend/internal/worker backend/cmd/worker
git diff --check
```

- [ ] **Step 2: Run full backend/frontend/deployment verification**

```bash
make go-test go-vet go-build verify-openapi docker-verify-secrets
cd frontend && npm run verify
```

Expected: all commands exit 0.

- [ ] **Step 3: Request code review and address blocking findings**

Review only this feature's diff against the approved design. Re-run affected tests after every review fix.

- [ ] **Step 4: Rebuild Docker**

```bash
make docker-up
```

Expected: db/api/worker/web healthy with rebuilt images.

- [ ] **Step 5: Apply and inspect runtime DeepSeek settings**

Call the safe ID-only API for the existing DeepSeek Key, then verify inside Worker (without printing the token):

```bash
docker exec ai-study-tutor-worker-1 sh -lc 'printf "%s\n" "$HOME"; test -f "$HOME/.claude/settings.json"; stat -c "%a %n" "$HOME/.claude/settings.json"'
```

Expected: HOME is `/var/lib/ai-study-tutor`, file exists, mode is `600`, and the host `~/.claude/settings.json` was not modified by this action.

- [ ] **Step 6: Run real DeepSeek connectivity acceptance**

Create a new targeted connectivity Job for the saved DeepSeek Key, poll it to terminal state, and query its redacted `ai_runs` record. Expected: Job `succeeded`, Key `healthy`, model `deepseek-v4-flash`, and no secret in any response/log.

- [ ] **Step 7: Verify Worker Claude CLI runtime**

Run `claude --version` and one minimal non-interactive request inside Worker using its runtime HOME, or a safe existing URL/quiz task if available. Expected: execution occurs in Worker and uses the applied runtime settings without a host Claude process.

- [ ] **Step 8: Final diff and status audit**

Confirm unrelated pre-existing user changes remain untouched and report exact verification/live-acceptance evidence.
