# 前端 Agent 分阶段执行手册

本手册用于独立驱动 React 前端 Agent。每次只执行一个阶段；前一阶段的测试、视觉/交互交接物和后端发布契约必须被验收后，才进入下一阶段。

## 使用规则

1. 每个前端阶段都附带“公共上下文 Prompt”和当前阶段 Prompt。
2. 后端尚未提供 `api/openapi.yaml` 时，前端可完成 Shell、路由、tokens、MSW 边界，但不得手写重复 DTO 或发明接口。
3. 所有实体数据由 TanStack Query 管理；客户端 Store 只保存 UI 偏好、未同步答案草稿、SSE 游标和任务通知。
4. Agent 不得修改后端状态机、评分、掌握度、Job 语义、Claude Key 边界或 OpenAPI；发现缺口必须记录给后端。

## 公共上下文 Prompt

```text
你负责 AI Study Tutor Web 重构的 React 前端。用户是单个个人用户，在本机浏览器学习；首屏是实际产品，不是营销页。

固定架构：React + TypeScript + Vite 位于 frontend；Go/Gin 后端通过同源 REST/SSE 提供数据；发布契约为 api/openapi.yaml；Claude Key 只存在 Worker Docker secret，浏览器不可读取或提交。

开始前阅读：
- docs/web-rebuild/README.md
- docs/web-rebuild/01-product-requirements.md
- docs/web-rebuild/02-frontend-handoff.md
- docs/web-rebuild/04-api-contract.md
- docs/web-rebuild/08-frontend-codex-prompt.md
- docs/web-rebuild/11-frontend-agent-playbook.md
- api/openapi.yaml（若不存在，仅把 docs/web-rebuild/openapi.yaml 用作设计参考并在报告中标记阻塞）

硬约束：
- 先检查 git status，保留用户已有修改，不重置工作树。
- 使用 React、TypeScript strict、Vite、TanStack Router/Query、React Hook Form + Zod、Radix 或等价可访问组件、Lucide、MSW、Vitest、Playwright。
- API 类型和 Client 必须从发布 OpenAPI 生成，生成物不可手改；不把业务实体复制到全局 Store。
- API 默认使用同源相对路径；只允许 VITE_ 非敏感变量；绝不在前端出现 API Key/数据库密码。
- 界面是安静、密集、可扫描的学习工作台：不做营销 Hero、紫色渐变、无意义卡片堆砌或装饰性动效。
- 必须支持键盘、焦点管理、屏幕阅读器、reduced motion、窄屏；文本不能重叠或溢出。

报告必须包含：修改文件、用户流程、API/缓存变化、实际测试/构建输出、未完成依赖和给下一阶段的交接物。
```

## F0：契约确认与前端工程底座

```text
任务：建立 frontend 工程、质量门禁和 API 契约边界；不假设后端已运行。

执行：
1. 创建 Vite + React + TypeScript strict；配置路径别名、ESLint、格式化、Vitest、RTL、Playwright、bundle analyzer。
2. 配置现代浏览器 target、路由级 lazy loading、manual chunks、source map 策略和 VITE_ 环境类型。
3. 实现 OpenAPI Client 生成脚本。若 api/openapi.yaml 缺失，创建可替换的 adapter/MSW 接线，不手写重复 API DTO，并在报告中阻塞联调。
4. 建立全局 Error Boundary、Query Client、Problem Details 解析、Request ID 诊断和无障碍基础样式。

验收：lint、typecheck、unit test、production build、bundle report 可运行；生成命令可重复；无敏感环境变量。
交接物：工程命令、依赖清单、Client 生成状态、后端契约阻塞项。
```

## F1：应用 Shell、信息架构与设计系统

```text
任务：实现实际产品 Shell、路由层级和视觉 tokens，不接业务细节。

执行：
1. 建立 /dashboard、/courses、课程/课时子路由、/attempts、/jobs、/weaknesses、/progress、/settings。
2. 设计可访问 tokens（颜色、排版、间距、边框、状态、动效），创建 App Shell、导航、面包屑、命令区、空/加载/错误框架。
3. 任务中心必须全局可见；答题路由使用专注模式；设置页不展示 Key。
4. 验证桌面和窄屏布局、tab 顺序、dialog 焦点与 reduced motion。

验收：路由 lazy loading、生效的导航状态、键盘可达、无营销首页；关键组件有组件测试。
交接物：路由表、tokens、组件清单、状态矩阵。
```

## F2：Client、Query、表单与错误语义

```text
任务：接入生成 Client、Query Key、表单和稳定错误映射。

执行：
1. 建立 courses/course/lessons/lesson/documents/quizzes/quiz/attempt/jobs/job/weaknesses/progress Query Keys。
2. 只精确失效相关 Query；禁止全局 invalidateQueries。
3. 实现 UUID Idempotency-Key 生命周期：同一次创建/任务意图的重试、双击、恢复复用同一 Key。
4. 实现 Problem code 映射：validation_error、version_conflict、resource_busy、idempotency_in_progress、claude_not_configured、claude_unavailable、job_not_retryable、storage_full、upload_too_large、network_error。
5. 使用 MSW 覆盖成功、失败、冲突和加载状态；保留响应 X-Request-ID 用于诊断。

验收：表单错误绑定、重复提交保护、错误不清空输入、Query 失效测试通过。
交接物：Client 调用层、错误映射表、MSW fixtures。
```

## F3：课程、课时、导入与任务中心

```text
任务：实现课程浏览、课时创建、文本/URL/文件导入和全局任务体验。

执行：
1. 实现课程/课时列表、创建、编辑、归档/删除确认、游标分页与空状态。
2. 实现 URL、文本、上传素材的表单；清楚区分上传进度和后台 Job 进度。
3. 接到 resource_busy 时展示 active_job_id 并关联现有 lesson_generation，不能换 Key 再建任务。
4. 实现 Job 列表/详情、阶段、可重试失败、取消与重试操作。

验收：创建课程到导入文本课时的 MSW/集成流程通过；离开页面后任务仍在全局任务中心可见。
交接物：课程/任务流程、Mutation 与失效关系、Job UI 状态表。
```

## F4：Markdown 阅读、文档版本与进度

```text
任务：实现安全、舒适的长文阅读和学习进度视图。

执行：
1. 使用安全 Markdown 渲染，禁用原始 HTML/脚本；支持稳定标题锚点、目录、复制代码、KaTeX 降级和懒加载高亮。
2. 使用 DocumentSummaryCollection 显示当前文档；文档历史版本用 page_info 游标分页。
3. 内容更新时提示，不在阅读中强制替换滚动位置。
4. 实现 Weakness/Progress 查询与趋势展示；前端不重算掌握度。

验收：XSS 安全测试、长文性能测试、文档版本/空状态/窄屏阅读测试通过。
交接物：Markdown 安全配置、文档缓存策略、进度组件接口。
```

## F5：Quiz、Attempt 与可靠自动保存

```text
任务：实现考试专注流程、单题自动保存、刷新恢复和提交。

执行：
1. 显示学生可见 Quiz 快照，提交前不渲染标准答案/解析。
2. 输入立即写入内存和 sessionStorage；800ms debounce PATCH；同题串行、不同题并发。
3. answer_version 初始为 0，成功回写 answer_version/attempt_version；乱序响应取最大 Attempt version。
4. 切题、visibilitychange、离开和提交前 flush；离线/失败显式显示未同步。
5. 409 使用 current_answer/current_answer_version 安全合并；不能判断时让用户选择，绝不静默覆盖。
6. 区分 200 AttemptSubmitCompleted（无 job）与 202 AttemptSubmitAccepted（必有 job），提交成功锁定作答。
7. grading_failed 保留答案/客观分；有 Job 用 retry，无 Job/旧数据用 Attempt grading-jobs。

验收：autosave、并发、刷新恢复、冲突、flush、200/202、grading_failed 的组件和 E2E 测试通过。
交接物：autosave 状态机、sessionStorage 格式、Attempt 结果页行为。
```

## F6：SSE、恢复与跨页一致性

```text
任务：实现可靠的 Job SSE 客户端与跨页面任务一致性。

执行：
1. 保存 Last-Event-ID，使用带抖动指数退避重连，最大 15 秒；重连后请求 active=true Job 列表。
2. PersistedJobStreamEvent 按 job_id 合并状态/进度；不让高频进度重渲染整棵应用。
3. stream.resync_required 使用 high_watermark_event_id 更新游标，调用 active_jobs_url 和当前资源 REST 对账；不能当 Job Event。
4. heartbeat 只更新连接活性，不写 Job Cache，也不改变 Last-Event-ID。
5. 在课程、任务详情、考试批阅结果和通知中心一致展示 Job 变化。

验收：MSW/测试流覆盖回放、断线、history expired、heartbeat、慢消费者和最终 REST 对账。
交接物：SSE adapter、重连策略、事件诊断面板（开发模式）。
```

## F7：前端发布验收

```text
任务：完成前端质量、安全、性能和 Docker 联调验收，不新增业务功能。

执行：
1. 补齐 Playwright：课程/文本导入、Job 成功、笔记阅读、出题、自动保存/刷新、客观与主观提交、失败重试、窄屏键盘路径。
2. 检查可访问性：焦点、label、live region、颜色、reduced motion、移动端文本与点击区。
3. 检查性能：初始 gzip JS、小路由/Markdown/图表懒加载、长列表分页、上传不读入 JS 内存。
4. 验证生产 build 在 Caddy 静态回退与同源 API/SSE 下工作；构建产物和环境变量没有 Key。
5. 输出后端契约缺口、视觉残余风险和可发布/不可发布结论。

验收：lint、typecheck、unit、E2E、production build、bundle report 和浏览器联调均有真实输出。
交接物：发布检查报告、性能数据、前端 Docker 构建要求。
```
