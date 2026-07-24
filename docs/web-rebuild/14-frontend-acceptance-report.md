# AI Study Tutor Web 前端发布验收报告

日期：2026-07-21

## 结论

**GO：前端与本机 Web/API/Worker 运行链路可发布。**

- React production build 已在真实 Caddy 容器中运行；PostgreSQL -> migrate -> API/Worker -> Web 启动顺序和健康检查全部通过。
- 同源 REST、SSE、SPA fallback、静态缓存、seed Job 处理、桌面/390px 浏览器与 WCAG A/AA 自动扫描通过。
- 三个应用容器均为 UID/GID 10001、只读 rootfs、`cap_drop: ALL`、`no-new-privileges`；只有 Worker 挂载 Claude secret。
- 未配置 Claude Key 时可按契约降级发布；正式启用 AI 前仍需在 Worker secret 中配置 Key，并运行真实 Claude 连通性测试。

## 修改文件

前端与验收：

- `apps/web/e2e/quiz-generation.spec.ts`、`apps/web/e2e/shell.spec.ts`：补齐测验生成/失败重试和窄屏键盘路径。
- `apps/web/e2e-production/release.spec.ts`、`apps/web/playwright.production.config.ts`：新增真实 Docker/Caddy production E2E 与 axe WCAG 扫描。
- `apps/web/src/features/quizzes/generate-quiz-button.tsx`、测验路由、缓存失效和 MSW handlers：接入发布契约的测验生成 Job。
- `apps/web/src/components/app-shell/command-palette.tsx`：修复 `kbd` 颜色对比度。
- 各加载态组件/路由：为 `aria-busy` 容器补 `role="status"`。
- `apps/web/src/features/documents/markdown/markdown-reader.tsx`、`router.tsx`、`vite.config.ts`：重型阅读依赖按路由懒加载并关闭全局预取。
- `apps/web/package.json`、`package-lock.json`：新增 production 验收/指标命令、axe；固定已修复的 `js-yaml` 传递依赖。
- `apps/web/scripts/production-bundle-metrics.mjs`：采集真实首屏资源与 gzip 指标。

发布与文档：

- `.dockerignore`：构建上下文从 382 MB 降至 Web 约 0.5-1.7 MB、Go 约 1 MB，并排除 secret/data/node_modules/dist。
- `apps/web/Dockerfile`、`apps/web/Caddyfile`：非 root Caddy、SPA fallback、同源 REST/SSE、安全 Header、hashed asset 长缓存；移除不需要的 Caddy 低端口 file capability。
- `Dockerfile.api`、`Dockerfile.worker`：Go 多阶段非 root 镜像；Worker 包含 ffmpeg、pdftotext、yt-dlp。
- `deploy/compose.yaml`、`.env.example`、`secrets/README.md`、`scripts/init.sh`、`scripts/release-smoke.sh`：可执行单机发布拓扑和检查脚本。
- `Makefile`、`README.md`、`docs/web-rebuild/05-deployment-operations.md`：接入 docker init/build/up/seed/doctor/smoke/down 命令并更新运行状态。

## 用户流程

- 课程创建、文本课时导入、后台 Job 全局可见、成功终态发布。
- Job SSE 进度、重连、`stream.resync_required`、失败重试。
- 笔记/原文阅读、目录、代码、公式、历史版本、长列表分页。
- 测验生成、客观/主观作答、自动保存与刷新恢复、冲突处理、提交与失败重试。
- 窄屏键盘路径：skip link -> 主内容 -> 命令面板 -> 任务中心；无页面级横向溢出。
- reduced motion、焦点返回、表单 label/error、live/status region 均有自动化覆盖。

## API 与缓存

- 浏览器只使用同源 `/api`、`/healthz`、`/readyz`；Claude Key 和数据库密码不进入浏览器、build arg 或 Web 镜像。
- API 类型与 Client 仍由 `api/openapi.yaml` 生成，未手改生成物。
- 测验生成接受后精确失效 lesson、course lessons、quizzes、jobs 和新建 job；业务实体不复制到全局 Store。
- 联调时后端新增 `0002_job_error_retryable`；重建镜像并由 migrate 应用后，Jobs/Lessons API 恢复 200，迁移表为 `0001`、`0002`。

## 性能数据

- 真实 production `/dashboard`：33 个 asset 请求，其中 32 个 JS；初始 JS gzip **142.59 KiB**，CSS gzip **6.28 KiB**。
- 首屏未请求 `markdown-reader`、KaTeX、课程表单或作答重块；Markdown reader gzip 约 143.92 KiB，仅阅读路由按需加载。
- `dist/` 11 MiB；`dist/bundle-report.html` 948 KiB。
- 上传使用 `FormData + XMLHttpRequest` 流程上报进度，未使用 `FileReader/readAs*` 把文件读入 JS 内存。
- 列表使用 OpenAPI `page_info` 游标分页；文档历史、任务、课程、课时、测验、弱点均有分页/加载更多路径。

## 安全与可访问性

- `npm audit`：生产与完整依赖树均为 0 vulnerability。
- secret 扫描：源码与 `dist` 未发现真实 Key、数据库 DSN；`dist` 无 `.env`、`secret`、`key` 文件。
- 运行挂载：API 仅有 PostgreSQL secret；Worker 有 PostgreSQL + Anthropic secret；Web 无 secret。
- axe：桌面与 390px 的 dashboard/courses/jobs 均为 0 个 WCAG 2 A/AA violation。
- Playwright 截图人工巡检未发现文本重叠、裁切或不连贯布局。

## 实际输出

- `npm run verify`：format、lint、typecheck、unit、build 全部 exit 0；`29 passed` test files，`175 passed` tests；Vite `built in 4.57s`。
- `npm run e2e`：`47 passed, 1 skipped`（桌面项目跳过窄屏专用用例，窄屏项目已通过）。
- `npm run e2e:production`：`5 passed, 1 skipped`；真实 Caddy/REST/SSE/seed 阅读流与 axe 通过。
- `npm run analyze:production`：JS 142.59 KiB gzip、CSS 6.28 KiB gzip、`unexpectedLazyAssets: []`。
- `npm audit --json`：0 low/moderate/high/critical。
- `make verify`：docs/OpenAPI/breaking check、Go vet、Go race tests、Go build 全部通过；发布 OpenAPI 50 operations、103 responses、290 refs。
- `docker compose ... config --quiet`：exit 0；展开配置不含 secret 内容。
- `docker compose ... up -d --no-build --wait`：db/api/worker/web healthy，migrate exited 0。
- `release-smoke.sh`：REST、SSE、SPA fallback、immutable assets 通过。
- `doctor`：无维护模式、活跃租约、过期 temporary 或 creating backup。

## Docker 发布要求

1. Node 22 构建 Web；Go 1.25 构建 API/Worker；运行时为 Alpine/Caddy。
2. 执行 `make docker-up`，可选 `make docker-seed`，随后 `make docker-smoke`、`make docker-doctor`。
3. 只映射 `127.0.0.1:${APP_PORT}:8080`；数据库和 API 不发布宿主端口。
4. `deploy/.env` 只放非敏感变量；PostgreSQL 密码和 Claude Key 必须使用 Compose file secret。
5. 正式启用 AI 时设置 `CLAUDE_MODEL_DEFAULT`，写入 Worker secret，并执行 Claude 连通性 smoke。

## 已知缺口与残余风险

- 发布 OpenAPI 仍缺少“按课时列出 Attempt 历史”端点；课时作答页只能引导从测验开始新作答。这是后端契约缺口，不由前端伪造接口。
- 本次未持有真实 Claude Key，因此验证了降级启动和 secret 隔离，但没有调用远程 Anthropic API。
- seed 覆盖合成中文长文；真实用户的极端超长标题、超大错误详情和超大上传仍建议在发布后观察日志与布局指标。
- `web-design-guidelines` 技能配置的上游规则 URL 当前返回 404；本次以 axe WCAG、eslint-jsx-a11y、Playwright 语义断言和截图巡检替代。
