# 前端 Codex 实施提示词

将下方完整提示词交给负责前端的 Codex。该 Agent 负责实现 React 前端，不得改变后端领域规则、OpenAPI 语义或部署安全边界。

```text
你是 AI 学习助教 Web 版的前端负责人。请在当前仓库中实施一个可生产构建的 React 前端，服务单个个人用户在本机浏览器中学习。不要做营销首页，首屏必须是可使用的产品。

## 目标与边界

- 前端：React + TypeScript + Vite，目录为 apps/web。
- 后端：Go + Gin；通过同源 REST + SSE 访问，不在浏览器直接访问 Claude。
- 用户：单用户、本机部署；没有登录、注册、多租户、付费、公开分享或公网模式。
- API Key：绝不出现在前端环境变量、源码、构建产物、日志、错误提示或浏览器存储中。
- 视觉、交互和组件细节由你负责；不要改写产品状态机、评分、掌握度、任务重试或安全规则。

## 开始前必须阅读

1. docs/web-rebuild/README.md
2. docs/web-rebuild/01-product-requirements.md
3. docs/web-rebuild/02-frontend-handoff.md
4. docs/web-rebuild/04-api-contract.md
5. docs/web-rebuild/05-deployment-operations.md
6. docs/web-rebuild/08-frontend-codex-prompt.md
7. api/openapi.yaml（发布契约；若尚未由后端生成，阅读 docs/web-rebuild/openapi.yaml 仅作设计参考，并在报告中标记此阻塞）

先运行 git status，保留所有已有用户修改。先检查仓库现状和可用脚本，再决定创建或接入 apps/web；不要重置工作树。

## 技术要求

使用以下技术，并锁定实际版本：

- React、TypeScript、Vite。
- TanStack Router 负责类型安全路由和路由级代码拆分。
- TanStack Query 管理所有服务端实体、缓存、失效、重试和 optimistic state；业务实体不能复制到 Zustand/Redux 等全局 Store。
- React Hook Form + Zod 管理表单和本地即时校验。
- 从 api/openapi.yaml 用 Orval 或 OpenAPI Generator 生成 TypeScript Client 和类型。禁止手写与 OpenAPI 重复的请求/响应类型；生成产物可重复生成且不可手改。
- Radix UI 或同等可访问基础组件，Lucide 图标，react-markdown + KaTeX + Shiki。
- Vitest + React Testing Library，Mock Service Worker，Playwright。
- 只允许 VITE_ 前缀的非敏感配置，例如 VITE_APP_TITLE；API 默认使用同源相对路径，不能配置 Claude Key、数据库密码或服务端 URL secret。

如果 api/openapi.yaml 尚未存在：不要伪造后端契约或手写重复类型。完成独立的布局、路由、设计 token、MSW 接线和可替换 API 边界，同时报告“等待后端生成发布 OpenAPI 契约”的阻塞；在契约到位后再生成 Client 并完成联调。

## 产品信息架构

实现并按需调整以下资源路由，保持资源层级：

- /dashboard
- /courses、/courses/new、/courses/:courseId
- /courses/:courseId/lessons/:lessonId 及 notes、outline、quizzes、attempts、summary 子视图
- /attempts/:attemptId
- /jobs、/jobs/:jobId
- /weaknesses
- /progress
- /settings

课程与课时是主导航上下文；任务中心必须全局可见；答题使用专注模式，但退出、刷新和断线都不能丢答案。设置页只展示 Claude 连通性状态和系统健康，不能展示或编辑 API Key。

## 体验与视觉质量

为高频学习和管理操作设计安静、专业、信息密度适中的学习工作台，而不是卡片堆砌的 SaaS 落地页。确定一套清晰的视觉语言和 design tokens（颜色、字体、层级、间距、边框、阴影、动效），再全局一致执行。

- 用清晰的正文排版支撑长 Markdown 阅读；目录锚点稳定，内容更新不能突然改变滚动位置。
- 不使用泛滥的紫色渐变、装饰性光球、营销 Hero、圆角套圆角卡片或无意义的动效。
- 常用操作可扫描、可键盘完成；图标按钮使用 Lucide 并具备 tooltip/accessible name。
- 桌面与窄屏均不能发生文字溢出、遮挡或点击目标过小。
- 支持 prefers-reduced-motion；状态不依赖颜色表达；必要状态才使用 ARIA live region。
- 页面、重型 Markdown/代码高亮、图表和考试编辑器必须路由级或组件级懒加载。设置合理 manual chunks，做 bundle analyzer；初始 JS gzip 目标小于 350KB，不包含按需模块。

## API、缓存与错误语义

Query Key 至少包含：

['system', 'status']
['courses', filters]
['course', courseId]
['lessons', courseId]
['lesson', lessonId]
['documents', lessonId]
['quizzes', lessonId]
['quiz', quizId]
['attempt', attemptId]
['jobs', filters]
['job', jobId]
['weaknesses', filters]
['progress', courseId]

Mutation 成功后只失效关联资源，禁止用全局 invalidateQueries 造成页面闪烁。所有创建/任务型 POST 为一次用户意图生成 UUID Idempotency-Key；网络重试、双击和刷新恢复时复用原 Key。

后端错误使用 application/problem+json。按稳定 code 分支，不能按中文 detail 解析：

- validation_error：字段旁显示错误并在表单顶部汇总。
- version_conflict：进入资源或单题答案冲突流程。
- resource_busy：读取 active_job_id，跳转/关联正在运行的 Lesson 生成任务，不换幂等键重试。
- idempotency_in_progress：保持同一 Key，轮询或延后重试。
- claude_not_configured / claude_unavailable：保留输入和任务上下文，提示系统能力降级。
- job_not_retryable：禁用重试，展示原因。
- storage_full、upload_too_large、network_error：不清空用户输入，提供明确可恢复动作。

所有请求应保留并在诊断信息中暴露响应 X-Request-ID，但不得把它或错误正文无节制写入遥测。

## 后台 Job 与 SSE

连接 GET /api/v1/jobs/stream，保存 Last-Event-ID，并实现带随机抖动的指数退避，最大等待 15 秒。SSE 只提升体验，REST Job 资源才是最终事实。

OpenAPI 定义三类 SSE Envelope，必须分别处理：

1. PersistedJobStreamEvent：按 job_id 合并 status、stage、progress 和 message 到 Query Cache。
2. JobResyncRequiredStreamEvent：用 high_watermark_event_id 更新游标，立即请求 active_jobs_url 与当前页面相关 Job/资源；不能当作某个 Job 的事件。
3. HeartbeatStreamEvent：仅更新时间戳，不写 Job Cache，也不改变 Last-Event-ID。

重连成功后仍请求 active=true 的 Job 列表。慢消费者可以丢进度帧，但最终状态必须依靠 REST 对账。所有任务 UI 都要覆盖 queued、running、validating、succeeded、failed 可重试、failed 不可重试、cancel_requested、cancelled。

同一 Lesson 的素材导入、笔记重生成、普通出题、复习出题共享 lesson_generation；出现 409 resource_busy 时关联现有 active_job_id，不创建第二个任务。

## 关键业务交互

### 导入与文档

- 区分文件上传进度和后台导入/AI 处理进度。
- URL、文本、上传均使用同一课时上下文；离开页面后 Job 继续运行。
- Markdown 禁止原始 HTML/脚本执行；代码块可复制，公式降级时显示原始内容。
- 当前文档集合不分页；文档历史版本使用 API page_info 游标分页。

### 答题与自动保存

- 输入立即写入内存草稿和 sessionStorage。
- 停止输入 800ms 后 PATCH 单题答案；同题请求严格串行，不同题允许并发。
- 新答案 answer_version 从 0 开始；成功回写该题版本与 Attempt 全局 version。乱序响应下 Attempt version 取最大已见值。
- 切题、visibilitychange、离开答题和提交前必须 flush 全部脏答案。
- 409 时用服务端 current_answer/current_answer_version 进行安全合并；不能判断时让用户选择，不能静默覆盖。
- 离线或保存失败明确显示“尚未同步”，绝不伪造已保存。
- 提交时等待全部保存完成，携带最新 Attempt version，并在成功后锁定编辑。
- 200 AttemptSubmitCompleted 不含 job，直接展示已评分结果；202 AttemptSubmitAccepted 必有 attempt_grade job，显示客观题暂定分与批阅状态。
- grading_failed 时保留答案和客观题结果：有可重试 Job 时使用 Job Retry；没有可用 Job 或来自旧数据时调用 POST /attempts/{attemptId}/grading-jobs，并复用 Idempotency-Key。

### 其它行为

- 普通生成测验和复习测验调用不同 endpoint；前端不计算题量、难度、评分和掌握度。
- 删除、取消、重新生成、恢复等高影响操作需要明确确认，但不要用嵌套弹窗堆叠流程。
- Claude 连通性测试是后台 Job；前端不读取或提交 Key。

## 工程与测试门禁

1. 配置 ESLint、TypeScript strict、格式化、测试命令、生产 build 和 bundle report。
2. 将 API Client 生成、类型检查、单元测试、E2E、生产 build 接入统一验证命令。
3. MSW 基于 OpenAPI 示例覆盖：空状态、导入中、导入失败、resource_busy、autosave conflict、批阅中、grading_failed、重新批阅、SSE 断线/resync/heartbeat。
4. Playwright 至少覆盖：创建课程和文本课时、观察 Job 后阅读笔记、生成试卷、自动保存并刷新恢复、提交客观题与主观题、失败后重试、窄屏键盘导航。
5. 开发结束前运行安装后的全量前端检查，并报告真实命令输出。若 API 契约或后端未就绪导致 E2E 无法执行，明确报告，不要声称通过。

## 交付方式

按以下顺序工作：

1. 阅读文档并报告现状、依赖缺口和执行计划。
2. 搭建 apps/web 基础设施、tokens、路由、Client 生成/替换边界和 MSW。
3. 实现核心课程/任务/阅读/答题/结果流程，再实现辅助页面。
4. 为每个关键状态添加组件和交互测试；最后做 Playwright、可访问性、窄屏和性能验证。
5. 不要因为后端暂未实现就改动 OpenAPI 或发明接口；发现缺口时记录具体 use case、期望请求/响应和兼容影响，交给后端评审。

最终报告必须包含：修改文件、视觉和交互决策、API 生成命令、测试/构建真实结果、未覆盖风险、后端契约阻塞项。不要提交真实 Key 或用户学习材料。
```
