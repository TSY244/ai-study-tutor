# Claude 网关 per-Key base_url Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **Git 策略（会话级强制）:** 所有 `git commit` 步骤必须先取得用户明确确认再执行；未获确认时跳过提交，只保留工作区改动。

**Goal:** 为 Claude 网关每个 Key 增加独立 `base_url`，前端设置页表单可直接编辑并与 Key 一并写入 keys.json；消费方按 Key.base_url > 进程 env ANTHROPIC_BASE_URL 优先级解析。

**Architecture:** keys.json 的 Key 记录增加可选 `base_url` 字段（omitempty 向后兼容）；API 创建/更新支持该字段（空串=清除）；网关 Router 用 `messagesEndpoint()` 把基础 URL 转为完整 Messages 端点；两个 CLI 路径与 Claude Code settings 导出注入解析后的 base URL；OpenAPI 纯新增字段，前端重新生成客户端。

**Tech Stack:** Go(claudegateway/worker)、OpenAPI + orval 生成、React + TanStack Query + shadcn 风格组件。

**Spec:** `docs/superpowers/specs/2026-07-26-claude-key-base-url-design.md`

---

## File Structure

- **Modify** `backend/internal/claudegateway/store.go` — `Key`/`KeyPatch`/`SafeKey` 增字段，`Update` 应用，`safeView` 输出。
- **Modify** `backend/internal/claudegateway/handler.go` — 创建/更新 DTO + `validateBaseURL` + `writeClaudeCodeSettings` 覆盖。
- **Modify** `backend/internal/claudegateway/router.go` — `messagesEndpoint` 助手 + `attempt` 端点解析。
- **Modify** `backend/internal/worker/lessonimport/claude_url_cli.go`、`backend/internal/worker/aihandlers/quiz_cli.go` — env 注入优先级。
- **Modify** `api/openapi.yaml` — 三个 schema 增 `base_url`。
- **Modify** `frontend/src/features/claude-gateway/claude-key-dialog.tsx`、`claude-gateway-section.tsx`、`src/mocks/fixtures.ts`；重新生成 `frontend/src/api/generated/**`。
- **Tests:** 上述各包既有测试文件追加用例，不新建测试文件。

---

### Task B1: claudegateway 存储与 API

**Files:**
- Modify: `backend/internal/claudegateway/store.go`(Key 约 line 28-42;KeyPatch 约 line 186-196;Update 约 line 201+;SafeKey 约 line 400;safeView 约 line 425)
- Modify: `backend/internal/claudegateway/handler.go`(createKeyRequest line 95;createKey line 107;updateKeyRequest line 178;updateKey line 190;writeClaudeCodeSettings line 324)
- Test: `backend/internal/claudegateway/store_test.go`、`backend/internal/claudegateway/handler_integration_test.go`、`backend/internal/claudegateway/router_test.go`(writeClaudeCodeSettings 测试所在文件以实现者核实为准，可能在 handler 侧测试文件）

- [ ] **Step 1: store 层失败测试**

`store_test.go` 追加（风格对齐既有用例）:

```go
func TestStoreBaseURLRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.json")
	s := NewStore(path)
	k, err := s.Create(Key{Name: "gw", APIKey: "sk-ant-x", Model: "claude-test", BaseURL: "https://gw1.example"})
	if err != nil {
		t.Fatal(err)
	}
	keys, err := NewStore(path).Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0].BaseURL != "https://gw1.example" {
		t.Fatalf("base_url 应随 keys.json 往返: %+v", keys)
	}
	// 更新语义：nil=保持；空串=清除；非空=设置。
	if _, err := s.Update(k.ID, KeyPatch{Name: ptr("gw2")}); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Load(); got[0].BaseURL != "https://gw1.example" {
		t.Fatalf("nil 补丁不得改动 base_url: %q", got[0].BaseURL)
	}
	if _, err := s.Update(k.ID, KeyPatch{BaseURL: ptr("")}); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Load(); got[0].BaseURL != "" {
		t.Fatalf("空串补丁应清除 base_url: %q", got[0].BaseURL)
	}
	if _, err := s.Update(k.ID, KeyPatch{BaseURL: ptr("https://gw2.example")}); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Load(); got[0].BaseURL != "https://gw2.example" {
		t.Fatalf("非空补丁应设置 base_url: %q", got[0].BaseURL)
	}
}

func TestStoreLegacyKeysJSONWithoutBaseURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.json")
	legacy := `{"keys":[{"id":"ck_old","name":"old","api_key":"sk-ant-old","model":"m","enabled":true,"priority":1,"weight":1}]}`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	keys, err := NewStore(path).Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0].BaseURL != "" {
		t.Fatalf("旧格式应兼容加载且 base_url 为空: %+v", keys)
	}
}

func ptr[T any](v T) *T { return &v }
```

（若 store_test.go 已有等价 ptr 助手则复用，不要重复定义；os/filepath 按需补 import。)

- [ ] **Step 2: 运行确认失败** — `go test -C backend ./internal/claudegateway -run 'TestStoreBaseURL|TestStoreLegacy' -count=1` → 编译失败。

- [ ] **Step 3: store 实现**

1) `Key` 在 `Model` 字段后加：

```go
	BaseURL                string    `json:"base_url,omitempty"`
```

2) `KeyPatch` 加 `BaseURL *string`。

3) `Update` 在 `patch.Model` 应用块后加：

```go
		if patch.BaseURL != nil {
			k.BaseURL = strings.TrimSpace(*patch.BaseURL)
		}
```

（归一化/校验由 handler 完成；store 保持 nil=保持、空串=清除。)

4) `SafeKey` 加 `BaseURL string \`json:"base_url"\``;`safeView` 加 `BaseURL: k.BaseURL,`。

- [ ] **Step 4: 运行确认通过** — 同上命令 → PASS。

- [ ] **Step 5: handler 层失败测试**

`handler_integration_test.go` 追加（对齐既有 create/update 用例的构造方式；断言响应 JSON 里 `base_url` 字段）:

- 创建带 `base_url:"https://gw1.example/"` → 201，响应 `base_url == "https://gw1.example"`（去尾 `/`)；列表 GET 同样含该字段；响应不含完整 api_key。
- 创建带 `base_url:"http://insecure.example"` / `"https://user:pw@gw.example"` → 400。
- 更新：`{"base_url":"https://gw2.example"}` → 200 且生效；`{"base_url":""}` → 200 且清除；不传该字段 → 保持不变。
- `writeClaudeCodeSettings`（单元级，t.TempDir() 目标文件）:Key 有 BaseURL 时写入该值；无 BaseURL 时仍写 `https://app.haoshuai.cc.cd`。

- [ ] **Step 6: 运行确认失败** — `go test -C backend ./internal/claudegateway -count=1` → 编译失败/断言失败。

- [ ] **Step 7: handler 实现**

`handler.go`(import 增加 `net/url`):

```go
// validateBaseURL 校验并归一化 per-Key base_url：空输入表示未设置（合
// 法）；非空必须是 https、有 host、无 userinfo，存储前去尾 /。
func validateBaseURL(raw string) (string, bool) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "", true
	}
	u, err := url.Parse(v)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return "", false
	}
	return strings.TrimRight(v, "/"), true
}
```

1) `createKeyRequest` 加 `BaseURL string \`json:"base_url"\``;`createKey` 在构造 `key` 前：

```go
	baseURL, ok := validateBaseURL(req.BaseURL)
	if !ok {
		badRequest(c, "base_url 必须是 https URL，且不得包含用户名密码。")
		return
	}
```

并把 `BaseURL: baseURL` 加入 `Key{...}` 字面量。

2) `updateKeyRequest` 加 `BaseURL *string \`json:"base_url"\``;`updateKey` 在 `patch` 构造（`BaseURL: req.BaseURL` 加入字面量）后：

```go
	if patch.BaseURL != nil {
		v, ok := validateBaseURL(*patch.BaseURL)
		if !ok {
			badRequest(c, "base_url 必须是 https URL，且不得包含用户名密码。")
			return
		}
		patch.BaseURL = &v // 空串 = 清除，回退环境变量
	}
```

3) `writeClaudeCodeSettings`:

```go
	baseURL := key.BaseURL
	if baseURL == "" {
		baseURL = "https://app.haoshuai.cc.cd" // 既有默认，未设置 base_url 的 Key 行为不变
	}
	doc.Env.AnthropicBaseURL = baseURL
```

（替换现 `doc.Env.AnthropicBaseURL = "https://app.haoshuai.cc.cd"` 一行。)

- [ ] **Step 8: 运行确认通过** — `go test -C backend ./internal/claudegateway -count=1` → PASS。

- [ ] **Step 9: Commit（需用户确认）**

```bash
git add backend/internal/claudegateway/
git commit -m "feat(claudegateway): Key 增加 base_url 字段与管理端点支持"
```

---

### Task B2: 消费方（Router + 两个 CLI)

**Files:**
- Modify: `backend/internal/claudegateway/router.go`(attempt 约 line 193-214)
- Modify: `backend/internal/worker/lessonimport/claude_url_cli.go`(Extract 约 line 84-111;env 约 line 204-217)
- Modify: `backend/internal/worker/aihandlers/quiz_cli.go`(Generate;env)
- Test: `backend/internal/claudegateway/router_test.go`、`backend/internal/worker/lessonimport/claude_url_cli_test.go`、`backend/internal/worker/aihandlers/quiz_cli_test.go`

- [ ] **Step 1: Router 失败测试**

`router_test.go` 追加：

```go
func TestRouterAttemptUsesKeyBaseURL(t *testing.T) {
	var paths []string
	var mu sync.Mutex
	mk := func() *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			paths = append(paths, r.URL.Path)
			mu.Unlock()
			w.Header().Set("content-type", "application/json")
			_, _ = io.WriteString(w, `{"model":"claude-test","stop_reason":"tool_use","content":[{"type":"tool_use","name":"structured_result","input":{"ok":true}}]}`)
		}))
	}
	srvA, srvB := mk(), mk()
	defer srvA.Close()
	defer srvB.Close()

	store := /* 写 keys.json：两个启用 Key 均 priority 1、weight 相同，srvA 的 Key
	           必须排在文件第一位（同优先级同权重按文件顺序胜出，保证断言确定）；
	           httptest 是 http —— store 层不校验 scheme，直接写文件 */
	/* 复用包内既有测试设施：newTestRouter / writeRouterKeys；State 用
	   NewStateTracker(filepath.Join(t.TempDir(), "state.json"), nil)；Logger 字段省略 */
	r := &Router{Store: store, State: <如上 StateTracker>, Endpoint: "http://unused.example/v1/messages", DefaultModel: "claude-test", MaxOutputTokens: 64}
	if _, err := r.Generate(context.Background(), aipkg.BuildConnectivityRequest()); err != nil {
		t.Fatal(err)
	}
	// 请求应到达 priority 最高 Key 的 srvA，且路径带 /v1/messages 后缀。
	if len(paths) != 1 || paths[0] != "/v1/messages" {
		t.Fatalf("应请求 Key 的 base_url 并补全 /v1/messages: %v", paths)
	}
}
```

关键点：httptest 是 http URL,**绕开 handler 校验直接写 keys.json**(store 层无 scheme 校验）；断言 `r.URL.Path == "/v1/messages"` 钉住后缀转换。另加两个用例：Key 无 BaseURL 时回退 `r.Endpoint`;`GenerateForKey` 同样走 Key 的 base_url。

- [ ] **Step 2: 运行确认失败** — `go test -C backend ./internal/claudegateway -run TestRouterAttempt -count=1` → 编译失败（messagesEndpoint 未定义）。

- [ ] **Step 3: Router 实现**

`router.go`:

```go
// messagesEndpoint 把基础 URL 归一为完整 Messages 端点；规则与
// config.ClaudeConfig.AnthropicMessagesEndpoint 一致。
func messagesEndpoint(base string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if strings.HasSuffix(base, "/v1/messages") {
		return base
	}
	return base + "/v1/messages"
}
```

`attempt` 中端点解析改为：

```go
	endpoint := r.Endpoint
	if key.BaseURL != "" {
		endpoint = messagesEndpoint(key.BaseURL)
	}
	provider := aipkg.NewAnthropicProvider(aipkg.AnthropicConfig{
		APIKey: key.APIKey, Model: model, Endpoint: endpoint,
		MaxOutputTokens: r.MaxOutputTokens, Timeout: ..., HTTPClient: r.HTTPClient,
	})
```

（`strings` 已在 router.go import；若无则补。)

- [ ] **Step 4: 运行确认通过** — `go test -C backend ./internal/claudegateway -count=1` → PASS。

- [ ] **Step 5: CLI env 失败测试**

`claude_url_cli_test.go` 与 `quiz_cli_test.go` 各追加表驱动用例：

```go
// 以 quiz_cli_test.go 为例（claude_url_cli_test.go 同构，调 Extract）：
func TestQuizCLIEnvBaseURLPrecedence(t *testing.T) {
	withURL := writeQuizCLIGatewayKeys(t, quizCLITestKey("k", quizCLIKeySentinel, 1, 100, true))
	// 给 keys.json 的 Key 补 base_url 字段：
	//   修改 quizCLITestKey 帮助函数签名或在本测试中直接写 map（推荐直接写，避免动既有用例）
	...
	envOf := func(cmd proc.Cmd) map[string]string { /* 解析 cmd.Env */ }
	// 用例 1：Key 有 base_url → env 的 ANTHROPIC_BASE_URL 用 Key 的（非构造 BaseURL）
	// 用例 2：Key 无 base_url → 用构造 BaseURL
	// 用例 3：Key 无 base_url 且构造 BaseURL 为空 → env 不含 ANTHROPIC_BASE_URL
}
```

- [ ] **Step 6: 运行确认失败** — 两包分别 `go test -C backend ./internal/worker/aihandlers ./internal/worker/lessonimport -run 'BaseURL' -count=1` → 失败。

- [ ] **Step 7: CLI 实现**

`claude_url_cli.go` 与 `quiz_cli.go` 同样改动（以 quiz_cli.go 为例）:

`Generate/Extract` 中：

```go
	baseURL := key.BaseURL
	if baseURL == "" {
		baseURL = g.BaseURL
	}
```

`env` 签名改为 `env(apiKey, baseURL string)`，函数体用参数 `baseURL` 替换原 `g.BaseURL`/`x.BaseURL` 引用；调用点 `Env: g.env(key.APIKey, baseURL)`。

注意 `claude_url_cli.go:26` 的头注释提到"keys.json 的 Key 记录没有 base_url 字段"——更新该注释为现状（Key.base_url 优先，进程 env 兜底）。

- [ ] **Step 8: 运行确认通过** — `go test -C backend ./internal/worker/aihandlers ./internal/worker/lessonimport ./internal/claudegateway -count=1` → PASS。

- [ ] **Step 9: Commit（需用户确认）**

```bash
git add backend/internal/claudegateway/router.go backend/internal/claudegateway/router_test.go backend/internal/worker/
git commit -m "feat(worker): 消费方按 Key.base_url 优先解析 Claude 端点"
```

---

### Task B3: OpenAPI 与前端设置页

**Files:**
- Modify: `api/openapi.yaml`(ClaudeKey line 1621;ClaudeKeyCreate line 1743;ClaudeKeyUpdate line 1791)
- Regenerate: `frontend/src/api/generated/**`(`cd frontend && npm run api:gen`)
- Modify: `frontend/src/features/claude-gateway/claude-key-dialog.tsx`
- Modify: `frontend/src/features/claude-gateway/claude-gateway-section.tsx`(KeyCard)
- Modify: `frontend/src/mocks/fixtures.ts`

- [ ] **Step 1: OpenAPI 三个 schema 增字段**

`ClaudeKey` properties（不进 required):

```yaml
        base_url:
          type: string
          description: 该 Key 专属的 Anthropic base URL；空字符串表示未设置，使用部署侧 ANTHROPIC_BASE_URL。
```

`ClaudeKeyCreate` properties:

```yaml
        base_url:
          type: string
          description: 可选；非空必须是 https URL（无 userinfo），存储时去尾 /。
```

`ClaudeKeyUpdate` properties:

```yaml
        base_url:
          type: string
          description: 可选；省略保持不变，空字符串表示清除（回退部署侧 ANTHROPIC_BASE_URL）。
```

- [ ] **Step 2: 契约校验**

Run: `make verify-openapi && make check-openapi-breaking`
Expected: 通过（纯新增）。

- [ ] **Step 3: 重新生成客户端**

Run: `cd frontend && npm run api:gen`
Expected: `src/api/generated/model/claudeKey*.ts` 出现 `base_url?: string`。

- [ ] **Step 4: 表单与展示**

`claude-key-dialog.tsx`:

1) `KeyDraft` 加 `baseUrl: string`;`emptyDraft` 加 `baseUrl: ''`;`draftFor` 加 `baseUrl: keyRecord.base_url ?? ''`。
2) `submit` 校验：非空时须以 `https://` 开头，否则 `setFormError('Base URL 必须是 https:// 开头的地址')`。（前端只做前缀校验，完整校验以后端为准——有意不对称。)
3) 组装 payload:`common.base_url = draft.baseUrl.trim()`；创建时若为空则从 payload 删除该键（`if (!common.base_url) delete common.base_url`——仅在创建分支），编辑分支总是发送（空串清除）。
4) UI：在 model 字段后加文本 `Input`(`id="claude-key-base-url"`,Label "Base URL",placeholder `https://api.anthropic.com`,`autoComplete="off"`)，下方 `<p className="text-xs text-text-subtle">留空使用部署环境变量 ANTHROPIC_BASE_URL。</p>`；样式对齐既有 name/model 字段。

`claude-gateway-section.tsx` 的 `KeyCard` 指标 `<dl>` 加一行：`Base URL` → `keyRecord.base_url || '默认（环境变量）'`（等宽/截断样式对齐邻近行）。

`fixtures.ts` 的 `demoClaudeKeys` 每条加 `base_url: ''`（或一条示例值）。

- [ ] **Step 5: 前端验证**

Run: `cd frontend && npm run typecheck && npm run test && npm run build`
Expected: 全过。

- [ ] **Step 6: Commit（需用户确认）**

```bash
git add api/openapi.yaml frontend/
git commit -m "feat(settings): Claude Key 表单支持 per-Key base_url"
```

---

### Task B4: 全量回归与生产冒烟

**Files:** 无新增改动。

- [ ] **Step 1:** `make go-fmt && make go-vet && make go-test` → 全过。
- [ ] **Step 2:** `cd frontend && npm run typecheck && npm run test && npm run build` → 全过（若 B3 已做则确认无回归）。
- [ ] **Step 3:** 重建并重启容器：`docker compose --env-file deploy/.env -f deploy/compose.yaml build api worker web && docker compose --env-file deploy/.env -f deploy/compose.yaml up -d --no-deps api worker web`。
- [ ] **Step 4:** 用户在设置页为 Key 填入 `https://wahaapi.top` 与有效 api_key 保存（或提供值由执行者经 API 写入）→ 重发课时《做空》测验生成 → 断言 job 成功、quiz 发布、`ai_runs` 恰一条 `quiz-cli/v1` 记录。

---

## 自查清单（执行完毕标准）

- [ ] keys.json 旧格式兼容；Update nil/空串/非空三语义有测试
- [ ] Router 请求路径含 `/v1/messages`；GenerateForKey 继承 per-Key 端点
- [ ] 两个 CLI 的 env 注入优先级三态（Key 值/构造值/缺省）有测试
- [ ] 前端创建留空不发送、编辑空串清除；KeyCard 展示
- [ ] `check-openapi-breaking` 通过；全量 go-test 与前端测试通过
- [ ] api_key 不出现在任何响应/日志/错误 detail
