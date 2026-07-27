# AI 学习助教 Web 前端（frontend/）

单用户、本机部署的 React 前端。首屏即产品，无登录页、无营销页。
后端（Go/Gin）通过同源 REST/SSE 提供数据；浏览器不接触 Claude Key。

技术栈：React 19 + TypeScript strict + Vite 8 + TanStack Router/Query +
React Hook Form + Zod + Radix UI + Tailwind CSS 4 + Lucide +
orval（OpenAPI 生成）+ MSW + Vitest/RTL + Playwright。

## 工程命令

| 命令                              | 作用                                                                   |
| --------------------------------- | ---------------------------------------------------------------------- |
| `npm run dev`                     | 开发服务器（真实后端，经 Vite 同源代理到 `API_PROXY_TARGET`）          |
| `npm run dev:mock`                | 开发服务器（MSW 接管 API，后端未运行时使用）                           |
| `npm run api:gen`                 | 从 OpenAPI 生成 Client/类型/MSW handlers（见下文）                     |
| `npm run routes:gen`              | 重新生成 TanStack Router 路由树（dev/build 自动执行）                  |
| `npm run lint`                    | ESLint（flat config，含 jsx-a11y、react-hooks）                        |
| `npm run format` / `format:check` | Prettier 写入 / 校验                                                   |
| `npm run typecheck`               | `tsc -b`（app + node 两个 project）                                    |
| `npm run test`                    | Vitest + RTL 单元/组件测试（MSW node）                                 |
| `npm run e2e`                     | Playwright E2E（自动以 `dev:mock` 启动；首次需 `npm run e2e:install`） |
| `npm run build`                   | 生产构建，同时输出 `dist/bundle-report.html`（gzip/brotli treemap）    |
| `npm run analyze`                 | 等同于 build（bundle report 总是产出）                                 |
| `npm run verify`                  | 统一门禁：format:check + lint + typecheck + test + build               |

## API Client 生成（契约边界）

**当前状态：已从发布契约 `api/openapi.yaml` 生成（后端已于 F0 期间发布）。**

```
npm run api:gen
```

脚本 `scripts/generate-api.mjs` 的流程：

1. 解析契约来源：`--source=<path>` > `API_CONTRACT_PATH` > `api/openapi.yaml`（发布契约）
   > 回退 `docs/web-rebuild/openapi.yaml`（设计草案，会打印警告）。
2. 把顶层 `x-common-response-headers`（`X-Request-ID`）确定性展开到每个响应
   （发布契约已展开，此步幂等；对设计草案/旧契约必须）。
3. 调 orval v8 生成到 `src/api/generated/`：
   - `model/`：全部 DTO 类型（snake_case，与契约一致）
   - `endpoints/<tag>/`：按 tag 拆分的 fetch 函数 + MSW faker handlers
   - 文件头标注来源与 `GENERATED FILE - DO NOT EDIT`；**生成物不可手改**
4. 已验证可重复：连续两次生成逐字节一致。

契约变更流程：后端 `make generate-openapi` 更新 `api/openapi.yaml` →
前端 `npm run api:gen` → 审查 `src/api/generated` 的 git diff（CI 可加
`npm run api:gen && git diff --exit-code src/api/generated` 防止漂移）。

### 调用约定

```ts
import { listCourses, createCourse } from '@/api/generated/endpoints/courses/courses'

// 查询：返回解析后的 body（T）
const list = await listCourses({ archived: false })

// 创建/任务型 POST：Idempotency-Key 经 options.headers 透传（生成函数签名不含 header 参数）
const key = createIdempotencyKey() // 一次用户意图一个 UUID，重试/双击/恢复复用
const course = await createCourse(body, { headers: { 'Idempotency-Key': key } })
```

所有请求经过 `src/api/http.ts` 的 `apiFetch`（orval mutator）：

- 只走同源相对路径；生产 Caddy 反代，开发 Vite proxy。
- 自动生成 `X-Request-ID`，以响应头为准记入 `src/api/diagnostics.ts` 环形缓冲（仅元数据，不外发）。
- 非 2xx 一律抛 `ApiProblemError`（`src/api/problem.ts`）：`code`、`status`、
  `fieldErrors`、`extensions`（`active_job_id`、`current_answer_version` 等）、
  `retryAfterSeconds`。按稳定 `code` 分支，不解析 `detail`。
- FormData（上传）不手设 Content-Type；204 返回 `undefined`。

`text/event-stream` 操作不会进入 Orval REST client/MSW 生成物，因为一次性
`Promise<T>` 无法表达常驻流。任务 SSE 的唯一支持入口是
`src/api/job-stream.ts`；测试使用 `src/mocks/job-stream-mock.ts`。

## 缓存与状态

- 服务端状态唯一事实来源是 TanStack Query；Query Key 约定集中在
  `src/api/query-client.ts` 的 `queryKeys`（与交接文档 §5.1 一致）。
- Mutation 不自动重试（幂等键生命周期在 F2 实现）；Query 对 5xx/network_error
  最多重试 2 次（带抖动），4xx Problem 不重试。
- 客户端 Store 仅允许 UI 偏好、答案草稿、SSE 游标、任务通知（F2+ 引入）；
  业务实体禁止复制进全局 Store。

## 任务事件流（F6，`src/api/job-stream.ts`）

- 传输：fetch + ReadableStream 解析 SSE（非 EventSource——需要携带初始
  `Last-Event-ID` 并自控重连节奏）；同源相对路径 `/api/v1/jobs/stream`；
  传输层可注入，测试用内存假流（`src/test/stream-driver.ts`）。
- 游标：持久化事件 SSE id 存 sessionStorage，重连以 `Last-Event-ID` 头发送；
  重连为抖动指数退避（1s 起步 ×2，±25%，封顶 15s），重连成功即请求
  `active=true` 任务列表并失效已观察任务查询（REST 对账）。
- `stream.resync_required`：不是 Job Event——按 `high_watermark_event_id`
  更新游标，请求活动任务并失效当前观察中的资源，随流继续。
- `heartbeat`：只更新连接活性诊断（`src/api/job-stream-status.ts`），
  不写 Job Cache，不推进游标。
- 合并：持久化事件按 job_id 就地写入 Query Cache（普通 `JobList` 与无限
  分页 `{pages}` 两种形态都覆盖），高频进度不重取、不整树重渲染；
  终态事件再精准失效（任务/课时/文档/测验/作答），驱动跨页一致；
  事件时间线（`['job-events', id]`，REST 审计 Schema）仅在被观察时重取。
- 诊断面板：设置页「任务事件流（开发诊断）」，仅 `import.meta.env.DEV`。

## MSW 接线

`src/mocks/handlers.ts` = curated handlers（确定性 fixture，回显 X-Request-ID）
优先 + orval 生成的 faker handlers 兜底覆盖全部端点。

- 单元测试经 `src/test/setup.ts`（`onUnhandledRequest: 'error'`）强制显式声明依赖。
- 场景 fixtures（resource_busy、autosave 冲突、grading_failed 等）在 F2+ 补充。
- **SSE（F6）**：客户端是 fetch 传输，Service Worker 可拦截；
  `src/mocks/job-stream-mock.ts` 提供常驻流 + 控制器，浏览器接线挂到
  `window.__jobStreamMock`（push/closeAll/connections/lastCursor）供 E2E；
  `handlers.ts` 的 onPush 桥把流事件写进任务/课时/作答存储（模拟 Worker
  写库），保证「推送 → 失效 → REST 重取」状态一致。注意该 handler 必须
  先于 `/jobs/:jobId` 声明，否则 `stream` 会被当作 jobId 匹配。

## 环境变量

只允许 `VITE_` 前缀非敏感配置（见 `.env.example`）：

- `VITE_APP_TITLE`：应用标题。
- `VITE_ENABLE_MSW`：开发时启用 MSW（生产构建必须为 false）。
- `API_PROXY_TARGET`（非 VITE_）：仅 Vite dev server 代理用，不进 bundle。

`src/env.ts` 启动时扫描所有 `VITE_` 变量名，命中
`KEY/SECRET/TOKEN/PASSWORD/CREDENTIAL` 即拒绝启动（含单测）。
API Key 只能存在于 Worker Docker secret；前端任何层都不可读取或提交。

## 构建与性能

- target：`baseline-widely-available`（现代浏览器基线）。
- 路由级代码拆分：`@tanstack/router-plugin` `autoCodeSplitting`（当前
  `dashboard` 为独立 chunk）。
- manualChunks（函数形式，rolldown 兼容）：`react-vendor`、`tanstack`、`forms`。
- sourcemap：`hidden`——产出独立 `.map` 供本地诊断，bundle 不引用、不上传。
- MSW 仅在 DEV 且 `VITE_ENABLE_MSW=true` 时经动态 import 加载；
  生产 dist 不含 MSW/faker（已验证构建产物清单）。
- 当前首屏 JS（F1，含 Shell/Radix）：index 5.6 + _shell 26.2 + tanstack 36.6 +
  react-vendor 59.7 ≈ **128KB gzip**（目标 <350KB，不含按需 chunk）；
  每个路由独立 chunk（0.3–2.2KB gzip），专注模式不加载 Shell。

## 路由表（F1）

文件路由 + `@tanstack/router-plugin` autoCodeSplitting，页面组件按需加载。

```text
/                                    → 重定向 /dashboard
attempts/$attemptId                  专注模式（无 App Shell）
_shell（路径无关布局，挂 App Shell）
├── dashboard                        学习总览
├── courses/                         课程列表
│   ├── new                          新建课程（F2/F3 接表单）
│   └── $courseId/                   课程详情（课时列表）
│       └── lessons/$lessonId/       课时布局（标题 + 子视图导航）
│           ├── index → notes        重定向
│           ├── notes | outline      笔记/大纲（F4）
│           ├── quizzes | attempts   测验/作答（F3/F5）
│           └── summary              总结（手写总结 + AI 评估反馈）
├── jobs/ | jobs/$jobId              任务中心列表（游标分页）/ 详情
├── weaknesses                       弱点列表
├── progress                         按课程掌握度
└── settings                         Claude 状态/系统信息/请求诊断（不露 Key）
```

面包屑由路由 `staticData.crumb` 生成（`$course`/`$lesson` 占位符经共享
Query Cache 替换为实体标题，零额外请求）；仅一级 crumb 时不渲染。

## App Shell 与设计系统（F1）

- **Shell**（`src/components/app-shell/`）：skip link → 顶栏（标题、主导航
  `aria-current`、命令面板 ⌘K/Ctrl+K、任务中心常驻入口带进行中角标）→
  面包屑 → 主内容。窄屏导航横向滚动不折行。
- **任务中心**：`active=true` 查询；实时性由全局任务事件流驱动（F6：
  事件就地合并 + 终态精准失效 + 断线 REST 对账，不轮询）；Popover 列
  最近任务，页脚进 /jobs，断线重连时显示克制提示。
- **命令面板**：Radix Dialog + combobox/listbox（`aria-activedescendant`，
  焦点不离开输入框）；⌘K/Ctrl+K 全局开关；F1 仅导航命令。
- **专注模式**：`/attempts/:id` 不走 `_shell`，无全局导航/任务中心，
  仅退出链路 + 测验标题 + 作答状态。

### Design tokens（`src/styles/globals.css @theme`）

| 类别 | tokens                                                                 |
| ---- | ---------------------------------------------------------------------- |
| 表面 | `canvas` `surface` `surface-sunken`                                    |
| 边框 | `border` `border-strong`                                               |
| 文本 | `text` `text-muted` `text-subtle`（AA 对比度）                         |
| 强调 | `accent` `accent-strong` `accent-contrast` `accent-bg` `accent-subtle` |
| 状态 | `ok` `warn` `danger` `info` + 各自 `-bg`（状态配文本，不单靠颜色）     |
| 排版 | `font-sans` `font-mono`；html 14px，行高 1.55                          |
| 圆角 | `radius-sm/md/lg`（4/6/8px）                                           |
| 阴影 | `shadow-popover` `shadow-dialog`                                       |
| 动效 | 约定 `duration-120/180` + `motion-safe:`；全局 reduced-motion 压零     |

### 组件清单（`src/components/`）

- `ui/`：`Button`/`buttonClasses`（primary/secondary/ghost/danger × sm/md）、
  `Badge`（6 变体，文本必填）、`Skeleton`/`SkeletonText`/`SkeletonRows`、
  `EmptyState`（图标+标题+原因+唯一操作）、`ErrorState`（Problem→文案+code+
  Request ID+重试，role=alert）、`PageHeader`（标题+说明+命令区）、
  `Dialog`（Radix 封装）、`PathLink`（运行时路径 Link）
- `app-shell/`：`AppShell`、`Breadcrumbs`、`CommandPalette`、`JobsCenter`
- `ErrorBoundary`/`ErrorFallback`、`FeaturePending`（阶段占位）

### 状态矩阵（`src/lib/status.ts`，测试强制与契约枚举同步）

| 枚举          | 映射                                                                                                                                                                                            |
| ------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| JobStatus     | queued 排队中/neutral · running 运行中/info · validating 校验中/info · succeeded 已成功/ok · failed 失败/danger · cancel_requested 取消中/warn · cancelled 已取消/neutral；前四者 `isJobActive` |
| LessonStatus  | draft 草稿 · processing 生成中/info · ready 就绪/ok · failed 生成失败/danger · ready_with_warning 部分就绪/warn                                                                                 |
| AttemptStatus | in_progress 作答中/info · submitted 已提交 · grading 批阅中/info · graded 已批阅/ok · grading_failed 批阅失败/danger                                                                            |
| QuizStatus    | draft 草稿 · ready 可用/ok · archived 已归档                                                                                                                                                    |
| 系统组件      | ok 正常/ok · degraded 降级/warn · unavailable 不可用/danger · unknown 未知；未知值回退原文+neutral                                                                                              |

MSW curated fixtures（`src/mocks/fixtures.ts`）：2 课程、4 课时
（ready/processing/failed 三态）、3 任务（running/failed/succeeded）、
2 测验 + 4 作答（进行中、批阅失败 ×2、已批阅）、空弱点列表、按课程进度。

## 可访问性基础

- 键盘：skip link、焦点环 `:focus-visible`、Dialog/Popover 焦点圈禁与
  焦点归还（Radix）、命令面板 combobox 键盘模型、路由切换后主内容可聚焦。
- 屏幕阅读器：状态 Badge 必带文本；进度用原生 `progressbar` role + 数值；
  错误 `role="alert"`；地标：主导航/面包屑/课时子视图/主内容。
- reduced motion：全局动画压 ~0ms、平滑滚动关闭（E2E 实测）。
- 窄屏：390px project E2E 无横向溢出；导航/面包屑横向滚动。
- F7 验收项：live region 细化、移动端点击区复査。

## 目录结构

```
frontend/
├── api/                      # （仓库根 api/openapi.yaml 为契约输入）
├── e2e/                      # Playwright：smoke.spec.ts（底座）+ shell.spec.ts（F1 壳层）
├── scripts/generate-api.mjs  # 契约展开 + orval 生成入口
├── src/
│   ├── api/
│   │   ├── generated/        # orval 生成物（不可手改，已提交）
│   │   ├── http.ts           # apiFetch mutator（同源、X-Request-ID、Problem）
│   │   ├── problem.ts        # Problem Details -> ApiProblemError
│   │   ├── diagnostics.ts    # 请求诊断环形缓冲（仅元数据）
│   │   └── query-client.ts   # QueryClient + queryKeys 约定
│   ├── components/
│   │   ├── app-shell/        # AppShell / Breadcrumbs / CommandPalette / JobsCenter
│   │   ├── ui/               # Button/Badge/Skeleton/EmptyState/ErrorState/PageHeader/Dialog/PathLink
│   │   ├── error-boundary.tsx
│   │   └── feature-pending.tsx
│   ├── lib/                  # cn / format / status（状态矩阵）/ idempotency
│   ├── mocks/                # MSW browser/server/handlers/fixtures
│   ├── routes/               # 文件路由（_shell 布局 + attempts 专注模式；- 前缀文件不进路由树）
│   ├── styles/globals.css    # tokens + a11y 基础
│   ├── test/                 # vitest setup、renderWithClient/renderWithRouter、mock helpers
│   ├── env.ts                # VITE_ 类型化访问 + 敏感变量防护
│   └── routeTree.gen.ts      # 路由树生成物（已提交）
└── vite.config.ts            # 含 vitest 配置
```

## 后端契约阻塞项

**无未决阻塞。** `api/openapi.yaml` 已由后端发布（`make generate-openapi` 产物，
X-Request-ID 已逐响应展开），本工程 Client/MSW 即从该契约生成。

已知差异与后续依赖：

1. 契约 `info.version` 仍为 `0.3.0-draft`，语义同设计源；正式发布时关注版本号变化。
2. 生成函数不携带 `Idempotency-Key` 签名参数，统一走 `options.headers`（见上）。
3. SSE 三类 Envelope（PersistedJobStreamEvent / JobResyncRequiredStreamEvent /
   HeartbeatStreamEvent）已生成类型；流式读取不走生成 fetch 函数，F6 实现 adapter。
4. 后端运行时（Gin API + Worker + Postgres）尚未就绪；联调用 `npm run dev`
   （同源代理到 `127.0.0.1:8080`）或先用 `npm run dev:mock`。

## 给 F2+ 的交接

- 路由与 Shell 已就位；新页面挂到对应资源路由下，保持文件路由 +
  autoCodeSplitting，组件放 `src/features/<资源>/` 或页面路由文件内。
- `queryKeys` 已覆盖全部资源；失效只许精确失效，禁止全局 `invalidateQueries()`。
- Radix dialog/popover 已在 Shell 使用；select/tabs/toast/tooltip 已装未用（F2+）。
- RHF + Zod 已安装；表单错误用 `ApiProblemError.fieldErrors` 绑定（F2）。
- `PathLink` 只用于面包屑等运行时路径；新代码优先类型安全 `Link to="..."`。
- MSW 场景 fixtures（resource_busy、autosave 冲突、grading_failed 等）按测试
  需要追加到 `src/mocks/fixtures.ts`；curated handlers 优先于生成兜底。
- 测试约定：单测 `onUnhandledRequest: 'error'`，路由级组件测试用
  `renderWithRouter`（首帧异步，首个断言用 `findBy*`）；E2E 固定经
  `dev:mock` 运行，桌面 + 390px 窄屏两个 project；reduced motion 仿真须用
  `test.use({ contextOptions: { reducedMotion: 'reduce' } })`（Playwright 1.61
  起不再是一等 test option）。
