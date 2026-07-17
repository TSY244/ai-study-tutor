# 前端开发交接文档

## 1. 文档定位

本文提供前端开发所需的产品边界、信息架构、技术约束、交互状态和验收标准，不提供具体页面视觉稿。前端开发负责视觉语言、排版、组件细节和动效，但不得改变核心业务状态和接口契约。

## 2. 技术基线

建议技术栈：

- React + TypeScript + Vite。
- TanStack Router：类型安全路由和页面级数据依赖。
- TanStack Query：服务端状态、缓存、失效和重试。
- React Hook Form + Zod：表单状态和前端输入校验。
- OpenAPI Generator 或 Orval：由 `openapi.yaml` 生成 API Client 和类型。
- Radix UI 或等价无样式可访问组件：Dialog、Popover、Select、Tabs、Toast。
- Tailwind CSS 或团队既有样式方案。
- Lucide Icons。
- `react-markdown` + KaTeX + Shiki：Markdown、数学公式和代码块。
- Vitest + React Testing Library；Playwright 负责关键业务流程。

具体依赖版本由实施时锁定。禁止手写一套与 OpenAPI 重复的请求/响应 TypeScript 类型。

## 3. 前端职责边界

前端负责：

- 展示和操作课程、课时、任务、试卷、作答、反馈、薄弱点和系统状态。
- 对用户输入做即时校验和可理解的错误展示。
- 订阅 SSE，并在断线后恢复任务状态。
- 考试答案本地暂存、自动保存、冲突处理和提交前刷新。
- 对耗时任务提供非阻塞反馈。
- 对 Markdown 做安全、可访问、适合长文阅读的渲染。

前端不负责：

- 计算最终分数和掌握度。
- 决定题量、难度或复习策略。
- 解析媒体、拼接 Claude Prompt 或校验 AI 业务结果。
- 保存或展示 Anthropic API Key。
- 用本地状态伪造后端任务成功。

## 4. 信息架构与路由

路由名称可调整，但资源层级应保持一致：

```text
/
├── /dashboard
├── /courses
│   ├── /new
│   └── /:courseId
│       └── /lessons/:lessonId
│           ├── /notes
│           ├── /outline
│           ├── /quizzes
│           ├── /attempts
│           └── /summary
├── /jobs
│   └── /:jobId
├── /attempts/:attemptId
├── /weaknesses
├── /progress
└── /settings
```

设计原则：

- 课程和课时是主要导航上下文。
- 任务中心是全局能力，不要求用户停留在发起任务的页面。
- 考试过程应使用专注模式，但不能丢失自动保存和退出恢复能力。
- 任务详情、错误详情和确认操作使用适当的页面或对话框，避免层层嵌套弹窗。
- 不创建营销型首页，首屏就是实际产品。

## 5. 全局应用状态

### 5.1 服务端状态

全部业务数据通过 TanStack Query 管理。建议 Query Key：

```ts
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
```

Mutation 成功后按资源关系精确失效，避免全局 `invalidateQueries()` 导致所有页面刷新。

### 5.2 客户端状态

仅下列状态可以由轻量客户端 Store 管理：

- 主题和界面偏好。
- 当前课程/课时的最近访问记录。
- 尚未发送的考试答案草稿。
- SSE 连接状态和最近事件 ID。
- 可关闭但需要跨页面保留的任务通知。

业务实体不得复制到全局 Store，与 Query Cache 形成双事实来源。

## 6. 关键交互规范

### 6.1 创建和导入课时

- URL、文本和文件上传使用相同的课时创建语义。
- 上传前展示文件名、类型、大小和限制。
- 大文件上传必须显示独立上传进度；上传完成不代表导入任务完成。
- API 返回 `lesson` 和 `job` 后立即进入可观察状态。
- 用户离开页面后，任务继续执行；全局区域展示活动任务数量。
- 重复点击由前端禁用提交和 `Idempotency-Key` 双重保护。

### 6.2 后台任务

任务 UI 必须区分：

- 排队中。
- 正在运行及当前阶段。
- 正在校验。
- 成功。
- 失败且可重试。
- 失败且不可重试。
- 正在取消。
- 已取消。

SSE 事件只用于更新体验，任务详情 REST 响应是最终状态来源。SSE 重连后必须重新拉取活动任务，避免漏事件。

建议重连流程：

1. 保存服务端事件 ID。
2. 断线后采用带抖动的指数退避，最大 15 秒。
3. 使用 `Last-Event-ID` 重连。
4. 重连成功后重新请求活动任务列表。
5. 收到 `stream.resync_required` 时，使用事件中的 `high_watermark_event_id` 更新游标，并立即请求 `active_jobs_url` 和当前页面相关资源；该事件不是某个 Job 的状态，不能写入 Job Cache。
6. `heartbeat` 只更新连接活性时间，不写入 Job Cache，也不覆盖已保存的 Last-Event-ID。
7. 浏览器进入后台时不停止答案自动保存。

同一 Lesson 的素材导入、内容重生成、普通出题和复习出题统一只允许一个活动生成 Job。`409 resource_busy` 返回 `active_job_id` 与 `generation_family=lesson_generation` 时，前端关联并展示现有任务，不自动更换 `Idempotency-Key` 重复提交。

### 6.3 Markdown 阅读

- 禁止直接执行 Markdown 中的 HTML 和脚本。
- 标题生成稳定锚点，支持目录定位。
- 代码块支持复制；数学公式在加载失败时显示原始表达式。
- 超长内容避免一次性进行高成本语法高亮。
- 文档版本变化时显示“内容已更新”，不在用户阅读过程中突然替换滚动位置。
- 教师教纲和学生笔记是两个内容类型，不使用权限角色隐藏。

### 6.4 考试答案自动保存

每道答案具有独立 `answer_version`，未保存过的题从 `0` 开始；Attempt 另有只用于最终提交的全局 `version`。建议算法：

1. 输入时立即更新内存草稿，并写入 `sessionStorage`。
2. 停止输入 800ms 后发送 PATCH。
3. 请求包含 `answer`、`client_updated_at` 和该题当前 `answer_version`；同一道题的请求串行，不同题可以并发。
4. 服务端成功后返回新的 `answer_version` 和 `attempt_version`，清除对应本地脏标记。并发响应乱序时，全局 Attempt 版本取已见最大值。
5. 切题、页面隐藏、退出考试和提交前立即 flush。
6. 单题 `409 version_conflict` 时使用响应中的当前题版本/答案合并；若服务端版本来自当前设备旧请求则以最新本地草稿重试，无法安全判断时要求用户选择。
7. 离线时保留草稿并明确显示“尚未同步”，禁止假装已保存。

服务端每次成功保存答案都会在锁定 Attempt 后递增其全局 `version`。提交按钮必须等待全部题目保存完成，再携带最新 `attempt_version` 提交；提交成功后客户端锁定答案编辑。

### 6.5 批阅结果

- 提交后先展示客观题暂定分，并明确最终成绩等待 AI 批阅。
- 批阅期间用户可以离开；完成后通过任务通知进入结果页。
- 批阅失败时仍展示已提交答案和客观题结果，提供重试入口。
- 有失败 Job ID 时可调用 Job Retry；旧系统导入或不可重试的批阅使用 `POST /attempts/{attemptId}/grading-jobs`。两者都复用同一次用户意图的幂等键，已有活动批阅任务时跳转到该任务。
- 单题反馈至少展示得分、判断、反馈、正确理解和薄弱知识点。
- “生成复习测验”必须调用复习专用接口，不能调用普通出题接口。

## 7. 错误处理

后端错误使用 `application/problem+json`。前端按 `code` 而不是按自然语言 `detail` 分支。

| 错误类别 | 前端行为 |
| --- | --- |
| `validation_error` | 将字段错误绑定到对应输入，并在表单顶部汇总 |
| `not_found` | 展示资源不存在，并提供返回有效父级资源的动作 |
| `version_conflict` | 执行答案或编辑冲突处理 |
| `job_not_retryable` | 禁用重试，展示原因和建议动作 |
| `claude_unavailable` | 保留任务和输入，提示稍后重试并链接系统状态 |
| `storage_full` | 阻止新导入，提示清理或更换数据目录 |
| `upload_too_large` | 在选择文件后和服务端响应后都明确展示限制 |
| `network_error` | 保留用户输入，提供重试；不得清空表单 |

Toast 只用于短暂、非阻塞反馈。需要用户处理的错误必须留在相关页面或任务详情中。

## 8. 加载与空状态

- 首次加载使用与内容结构一致的 Skeleton，不使用全屏旋转图标覆盖已有内容。
- 后台刷新不清空当前内容。
- 空状态说明为什么为空，并只提供一个最相关的下一步操作。
- “没有课程”“课程没有课时”“课时正在生成”“生成失败”是不同状态。
- 列表分页优先采用游标分页；第一阶段数据量小时也必须遵守接口 `page_info`。

## 9. 可访问性

- 所有功能可通过键盘完成。
- 焦点在 Dialog 打开/关闭、路由切换和错误发生后有明确归属。
- 状态不只依靠颜色；任务进度和错误包含文本。
- 表单控件有可见 Label 和关联错误描述。
- SSE 更新不频繁打断屏幕阅读器；只对关键状态变化使用适度 live region。
- 考试题目使用语义化分组，选项具有完整可读标签。
- 支持 `prefers-reduced-motion`。

## 10. 性能要求

- 路由级代码拆分，首屏不加载考试编辑器、图表和代码高亮全部依赖。
- Markdown 渲染和高亮按内容类型延迟加载。
- 大列表使用分页，不一次请求所有作答和任务事件。
- 上传不将完整文件读入 JS 内存。
- SSE 消息按任务 ID 合并更新，避免每个进度事件导致全应用重渲染。
- Bundle 分析进入 CI；前端初始 JS gzip 目标小于 350KB，不含按需 Markdown/图表块。

## 11. 测试要求

### 11.1 单元/组件测试

- Problem Details 到 UI 状态的映射。
- 任务状态标签和可用操作。
- 答案自动保存、flush、失败重试和冲突处理。
- Markdown 安全渲染。
- 表单校验和重复提交保护。

### 11.2 集成测试

- 使用 Mock Service Worker 或等价方案基于 OpenAPI 示例模拟 API。
- 验证 SSE 断线、重连、漏事件后的 REST 对账。
- 验证 `stream.resync_required` 不被当作 Job Event，并从高水位恢复订阅。
- 验证批阅失败仍保留答案和客观题成绩。
- 验证 Submit `200 AttemptSubmitCompleted` 不含 Job，`202 AttemptSubmitAccepted` 必含 Job。

### 11.3 Playwright E2E

- 创建课程并导入文本课时。
- 观察任务成功并打开笔记。
- 生成试卷、自动保存答案、刷新恢复、提交和查看结果。
- 任务失败、重试并成功。
- 备份入口和系统状态只验证交互，不在浏览器测试真实恢复数据库。

## 12. 前端交付物

前端开发需要交付：

- 可运行的 React 工程和生产构建。
- 基于 OpenAPI 自动生成的 API Client。
- API Client 由展开公共 Response Header 后的 `api/openapi.yaml` 生成，并保留原始 `X-Request-ID` 供诊断。
- 路由、Query Key 和错误映射说明。
- 组件 Story 或等价可视化验收环境。
- 单元、集成和关键 E2E 测试。
- 前端环境变量说明，仅允许非敏感构建配置。
- Caddy 静态资源回退和缓存策略说明。

## 13. 前端验收清单

- [ ] 没有登录页，初始化完成后直接进入产品。
- [ ] 日常操作不要求打开终端。
- [ ] 所有 AI 操作非阻塞并可从任务中心恢复。
- [ ] 文件上传进度与 AI 处理进度明确区分。
- [ ] 考试答案刷新后可恢复。
- [ ] 批阅失败不丢答案。
- [ ] 普通错误不会清空用户输入。
- [ ] 前端构建物不包含 Anthropic API Key。
- [ ] 页面可使用键盘操作，文本在常见桌面和窄屏下不重叠。
- [ ] 所有请求类型来自 OpenAPI 生成结果。
