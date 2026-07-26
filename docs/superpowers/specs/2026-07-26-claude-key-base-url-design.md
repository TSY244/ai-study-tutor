# Claude 网关 per-Key base_url 设计

**日期：** 2026-07-26  
**状态：** 规格评审已通过（2026-07-26;1 个 Issue(/v1/messages 后缀转换）已修正，3 条建议已吸收）  
**范围：** 为 Claude 网关的每个 Key 增加独立 `base_url` 字段，前端设置页表单可直接编辑，保存时与 Key 一并写入 keys.json；网关 Router、两个 CLI 路径与 Claude Code settings 导出按"Key.base_url > 进程环境变量 ANTHROPIC_BASE_URL"优先级消费。

## 目标

用户在设置页编辑某个 Claude Key 时，可以在同一表单里填写 Base URL（如 `https://wahaapi.top`)，点击保存后与 Key 一起持久化到 keys.json，全部消费方热生效（keys.json 每次调用重读），无需改 `.env`、无需重启容器。不同 Key 可分属不同网关；未设置 base_url 的 Key 保持现状（走进程级 `ANTHROPIC_BASE_URL`)。

## 已确认的行为

- base_url **每个 Key 独立**（用户已确认），不做全局设置。
- 可空。创建时省略/留空 = 不设置；更新时字段缺席 = 保持不变，**空字符串 = 清除**（回退环境变量）。
- 校验：非空时必须是 https URL、有 host、无 userinfo；存储前去空格并去掉尾部 `/`。
- base_url 不是机密：出现在 Key 的安全视图（list/get 响应）与前端列表展示中。
- 消费优先级统一为 `Key.base_url` 非空时用之，否则回退现有默认值：
  - 网关 Router(`attempt`)：回退 `r.Endpoint`（进程 env 推导）。
  - `ClaudeURLCLIExtractor` / `ClaudeQuizCLIGenerator`：回退各自 `BaseURL` 字段（装配自进程 env)。
  - `writeClaudeCodeSettings`：回退现有硬编码默认值 `https://app.haoshuai.cc.cd`（保持现状，仅当 Key 有 base_url 时覆盖）。
- OpenAPI:`ClaudeKey`/`ClaudeKeyCreate`/`ClaudeKeyUpdate` 三个 schema 增加 `base_url`；纯新增，不破坏既有契约。`ClaudeKey.base_url` 不进 required 列表（避免 response 侧 breaking 判定）,Go 侧始终序列化（空串表示未设置）。创建端点的幂等哈希基于整个请求体（`httpx.CanonicalJSON(req)`),新字段自动纳入，无需额外处理。
- 前端：表单校验（可空；非空须 https:// 开头）；创建时留空则不发送该字段；编辑时总是发送（空串清除）;`KeyCard` 展示 base_url，未设置显示"默认（环境变量）"。前端只做前缀级校验、后端做完整校验（host/userinfo)，这一不对称是**有意**的：后端为权威，实现时不得"对齐"两边。

## 背景与边界

现状 keys.json 的 Key 记录没有 base_url 字段，所有消费方统一使用进程级 `ANTHROPIC_BASE_URL`（当前指向 `https://app.haoshuai.cc.cd`，该账户余额已耗尽）。切换到新网关目前必须改 `deploy/.env` 并重启容器，且对该网关下所有 Key 一刀切。

边界与非目标：

- 不改动 Key 的选择/冷却/failover 语义；base_url 不参与路由决策，只在已选中 Key 后决定发往哪个端点。
- 不改 `ANTHROPIC_BASE_URL` 环境变量本身的装配；它仍是所有 Key 的兜底。
- api_key 安全约束不变：不出现在日志、错误 detail、`ai_runs`、响应中；base_url 不是机密。
- 不处理 keys.json 之外持久化的任何配置。

## 组件改动

### 后端 `backend/internal/claudegateway`

- `store.go`:`Key` 增加 `BaseURL string \`json:"base_url,omitempty"\``(Load 兼容旧文件）;`KeyPatch` 增加 `BaseURL *string`;`Update` 按 nil=保持/空串=清除/非空=设置 应用（handler 已先行校验归一化）;`SafeKey` 与 `safeView` 增加 `base_url`。
- `handler.go`:`createKeyRequest` 增加 `BaseURL string`;`updateKeyRequest` 增加 `BaseURL *string`；新增 `validateBaseURL(raw string) (string, bool)` 纯函数（trim、https、host、无 userinfo、去尾 `/`)，创建/更新接入；错误文案中文且不回显 api_key。
- `router.go`:`attempt` 端点解析 `endpoint := r.Endpoint; if key.BaseURL != "" { endpoint = messagesEndpoint(key.BaseURL) }`。注意 `Router.Endpoint` 与 `NewAnthropicProvider` 的入参是**完整 Messages 端点**（含 `/v1/messages`，见 `config.ClaudeConfig.AnthropicMessagesEndpoint`,config.go:113-120)，而 `base_url` 是基础 URL，必须转换：新增未导出助手 `messagesEndpoint(base string) string`，规则与 config 完全一致（去尾 `/`；已含 `/v1/messages` 后缀则原样；否则追加）。测试须断言请求路径含 `/v1/messages`(httptest handler 校验 r.URL.Path)，防止静默丢后缀。`GenerateForKey`(Key 连通性测试）同样流经 `attempt`，自动继承该行为，加一条断言钉住。
- `handler.go` 的 `writeClaudeCodeSettings`:`key.BaseURL` 非空时覆盖硬编码默认。

### 后端 CLI 路径

- `lessonimport/claude_url_cli.go` 与 `aihandlers/quiz_cli.go`:`Generate/Extract` 内解析 `baseURL := key.BaseURL; if baseURL == "" { baseURL = x.BaseURL }`,`env` 注入解析后的值（两个文件的 env 构造函数各加一个参数）。

### OpenAPI 与前端

- `api/openapi.yaml` 三个 schema 增加 `base_url`（类型 string；创建/更新可选；响应视图可选、空串语义注明）。
- 前端 `npm run api:gen` 重新生成客户端与模型类型。
- `frontend/src/features/claude-gateway/claude-key-dialog.tsx`:`KeyDraft`/`emptyDraft`/`draftFor` 增加 `baseUrl`;`submit` 组装（创建留空省略、编辑总是发送）；新增文本 `Input`（样式对齐 name/model 字段）,placeholder `https://...`，非空校验 https:// 前缀；辅助说明"留空使用环境变量 ANTHROPIC_BASE_URL"。
- `claude-gateway-section.tsx` 的 `KeyCard` 指标 `<dl>` 增加一行展示。
- `src/mocks/fixtures.ts` 的 `demoClaudeKeys` 补字段。

## 错误处理

| 条件 | 结果 |
| --- | --- |
| 创建/更新提供非法 base_url（非 https、缺 host、含 userinfo) | 400 `bad_request`，中文文案，不回显敏感值 |
| base_url 指向不可达主机 | 沿用既有消费方错误码（`claude_unavailable` 等），不新增错误码 |

## 测试与验收

全部 mock/fake，不请求真实网关。

- `store_test.go`：带 base_url 的创建/加载往返；Update 设置、清除（空串）、保持（nil）三种语义；旧版无 base_url 的 keys.json 兼容加载。
- `handler_integration_test.go`(claudegateway)：创建合法/非法 base_url；更新设置与清除；响应含 base_url 且不含完整 api_key。
- `router_test.go`：两个 Key 各指不同 httptest 端点，证明按 Key 路由；未设置时回退 Router.Endpoint。
- `claude_url_cli_test.go` / `quiz_cli_test.go`：env 注入优先 Key.base_url，未设置时用构造 BaseURL。
- `writeClaudeCodeSettings` 测试：覆盖与回退两种情形。
- 前端：`npm run typecheck`、`npm run test`、`npm run build`。
- 契约：`make verify-openapi`、`make check-openapi-breaking`。
- 全量：`make go-fmt`、`make go-vet`、`make go-test`。

## 验收路径（生产）

用户在设置页编辑 Key → 填 `https://wahaapi.top` 与新 api_key → 保存 → keys.json 热生效 → 重跑课时《做空》测验生成冒烟（job 成功、quiz 发布、`ai_runs` 为 `quiz-cli/v1`)。
