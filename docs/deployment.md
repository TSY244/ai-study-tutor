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
                                                                       └─► [egress] ──► 外网（Anthropic API；URL 素材由容器内 Claude Code CLI WebFetch 抓取白名单站点）
```

- `frontend`、`backend` 是 internal 网络，不出网。
- 只有 `web` 暴露端口，且只绑定 `127.0.0.1`；`db` 不暴露任何端口。
- 只有 `worker` 能访问外网（`egress` 网络），AI 调用从这里出去；URL 素材由 Worker 容器内 Claude Code CLI 的 WebFetch 客户端工具抓取，需要出网访问白名单站点（不用 yt-dlp 下载 URL）。

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
| Claude Code 运行时设置 | 命名卷 `app_data` 内的 `/var/lib/ai-study-tutor/.claude/settings.json` | API 原子写入，Worker Claude Code 读取；宿主机 `~/.claude` 不会挂载，web 不可见 |

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

每张 Claude Key 卡片还可选择「应用到运行服务」。确认后，API 从已保存的 Key 中读取密钥，在共享 `app_data` 卷内原子覆盖运行环境的 `~/.claude/settings.json`；浏览器只提交 Key ID，不会接收或重新发送完整 Key。后续 Worker Claude Code 任务以该文件中的应用选择器为准；文件没有应用选择器时才回退到网关的优先级/权重路由。API 和 Worker 的 `HOME` 都是 `/var/lib/ai-study-tutor`，宿主机的 `~/.claude` 不会挂载进任何容器，Web 也看不到该配置。

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
| `ANTHROPIC_BASE_URL` | `https://api.anthropic.com` | Anthropic 兼容 API 的部署级基础地址，同时提供给 API 和 Worker。单个 Key 未设置 Base URL 时，API 将该值写入运行时 Claude Code 设置，Worker 也以相同值解析并执行；普通 Messages 调用使用 `<base>/v1/messages`。URL 导入由 Worker 容器内 Claude Code CLI 的 WebFetch 客户端工具抓取，且必须配置 gateway Key（无 gateway Key 时 URL 导入报 `claude_not_configured`，不回退 legacy 单 Key） |
| `CLAUDE_MODEL_DEFAULT` | 空 | 覆盖默认模型，空为用代码内置默认 |
| `CLAUDE_REQUEST_TIMEOUT` | `300s` | 单次 AI 请求超时 |
| `CLAUDE_MAX_OUTPUT_TOKENS` | `8192` | AI 输出上限 |
| `WORKER_CONCURRENCY` | `1` | Worker 并行处理的任务数 |
| `WORKER_JOB_TIMEOUT` | `30m` | 单个任务总超时 |
| `JOB_LEASE_DURATION` | `60s` | 任务租约时长（Worker 崩溃后回收依据） |
| `MAINTENANCE_LEASE_DURATION` / `MAINTENANCE_HEARTBEAT_INTERVAL` | `45s` / `10s` | 备份期间维护锁的租约与心跳 |
| `MAX_UPLOAD_BYTES` / `MAX_DOWNLOAD_BYTES` | `2147483648` | 上传素材 / 下载远程素材上限（2 GB） |
| `URL_ALLOWLIST` | `youtube.com,bilibili.com` | URL 素材主机白名单，逗号分隔；父域条目允许其子域，API 与 Worker 使用同一值；Worker 侧另做 canonicalClaudeURL 规范化（剥离追踪参数、拒绝 userinfo/端口、250 字符上限）后由容器内 Claude Code CLI WebFetch 提取（Worker 需出网访问这些站点） |
| `URL_API_DNS_VALIDATION` | `false` | 标准 Compose 的 API 无出网能力，因此只做协议/白名单校验；URL 实际抓取由 Worker 容器内 Claude Code CLI WebFetch 完成（抓取在 Job 内直接对白名单站点出网，不做预检），不再依赖 API 侧 DNS/重定向链校验 |
| `TRANSCRIPTION_MODE` | `subtitle_only` | 上传音视频的转写策略；`subtitle_only` 只用已有字幕不做本地转写；URL 素材不适用（走容器内 Claude Code CLI WebFetch） |
| `WHISPER_MODEL_PATH` | 空 | 本地 Whisper 模型路径（启用本地转写时用，仅上传音视频） |
| `BACKUP_DIR` / `IMPORT_REPORT_DIR` / `LEGACY_SOURCE_DIR` | `../backups` 等 | 一次性工具容器挂载的宿主机目录，相对 `deploy/` |
| `CLAUDE_GATEWAY_CONFIG_DIR` | `../data/claude-gateway` | Claude 多 Key 网关配置目录（`keys.json`），只挂载进 `api`/`worker`，init 脚本创建为 0700 |
| `COOKIES_CONFIG_DIR` | `../data/cookies` | 站点登录 Cookie 目录（`bilibili.txt` 放 B 站 `SESSDATA`，Netscape 格式，0600），`api` 读写挂载、`worker` 只读挂载到 `/config/cookies`；推荐在 Web 设置页「B 站 Cookie」保存/替换/删除并「检测可用性」（Worker 请求 B 站 nav 端点验证登录状态），也可手动放文件；B 站字幕需登录才能获取（含 AI 字幕 ai-zh），无 Cookie 时 bilibili 链接只能提取页面文本 |
| `TOOLS_UID` / `TOOLS_GID` | `10001` | 工具容器运行 UID/GID；Linux 上 init 脚本自动改为当前用户 |

改完 `.env` 后需要 `make docker-up`（配置类变更）或 `docker compose ... up -d --force-recreate` 生效；密钥文件内容变更同样需要重建容器（secret 在容器启动时挂载）。

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

恢复是破坏性操作（覆盖现有数据库与对象文件），命令内置 `RESTORE_CONFIRM=RESTORE` 二次确认。恢复不会还原运行时 Claude Code 设置、`data/claude-gateway/keys.json` 或 `deploy/secrets/`；完成后运行 `make docker-up` 重启服务，再重新配置并应用 Claude Key。

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
