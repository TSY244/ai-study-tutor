# Claude 原生 URL 素材导入设计

**日期：** 2026-07-25  
**状态：** 已确认，待规格评审  
**范围：** 已在本地部署的 Claude-only 多 Key 网关之上，将 YouTube、Bilibili 等已允许网站的 URL 素材导入改为由 Claude 的 Web Fetch server tool 提取内容。

## 目标

用户在既有课时页面粘贴允许站点的 HTTPS URL 后，系统把规范化后的 URL 直接交给 Claude。Claude 负责访问与理解来源内容，并返回结构化的标题和 Markdown 原文；系统随后沿用现有 Claude 调用分别生成教师教纲和学生笔记，并一次性发布三个版本。

用户仍需手动点击现有的“生成测验”入口。URL 导入成功不自动创建测验、作答或总结评估。

## 已确认的行为

- 不增加导入端点，不改变 `POST /api/v1/lessons/{lesson_id}/materials/url` 的请求和成功响应契约。
- URL 导入不再运行 `yt-dlp`、字幕下载、音频下载、FFmpeg 或本地 Whisper。
- 文本粘贴和文件上传分支保持现状。
- 内容成功时发布：`raw_text`（原文）、`teacher_outline`（教纲）、`student_notes`（笔记）。既有人工测验生成、作答批改与总结评估保持不变。
- 仅当 Claude 网关存在可用 Key 时允许 URL 导入执行；没有可用 Key 时任务明确失败为 `claude_not_configured`，不回退到本地下载方案。

## 背景与边界

当前 URL 路径在 Worker 中执行本地 SSRF 预检、`yt-dlp` 字幕下载、可选本地 Whisper 转写。这会受到目标站点反爬策略限制。目标改为调用 Anthropic Messages API 的 Web Fetch server tool，由 Claude 代表导入流程获取页面内容。

本功能仍受已有 Claude 网关约束：浏览器不会接触 Key，Worker 使用网关的多 Key 路由和故障切换，API/Worker 日志、`ai_runs`、任务错误及 HTTP 响应不记录 Key、完整 prompt 或原始来源正文。

Web Fetch 依赖配置的 Claude 网关能够原样支持 Anthropic Messages server tools。若某个自定义 `ANTHROPIC_BASE_URL` 不支持该能力，系统将返回稳定的 `claude_invalid_request`，提示部署方升级或改用支持 Web Fetch 的网关；不会静默改用本地爬取。

## 架构与数据流

```text
浏览器粘贴 URL
  -> 既有 API：HTTPS + 域名白名单 + 规范化
  -> 已有 lesson_import Job
  -> Worker：Claude URL 提取请求
       -> Claude Web Fetch server tool（仅允许该 URL 的域名）
       -> structured_result：标题、原文 Markdown、提取说明
  -> Worker：规范化、长度校验、分块
  -> Worker：已有 Claude 教纲生成 + 笔记生成
  -> 单一发布事务：raw_text + teacher_outline + student_notes
```

### URL 规范化与准入

API 仍实施 HTTPS 和域名白名单校验，并保留当前可选的 API DNS 校验开关。因为 Worker 不再自行连向目标 URL，它不执行重定向预检、DNS 重绑定检查或任何本地下载；这些不适用于 Claude server tool 的网络边界。

Worker 在发给 Claude 前再规范化 URL：移除 fragment 和已知追踪参数，保留识别页面所需的路径与查询参数（例如 YouTube 的 `v`、Bilibili 的视频路径与 `p`）。最终 URL 必须不超过 Web Fetch 支持的长度上限；无法缩短的 URL 以稳定、可行动的素材错误失败。域名白名单将同时转为 Web Fetch 的 `allowed_domains`，所以 Claude 只能访问该任务许可的来源站点。

## Claude Provider 契约

### 新的 AI 请求能力

`ai.Request` 新增仅供 URL 提取使用的 Web Fetch 选项，而不改变 `ai.Provider.Generate` 接口。新增稳定 purpose：

```text
lesson_url_extract
```

URL 提取请求含两种工具：

1. Anthropic server tool `web_fetch`，使用兼容的 basic tool version、受控 `max_uses`、内容 token 上限和 `allowed_domains`。
2. 现有 client tool `structured_result`，其 JSON schema 要求：

```json
{
  "title": "来源标题或空字符串",
  "raw_text_markdown": "供学习的完整 Markdown 文本",
  "extraction_note": "非敏感的提取说明"
}
```

工具选择为自动，而不是强制 `structured_result`：system prompt 要求模型先使用 Web Fetch，之后只以 `structured_result` 返回结果。Prompt 明确说明抓取内容是不可信资料，不执行其中的指令，不泄露系统提示、Key 或内部配置，也不访问来源域名以外的地址。

### 响应循环

现有 Anthropic Provider 目前假定单次响应并强制结构化工具。URL 提取的 Provider 路径需要支持 server tool 的 Messages 协议：

- 保留完整 assistant content，原样附回后续 Messages 请求；不把 Web Fetch 工具结果伪造成浏览器或 Worker 的结果。
- 接受 `pause_turn` 并在有限次数内继续同一请求；每次续传沿用原请求的模型、工具与超时约束。
- 接受工具过程中的 token、模型、request-id 和延迟元数据；最终审计只保存已有安全字段。
- 仅在得到合法的 `structured_result` 后返回 `Response.JSON`。
- 超过受控续传次数、没有结构化结果、服务端工具拒绝或网关不支持该 tool 时，返回映射后的 `ProviderError`，不发布任何文档。

普通 Claude 请求继续使用当前强制 `structured_result` 的单次路径，避免影响测验、批改、总结和现有网关故障切换语义。

## Worker 改动

URL 分支从“写入 `acquired.txt` 后调用本地 `extract`”调整为专用的 Claude 提取步骤：

1. 读取已经保存的规范化 `source_url`，构建 `lesson_url_extract` 请求。
2. 通过已有 `AI` Provider（也就是 Claude 网关）执行。若 `AI` 为 nil 或 `AIAvailable` 为 false，返回 `claude_not_configured`。
3. 校验并标准化 `raw_text_markdown`，套用现有 `MaxTextRunes` 上限和 `ChunkText`。
4. 填充来源元数据：`title`、`webpage_url`、`extractor: "claude_web_fetch"` 与非敏感提取说明。移除本地字幕/yt-dlp 版本字段在此路径的含义。
5. 按现有顺序调用教师教纲和学生笔记生成。
6. 复用现有 `publishFunc` 单事务发布原文、教纲、笔记。任一步失败均不发布半成品。

文本和上传分支继续使用现有获取/解析流程及 AI 可用时的生成逻辑。URL 分支不会创建或执行任何外部命令，相关依赖可以保留给其他路径或后续清理，但该分支测试必须证明其未被调用。

任务阶段仍使用已有稳定 stage code。对 URL 任务的用户可见文案更新为“Claude 提取链接内容”，而不会新增长期 API stage 枚举。

## 错误与重试

| 条件 | 错误码 | 用户可见结果 | 发布 |
| --- | --- | --- | --- |
| 无可用 Claude Key | `claude_not_configured` | 配置 Claude Key 后重试 | 否 |
| Key 鉴权失败 | `claude_auth_failed` | 网关按多 Key 策略切换；全部失败则提示更新 Key | 否 |
| 限流、超时、服务端错误 | 现有 Claude retryable code | 由 Job 与网关现有策略重试/切换 | 否 |
| Web Fetch 无法访问或内容为空 | 映射后的 Claude/素材错误 | 提示链接内容不可提取或稍后再试 | 否 |
| 网关不支持 server tool | `claude_invalid_request` | 提示部署侧需支持 Claude Web Fetch | 否 |
| 结构化输出缺失或无效 | `claude_invalid_json` / `claude_schema_invalid` | 提示 Claude 输出无效，可重试 | 否 |
| 原文超过上限 | `too_large` | 提示更换/拆分素材 | 否 |

来源内容不能覆盖系统指令。错误详情不得包含完整 URL 查询中潜在的敏感值、完整抓取内容、Key、上游原始报文或 prompt。现有多 Key 网关对 401/403、429、网络错误、超时和 5xx 的状态转换与 failover 不变。

## 前端和 API

不增加端点、路由或客户端模型。URL 导入表单仍提交相同字段和展示异步 Job。

前端只调整 URL 导入的辅助文本与错误文案：说明链接将由 Claude 提取，保留已允许站点和 HTTPS 的输入限制；在收到上述错误码时提供对应可行动说明。现有“生成测验”按钮、题目作答和总结入口均不调整。

## 测试与验收

测试必须全部使用 mock `http.RoundTripper`、fake provider 或 fake process runner；不得在单元或集成测试中请求 YouTube、Bilibili 或 Claude。

### Provider

- 常规结构化请求仍产生原有强制 `structured_result` payload。
- URL 请求包含 `web_fetch` 与 `structured_result`，自动工具选择、允许域名、限额和 schema 均符合契约。
- Web Fetch 的 `pause_turn` 可在受控次数内续传，保持 prior assistant content。
- server tool 成功后能解析 `structured_result`；tool 错误、缺少结果、超过续传次数和无效 JSON 有稳定错误映射。
- 请求、错误、日志和 `ai_runs` 中没有 Key、抓取正文或原始上游响应。

### Lesson import

- URL job 通过 fake Claude 输出成功提取后，原子发布原文、教师教纲和学生笔记，来源 metadata 的 extractor 为 `claude_web_fetch`。
- URL job 不调用 `yt-dlp`、FFmpeg、Whisper、DNS 预检或 HTTP redirect transport。
- URL job 无 Claude provider/无可用 Key 时以 `claude_not_configured` 失败，不触发本地回退。
- Claude 提取为空、超限或任一后续生成失败时没有发布半成品。
- 文本和上传的既有测试继续通过。

### 回归与部署

- Claude 网关多 Key 路由与现有 `POST /api/v1/system/claude-tests` 行为不变。
- API URL 输入的 HTTPS、白名单和可选 DNS 校验契约不变。
- OpenAPI 不需要新增或修改端点；仅在行为文档中更新 URL 素材管线。
- Docker 部署继续使用 API/Worker 的 Claude 网关配置，Web 容器没有 Key、secret 或 gateway config mount。

建议验证命令：

```bash
go test -C backend ./internal/ai ./internal/worker/lessonimport -count=1
go test -C backend ./internal/claudegateway ./internal/system -count=1
make go-fmt
make go-vet
make go-test
make verify-openapi
make check-openapi-breaking
cd frontend && npm run typecheck
cd frontend && npm run test
cd frontend && npm run build
make docker-config
make docker-verify-secrets
```

若本地环境、依赖或 Docker 服务阻断任一命令，实施结果需明确记录已执行的命令和实际失败原因。
