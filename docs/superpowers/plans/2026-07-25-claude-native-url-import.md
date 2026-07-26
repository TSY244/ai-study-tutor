# Claude 原生 URL 素材导入 Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将允许的课程 URL 交由 Claude Web Fetch 提取为原文，并原子发布原文、教师教纲和学生笔记，同时保留手动生成测验的既有流程。

**Architecture:** 在现有 `ai.Provider` 请求对象上增加仅用于 `lesson_url_extract` 的 Web Fetch 选项。Anthropic Provider 为该请求走受控的 server-tool/`pause_turn` 循环，现有多 Key Router 无需改接口即可保留其失败转移策略。Worker 为 URL 建立独立的 Claude 提取分支，绕过本地 DNS、重定向、yt-dlp、FFmpeg 和 Whisper；文本与上传仍走现有路径。

**Tech Stack:** Go 1.24、Anthropic Messages API、Go `net/http` fake `RoundTripper`、PostgreSQL 集成测试、Vite/React/Vitest。

---

## File Structure

| 文件 | 职责 |
| --- | --- |
| `backend/internal/ai/provider.go` | 声明 URL 提取 purpose、Web Fetch 请求配置、可重试的 `material_fetch_failed` 错误和 prompt hash 覆盖范围。 |
| `backend/internal/ai/prompts.go` | 构建注入安全的 URL 提取 prompt 与严格结果 schema。 |
| `backend/internal/ai/anthropic.go` | 保持普通强制结构化调用不变；为 URL 请求实现 Web Fetch tools、beta header、受控 continuation 和响应证据校验。 |
| `backend/internal/ai/{provider_test.go,anthropic_test.go}` | 覆盖请求契约、headers、`pause_turn`、工具结果、错误映射和红线回归。 |
| `backend/internal/claudegateway/router_test.go` | 证明来源访问错误不会被误判为 Key 故障或触发 Key failover。 |
| `backend/internal/worker/lessonimport/claude_url.go` | URL 再校验/规范化、每任务允许域名集、Claude URL 结果解析和安全错误转换。 |
| `backend/internal/worker/lessonimport/claude_url_test.go` | 覆盖规范化、URL 边界、无本地网络/进程回退、结果校验和脱敏。 |
| `backend/internal/worker/lessonimport/handler.go` | 为 URL 选择 Claude 分支、记录 URL 提取 `ai_run`、要求三份文档齐全才发布。 |
| `backend/internal/worker/lessonimport/acquire.go` | 保留给文本/上传和既有本地能力；URL 分支不再从此处调用。 |
| `backend/internal/worker/worker_integration_cases_test.go` | 为 URL Job 加入真实发布事务的 all-or-nothing 回归用例。 |
| `backend/internal/claudegateway/{router.go,router_test.go}` | 提供 URL 专用的仅网关 Provider，保证它永不回退到 legacy 单 Key。 |
| `backend/cmd/worker/{main.go,claude_providers_test.go}` | 将 URL 专用 Provider/availability 注入 lesson import，并回归 legacy 非 URL 行为。 |
| `frontend/src/features/materials/{url-import-form.tsx,import-panel.test.tsx,active-job-card.tsx}` | 显示“由 Claude 提取链接内容”的准确说明，并在异步 Job 卡中显示 code-first 错误。 |
| `frontend/src/routes/_shell/jobs/{$jobId.tsx,$jobId.test.tsx,index.tsx}`、`frontend/src/features/jobs/{job-actions.tsx,job-actions.test.tsx}` | 在任务详情、列表和不可重试操作使用 code-first 异步错误呈现并测试 URL Job 失败状态。 |
| `frontend/src/api/{problem-messages.ts,problem-messages.test.ts}` | 为 URL 提取的稳定 Job/API 错误码提供可行动、非敏感的呈现文案。 |
| `docs/web-rebuild/{03-backend-architecture.md,05-deployment-operations.md,06-agent-prompts.md,10-backend-agent-playbook.md,12-domain-decisions.md,13-worker-pipeline.md}`、`docs/deployment.md`、`README.md` | 将 URL 路线、运行前提和故障排查更新为 Claude Web Fetch，并标记旧 URL 下载描述为已替代。 |

### Task 1: Define The URL-Extraction AI Contract

**Files:**
- Modify: `backend/internal/ai/provider.go`
- Modify: `backend/internal/ai/prompts.go`
- Test: `backend/internal/ai/provider_test.go`

- [ ] **Step 1: Write failing contract tests for the new request and error classification.**

  Add `TestBuildLessonURLExtractRequest` and extend `TestProviderErrorRetryMatrix`. The test must assert a `lesson_url_extract` request has the fixed 16,384 output cap, `web_fetch_20250910` options (`max_uses=3`, `max_content_tokens=12000`, `max_continuations=2`), no empty allowed domain, and a schema requiring `title`, `raw_text_markdown`, and `extraction_note`.

  ```go
  req, err := BuildLessonURLExtractRequest(LessonURLExtractPromptInput{
      URL: "https://www.youtube.com/watch?v=abc",
      AllowedDomains: []string{"www.youtube.com", "youtube.com"},
  })
  if req.Purpose != PurposeLessonURLExtract || req.WebFetch == nil { t.Fatal("...") }
  if req.WebFetch.MaxUses != 3 || req.MaxOutputTokens != 16384 { t.Fatal("...") }
  ```

  Add a hash regression asserting two otherwise identical requests with different canonical URLs yield different `PromptHash` values, without storing either URL in `RedactedLogFields`.

- [ ] **Step 2: Run the focused test to prove it fails.**

  Run: `go test -C backend ./internal/ai -run 'Test(BuildLessonURLExtractRequest|ProviderErrorRetryMatrix)' -count=1`

  Expected: FAIL because the purpose, Web Fetch options, builder, and `material_fetch_failed` classification do not exist.

- [ ] **Step 3: Add the smallest provider-level types and prompt builder.**

  In `provider.go`, add `PurposeLessonURLExtract`, a `WebFetchOptions` field on `Request`, and a stable `ErrMaterialFetchFailed = "material_fetch_failed"`. `WebFetchOptions` must include only `URL`, `AllowedDomains`, `MaxUses`, `MaxContentTokens`, and `MaxContinuations`; it is worker-only request data and has no HTTP/API DTO.

  Ensure `PromptHash` serializes all Web Fetch option values with unambiguous delimiters and `RedactedLogFields` continues to return metadata/hash only. Add a constructor/helper that permits `ErrMaterialFetchFailed` to carry an explicit retryability without exposing upstream text.

  In `prompts.go`, add versioned constants and a JSON schema with `additionalProperties: false`. The system prompt must require fetching the supplied canonical URL before `structured_result`, require complete retrievable source text rather than summaries, and treat fetched text as untrusted data that cannot modify instructions or request secrets.

- [ ] **Step 4: Run focused AI contract tests.**

  Run: `go test -C backend ./internal/ai -run 'Test(BuildLessonURLExtractRequest|ProviderErrorRetryMatrix|PromptMetadataIsVersionedAndRedacted)' -count=1`

  Expected: PASS.

- [ ] **Step 5: Commit the contract.**

  ```bash
  git add backend/internal/ai/provider.go backend/internal/ai/prompts.go backend/internal/ai/provider_test.go
  git commit -m "feat: define Claude URL extraction request"
  ```

### Task 2: Implement The Anthropic Web Fetch Protocol

**Files:**
- Modify: `backend/internal/ai/anthropic.go`
- Test: `backend/internal/ai/anthropic_test.go`

- [ ] **Step 1: Write failing HTTP-adapter tests before changing the wire model.**

  Add table-driven fake-transport tests that inspect each request body/header and return synthetic Messages JSON. Cover:

  - ordinary connectivity/quiz requests still force only `structured_result` with `tool_choice={type:"tool",name:"structured_result"}` and no beta header;
  - URL requests send `anthropic-beta: web-fetch-2025-09-10`, `tool_choice={type:"auto"}`, the exact server tool payload, and the existing `structured_result` client tool;
  - a valid `structured_result` without a successful canonical `server_tool_use` plus `web_fetch_tool_result` fails `material_fetch_failed`;
  - successful canonical fetch followed by `structured_result` succeeds;
  - `pause_turn` appends the exact prior assistant content and one continuation user message, preserves tools/model/system, and aggregates usage/request id/latency;
  - a third unresolved `pause_turn` maps to `claude_timeout`;
  - tool failure maps temporary versus permanent `material_fetch_failed`, truncation maps `material_too_large`, and HTTP 400 tool rejection remains `claude_invalid_request`.

  Include secrets and source-text sentinels only in mock upstream payloads; assert all returned `ProviderError.Detail` values exclude them.

- [ ] **Step 2: Run the adapter test subset to verify failure.**

  Run: `go test -C backend ./internal/ai -run 'TestAnthropicProvider(WebFetch|ExtractsStructuredToolUseAndUsage)' -count=1`

  Expected: FAIL because the current adapter only understands a single `tool_use` response.

- [ ] **Step 3: Add a URL-only Messages loop without changing ordinary requests.**

  Keep `Generate` as the public entry point. Dispatch to a dedicated internal URL path only when `req.WebFetch != nil`; preserve the old one-request builder/parser for every other purpose.

  Make message and content-block wire structs able to preserve assistant content byte-for-byte across continuations. Model the server tool, its use block, and its result/error block explicitly enough to:

  ```go
  canonicalFetchSucceeded := false
  // Set true only for the exact canonical request URL plus a non-error,
  // non-truncated corresponding server result.
  if structured != nil && canonicalFetchSucceeded { return response, nil }
  ```

  Send at most initial request plus two continuations. Sum all four usage counters, retain the final non-empty model/request id/stop reason, and measure one logical-call latency. Never log or surface raw request bodies, raw content blocks, upstream messages, or keys.

- [ ] **Step 4: Run adapter and gateway regression tests.**

  Run: `go test -C backend ./internal/ai ./internal/claudegateway -count=1`

  Expected: PASS; this proves normal structured calls and multi-Key failure routing remain intact.

- [ ] **Step 5: Commit the protocol adapter.**

  ```bash
  git add backend/internal/ai/anthropic.go backend/internal/ai/anthropic_test.go
  git commit -m "feat: support Claude Web Fetch URL extraction"
  ```

### Task 3: Canonicalize And Authorize Worker URL Requests

**Files:**
- Create: `backend/internal/worker/lessonimport/claude_url.go`
- Create: `backend/internal/worker/lessonimport/claude_url_test.go`
- Modify: `backend/internal/worker/lessonimport/acquire.go`

- [ ] **Step 1: Write failing unit tests for `canonicalClaudeURL`.**

  Test a pure helper that reparses the persisted value and returns `(canonicalURL, allowedDomains, error)`. Required examples:

  ```go
  // Removes source tracking and permits only the source pair.
  got, domains, err := canonicalClaudeURL(
      "https://www.youtube.com/watch?v=abc&utm_source=x&list=p1", []string{"youtube.com", "bilibili.com"})
  // got == "https://www.youtube.com/watch?list=p1&v=abc"
  // domains == []string{"www.youtube.com", "youtube.com"}
  ```

  Cover Bilibili `p`, removal of `spm*`, `trackid`, `vd_source`, and `share_*`, sorted generic queries, duplicate/empty allowlist entries, longest matching root, non-HTTPS/userinfo/port/non-allowlisted rejection, and the 250-character limit. Tests must not resolve DNS or send HTTP.

- [ ] **Step 2: Run the helper test to verify failure.**

  Run: `go test -C backend ./internal/worker/lessonimport -run TestCanonicalClaudeURL -count=1`

  Expected: FAIL because the helper does not exist.

- [ ] **Step 3: Implement the pure helper and URL-result parser.**

  Reuse `material.HostAllowed`; select the longest matching allowlist root, de-duplicate `[hostname, root]`, reject userinfo/port and enforce the length after query re-encoding. Do not invoke `Resolver`, `Transport`, `proc.Runner`, `yt-dlp`, FFmpeg, or Whisper from this file.

  Add a small private decoded result type and parser for `title`, `raw_text_markdown`, and `extraction_note`. It must normalize and validate original text later in the handler, but reject malformed JSON/schema material as a Claude error before publishing. Extend `sourceInfo` with a non-sensitive `ExtractionNote` field and use `Extractor: "claude_web_fetch"` plus `ParserVersion: "claude-web-fetch/v1"` for this path.

  Remove the `material.KindURL` branch from `acquire`; leave local subtitle/audio helpers in place for unrelated file/media behavior, but make them unreachable for URL material.

- [ ] **Step 4: Run helper and legacy acquisition tests.**

  Run: `go test -C backend ./internal/worker/lessonimport -run 'Test(CanonicalClaudeURL|CheckURLHost|PickSubtitleFile|ClassifyProcErr)' -count=1`

  Expected: PASS; the old local helpers remain covered but no longer own URL imports.

- [ ] **Step 5: Commit Worker URL boundary code.**

  ```bash
  git add backend/internal/worker/lessonimport/claude_url.go backend/internal/worker/lessonimport/claude_url_test.go backend/internal/worker/lessonimport/acquire.go
  git commit -m "feat: canonicalize Claude URL import sources"
  ```

### Task 4: Route URL Jobs Through Claude And Publish Atomically

**Files:**
- Modify: `backend/internal/worker/lessonimport/handler.go`
- Modify: `backend/internal/worker/lessonimport/claude_url_test.go`
- Modify: `backend/internal/worker/worker_integration_cases_test.go`
- Modify: `backend/internal/claudegateway/router.go`
- Modify: `backend/internal/claudegateway/router_test.go`
- Modify: `backend/cmd/worker/main.go`
- Modify: `backend/cmd/worker/claude_providers_test.go`

- [ ] **Step 1: Add failing URL-handler tests.**

  Use the existing fake provider and Worker integration harness. Queue three valid structured responses in this order: URL extract, teacher outline, student notes. Assert the URL job publishes exactly `raw_text`, `teacher_outline`, and `student_notes`; writes source metadata with `extractor=claude_web_fetch`; records an `ai_runs` row with purpose `lesson_url_extract`; and never calls a fake `proc.Runner`, Resolver, or `http.RoundTripper` configured to fail on access.

  Add cases for: no usable provider (`claude_not_configured`); empty/oversize extracted text (`material_empty_content`/`material_too_large`); a URL extract error; and either document-generation error. For every failure assert no new active `raw_text`, `teacher_outline`, or `student_notes` document was published.

  In the successful and failed fake-provider responses, inject distinct URL-query, prompt, raw-source and Key-like sentinels. Query the recorded `ai_runs` and the final Job error/result and assert none contains those values. Keep sentinels out of browser-facing MSW fixtures and snapshots; this test proves the persistence boundary does not leak them.

  Add `router_test.go` cases in which a selected Key returns retryable and non-retryable `material_fetch_failed`: the router must return that result without attempting the second Key or mutating the first Key to cooling/auth-failed, while the retryability remains visible to the Job runner. Keep existing auth/429/timeout failover assertions unchanged.

  Add `claude_providers_test.go` cases for a legacy `ANTHROPIC_API_KEY_FILE` plus zero configured gateway Keys: ordinary `AI`/non-URL behavior may retain legacy compatibility, but `URLImportAvailable` is false and `URLImportAI` returns `claude_not_configured` without making an HTTP call. After adding a gateway Key, URL calls use that Key; removing it before the request must again fail without legacy fallback.

- [ ] **Step 2: Run only the new URL tests to verify failure.**

  Run: `go test -C backend ./internal/worker/lessonimport ./internal/worker ./internal/claudegateway ./cmd/worker -run 'Test.*(ClaudeURL|URLImport|MaterialFetch)' -count=1`

  Expected: FAIL because URL imports currently call `acquireURL` and can silently publish raw text without generated documents.

- [ ] **Step 3: Add the explicit URL execution branch.**

  Refactor `claudegateway.Router.Generate` through one private routing method with an explicit legacy-fallback flag. Expose a small `GatewayOnly()` `ai.Provider` wrapper that always uses configured gateway Keys and returns `claude_not_configured` when none is enabled; it must never call `Router.Legacy`. Preserve the public `Generate` legacy fallback for existing non-URL work.

  Add `URLImportAI` and `URLImportAvailable` fields to the Worker provider bundle and lesson-import handler. `buildClaudeProviders` injects `gateway.GatewayOnly()` and a hot-reloaded `gateway.EnabledKeyCount() > 0` predicate only when a gateway config exists. A direct legacy key or fake/non-Anthropic provider is never a URL-import substitute. Register these fields in `main` while keeping the existing `AI`/`LessonImportAI` wiring intact for text/upload and all other Claude jobs.

  In `Handler.Execute`, branch on `material.KindURL` before calling the generic file acquisition/detection/extraction sequence. Keep stable stage codes but use `StageExtracting` with the message `Claude 提取链接内容`; pass the canonical URL to `BuildLessonURLExtractRequest`, invoke `h.URLImportAI.Generate`, and create its redacted `ai_runs` record exactly once.

  Map `ErrMaterialFetchFailed` to `worker.ErrCodeMaterialFetchFailed` while preserving the provider retry flag. Map empty and max-rune failures to existing `worker.ErrCodeEmptyContent` and `worker.ErrCodeTooLarge`. If `h.URLImportAI` is nil or `URLImportAvailable` is false, return a non-retryable `claude_not_configured`; do not use the old raw-text-only degradation or a direct legacy Key for URLs.

  After the successful URL extraction, reuse `Normalize` and `ChunkText`. Refactor document generation so it accepts an explicit `ai.Provider`: URL outline/notes use the same `URLImportAI` gateway-only provider, while text/upload keep `h.AI` and their current optional generation semantics. This prevents Key removal between extraction and generation from causing a legacy fallback. For URL only, require both generated Markdown values before `publishFunc`; call it only after all three document strings are non-empty and validated, preserving its existing transaction as the all-or-nothing boundary.

- [ ] **Step 4: Run import, AI handler, and gateway regression tests.**

  Run: `go test -C backend ./internal/worker/lessonimport ./internal/worker ./internal/worker/aihandlers ./internal/claudegateway ./cmd/worker -count=1`

  Expected: PASS, including existing text/upload, manual quiz, grading, and gateway failure behavior.

- [ ] **Step 5: Commit the Worker integration.**

  ```bash
  git add backend/internal/worker/lessonimport/handler.go backend/internal/worker/lessonimport/claude_url_test.go backend/internal/worker/worker_integration_cases_test.go backend/internal/claudegateway/router.go backend/internal/claudegateway/router_test.go backend/cmd/worker/main.go backend/cmd/worker/claude_providers_test.go
  git commit -m "feat: import URLs through Claude Web Fetch"
  ```

### Task 5: Update URL Import Guidance And Error Presentations

**Files:**
- Modify: `frontend/src/features/materials/url-import-form.tsx`
- Modify: `frontend/src/features/materials/import-panel.test.tsx`
- Modify: `frontend/src/features/materials/active-job-card.tsx`
- Modify: `frontend/src/routes/_shell/jobs/$jobId.tsx`
- Create: `frontend/src/routes/_shell/jobs/$jobId.test.tsx`
- Modify: `frontend/src/routes/_shell/jobs/index.tsx`
- Modify: `frontend/src/features/jobs/job-actions.tsx`
- Modify: `frontend/src/features/jobs/job-actions.test.tsx`
- Modify: `frontend/src/api/problem-messages.ts`
- Modify: `frontend/src/api/problem-messages.test.ts`

- [ ] **Step 1: Write the failing component assertion.**

  Extend the existing URL import test to switch to the 链接 tab and require concise helper text stating that Claude extracts allowed HTTPS links in the background. Keep the assertions that non-HTTPS input is blocked and the same `POST /materials/url` request body is submitted.

  Add `presentJobError` assertions for `url_blocked`, `material_fetch_failed`, `material_empty_content`, `material_too_large`, `claude_timeout`, `claude_invalid_request`, `claude_invalid_json`, and `claude_schema_invalid`: each message must be actionable, contain no source URL/key/prompt sentinel, and preserve the Job's retryable flag. If `KnownProblemCode` cannot represent Worker-only codes, use a narrow local Job-code presentation map instead of changing the OpenAPI contract.

  Add a failed asynchronous URL Job fixture/test for `ActiveJobCard`, `/jobs/$jobId`, the jobs index row, and `JobActions`. Each surface must show the code-first message for `material_fetch_failed` instead of `job.error.detail`, keep the error code/retry state visible where it already is, and verify a sentinel in the backend detail never renders. Add one non-retryable `url_blocked` or `material_too_large` case so the `JobActions` reason also cannot echo raw detail.

- [ ] **Step 2: Run the focused Vitest case to verify failure.**

  Run: `cd frontend && npm run test -- --run src/features/materials/import-panel.test.tsx src/api/problem-messages.test.ts 'src/routes/_shell/jobs/$jobId.test.tsx' src/features/jobs/job-actions.test.tsx -t 'URL 导入|presentJobError|任务失败|不可重试'`

  Expected: FAIL because the helper text, Job-error presentation utility, and asynchronous URL failure rendering are absent.

- [ ] **Step 3: Add only the explanatory copy.**

  Add a short descriptive paragraph below the URL label in `UrlImportForm`; do not add controls, API calls, key fields, automatic quiz generation, or local extraction options. Preserve form accessibility and current idempotency behavior.

  Add a `presentJobError(code, retryable)` helper and stable Chinese presentations for the new URL execution failures. `material_fetch_failed` must distinguish temporary extraction from inaccessible content only through the Job retryable signal, never upstream detail; empty/oversize content must explain the next action; `claude_invalid_request` must say the deployment gateway needs Web Fetch support. Use this helper in `ActiveJobCard`, the job-detail alert, `JobRow` in the jobs index, and the non-retryable reason in `JobActions` instead of directly rendering `job.error.detail`; retain code and retry-state display. Do not parse natural-language detail, echo a URL, or alter the route/API client.

- [ ] **Step 4: Run typecheck and the focused UI test.**

  Run: `cd frontend && npm run typecheck && npm run test -- --run src/features/materials/import-panel.test.tsx src/api/problem-messages.test.ts 'src/routes/_shell/jobs/$jobId.test.tsx' src/features/jobs/job-actions.test.tsx`

  Expected: PASS.

- [ ] **Step 5: Commit the frontend copy.**

  ```bash
  git add frontend/src/features/materials/url-import-form.tsx frontend/src/features/materials/import-panel.test.tsx frontend/src/features/materials/active-job-card.tsx 'frontend/src/routes/_shell/jobs/$jobId.tsx' 'frontend/src/routes/_shell/jobs/$jobId.test.tsx' frontend/src/routes/_shell/jobs/index.tsx frontend/src/features/jobs/job-actions.tsx frontend/src/features/jobs/job-actions.test.tsx frontend/src/api/problem-messages.ts frontend/src/api/problem-messages.test.ts
  git commit -m "docs: clarify Claude URL import"
  ```

### Task 6: Update Operational Documentation And Run Full Verification

**Files:**
- Modify: `docs/web-rebuild/03-backend-architecture.md`
- Modify: `docs/web-rebuild/05-deployment-operations.md`
- Modify: `docs/web-rebuild/06-agent-prompts.md`
- Modify: `docs/web-rebuild/10-backend-agent-playbook.md`
- Modify: `docs/web-rebuild/12-domain-decisions.md`
- Modify: `docs/web-rebuild/13-worker-pipeline.md`
- Modify: `docs/deployment.md`
- Modify: `README.md`

- [ ] **Step 1: Update behavior documentation.**

  Replace only the URL-specific `yt-dlp`/subtitle/Whisper walkthrough with the Claude Web Fetch route: API HTTPS/allowlist validation, Worker revalidation/canonicalization, per-Job allowed domains, Web Fetch gateway prerequisite, 12k/16,384 limits, stable errors, and all-or-nothing document publication. Audit `05`, `06`, `10`, `12`, and `README.md`; update live instructions and mark historic domain decisions as superseded where they describe local URL download. Preserve descriptions of local parsers/transcription for uploads and non-URL media where still true.

  In deployment docs, state that URL import depends on the configured `ANTHROPIC_BASE_URL` forwarding Messages server tools and `anthropic-beta`; it does not require direct Worker access to YouTube/Bilibili. Do not claim the Web container receives a secret or gateway mount.

- [ ] **Step 2: Run documentation and contract guards.**

  Run: `git diff --check && make verify-openapi && make check-openapi-breaking`

  Expected: PASS; no OpenAPI change is required because the URL endpoint contract remains unchanged.

- [ ] **Step 3: Run the required backend and frontend suites.**

  Run:

  ```bash
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

  Expected: every command passes. If a command is blocked by unavailable Docker/dependencies, record its exact command and error before deployment.

- [ ] **Step 4: Build and redeploy only after verification.**

  Run: `docker compose -f deploy/compose.yaml up -d --build`

  Then verify healthy service state and execute a local smoke submission against the unchanged URL endpoint using a non-sensitive URL. Do not use a production Key in tests or logs; inspect the resulting Job for `claude_web_fetch` metadata and all three document kinds.

- [ ] **Step 5: Commit docs and record deployment evidence.**

  ```bash
  git add docs/web-rebuild/03-backend-architecture.md docs/web-rebuild/05-deployment-operations.md docs/web-rebuild/06-agent-prompts.md docs/web-rebuild/10-backend-agent-playbook.md docs/web-rebuild/12-domain-decisions.md docs/web-rebuild/13-worker-pipeline.md docs/deployment.md README.md
  git commit -m "docs: document Claude URL import pipeline"
  ```
