# Docker 部署指南

本文档是 AI Study Tutor 的完整部署手册。**生产应用全部以 Docker 容器运行**，宿主机无需安装 Go、Node.js、Python 或 PostgreSQL。只有从源码仓库运行 Compose 边界验证或部署演练时，宿主机额外需要 Python 3.11+；检查器只使用标准库，不是应用运行时依赖。

适用场景：单用户、本机（或内网单机）部署。所有命令在仓库根目录执行。

## 1. 架构总览

### 1.1 容器清单

| 容器 | 镜像 | 作用 | 常驻/一次性 |
| --- | --- | --- | --- |
| `db` | `postgres:17-alpine` | PostgreSQL 17 数据库 | 常驻 |
| `migrate` | `ai-study-tutor-api` | 数据库结构迁移（`cmd/migrate`） | 一次性，启动链中自动执行 |
| `api` | `ai-study-tutor-api` | Go + Gin REST/SSE API | 常驻 |
| `worker` | `ai-study-tutor-worker` | 后台任务 Worker（AI 生成、课程导入等） | 常驻 |
| `web` | `ai-study-tutor-web` | Caddy 托管前端静态资源 + API 反向代理 | 常驻 |
| `seed` | `ai-study-tutor-tools` | 写入演示数据（幂等） | 一次性，`tools` profile |
| `doctor` | `ai-study-tutor-tools` | 数据一致性诊断 | 一次性，`tools` profile |
| `backup` | `ai-study-tutor-tools` | 创建备份 | 一次性，`tools` profile |
| `restore` | `ai-study-tutor-tools` | 从备份恢复 | 一次性，`manual` profile |
| `maintenance-repair` | `ai-study-tutor-tools` | 清理卡死的维护锁 | 一次性，`manual` profile |
| `legacy-import` | `ai-study-tutor-tools` | 旧 Python 系统数据导入 | 一次性，`manual` profile |

常驻服务只有 4 个：`db`、`api`、`worker`、`web`。`migrate` 在每次启动时先于 `api`/`worker` 自动运行并退出，保证结构迁移始终跟随镜像版本。

### 1.2 网络拓扑

```text
浏览器 ──► 127.0.0.1:8080 ──► [public] web [frontend] ──► api ──► [backend] ──► db
                                                              worker ──┤
                                                                       └─► [egress] ──► 外网（Anthropic / OpenAI 兼容 API；B 站由 bili-cli 获取文字和画面，其他 URL 由 Pi SDK 获取）
```

- `frontend`、`backend` 是 internal 网络，不出网。
- 只有 `web` 暴露端口，且只绑定 `127.0.0.1`；`db` 不暴露任何端口。
- 只有 `worker` 能访问外网（`egress` 网络），AI 调用从这里出去；B 站素材由 Worker 内 bili-cli 获取字幕/转写与采样画面，再由 Pi SDK 进行图文总结；其他 URL 继续由 Pi SDK 获取。Worker 需访问白名单站点及 B 站 API/CDN。

### 1.3 数据与密钥

| 内容 | 位置 | 说明 |
| --- | --- | --- |
| 数据库 | 命名卷 `pg_data` | 升级重建容器不丢数据 |
| 应用文件（素材、导出等） | 命名卷 `app_data` | `api`/`worker` 共享 |
| 备份输出 | `./backups/`（宿主机） | 由 `backup` 容器写入 |
| 导入报告 | `./import-reports/`（宿主机） | 由 `legacy-import` 容器写入 |
| 数据库密码 | `deploy/secrets/postgres_password.txt` | Docker secret，只挂载给需要的容器 |
| Claude API Key | `deploy/secrets/anthropic_api_key.txt` | Docker secret，**只挂载给 worker** |
| Claude 网关配置（多 Key） | `./data/claude-gateway/`（宿主机，权限 0700） | 存放 `keys.json`，**只挂载给 api/worker**，web 不可见 |
| Pi 模型选择兼容设置 | 命名卷 `app_data` 内的 `/var/lib/ai-study-tutor/.claude/settings.json` | API 原子写入，Worker Pi 兼容读取；宿主机 `~/.claude` 不会挂载，web 不可见 |

两个命名卷由 Docker 管理，删除容器不影响数据；`docker compose down -v` 才会删除卷（危险操作，见 §7.3）。

## 2. 前置要求

- Docker Desktop 4.x（macOS/Windows）或 Docker Engine 24+ 与 Compose v2 插件（Linux）。
- Python 3.11+ 仅用于仓库中的 `make test-deploy-boundaries`、`make docker-verify-secrets` 和 `make docker-drill`；演练会调用 Compose 边界检查器。不需要第三方 Python 包，正常构建、启动和运行应用不依赖宿主机 Python。
- 可用磁盘 ≥ 10 GB（镜像 + 数据卷），内存建议 ≥ 8 GB（Worker 限制 2 CPU / 4 GB）。
- 首次构建需要外网拉取基础镜像与依赖；之后可离线重建。

验证环境：

```bash
docker version
docker compose version
# 仅运行仓库部署边界验证时需要：
python3 --version
```

## 3. 首次部署

### 3.1 获取代码

```bash
git clone https://github.com/TSY244/ai-study-tutor.git
cd ai-study-tutor
```

### 3.2 初始化部署文件

```bash
make docker-init        # 等价于 ./deploy/scripts/init.sh
```

该脚本只做本地准备，不启动任何容器：

- 从 `deploy/.env.example` 复制生成 `deploy/.env`（已存在则跳过）；
- 生成随机数据库密码 `deploy/secrets/postgres_password.txt`；
- 创建空的 `deploy/secrets/anthropic_api_key.txt`；
- 创建 `data/`、`backups/`、`import-reports/` 目录；
- 把上述文件权限收紧为 `600`/`700`。

### 3.3 配置 Claude API Key（可选但推荐）

把 Key 写入 secret 文件（内容就是 Key 本身，无引号无换行之外的字符）：

```bash
printf 'sk-ant-...' > deploy/secrets/anthropic_api_key.txt
chmod 600 deploy/secrets/anthropic_api_key.txt
```

**不配置也能启动**：系统以降级模式运行，课程导入、文本处理等确定性功能正常，AI 生成类任务（出题、批改、总结评估）会明确失败并可在配置后重试。Key 只挂载到 `worker` 容器，`api` 和 `web` 永远接触不到。

部署后也可以在「设置」页直接管理多个 Claude Key：网关按优先级自动故障转移，配置保存在 `data/claude-gateway/keys.json`（只挂载进 `api`/`worker`）；当没有任何启用的 Key 时，上面的旧单 Key secret 文件仍作为回退生效。

每张 Claude Key 卡片还可选择「应用到运行服务」。确认后，API 从已保存的 Key 中读取密钥，在共享 `app_data` 卷内原子覆盖运行环境的 `~/.claude/settings.json`；浏览器只提交 Key ID，不会接收或重新发送完整 Key。后续 Worker Pi 任务以该文件中的应用选择器为准；文件没有应用选择器时才回退到网关的优先级/权重路由。API 和 Worker 的 `HOME` 都是 `/var/lib/ai-study-tutor`，宿主机的 `~/.claude` 不会挂载进任何容器，Web 也看不到该配置。执行引擎已替换为 Pi SDK；为沿用现有 Key 选择，保留旧设置文件路径和 API 路由。该文件不是 Pi 的原生配置，不再启动 Claude Code。

如需调整端口、模型、上传大小等，编辑 `deploy/.env`（完整配置项见 §4）。

### 3.4 构建并启动

```bash
make docker-up
```

这一条命令完成：配置校验 → 构建 `api`/`worker`/`web`/`tools` 四个镜像 → 启动 `db` → 自动跑 `migrate` → 启动 `api`/`worker`/`web` → 等待全部健康检查通过。首次构建约 5–15 分钟（前端镜像内会跑完整单元测试，测试不过则构建失败，这是质量门禁）。

### 3.5 验证部署

```bash
make docker-smoke     # REST、SSE、SPA 回退、静态资源缓存冒烟检查
make docker-seed      # 可选：写入演示课程数据（幂等，可重复执行）
make docker-verify-secrets # 可选：运行单元测试并校验渲染后的 Compose 边界（需 Python 3.11+）
```

然后浏览器访问 `http://127.0.0.1:8080`。

> smoke 输出里出现一行 `curl: (28) Operation timed out ...` 是 SSE 检查的预期行为（故意截断长连接），只要最后一行是 `release smoke passed` 即为通过。

## 4. 配置项参考（deploy/.env）

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `APP_VERSION` | `local` | 镜像标签。发布时建议设为 git 提交号，便于回滚 |
| `APP_PORT` | `8080` | 对外端口（绑定 127.0.0.1） |
| `APP_ENV` | `production` | 运行环境标识 |
| `LOG_LEVEL` | `info` | 日志级别（`debug`/`info`/`warn`/`error`） |
| `POSTGRES_DB` / `POSTGRES_USER` | `ai_study_tutor` | 数据库名与账号（密码在 secret 文件，不在此） |
| `AI_PROVIDER` | `anthropic` | AI 提供方；`fake` 仅用于部署演练 |
| `ANTHROPIC_BASE_URL` | `https://api.anthropic.com` | Anthropic 兼容 API 的部署级基础地址，同时提供给 API 和 Worker。单个 Key 未设置 Base URL 时，API 将该值写入运行时 Pi 兼容设置，Worker 也以相同值解析并执行；普通 Messages 调用使用 `<base>/v1/messages`。URL 导入必须配置 gateway Key；B 站由 bili-cli 获取后交给支持图片输入的模型通过 Pi 总结，其他 URL 由 Pi SDK 获取（无 gateway Key 时 URL 导入报 `claude_not_configured`，不回退 legacy 单 Key） |
| `CLAUDE_MODEL_DEFAULT` | 空 | 覆盖默认模型，空为用代码内置默认 |
| `CLAUDE_REQUEST_TIMEOUT` | `300s` | 单次 AI 请求超时 |
| `CLAUDE_MAX_OUTPUT_TOKENS` | `8192` | AI 输出上限 |
| `WORKER_CONCURRENCY` | `1` | Worker 并行处理的任务数 |
| `WORKER_JOB_TIMEOUT` | `30m` | 单个任务总超时 |
| `JOB_LEASE_DURATION` | `60s` | 任务租约时长（Worker 崩溃后回收依据） |
| `MAINTENANCE_LEASE_DURATION` / `MAINTENANCE_HEARTBEAT_INTERVAL` | `45s` / `10s` | 备份期间维护锁的租约与心跳 |
| `MAX_UPLOAD_BYTES` / `MAX_DOWNLOAD_BYTES` | `2147483648` | 上传素材 / 下载远程素材上限（2 GB） |
| `URL_ALLOWLIST` | `youtube.com,bilibili.com` | URL 素材主机白名单，逗号分隔；父域条目允许其子域，API 与 Worker 使用同一值；Worker 侧另做 canonicalClaudeURL 规范化（剥离追踪参数、拒绝 userinfo/端口、250 字符上限）后提取；B 站仅接受 `/video/BV…` 或 `/video/av…` 链接并保留 `p` 分 P，其余站点沿用 Pi SDK（Worker 需出网访问这些站点） |
| `URL_API_DNS_VALIDATION` | `false` | 标准 Compose 的 API 无出网能力，因此只做协议/白名单校验；URL 实际抓取由 Worker 内 bili-cli（B 站）或 Pi SDK（其他站点）完成（抓取在 Job 内直接对白名单站点出网，不做预检），不再依赖 API 侧 DNS/重定向链校验 |
| `TRANSCRIPTION_MODE` | `subtitle_only` | 上传音视频的转写策略；`subtitle_only` 只用已有字幕不做本地转写；URL 素材不适用（B 站使用 bili-cli 自己的字幕/ASR 策略） |
| `WHISPER_MODEL_PATH` | 空 | 本地 Whisper 模型路径（启用本地转写时用，仅上传音视频） |
| `BACKUP_DIR` / `IMPORT_REPORT_DIR` / `LEGACY_SOURCE_DIR` | `../backups` 等 | 一次性工具容器挂载的宿主机目录，相对 `deploy/` |
| `CLAUDE_GATEWAY_CONFIG_DIR` | `../data/claude-gateway` | Claude 多 Key 网关配置目录（`keys.json`），只挂载进 `api`/`worker`，init 脚本创建为 0700 |
| `COOKIES_CONFIG_DIR` | `../data/cookies` | 站点登录 Cookie 目录（`bilibili.txt` 放 B 站 `SESSDATA`，Netscape 格式，0600），`api` 读写挂载、`worker` 只读挂载到 `/config/cookies`；推荐在 Web 设置页「B 站 Cookie」保存/替换/删除并「检测可用性」（Worker 请求 B 站 nav 端点验证登录状态），也可手动放文件；B 站 AI 字幕通常需登录才能获取（如 ai-zh），Worker 每次读取后转换为任务独立的 bili-cli 凭证，获取完成即删除；无 Cookie 时尝试匿名获取，字幕不可用会明确标注，不回退页面简介 |
| `TOOLS_UID` / `TOOLS_GID` | `10001` | 工具容器运行 UID/GID；Linux 上 init 脚本自动改为当前用户 |

改完 `.env` 后需要 `make docker-up`（配置类变更）或 `docker compose ... up -d --force-recreate` 生效；密钥文件内容变更同样需要重建容器（secret 在容器启动时挂载）。

### 4.1 B 站图文提取

Worker 镜像内置 `bili-agent==1.2.1`、Python、Node 22、FFmpeg 和中文字体。运行时基于 Debian Python slim，以支持 bili-cli 的 Python 原生依赖；仍以 UID/GID 10001 和只读根文件系统运行。更新后重建 Worker：

```bash
docker compose -f deploy/compose.yaml --env-file deploy/.env up -d --build worker
```

登录推荐直接使用前端：打开「设置 → B 站 Cookie → 扫码登录」，用哔哩哔哩 App 扫码并确认。页面会显示等待扫码、已扫码待确认、成功、过期或失败；关闭会取消尚未完成的登录，过期可重新生成。登录操作由 Worker 执行，API 仍无外网访问能力；确认后 API 自动写入只读挂载对应的共享 Cookie 文件，导入任务立即复用。二维码与临时凭证在共享数据卷私有目录（0700/0600）中短暂保存，完整 Cookie 不进入浏览器或任务结果。成功导入、失败和取消时清理临时凭证，超过 10 分钟的遗弃会话定期清理。

需要排查 CLI 时，可从仓库根目录进入 Worker：

```bash
docker compose -f deploy/compose.yaml --env-file deploy/.env exec worker bash
export BILI_CLI_HOME="$HOME/.config/bili-cli"
bili auth login
bili auth status --wait 120
```

交互终端会显示二维码，命令也返回容器内 PNG 路径。手动 CLI 登录只保存到它自己的 `credential.json`，不会自动更新应用的共享 Cookie；日常使用请通过前端扫码登录。新版前端登录需要同时重建 API、Worker、Web 并应用迁移（`make docker-up`），仅更新 Worker 不会增加前端入口。

管线先运行 `bili video watch <BV/av> --page <p> --frames 9 --comments 0 --no-danmaku --max-chars 0 --transcript auto`。预览图缺失时运行 `bili video frames <BV/av> --page <p> --quality hd --count 9` 补取画面。未开启远端观看记录。Cookie 只在任务私有目录转换成 `credential.json`（0600），Claude 启动前整个凭证目录已删除。

Claude 读取完整字幕文件和总览图片，提取带时间的板书、公式、图表、代码、演示要点，再生成综合总结。完整字幕由程序拼接保留，避免模型重抄丢失；最终 `raw_text` 包含「视频文字、画面内容、综合总结」，后续教纲、笔记、测验和辅导沿用现有 Markdown 输入。素材元数据标记 `extractor=bili-cli`、`parser_version=bili-cli-multimodal/v1`。既有素材需重新导入。

模型必须支持图片输入。总览图为 9 个均匀采样点，预览帧可能去重，不能保证覆盖所有画面；无字幕/转写时会明确标注，画面不可用则失败。已有 ASR 可在 Worker HOME 的 `.config/bili-cli/asr.json` 中配置（模型路径使用绝对路径）；任务复制该配置并由 bili-cli 自动转写。镜像不预装 ASR 模型或本地推理引擎，使用本地 ASR 需自行扩展镜像，云端引擎按 bili-cli 文档配置。

## 5. 日常运维

### 5.1 查看状态与日志

```bash
docker compose --env-file deploy/.env -f deploy/compose.yaml ps
docker compose --env-file deploy/.env -f deploy/compose.yaml logs -f api
docker compose --env-file deploy/.env -f deploy/compose.yaml logs -f worker --tail=200
```

### 5.2 停止与启动

```bash
make docker-down      # 停止并删除容器，保留数据卷
make docker-up        # 重新启动（含 migrate，幂等）
```

### 5.3 升级版本

```bash
git pull
make docker-up        # 重新构建镜像并滚动替换；migrate 自动应用新迁移
```

回滚：`git checkout <旧版本>` 后同样 `make docker-up`。数据库迁移有对应的 down 脚本，但跨版本回滚前建议先做备份（§6.1）。

### 5.4 健康诊断

```bash
make docker-doctor    # 数据一致性检查（只读），发现问题会给出修复建议
```

也可以在浏览器里看「设置 → 系统状态」页，对应 `GET /api/v1/system/status`。

## 6. 数据管理

### 6.1 备份

```bash
make docker-backup
```

- 备份前自动进入维护模式（用户写操作暂停，Worker 任务安全停顿），完成后自动退出维护模式；
- 产物为 `backups/backup-<时间戳>/`，包含数据库 dump 与 `app_data/objects` 对象文件归档，不会打包整个 `app_data` 卷；
- 运行时 `~/.claude/settings.json`、Claude 网关配置以及 Docker secret 不在备份中；恢复后需要由操作员重新配置并在设置页再次应用一个 Key；
- 建议升级、回滚、大批量导入前都执行一次。

### 6.2 恢复

```bash
make docker-restore RESTORE_BACKUP=backup-20260725-120000
```

恢复是破坏性操作（覆盖现有数据库与对象文件），命令内置 `RESTORE_CONFIRM=RESTORE` 二次确认。恢复不会还原运行时 Pi 兼容设置、`data/claude-gateway/keys.json` 或 `deploy/secrets/`；完成后运行 `make docker-up` 重启服务，再重新配置并应用 Claude Key。

### 6.3 彻底重置（危险）

```bash
docker compose --env-file deploy/.env -f deploy/compose.yaml down -v
```

`-v` 会**删除 `pg_data` 与 `app_data` 两个数据卷**，所有课程、试卷、进度全部丢失，不可恢复。执行前确认已有备份。

## 7. 旧 Python 系统数据导入

如果之前用过 Python CLI/TUI 版本（数据在 `legacy-data/`），分三步：

```bash
# 1. 扫描：只读，产出扫描报告
make docker-legacy-scan
#    报告写入 import-reports/scan.json

# 2. 人工核对报告后编写 resolution 文件（处理冲突项），然后执行导入：
make docker-legacy-import LEGACY_IMPORT_ARGS='import --source /legacy --report /reports/import.json --resolution /reports/resolution.json'

# 3. 导入是幂等的，可安全重复执行；审计记录写入数据库 legacy_import_audit 表
```

旧数据目录默认从 `legacy-data/` 只读挂载，如需换位置改 `deploy/.env` 的 `LEGACY_SOURCE_DIR`。

## 8. 部署演练与维护修复

```bash
make docker-drill                  # 完整部署演练：备份/恢复/导入全流程在临时环境跑一遍
make docker-maintenance-repair     # Worker 在维护期间被强杀后，清理卡死的维护锁
```

`docker-drill` 需要设置 `RUN_DEPLOYMENT_DRILLS=1` 才会真正执行（Makefile 已带），适合大版本升级前验证运维手册仍然有效。演练末尾会调用 Compose 边界检查器，因此宿主机必须安装 Python 3.11+；脚本会在启动或删除任何容器前检查该前置条件。

## 9. 故障排查

| 症状 | 排查 |
| --- | --- |
| `web` 起不来、端口被占 | `lsof -i :8080` 找到占用进程，或改 `deploy/.env` 的 `APP_PORT` |
| `api` 健康检查失败 | 看 `logs api`；多数是 `migrate` 失败 → 看 `logs migrate`，通常是数据库没就绪或迁移冲突 |
| AI 任务全部失败 | 检查 `deploy/secrets/anthropic_api_key.txt` 是否为空或失效；`docker compose ... logs worker` 看具体错误 |
| macOS 上测试/数据库报 `could not create shared memory segment` | 泄漏的嵌入式 PostgreSQL 占满 SysV 共享内存：`ipcs -m` 查看，`pkill -f databasetest-runtime` 清理后用 `ipcrm -m` 释放残留段（仅影响本地跑 Go 测试，不影响容器部署） |
| 前端镜像构建在 `npm run test` 失败 | 这是质量门禁，先修测试再部署；不要在 Dockerfile 里跳过测试 |
| 工具容器写 `backups/` 权限拒绝（Linux） | 确认 `deploy/.env` 里 `TOOLS_UID`/`TOOLS_GID` 是当前用户（init 脚本会自动设置） |
| 忘了数据库密码 | 密码在 `deploy/secrets/postgres_password.txt`；重建密码需要同时重置数据库卷，等同数据重置 |

## 10. 安全要点

- 所有应用容器：`read_only` 根文件系统 + `cap_drop: ALL` + `no-new-privileges` + 非 root 用户运行。
- 密钥一律走 Docker secrets 文件挂载，不进镜像、不进环境变量明文、不进 Git（`deploy/secrets/*.txt` 已被 `.gitignore` 排除）。
- `deploy/.env` 权限 600，同样不提交。
- 对外只暴露 `127.0.0.1:8080`；如需局域网访问，自行在 `web` 的 `ports` 前加反向代理并配 TLS，不要直接把端口绑到 `0.0.0.0`。

## 11. 参考

- 更深的设计决策（网络分段、维护模式、备份一致性模型）：[web-rebuild/05-deployment-operations.md](./web-rebuild/05-deployment-operations.md)
- 部署演练报告：[web-rebuild/15-compose-deployment-drill-report.md](./web-rebuild/15-compose-deployment-drill-report.md)
- OpenAPI 契约（排障时核对接口行为）：[../api/README.md](../api/README.md)

### Pi 执行引擎

Worker 使用固定版本 `@earendil-works/pi-coding-agent@1.4.0`（Node >=22.19），通过 `/app/pi-runtime/agent.mjs` 运行独立任务。链接提取和常规/复习测验均走 Pi SDK；普通教纲、笔记及其他结构化 AI 能力按各 Key 的协议调用 HTTP 网关。网关支持 Anthropic Messages 和 OpenAI Chat Completions，图文总结需模型支持图片输入。

Pi 会话、凭证和模型目录仅存内存，关闭插件、技能和项目指令自动发现。任务结束按文件契约交接完整原文、图文总结或 `quiz.json`，并回传脱敏 token/耗时元数据；HTTP 401/403、限流和取消沿用既有任务错误码。旧 `claude_*` 错误码及 `claude-code-settings` API 路由保留兼容。

本地修改桥接代码后，在 `backend/pi-runtime` 执行 `npm ci && npm test`；测试通过模拟网关实际驱动 Pi 的读图、读字幕、写文件和 bash 校验。重新构建并启动 Worker/Web 镜像即可更新运行服务。

### 网关协议选择

在设置页添加或编辑 Key 时，可选择 `Anthropic（Messages）` 或 `OpenAI（Chat Completions）`。连接测试、普通结构化 AI 请求和应用到 Pi 的配置都使用该 Key 的协议。旧 `keys.json` 没有 `protocol` 字段时继续按 `anthropic` 解析；公开 API 路由和 `claude_*` 错误码保持兼容。

- Anthropic 使用 `x-api-key` 和 Messages 端点；未设置 Base URL 时继承 `ANTHROPIC_BASE_URL`。
- OpenAI 使用 `Authorization: Bearer` 和 Chat Completions 端点；未设置 Base URL 时使用 `https://api.openai.com/v1`，不继承 Anthropic 地址。可填写根域名、`/v1` 基础地址或完整 `/chat/completions` 端点，代理的路径前缀会保留。OpenAI 协议参考[官方 Chat Completions 文档](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create)。
- 结构化 HTTP 请求对 OpenAI 官方端点指定 `structured_result` 函数；兼容网关使用 `tool_choice: auto` 并在提示中要求调用该函数，避免部分网关对强制工具调用返回 503。结果仍需通过 JSON 和业务 Schema 校验。若普通文本请求成功而连接测试返回 503，应核对网关的工具调用兼容性。
- 编辑协议、模型、密钥或地址后，先保存并测试 Key，再点击「应用到运行服务的 Pi」。已应用配置与 Key 不匹配时，Pi 任务会提示重新应用，避免静默使用过期配置。
- 协议变更会重置 Key 的认证/冷却状态，并隔离旧协议尚未结束的请求结果。完整 Key 不进入列表响应、任务、日志或 Pi 输出。

### 额度不足排障

连接测试失败时，先区分 HTTP 认证错误和上游额度错误。已知机器码 `insufficient_user_quota`、`insufficient_token_quota`、`insufficient_quota` 等会映射为 `claude_quota_exhausted`，Key 显示「额度不足」。该状态不自动重试，也不参与普通路由；其他 Key 仍可故障转移。账户补充额度后手动测试同一 Key，成功后恢复健康状态。HTTP 401/403 没有明确额度错误码时仍按认证/权限失败处理。原始上游消息、余额和 Key 不写入任务、运行态或日志。

### 课程自动命名与总结格式

课程创建的 `title` 可省略或留空；首次成功发布 AI 教纲或笔记时自动命名，手动名称始终保留。部署需要执行 `0010_course_auto_title` 迁移。新生成的视频总结、教纲与笔记共用 [study-summary/v1 格式](ai-summary-format.md)，并在发布前校验；既有课程和历史文档不自动重写。

### Pi 流式任务进展

Pi SDK 对 Anthropic 和 OpenAI 均使用流式模型请求。桥接进程通过 stderr 的有界 NDJSON 实时发送启动、生成字符计数及 read/write/bash 工具状态；stdout 保持一个最终结果信封。Worker 在进程结束前解析这些状态，写入已有 `job.progress` 事件，前端任务页和进度卡经现有 SSE 自动刷新，不需要等待整次生成结束。

生成字符数是已接收的文本/工具参数增量计数，不是完成百分比。正文、思考过程、工具参数和工具结果不会进入任务事件；最终文档或试卷仍按原文件契约校验后发布。任务取消、超时或进度更新丢失租约时会中止子进程。部署本次变更需重建并启动 Worker/Web；继续使用现有 API 和 SSE 协议。
