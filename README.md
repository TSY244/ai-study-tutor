# AI Study Tutor · AI 学习助教

面向单用户、本机部署的 Web 学习应用。将文本、文件或视频链接整理成课程资料，生成教师大纲、学生笔记和知识点测验，再通过作答、批阅、学习总结与薄弱点追踪完成复习。

当前 Go API/Worker、React Web、OpenAPI 契约和 Docker Compose 部署均已实现。仓库保留旧 Python CLI/TUI，供行为基线验证和旧数据导入使用。

## 当前功能

| 功能 | 说明 |
| --- | --- |
| 课程与课时 | 创建、编辑、归档和删除课程，管理课时；课程标题可留空，首次成功生成教纲或笔记时由 AI 命名，手动名称保留 |
| 资料与阅读 | 导入文本、URL 或文件，查看原文、教师大纲、学生笔记及历史版本；支持 Markdown、公式、代码高亮与目录 |
| B 站视频 | 扫码登录或手动配置 Cookie，提取字幕/转写、章节和采样画面，生成图文总结；支持 `?p=2` 分 P 链接 |
| 知识点与测验 | 展示知识点解释、学习目标与原文依据；生成常规/复习测验，可指定题量及出题备注，题目关联知识点 |
| 作答与复习 | 答案自动保存、客观题判分、主观题后台批阅、学习总结评估、掌握度与薄弱点追踪 |
| 后台任务 | 任务中心与课时进度卡通过 SSE 更新进展，支持取消和对可重试任务发起重试；Pi 任务可显示生成与工具执行状态 |
| 模型配置 | 管理多个 Anthropic Messages / OpenAI Chat Completions 兼容 Key，测试连接、设置路由优先级并应用到 Pi |
| 页面体验 | 学习总览、最近学习入口、命令面板、窄屏布局和浅色/深色切换；默认跟随系统，手动选择后保存偏好 |
| 数据管理 | 页面提供单项删除，课程和课时列表支持批量删除；后台另提供内容新增、修改、版本冲突保护与级联删除 API |

课程内容管理的接口范围与删除规则见 [课程内容管理 API](docs/course-content-api.md)。文档、知识点、测验等资源的完整编辑能力以该 API 文档为准。

## 快速开始：Docker 部署

正常构建、启动和运行只需 Docker 与 Compose v2，宿主机无需安装 Go、Node.js、Python 或 PostgreSQL。首次构建需要网络下载镜像和依赖。完整环境要求与操作说明见 [Docker 部署指南](docs/deployment.md)。

在仓库根目录执行：

```bash
make docker-init    # 初始化部署配置、密钥文件和本地数据目录
make docker-up      # 构建镜像、自动应用数据库迁移并等待服务就绪
make docker-smoke   # 检查 REST、SSE、静态资源和 SPA 路由
```

浏览器访问 <http://127.0.0.1:8080>。如需演示课程，可额外执行 `make docker-seed`。

未配置模型 Key 也能启动应用；AI 生成功能需要完成下方配置。端口、URL 白名单、任务超时等配置位于 `deploy/.env`，示例见 [deploy/.env.example](deploy/.env.example)。默认只向本机开放 Web 端口。

### 配置模型

1. 打开「设置」，新增模型 Key，选择 Anthropic 或 OpenAI 协议，填写模型及兼容服务的 Base URL。
2. 保存后测试连接，确认该 Key 可用。
3. 点击「应用到运行服务的 Pi」，让后续 Pi 任务使用所选配置。更改已应用 Key 的协议、模型、地址或密钥后，重新应用。

链接提取和常规/复习测验生成使用 **Pi SDK**；教纲、笔记等其他 AI 能力通过 HTTP 网关调用。未应用指定 Key 时，Pi 可按网关优先级/权重选择可用 Key。网关配置运行时热加载，新增 Key 无需重启 Worker。

旧单 Key 配置仍可将 Anthropic Key 写入 `deploy/secrets/anthropic_api_key.txt`，作为普通 AI 调用的回退；**URL 导入需要在设置页配置启用的网关 Key**。B 站图文总结还要求模型支持图片输入。

模型协议、额度不足、连接失败和 Pi 配置细节见 [部署指南：Pi 执行引擎](docs/deployment.md#pi-执行引擎)。

### 开始一次学习

1. 新建课程和课时，导入文本、文件或允许站点的 URL；课程名称可留空等待 AI 命名。
2. 在课时中查看导入原文、教师大纲和学生笔记，任务进展会自动更新。
3. 生成测验，选择常规或复习模式、题量，并填写最多 3000 字的出题备注。
4. 完成作答并查看批阅结果，提交学习总结，再根据知识点、掌握度和薄弱点安排复习。

新生成的视频总结、教纲与笔记采用统一的 [study-summary/v1 格式](docs/ai-summary-format.md)。生成测验时先分析知识点，再校验原文依据、题目关联、题量、重复题干和解析；知识点与试卷在同一事务发布，历史试卷保留当时的知识点快照。

### B 站视频导入

在「设置 → B 站 Cookie」点击「扫码登录」，用哔哩哔哩 App 扫码并确认，系统自动保存登录状态；也可手动保存 Cookie 并检测可用性。登录任务由 Worker 执行。

Worker 使用 [bili-cli](https://github.com/kukemc/bili-cli)（`bili-agent==1.2.1`）获取完整字幕或已配置的语音转写、章节与 9 帧采样画面，再由 Pi 调用模型生成画面要点和综合总结。完整文字与图文总结共同写入 `raw_text`，用于后续教纲、笔记和测验。

采样画面不能保证覆盖视频全部内容。无字幕且未配置 ASR 时会标注口述内容缺失；无法获取画面时导入失败。镜像不预装 ASR 模型或本地推理引擎，已有素材需要重新导入才能获得图文总结。配置方式见 [部署指南：B 站图文提取](docs/deployment.md#41-b-站图文提取)。

## 架构与目录

前端使用 React、TypeScript、Vite、TanStack Router 和 TanStack Query；后端使用 Go、Gin 与 PostgreSQL Job Queue。Worker 独立处理导入、生成和批阅，API 通过 SSE 向前端发送任务事件。Caddy 托管前端并反向代理 API；API 与 Worker 共用数据库和文件存储。Compose 中只有 Worker 接入外网，数据库不暴露宿主机端口。

```text
backend/            Go API、Worker、数据库迁移及测试
backend/pi-runtime/ Pi SDK Node 桥接程序与测试
frontend/           React 前端、生成的 API Client、单元与浏览器测试
api/                OpenAPI 发布契约与请求/响应示例
deploy/             Docker Compose、配置示例与运维脚本
tools/              契约与文档校验脚本
docs/               部署、内容 API、总结格式及 Web 重构设计
legacy/python/      归档的 Python CLI/TUI 源码与测试
legacy-data/        旧数据导入源（本地目录，Git 忽略）
```

## 本地开发与校验

源码开发需要 Go 1.25.8+、Node.js 22.19+、npm、Python 3.11+ 和 uv；具体依赖版本以 `backend/go.mod` 与各 `package.json`、锁文件为准。Docker 部署不要求宿主机安装这些工具。

### 前端开发

```bash
cd frontend
npm ci
npm run dev:mock
```

打开 <http://127.0.0.1:5173>，MSW 提供模拟 API，无需启动后端。联调真实服务时，在 `frontend/.env.local` 中设置 `API_PROXY_TARGET=http://127.0.0.1:8080`，关闭 `VITE_ENABLE_MSW`（设为 `false` 或不设置），再运行 `npm run dev`。环境变量说明见 [frontend/.env.example](frontend/.env.example)。

### 校验命令

以下各组命令均从仓库根目录开始执行：

```bash
# 文档、OpenAPI、兼容性检查及 Go 格式/vet/竞态测试/构建
make verify

# Compose 部署边界测试（只使用 Python 标准库）
make test-deploy-boundaries

# 旧 Python 行为基线
make test-legacy
```

```bash
# 前端：格式、lint、类型、单元测试及生产构建
cd frontend
npm ci
npm run verify

# 浏览器测试：自动启动 MSW 开发服务，覆盖桌面和窄屏
npm run e2e:install
npm run e2e
```

```bash
# Pi 桥接：本地模拟网关驱动实际 SDK，无需真实模型 Key
cd backend/pi-runtime
npm ci
npm test
```

Go 集成测试使用嵌入式 PostgreSQL，无需另启 Docker 数据库；首次运行会下载测试数据库二进制。Pi 和浏览器测试需要能监听本地端口。

### 修改 API 契约

只编辑设计源 `docs/web-rebuild/openapi.yaml`，然后从仓库根目录生成发布契约与前端 Client：

```bash
make generate-openapi
make verify-openapi
cd frontend
npm run api:gen
```

生成文件与设计源一起提交，不手工修改 `api/openapi.yaml` 或前端生成代码。规则与兼容性检查见 [API 契约说明](api/README.md)。

## 升级与数据维护

```bash
make docker-backup   # 升级前备份数据库和对象文件
git pull
make docker-up       # 重建服务并自动应用迁移
make docker-smoke
```

涉及扫码登录、API 或页面变更时需要更新整套服务。`make docker-down` 保留数据卷；升级不要使用会删除数据卷的 `down -v`。

网关 Key、站点 Cookie、Docker secrets 和运行时模型选择需单独保管；常规备份不会包含这些配置。旧 Python 数据通过只读扫描和显式导入迁入新系统，运行时不做双写。备份恢复、健康诊断和旧数据迁移步骤见 [部署指南](docs/deployment.md)。

## 文档导航

- [Docker 部署与运维](docs/deployment.md)
- [课程内容管理 API](docs/course-content-api.md)
- [AI 学习总结格式](docs/ai-summary-format.md)
- [OpenAPI 发布与生成规则](api/README.md)
- [Web 重构设计与实施文档](docs/web-rebuild/README.md)
- [前端实施 Codex 提示词](docs/web-rebuild/08-frontend-codex-prompt.md)
- [旧 Python 版本说明](legacy/python/README.md)

API Key、Cookie 和个人学习数据仅保存在本地配置或数据目录中，不提交到仓库。
