# 后端 Agent 分阶段执行手册

本手册把 Go/Gin、Worker、PostgreSQL、Claude 与 Docker 重构拆为可独立验收的任务。每次只把“公共上下文”与一个阶段提示词交给一个 Agent；阶段验收通过并产出交接物后，才能进入下一阶段。

## 使用规则

1. 每次新开 Agent 任务前先提供公共上下文与本阶段 Prompt，附上上一阶段交接报告。
2. Agent 只能完成当前阶段，不得提前实现后续功能或重写用户已有修改。
3. API 的发布契约是 `api/openapi.yaml`；设计源是 `docs/web-rebuild/openapi.yaml`。必须先展开公共 Header，再生成发布契约与客户端。
4. 每个阶段结束前必须执行 Prompt 指定的真实命令，报告输出、风险和下阶段输入。
5. 任何契约冲突、数据删除、真实 Key 依赖、公共网络/多用户需求都必须停止并报告。

## 公共上下文 Prompt

```text
你负责 AI Study Tutor Web 重构的后端阶段。系统是单用户、本机部署的学习工具，不是 SaaS。

固定架构：Go + Gin 模块化单体；独立 Go Worker；PostgreSQL 任务队列；React 前端通过同源 REST/SSE 访问；Docker Compose 只监听 127.0.0.1；Claude 仅由 Worker 使用 Docker secret 调用 Anthropic API。

开始前阅读：
- docs/web-rebuild/README.md
- docs/web-rebuild/01-product-requirements.md
- docs/web-rebuild/03-backend-architecture.md
- docs/web-rebuild/04-api-contract.md
- docs/web-rebuild/05-deployment-operations.md
- docs/web-rebuild/07-migration-roadmap.md
- docs/web-rebuild/openapi.yaml
- docs/web-rebuild/10-backend-agent-playbook.md
- legacy/python/ 与 legacy-data/ 仅作为行为与迁移输入参考，不能修改 legacy-data。

硬约束：
- 先检查 git status；保留用户已有修改，不使用 reset/checkout 覆盖工作树。
- Handler 只做 HTTP 绑定和响应映射；业务在 application/domain，SQL 在 repository，Claude/子进程只在 Worker adapter。
- 任何业务写入都有明确事务边界；文件采用 temporary row -> hash -> atomic rename -> ready/link 的两阶段发布。
- 不记录 API Key、密码、完整学习材料、完整 Prompt 或上游原始响应。
- 不引入 Redis、RabbitMQ、微服务、Kubernetes、GraphQL、登录/多租户或公网能力。
- 测试默认使用 fake Claude/本地 fixture；不得要求真实 Key。

交付报告必须包含：修改文件、行为、迁移/API 变化、实际验证命令和输出、风险、给下一阶段的稳定接口。
```

## B0：基线、发布契约与工程约定

```text
任务：冻结旧 Python 行为基线，并建立 Go/前端共用的发布 OpenAPI 契约；不实现业务 API。

执行：
1. 在 legacy/python/ 用 `uv run --extra dev pytest -q` 记录当前基线；仅用脱敏 fixture，不读取 legacy-data 真实内容。
2. 为题量策略、客观判分、薄弱点和掌握度建立 JSON golden fixture；最小脱敏课程 fixture 必须提交到 `backend/testdata/legacy-fixtures/`，不能写入被忽略的 `legacy-data/`。
3. 实现设计源到 api/openapi.yaml 的确定性生成：展开 x-common-response-headers 到所有成功/错误 Response。
4. 添加 verify-docs、verify-openapi、test-legacy 命令；CI 检查每个发布 Response 含 X-Request-ID，引用可解析，operationId 唯一。
5. 写明发布 OpenAPI 的生成命令、输入和禁止手改规则。

验收：旧测试通过；fixture 无隐私/secret；api/openapi.yaml 能解析；前端可只依赖发布契约生成类型。
停止条件：设计源存在破坏性歧义时，列出 use case 和兼容影响，不自行发明字段。
```

## B1：Go 骨架、配置与 HTTP 门禁

```text
任务：创建可编译的 Go 工程、api/worker/migrate/legacy-import 入口和 HTTP 质量门禁，不实现业务链路。

执行：
1. 创建 backend/go.mod、backend/cmd/api、backend/cmd/worker、backend/cmd/migrate、backend/cmd/legacy-import，及 backend/internal/platform/{config,httpserver,database,logging,clock,id}。
2. 实现环境变量与 *_FILE secret 读取；Claude Key 缺失是能力降级，不影响 healthz/非 AI 功能。
3. 实现 /healthz、/readyz、Request ID、Problem Details、panic recovery、结构化日志、Host/Origin 校验、上传大小限制和 SSE 路由例外超时。
4. Gin 类型不得泄露到 domain/application；时间、ID、配置均可注入测试。
5. 接入 gofmt、go vet、go test -race、静态检查与 OpenAPI 校验。

验收：数据库不可用时 healthz=200、readyz=503；日志/错误快照无 secret；API/Worker 可优雅 SIGTERM；所有质量命令通过。
交接物：启动命令、配置矩阵、Problem 类型、Request ID 行为。
```

## B2：数据库、Job Queue、文件与维护恢复

```text
任务：实现 PostgreSQL schema、Repository、可靠 Job Queue、文件两阶段发布和维护恢复；暂不接 Claude。

执行：
1. 用版本化 migration 建立课程、课时、素材、文档、Quiz、Attempt、Weakness、Job、事件、幂等、备份、system_state、legacy import 表。
2. 用数据库约束表达不变量：course slug、Quiz item、active document、lesson_generation 活动 Job、idempotency 等。
3. 使用 SELECT FOR UPDATE SKIP LOCKED、lease、心跳、取消、指数退避和 worker_lost 回收；同一 Job 不可重复领取。
4. 实现 temporary/ready/quarantined/deleted 文件状态与恢复；业务实体不得引用 temporary 对象。
5. 实现 Maintenance Owner Lock + Business Gate + 数据库时间租约/heartbeat；manual maintenance-repair 先回收过期 Job/file Owner，再清理遗留维护状态。
6. 实现通用 Idempotency-Key 回放、冲突、in_progress 和文件发布恢复。

验收：真实 PostgreSQL 集成测试覆盖并发领取、租约恢复、部分唯一索引、幂等并发、SIGKILL 维护修复和文件恢复。
交接物：migration 版本、Repository 接口、Job 状态/错误码表、测试数据库启动方式。
```

## B3：课程、课时、素材、文档与导出 API

```text
任务：实现 Course/Lesson/Material/Document/Export REST 主链路；API 不下载 URL、不调用 Claude、不执行长任务。

执行：
1. 从 api/openapi.yaml 生成或校验 Go DTO；实现课程和课时 CRUD、游标分页、乐观锁和 Problem 映射。
2. 实现文本、URL、上传素材入口。上传流式写 temporary 文件并计算 hash；URL 只规范化/SSRF 校验后创建 Job。
3. 素材导入、文档重生成、普通/复习出题统一占用 lesson_generation；唯一冲突返回 409 resource_busy + active_job_id。
4. 实现 current documents（非分页 DocumentSummaryCollection）、文档版本历史（游标分页）、文件下载授权边界与导出 Job 创建。
5. 覆盖路径穿越、私网/重定向 SSRF、MIME 伪造、超限 multipart、软删除冲突与幂等回放。

验收：无 Claude Key 可完成 CRUD、素材入队、文档查询；响应与 OpenAPI 契约测试一致。
交接物：前端可用 API 列表、Query 失效关系、MSW 示例和稳定错误码。
```

## B4：确定性学习领域

```text
任务：实现 Quiz、Attempt、客观判分、Weakness 和 Mastery，不调用模型。

执行：
1. 将 legacy Python 策略迁移为表驱动 golden 测试；Question/Quiz 发布后不可变。
2. 实现普通与复习 Quiz 组装、Attempt 创建、学生可见题目快照、客观题规范化判分。
3. 实现单题 answer_version=0 自动保存；同题串行、不同题可并发；提交使用 Attempt 全局 version。
4. 拆分提交响应：200 AttemptSubmitCompleted 不含 Job；202 AttemptSubmitAccepted 必含 attempt_grade Job。
5. 实现 grading_failed 的 Job Retry 与 Attempt 级 grading-jobs 重建入口；追加 mastery snapshots 和 weakness evidence，重复消费不重复写入。

验收：golden 测试、提交事务、版本冲突、幂等、反馈可见性、失败重试和 mastery 重放均通过。
交接物：前端答题状态机、autosave/submit 示例、反馈 Schema 与进度 API。
```

## B5：Worker 素材管线

```text
任务：实现 Worker Runner、素材安全处理、标准化文本和文档发布；暂不调用 Claude 生成内容。

执行：
1. 实现 Job handler registry、阶段事件、取消、超时和按 job_id 隔离的临时目录。
2. 支持文本、PDF/DOCX/TXT/Markdown；URL 素材由容器内 Claude Code CLI WebFetch 提取原文（不再本地下载 yt-dlp，需出网访问白名单站点）；yt-dlp/ffmpeg 仅服务上传媒体，且只通过参数数组调用。
3. 实现 canonicalClaudeURL 规范化（强制 HTTPS、白名单、剥离追踪参数、250 字符上限）、文本上限、解析器版本（URL 为 claude-web-fetch/v1）和可复现 chunking。
4. 上传音视频处理 subtitle_only 与 whisper_local；无字幕时返回可行动 transcript_required（URL 素材不适用，走容器内 Claude Code CLI WebFetch）。
5. 所有发布走 B2 文件恢复协议；失败/取消不改变 active 文档。

验收：fixture + fake process runner 覆盖超时、取消、非零退出、SSRF、重复文件和中断发布。
交接物：Job kind/stage 表、素材错误码、可供前端演示的 seed 数据。
```

## B6：Claude Provider 与 AI 任务

```text
任务：仅在 Worker 中接入 Anthropic Provider，把结构化输出应用到已稳定的领域接口。

执行：
1. 定义 provider interface 与 Anthropic adapter；领域层不得依赖 SDK 类型。
2. 仅读取 ANTHROPIC_API_KEY_FILE；API 容器没有 Key、没有 Claude 出网网络。
3. 实现 connectivity、lesson import/regenerate、quiz/review quiz、attempt grade、wrong answer analyze、summary evaluate handlers。
4. Prompt/JSON Schema 版本化；ai_runs 记录模型、hash、token、request id、延迟和脱敏状态。
5. 处理 refusal、截断、429、5xx、超时、无效 JSON、Schema/领域校验失败；只有通过业务事务的结果才发布。

验收：Fake Provider 覆盖所有故障矩阵；默认 CI 不需要真实 Key；真实 smoke 必须显式 opt-in。
交接物：Claude 能力状态、可重试分类、前端可展示的 Job 错误语义。
```

## B7：SSE、发布契约与前端联调包

```text
任务：完成稳定 REST/SSE 联调包。

执行：
1. 维护 api/openapi.yaml、examples、生成命令与 breaking-change 检查。
2. 实现三类 SSE Envelope：PersistedJobStreamEvent 使用 JobStreamData；stream.resync_required 携带 high watermark/active_jobs_url；heartbeat 不带 SSE id。
3. 支持 Last-Event-ID、30 天回放、断档 REST 对账与慢消费者；SSE 不泄露学习正文或答案。
4. 为前端输出生成 TypeScript Client、MSW fixtures、seed 命令、错误码表、SSE 重连示例。

验收：真实响应契约测试、SSE 回放/resync/heartbeat 测试通过；前端无需猜测字段。
交接物：发布 OpenAPI、client 生成命令、fixtures、联调 URL 与版本号。
```

## B8：Docker、备份恢复与发布门禁

```text
任务：实现个人电脑 Compose 部署。前端未交付时只做 API/Worker/Tools，不伪造 Web 页面。

执行：
1. 创建多阶段 backend/Dockerfile.api、backend/Dockerfile.worker、backend/Dockerfile.tools；运行时非 root、只读 rootfs、最小权限。
2. 实现 compose、Caddy、secret 文件、db->migrate->api/worker 启动顺序；只有 Web 映射 127.0.0.1 端口。
3. 实现 backup、restore、doctor、maintenance-repair、legacy-import profiles；只有 Worker 有 Anthropic secret。
4. 做空环境启动、重启数据保留、Worker 强杀、维护恢复、备份到空卷恢复、SSE 代理与 secret 泄漏测试。

验收：docker compose config、健康检查、fake Claude 闭环、备份恢复与 doctor 通过。
交接物：部署命令、故障演练记录、前端 Docker 接口要求。
```

## B9：旧数据导入与发布审查

```text
任务：实现 legacy-import 并完成发布审查。

执行：
1. 只读扫描 legacy-data/，从 /legacy/course 和 /legacy/.tasks 生成 Manifest、冲突和 JSON 报告。
2. 实现 staging/import batch、source hash、幂等、resolution、reconciliation；pending/failed 历史批阅映射为 grading_failed + legacy_task_not_resumable。
3. 重复导入 fixture 不复制数据；不完整历史记录隔离而非伪造。
4. 以发布审查者身份检查 PRD、OpenAPI、并发、AI 故障、安全、Compose、恢复与前端交接物。

验收：scan 不写入；import 对账成功；预切换演练、回滚演练和发布审查 findings 完成。
交接物：导入报告、残余风险、可发布/不可发布结论。
```
