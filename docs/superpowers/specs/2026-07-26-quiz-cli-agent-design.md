# 测验生成改为 Claude Code CLI Agent 文件契约设计

**日期：** 2026-07-26  
**状态：** 规格评审已通过（2026-07-26,reviewer Approved;3 条建议已吸收）
**范围：** 将 `quiz_generate` / `review_quiz_generate` 两类 Job 的题目生成从"资料全文嵌入单次 Messages API 调用"改为"资料落盘为文件，由 worker 容器内 Claude Code CLI agent 自行阅读并按文件契约产出 quiz.json"。

## 目标

测验生成不再受单次响应 `max_tokens` 上限截断：agent 通过 Read 工具按需阅读资料，通过 Write/Bash 分轮写出结果文件，每一轮 agent 输出独立计数，不撞单响应 8192 token 上限（生产事故：job `3bee625c-f4b7-4a74-9cbe-667091b0aa34` 第一次 `claude_invalid_json`、第二次 `claude_output_truncated` 终态失败）。

输入侧同步受益：约 2 万字符的资料不再挤占 prompt，agent 自行分段读取。

## 已确认的行为

- 只改题目"生成"环节。`loading_context`（读 DB）、`quizAssembleInput` / `Validate`（领域校验）、`AssemblyService.AssembleInTx`（发布）、Job stage 文案与进度百分比、API 端点与前端全部不变。
- 输入文件固定两个：`raw_text.md`（课时原文，必填）、`student_notes.md`（学生笔记，可为空文件）。与现状一致：复习测验当前同样只使用这两份输入（`quizGenerateHandler.lessonContext` 不读取薄弱点）。
- 输出文件固定一个：`quiz.json`，JSON 结构等同于现有 `quizSchema`（`quiz-result/v1`）：`title` + `questions[]`，题目字段、枚举（type/difficulty/cognitive_level）不变。
- 结果校验语义不变：`quiz.json` 解析后复用现有 `quizResult` → `quizAssembleInput` → `input.Validate()` 流水线；校验失败仍映射 `claude_domain_invalid`（可重试）。
- 仅当存在 Claude 网关配置（`CLAUDE_GATEWAY_CONFIG_FILE` 非空，与 `URLExtractor` 同条件）时装配 CLI 生成路径；否则保持现有 Messages API Provider 路径不变（含 fake provider 的本地开发与测试部署）。
- 不自动推广到 attempt_grade、lesson_regenerate、summary_evaluate 等其他 Claude 流程。

## 背景与边界

生产上测验生成通过网关调用 Messages API，单次请求嵌入 raw_text（实测 15,110 字符）+ student_notes（4,073 字符），要求一次产出 8 题中文结构化 JSON，`CLAUDE_MAX_OUTPUT_TOKENS=8192`。中文 token 密度高，8 题完整 JSON 逼近或超过上限；输出截断（`stop_reason=max_tokens`）按设计不可重试，任务直接终态失败。

仓库已有同一病根的解法先例：`lessonimport/claude_url_cli.go` 的 `ClaudeURLCLIExtractor`（2026-07 替代 web_fetch server tool）——worker 容器内执行 `claude -p <prompt> --output-format json`，结果走文件契约绕开输出上限。本设计把同一模式应用到测验生成。

边界与安全约束与 URL 提取一致：

- CLI 凭证取自网关 keys.json（每次调用重选当前优先级最高的启用 Key），`api_key` 只注入子进程 env，绝不进入日志、错误 detail、`ai_runs` 或任何持久化位置；`base_url` 来自进程级 `ANTHROPIC_BASE_URL`。
- 子进程 env 白名单：`HOME`/`PATH`/`SHELL=/bin/bash`/`CI=1`/`NO_COLOR=1` + Anthropic 凭证。
- 资料文本是不可信数据：prompt 明确其不得修改指令、不得请求 secret/Key/prompt。
- 工作目录复用 runner 已有的 `tmp/{job_id}/`（`worker.Execution.WorkDir`，任务结束自动清理），不新增目录生命周期管理。

## 架构与数据流

```text
quizGenerateHandler.Execute（常规与复习同一 handler，Review 标志区分）
  ├─ loading_context(10%): lessonContext 读 DB（不变）
  ├─ 新增: 仅当走 CLI 路径时,将 raw / student 写入 ex.WorkDir/{raw_text.md, student_notes.md}(回退路径不写盘)
  ├─ planning(25%)（不变,仅 stage 事件;复习 job 的 stage 名为 planning_review,本设计不改名）
  ├─ generating_questions(55%):
  │     已装配 QuizCLI → ClaudeQuizCLIGenerator.Generate(ctx, workDir, req)
  │        ├─ keys.json 选最高优先级启用 Key
  │        ├─ claude --allowedTools "Bash Read Write" -p <prompt> --output-format json
  │        │     agent: Read 两个资料文件 → 命题 → 写 quiz.json（可分轮追加）→ 回复 done
  │        └─ 读 workDir/quiz.json → aipkg.Response{JSON, Usage, Latency, Model, ProviderRequestID=session_id}
  │     未装配 QuizCLI → 现有 b.Provider.Generate 路径（不变）
  ├─ callWith 记录恰好一条 ai_run（purpose=quiz_generate / review_quiz_generate）（不变）
  ├─ validating(85%): quizResult 解析 + quizAssembleInput + Validate（不变）
  └─ publishing(95%): AssemblyService.AssembleInTx（不变）
```

## 组件改动

### 新增 `backend/internal/worker/aihandlers/quiz_cli.go`

- `QuizGenerator` 接口：`Generate(ctx context.Context, workDir string, req aipkg.Request) (aipkg.Response, error)`，与 `lessonimport.URLExtractor` 形态对齐。
- `ClaudeQuizCLIGenerator` 实现，字段对齐 URL 提取器：`Proc proc.Runner`、`KeysFile`、`BaseURL`、`Home`、`ClaudeBin`（默认 `claude`）、`Timeout`（默认 10 分钟，job 取消/总时限优先）。
- 选 Key、env 白名单、信封（`claudeCLIEnvelope`）解析逻辑与 URL 提取器同构；`--allowedTools` 用 `"Bash Read Write"`（既有用户确认白名单 `"Bash Read Write WebFetch"` 的子集，本流程不需要 WebFetch）。
- 文件契约常量：`quizCLIInputRawFile = "raw_text.md"`、`quizCLIInputNotesFile = "student_notes.md"`、`quizCLIOutputFile = "quiz.json"`。

### Prompt 构造（`BuildQuizCLIPrompt`，纯函数）

输入：`LessonTitle`、`QuestionCount`（默认 8）、`Review`。输出中文 prompt，要点：

1. 列出两个输入文件的绝对路径（`{workDir}/raw_text.md`、`{workDir}/student_notes.md`；笔记文件可能为空，空表示无笔记）。
2. 要求用 Read 工具阅读全部资料后，为课时《标题》命 N 道{常规|复习}测验题；题目必须来自资料，不得编造。
3. 不可信数据声明（同上）。
4. 输出契约：将结果写入 `{workDir}/quiz.json`，并内嵌紧凑格式说明（`title` + `questions[]` 各字段、type/difficulty/cognitive_level 枚举、options 仅选择题需要、score > 0）；要求只写 JSON、不写 Markdown 围栏。
5. 内容较长时用 Bash `cat >>` 分次追加；写完用 Read 复查 JSON 完整后回复一行 `done`。

用于审计的 `aipkg.Request`：`Purpose` 沿用 `quiz_generate` / `review_quiz_generate`；新增 prompt 版本常量 `quiz-cli/v1` / `review-quiz-cli/v1`；`User` 为该 prompt（只含路径与要求，不含资料正文）；`SchemaVersion` 沿用 `quiz-result/v1`；`Schema` 不随 CLI 使用（结构化约束由文件契约 + 下游校验保证）。

### `aihandlers/handler.go`

- `Deps` 新增可选字段 `QuizCLI QuizGenerator`（nil 表示未装配）。
- `quizGenerateHandler` 持有该字段；`Execute` 中：资料写盘 → 构造 CLI prompt 与审计 Request → `h.callWith(ctx, ex, req, func(ctx, req) { return h.QuizCLI.Generate(ctx, ex.WorkDir, req) })`；`QuizCLI` 为 nil 时走现有 `BuildQuizRequest` + `h.call` 分支，行为与今天完全一致。

### `cmd/worker/main.go`

- `buildClaudeProviders` 在装配 `URLExtractor` 的同一分支（网关配置存在）内装配 `ClaudeQuizCLIGenerator`（同样的 Proc/KeysFile/BaseURL/Home/ClaudeBin），经 `aihandlers.Deps.QuizCLI` 注入。

## 错误与重试

| 条件 | 错误码 | 可重试 |
| --- | --- | --- |
| 网关无启用 Key / keys.json 读取失败 | `claude_not_configured` / `claude_unavailable` | 否 / 是 |
| CLI 二进制不存在 | `claude_not_configured` | 否 |
| CLI 调用超时（父 ctx 存活） | `claude_timeout` | 是 |
| 信封 `api_error_status` 401/403 | `claude_auth_failed` | 否 |
| 信封 `api_error_status` 429 | `claude_rate_limited` | 是 |
| 信封 `api_error_status` 5xx | `claude_unavailable` | 是 |
| 信封 `is_error` / 非 success / 非零退出且无有效信封 | `claude_unavailable` | 是 |
| `quiz.json` 缺失或非法 JSON | `claude_invalid_json` | 是 |
| 结果未通过 `quizAssembleInput`/`Validate` | `claude_domain_invalid` | 是 |

语义要点：`claude_output_truncated` 在本路径不再可能发生（文件契约消除单响应截断）；CLI 路径不经网关 Router，无 Key 冷却/failover 状态机，429/5xx 仅由 Job 重试（`max_attempts` 封顶）。注意上表与 URL 提取先例的映射**有意不同**：URL 提取把超时/429/5xx 折叠为 `material_fetch_failed`、把 `is_error`/非零退出视为不可重试；测验生成没有"抓取失败"语义，改用标准 `claude_timeout`/`claude_rate_limited`/`claude_unavailable` 并全部可重试，实现时不得向先例"对齐"。错误 detail 沿用现有安全文案，不含 Key、prompt 或资料正文；`ai_runs` 一条逻辑调用记录，usage 取 CLI 信封，Model 取所选 Key 记录，request_id 取 session_id。

## 前端和 API

无任何端点、路由、客户端模型或文案改动；`POST /lessons/{id}/quizzes` 契约与 Job 轮询行为不变。

## 测试与验收

测试全部使用 `proc.Fake`、fake provider 或 httptest mock，不得真实调用 Claude CLI 或 Anthropic。

### quiz_cli 单元测试（表驱动，对齐 `claude_url_cli_test.go`）

- fake CLI 按文件契约写出合法 `quiz.json` → 返回的 `Response.JSON` 可解为 `quizResult`，usage/latency/session 映射正确。
- `quiz.json` 缺失、非法 JSON → `claude_invalid_json`（可重试）。
- 信封 401/403/429/5xx、`is_error`、非零退出、CLI 缺失、超时 → 上表错误码与 retryable。
- prompt 包含两个输入文件绝对路径、题数、模式与不可信数据声明；不包含资料正文。
- api_key 只出现在子进程 env，不出现在错误、日志或 `ai_runs`（sentinel 断言）。

### handler 集成测试

- 注入 fake `QuizGenerator` 的 `quiz_generate` job 端到端成功：DB 出现 quiz + questions，ai_run 恰一条。
- `QuizCLI=nil` 时现有 fake provider 路径的全部集成测试不变、继续通过。
- 复习测验（`review_quiz_generate`）同路径验证 `source=retest`。

### 回归

- `go test -C backend ./internal/worker/aihandlers ./internal/worker/lessonimport ./internal/ai -count=1`
- `make go-fmt go-vet go-test`
- quiz policy golden 测试不变。

## 非目标

- 不把 CLI agent 路径推广到批改、总结、文档生成等其他 purpose。
- 不改动 keys.json 结构、网关 Router、前端、OpenAPI。
- 不为测验生成引入薄弱点文件输入（现状未使用，保持行为一致）。
