# Docker 部署与运维文档

## 1. 部署范围

本文面向个人电脑上的单用户部署：

- 使用 Docker Desktop 或 Docker Engine + Docker Compose v2。
- 浏览器只通过 `http://127.0.0.1:8080` 访问。
- 不提供公网访问、账号登录或多用户隔离。
- Worker 容器通过 API Key 调用远程 Anthropic Claude API。
- PostgreSQL 和文件存储位于个人电脑的 Docker 卷/绑定目录。

> Claude 模型不在 Docker 中本地运行。Docker 中运行的是 Go Worker、素材工具和 Anthropic API 客户端。

## 2. 前置条件

建议：

- Docker Desktop 近期稳定版，或 Linux Docker Engine + Compose plugin。
- Python 3.11+ 只用于源码仓库的 Compose 边界单元测试与验证脚本（纯标准库）；生产应用与容器运行时不依赖宿主机 Python。
- 至少 4 核 CPU、8GB 可分配内存。
- 至少 20GB 可用磁盘；视频素材用户应准备更多空间。
- 可访问 Anthropic API 的网络。
- 要使用 AI 功能时需要有效 `ANTHROPIC_API_KEY`；无 Key 的本地开发可降级启动。

首次安装前检查：

```bash
docker version
docker compose version
docker info
```

## 3. 目录布局

建议部署文件位于：

```text
deploy/
├── compose.yaml
├── Caddyfile
├── .env.example
├── .env                       # 不提交
├── secrets/
│   ├── anthropic_api_key.txt  # 不提交，权限 0600
│   └── postgres_password.txt  # 不提交，权限 0600
└── scripts/
    ├── init.sh
    ├── release-smoke.sh
    ├── verify-secrets.sh
    ├── verify_compose_boundary.py
    ├── test_verify_compose_boundary.py
    └── drill.sh

backups/                       # 备份包
import-reports/                # legacy-import 扫描报告
```

运行时业务文件保存在 Compose 命名卷 `app_data`，数据库保存在命名卷
`pg_data`；`backups/` 与 `import-reports/` 是只被一次性 Tools 容器绑定挂载
的宿主机目录。日常 `docker compose down` 不会删除这两个命名卷。

`.gitignore` 必须包含：

```gitignore
deploy/.env
deploy/secrets/
data/
backups/
```

不要忽略 `.env.example`、Compose、Caddyfile 和运维脚本。

## 4. 配置

### 4.1 非敏感 `.env`

`deploy/.env.example` 建议：

```dotenv
APP_VERSION=local
APP_PORT=8080
APP_ENV=production
LOG_LEVEL=info

POSTGRES_DB=ai_study_tutor
POSTGRES_USER=ai_study_tutor

AI_PROVIDER=anthropic
CLAUDE_MODEL_DEFAULT=replace-with-supported-model
CLAUDE_REQUEST_TIMEOUT=300s
CLAUDE_MAX_OUTPUT_TOKENS=8192

WORKER_CONCURRENCY=1
JOB_LEASE_DURATION=60s
MAINTENANCE_LEASE_DURATION=45s
MAINTENANCE_HEARTBEAT_INTERVAL=10s
MAX_UPLOAD_BYTES=2147483648
MAX_DOWNLOAD_BYTES=2147483648
URL_ALLOWLIST=youtube.com,bilibili.com
URL_API_DNS_VALIDATION=false

TRANSCRIPTION_MODE=subtitle_only
WHISPER_MODEL_PATH=

BACKUP_DIR=../backups
LEGACY_SOURCE_DIR=../legacy-data
IMPORT_REPORT_DIR=../import-reports
TOOLS_UID=10001
TOOLS_GID=10001
```

模型标识应通过配置注入，并在实际实施时从 Anthropic 官方模型文档选择受支持型号，不要将文档示例长期硬编码到业务代码。

`deploy/scripts/init.sh` 在 Linux 上应把 `TOOLS_UID/TOOLS_GID` 写为执行用户的 `id -u/id -g`，并用同一身份创建 `backups`、`import-reports`；Docker Desktop 可保留镜像固定 UID，但首次启动仍必须验证这些绑定目录可写。禁止为了绕过目录权限把 Tools 改为 root 常驻运行。

`AI_PROVIDER=fake` 仅用于 `deploy/scripts/drill.sh` 和 CI 的本地闭环；它不读取 Anthropic secret，也不生成学习正文。常规本机运行使用 `anthropic`，且只有 Worker 会读取 `ANTHROPIC_API_KEY_FILE`。

### 4.2 Secret

创建目录并限制权限：

```bash
mkdir -p deploy/secrets data backups
chmod 700 deploy/secrets
umask 077
```

写入 Anthropic Key：

```bash
printf '%s' '你的 ANTHROPIC_API_KEY' > deploy/secrets/anthropic_api_key.txt
```

若暂时不配置 Key，`init.sh` 也必须创建空的 0600 文件，保证 Compose secret 可挂载；Worker 将其识别为“未配置”而不是启动失败：

```bash
install -m 0600 /dev/null deploy/secrets/anthropic_api_key.txt
```

生成数据库密码：

```bash
openssl rand -base64 32 > deploy/secrets/postgres_password.txt
```

检查权限：

```bash
chmod 600 deploy/secrets/*.txt
```

Docker Compose secret 会以文件形式挂载到 `/run/secrets/<name>`。只有明确声明该 secret 的服务可以访问。Docker 官方文档建议 secret 优于普通环境变量，以降低意外泄漏到进程环境和日志的风险。

## 5. Compose 拓扑

当前可运行配置位于 `deploy/compose.yaml`；以下拓扑用于说明服务边界，实际发布以仓库配置和 `docker compose config` 展开结果为准：

```yaml
name: ai-study-tutor

services:
  db:
    image: postgres:17-alpine
    restart: unless-stopped
    environment:
      POSTGRES_DB: ${POSTGRES_DB}
      POSTGRES_USER: ${POSTGRES_USER}
      POSTGRES_PASSWORD_FILE: /run/secrets/postgres_password
    secrets:
      - postgres_password
    volumes:
      - pg_data:/var/lib/postgresql/data
    networks:
      - backend
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U $${POSTGRES_USER} -d $${POSTGRES_DB}"]
      interval: 5s
      timeout: 5s
      retries: 12
      start_period: 20s

  migrate:
    image: ai-study-tutor-api:${APP_VERSION}
    command: ["/app/migrate", "up"]
    environment:
      DB_HOST: db
      DB_PORT: "5432"
      DB_NAME: ${POSTGRES_DB}
      DB_USER: ${POSTGRES_USER}
      DB_PASSWORD_FILE: /run/secrets/postgres_password
    secrets:
      - postgres_password
    networks:
      - backend
    depends_on:
      db:
        condition: service_healthy
    restart: "no"

  api:
    image: ai-study-tutor-api:${APP_VERSION}
    build:
      context: ../backend
      dockerfile: Dockerfile.api
      target: runtime
    restart: unless-stopped
    environment:
      APP_ENV: ${APP_ENV}
      HTTP_ADDR: ":8080"
      LOG_LEVEL: ${LOG_LEVEL}
      DB_HOST: db
      DB_PORT: "5432"
      DB_NAME: ${POSTGRES_DB}
      DB_USER: ${POSTGRES_USER}
      DB_PASSWORD_FILE: /run/secrets/postgres_password
      APP_DATA_DIR: /var/lib/ai-study-tutor
      MAX_UPLOAD_BYTES: ${MAX_UPLOAD_BYTES}
    secrets:
      - postgres_password
    volumes:
      - app_data:/var/lib/ai-study-tutor
    networks:
      - frontend
      - backend
    depends_on:
      migrate:
        condition: service_completed_successfully
    healthcheck:
      test: ["CMD", "/app/api", "healthcheck"]
      interval: 10s
      timeout: 3s
      retries: 6
      start_period: 10s
    read_only: true
    tmpfs:
      - /tmp
    security_opt:
      - no-new-privileges:true
    cap_drop: [ALL]

  worker:
    image: ai-study-tutor-worker:${APP_VERSION}
    build:
      context: ../backend
      dockerfile: Dockerfile.worker
      target: runtime
    restart: unless-stopped
    environment:
      APP_ENV: ${APP_ENV}
      LOG_LEVEL: ${LOG_LEVEL}
      DB_HOST: db
      DB_PORT: "5432"
      DB_NAME: ${POSTGRES_DB}
      DB_USER: ${POSTGRES_USER}
      DB_PASSWORD_FILE: /run/secrets/postgres_password
      ANTHROPIC_API_KEY_FILE: /run/secrets/anthropic_api_key
      CLAUDE_MODEL_DEFAULT: ${CLAUDE_MODEL_DEFAULT}
      CLAUDE_REQUEST_TIMEOUT: ${CLAUDE_REQUEST_TIMEOUT}
      CLAUDE_MAX_OUTPUT_TOKENS: ${CLAUDE_MAX_OUTPUT_TOKENS}
      WORKER_CONCURRENCY: ${WORKER_CONCURRENCY}
      JOB_LEASE_DURATION: ${JOB_LEASE_DURATION}
      MAX_DOWNLOAD_BYTES: ${MAX_DOWNLOAD_BYTES}
      TRANSCRIPTION_MODE: ${TRANSCRIPTION_MODE}
      WHISPER_MODEL_PATH: ${WHISPER_MODEL_PATH}
      APP_DATA_DIR: /var/lib/ai-study-tutor
    secrets:
      - postgres_password
      - anthropic_api_key
    volumes:
      - app_data:/var/lib/ai-study-tutor
    networks:
      - backend
      - egress
    depends_on:
      migrate:
        condition: service_completed_successfully
    healthcheck:
      test: ["CMD", "/app/worker", "healthcheck"]
      interval: 20s
      timeout: 5s
      retries: 6
      start_period: 20s
    cpus: 2.0
    mem_limit: 4g
    read_only: true
    tmpfs:
      - /tmp
    security_opt:
      - no-new-privileges:true
    cap_drop: [ALL]

  web:
    image: ai-study-tutor-web:${APP_VERSION}
    build:
      context: ..
      dockerfile: frontend/Dockerfile
      target: runtime
    restart: unless-stopped
    user: "10001:10001"
    ports:
      - "127.0.0.1:${APP_PORT}:8080"
    networks:
      - public
      - frontend
    depends_on:
      api:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://127.0.0.1:8080/healthz"]
      interval: 10s
      timeout: 3s
      retries: 6
    read_only: true
    tmpfs:
      - /data:uid=10001,gid=10001,mode=0700
      - /config:uid=10001,gid=10001,mode=0700
    security_opt:
      - no-new-privileges:true
    cap_drop: [ALL]

  backup:
    image: ai-study-tutor-tools:${APP_VERSION}
    build:
      context: ../backend
      dockerfile: Dockerfile.tools
      target: runtime
    profiles: [tools]
    user: "${TOOLS_UID:-10001}:${TOOLS_GID:-10001}"
    command: ["/app/backup", "create"]
    environment:
      DB_HOST: db
      DB_PORT: "5432"
      DB_NAME: ${POSTGRES_DB}
      DB_USER: ${POSTGRES_USER}
      DB_PASSWORD_FILE: /run/secrets/postgres_password
      APP_DATA_DIR: /var/lib/ai-study-tutor
      APP_BACKUP_DIR: /backups
    secrets:
      - postgres_password
    volumes:
      - app_data:/var/lib/ai-study-tutor:ro
      - ${BACKUP_DIR}:/backups
    networks:
      - backend
    depends_on:
      db:
        condition: service_healthy
    read_only: true
    tmpfs:
      - /tmp
    security_opt:
      - no-new-privileges:true
    cap_drop: [ALL]

  restore:
    image: ai-study-tutor-tools:${APP_VERSION}
    build:
      context: ../backend
      dockerfile: Dockerfile.tools
      target: runtime
    profiles: [manual]
    user: "${TOOLS_UID:-10001}:${TOOLS_GID:-10001}"
    command: ["/app/restore", "${RESTORE_BACKUP}"]
    environment:
      DB_HOST: db
      DB_PORT: "5432"
      DB_NAME: ${POSTGRES_DB}
      DB_USER: ${POSTGRES_USER}
      DB_PASSWORD_FILE: /run/secrets/postgres_password
      APP_DATA_DIR: /var/lib/ai-study-tutor
      APP_BACKUP_DIR: /backups
    secrets:
      - postgres_password
    volumes:
      - app_data:/var/lib/ai-study-tutor
      - ${BACKUP_DIR}:/backups:ro
    networks:
      - backend
    depends_on:
      db:
        condition: service_healthy
    restart: "no"
    read_only: true
    tmpfs:
      - /tmp
    security_opt:
      - no-new-privileges:true
    cap_drop: [ALL]

  doctor:
    image: ai-study-tutor-tools:${APP_VERSION}
    build:
      context: ../backend
      dockerfile: Dockerfile.tools
      target: runtime
    profiles: [tools]
    user: "${TOOLS_UID:-10001}:${TOOLS_GID:-10001}"
    command: ["/app/doctor"]
    environment:
      DB_HOST: db
      DB_PORT: "5432"
      DB_NAME: ${POSTGRES_DB}
      DB_USER: ${POSTGRES_USER}
      DB_PASSWORD_FILE: /run/secrets/postgres_password
      APP_DATA_DIR: /var/lib/ai-study-tutor
    secrets:
      - postgres_password
    volumes:
      - app_data:/var/lib/ai-study-tutor:ro
    networks:
      - backend
    depends_on:
      db:
        condition: service_healthy
    restart: "no"
    read_only: true
    tmpfs:
      - /tmp
    security_opt:
      - no-new-privileges:true
    cap_drop: [ALL]

  maintenance-repair:
    image: ai-study-tutor-tools:${APP_VERSION}
    build:
      context: ../backend
      dockerfile: Dockerfile.tools
      target: runtime
    profiles: [manual]
    user: "${TOOLS_UID:-10001}:${TOOLS_GID:-10001}"
    command: ["/app/doctor", "--repair-stale-maintenance"]
    environment:
      DB_HOST: db
      DB_PORT: "5432"
      DB_NAME: ${POSTGRES_DB}
      DB_USER: ${POSTGRES_USER}
      DB_PASSWORD_FILE: /run/secrets/postgres_password
      APP_DATA_DIR: /var/lib/ai-study-tutor
      APP_QUARANTINE_DIR: /var/lib/ai-study-tutor/quarantine/maintenance-recovery
    secrets:
      - postgres_password
    volumes:
      - app_data:/var/lib/ai-study-tutor
    networks:
      - backend
    depends_on:
      db:
        condition: service_healthy
    restart: "no"
    read_only: true
    tmpfs:
      - /tmp
    security_opt:
      - no-new-privileges:true
    cap_drop: [ALL]

  legacy-import:
    image: ai-study-tutor-tools:${APP_VERSION}
    build:
      context: ../backend
      dockerfile: Dockerfile.tools
      target: runtime
    profiles: [manual]
    user: "${TOOLS_UID:-10001}:${TOOLS_GID:-10001}"
    entrypoint: ["/app/legacy-import"]
    command: ["scan", "--source", "/legacy", "--report", "/reports/scan.json"]
    environment:
      DB_HOST: db
      DB_PORT: "5432"
      DB_NAME: ${POSTGRES_DB}
      DB_USER: ${POSTGRES_USER}
      DB_PASSWORD_FILE: /run/secrets/postgres_password
      APP_DATA_DIR: /var/lib/ai-study-tutor
    secrets:
      - postgres_password
    volumes:
      - ${LEGACY_SOURCE_DIR}:/legacy:ro
      - app_data:/var/lib/ai-study-tutor
      - ${IMPORT_REPORT_DIR}:/reports:rw
    networks:
      - backend
    depends_on:
      db:
        condition: service_healthy
    restart: "no"
    read_only: true
    tmpfs:
      - /tmp
    security_opt:
      - no-new-privileges:true
    cap_drop: [ALL]

volumes:
  pg_data:

networks:
  public: {}
  frontend:
    internal: true
  backend:
    internal: true
  egress: {}

secrets:
  anthropic_api_key:
    file: ./secrets/anthropic_api_key.txt
  postgres_password:
    file: ./secrets/postgres_password.txt
```

注意：

- `public` 只连接 Web；`frontend`、`backend` 是内部网络。只有 Web 映射宿主机端口。
- Web 通过 `frontend` 访问 API；API 不连接 `public` 或 `egress`，因此不获得普通出网路径。
- Worker 同时连接 `backend` 和 `egress`，因此可访问 PostgreSQL 和 Anthropic API。
- API 没有 `egress` 网络，也没有 Anthropic secret。
- 实际发布镜像应固定版本，正式发布建议进一步固定 digest。
- `postgres:17-alpine` 是方案示例；实施时对数据库主版本做兼容验证，不自动追随 `latest`，升级主版本前先演练备份恢复。

Docker Compose 支持用 `service_healthy` 和 `service_completed_successfully` 控制依赖启动条件，因此数据库健康后才运行迁移，迁移成功后才启动 API/Worker。

## 6. Caddy 配置

参考：

```caddyfile
:8080 {
    encode zstd gzip

    @unexpectedHost not host localhost 127.0.0.1
    respond @unexpectedHost 421

    @api path /api/* /healthz /readyz
    handle @api {
        reverse_proxy api:8080 {
            flush_interval -1
        }
    }

    root * /srv
    handle {
        try_files {path} /index.html
        file_server
    }

    header {
        X-Content-Type-Options nosniff
        Referrer-Policy no-referrer
        X-Frame-Options DENY
        -Server
    }
}
```

要求：

- `/api/v1/jobs/stream` 不被代理缓冲。
- `index.html` 不做长期 immutable 缓存。
- 带内容哈希的静态资源可长期缓存。
- Content Security Policy 需根据前端实际资源调整，默认不允许外部脚本。

## 7. 镜像设计

### 7.1 API 镜像

多阶段构建：

1. Go Builder 只编译 `api` 与 `migrate`。
2. Runtime 使用精简基础镜像。
3. 非 root 用户运行。
4. 不安装 shell、ffmpeg、yt-dlp 和 Node。
5. 根文件系统只读，仅 `/tmp` 和绑定数据目录可写。

### 7.2 Worker 镜像

包含：

- Go Worker 二进制。
- `ca-certificates`。
- 固定版本的 `ffmpeg`、`yt-dlp`（仅处理上传媒体）；`nodejs`/`npm` 与 npm 全局安装（pin 大版本）的 Claude Code CLI——URL 素材由容器内 CLI 的 WebFetch 客户端工具抓取，Worker 需出网访问白名单站点。
- DOCX/PDF 等必要本地解析依赖。
- 可选 Whisper CLI（仅上传音视频转写），但不默认内置大型模型文件。

镜像构建记录工具版本。对 `yt-dlp` 等易变工具制定升级和回归测试流程。

### 7.3 Web 镜像

1. Node Builder 运行前端测试和构建。
2. 只将 `dist/` 和 Caddyfile 复制到 Caddy Runtime。
3. 镜像创建固定 UID/GID 10001 的非 root 用户，Caddy 在容器内监听 8080，因此不需要 `NET_BIND_SERVICE`，可保留 `cap_drop: ALL`。
4. 构建参数中不得出现 Anthropic API Key 或数据库密码。

### 7.4 Tools 镜像

只供 `tools` 或 `manual` profile 的一次性运维/迁移任务使用，包含：

- 编译后的 `backup`、`restore`、`doctor`、`seed` 和 `legacy-import`。
- 与数据库主版本匹配的 PostgreSQL client（`pg_dump`、`pg_restore`）。
- 固定版本的 `tar`、`zstd` 和校验工具。

Tools 镜像不承载常驻 HTTP 服务，不获得 Anthropic secret。`backup` 和只读 `doctor` 使用 `tools` profile；具有数据卷写权限的 `restore`、`maintenance-repair` 与 `legacy-import` 使用 `manual` profile，只由明确的本机命令运行。所有服务都使用非 root 用户、只读根文件系统、临时 `/tmp`、`no-new-privileges` 和 `cap_drop: ALL`。不要为了复用而把这些运维工具装回 API 镜像。

### 7.5 前端 Docker 接口

前端构建产物放入 `frontend/dist`，由 Caddy 的 `web` 服务在容器内 `:8080`
提供。前端不得自行发布端口、访问数据库、挂载数据卷或声明 Anthropic secret。
浏览器只访问同源路径：`/api/v1/*`、`/healthz`、`/readyz` 和
`/api/v1/jobs/stream`；Caddy 已为 SSE 配置 `flush_interval -1`。前端 Dockerfile
需保持非 root、只读根文件系统，并在运行期允许 Caddy 使用 `/data`、`/config`
两个 tmpfs。唯一宿主机发布端口是 `web` 的
`127.0.0.1:${APP_PORT}:8080`。

## 8. 首次部署

### 8.1 准备配置

```bash
make docker-init
```

写入两个 secret 后验证 Compose：

```bash
docker compose --env-file deploy/.env -f deploy/compose.yaml config
```

注意：`config` 输出可能展开非敏感环境变量，但不应输出 secret 文件内容。

### 8.2 构建

实现阶段可提供 Makefile：

```bash
make docker-build
```

等价 Compose 流程：

```bash
docker compose --env-file deploy/.env -f deploy/compose.yaml --profile tools build
```

### 8.3 启动

```bash
docker compose --env-file deploy/.env -f deploy/compose.yaml up -d
docker compose --env-file deploy/.env -f deploy/compose.yaml ps
```

检查：

```bash
curl -fsS http://127.0.0.1:8080/healthz
curl -fsS http://127.0.0.1:8080/readyz
```

浏览器打开：

```text
http://127.0.0.1:8080
```

首次启动后在系统设置中执行 Claude 连通性测试。若需要导入 bilibili 视频链接的登录可见字幕，在同一设置页的「B 站 Cookie」中保存 SESSDATA 并执行「检测可用性」。

若为了无 AI 的本地开发暂时使用空的 `anthropic_api_key.txt`，系统应以降级模式启动；所有依赖 Claude 的新操作返回 `claude_not_configured`。正式学习流程仍要求写入有效 Key。部署演练改用 `AI_PROVIDER=fake`，不需要真实 Key。

## 9. 日常运维

### 9.1 查看状态

```bash
docker compose --env-file deploy/.env -f deploy/compose.yaml ps
docker compose --env-file deploy/.env -f deploy/compose.yaml logs --tail=200 api worker
```

实时日志：

```bash
docker compose --env-file deploy/.env -f deploy/compose.yaml logs -f --since=10m api worker
```

### 9.2 重启

```bash
docker compose --env-file deploy/.env -f deploy/compose.yaml restart api worker web
```

Worker 重启后，运行任务会等待租约过期并恢复。不能通过手动修改数据库把任务直接设为成功。

### 9.3 停止

```bash
docker compose --env-file deploy/.env -f deploy/compose.yaml down
```

禁止日常使用：

```bash
docker compose down -v
```

`-v` 会删除 PostgreSQL 数据卷。

## 10. 备份

### 10.1 备份目标

一个备份包包含：

```text
backup-YYYYMMDD-HHMMSS/
├── manifest.json
├── database.dump
├── files.tar.zst
└── checksums.sha256
```

Manifest 至少记录：

- 应用版本和 Git revision。
- 数据库迁移版本。
- PostgreSQL 主版本。
- 创建时间。
- 文件数量、总大小和校验和。
- 是否存在活动 Job。

### 10.2 一致性策略

推荐备份命令：

```bash
docker compose --env-file deploy/.env -f deploy/compose.yaml --profile tools run --rm backup
```

Backup 工具应：

1. 持有 session-scoped Maintenance Owner Lock，创建带唯一 Owner 的 `backups(status=creating)`；重复维护进程立即失败。
2. 在 Business Gate exclusive lock 下进入 `draining`，写入 45 秒维护租约；独立连接每 10 秒续租。
3. 等待业务写事务、运行 Job 和 temporary file object 全部归零；超时则取消备份。
4. 再次获取并持续持有 Business Gate exclusive lock，复查归零并进入 `quiesced`，阻止新的发布事务。
5. 执行 PostgreSQL 自定义格式 `pg_dump`，并在同一静默窗口打包 `objects/` 和已发布导出；不包含 `tmp/`、temporary 对象和可重建中间目录。
6. 生成 Manifest 和 SHA-256，原子移动到最终备份目录；把 Backup 标为 ready 并清理维护状态。
7. 正常错误在 finally/defer 中把 Backup 标为 failed、清理维护状态并释放两个锁。

SIGKILL 不会执行 finally。进程/数据库连接消失后 Owner Lock 自动释放，但维护行只允许由安全修复协议清理：

```bash
docker compose --env-file deploy/.env -f deploy/compose.yaml \
  --profile manual run --rm maintenance-repair
```

普通 `doctor` 只读检查；`maintenance-repair` 是唯一有数据卷写权限的修复服务。它必须先成功获取 Owner Lock，再在 Business Gate exclusive lock 下确认租约按数据库时间过期；回收过期 Job lease，并用同一文件 Recovery 完成、隔离或删除所有已过期 temporary 对象。仅在复查没有活动 lease/temporary 后，才原子地把同一 Owner 的 `creating` Backup 标记为 `failed(maintenance_lease_expired)`、记录审计并恢复 `system_state=normal`。Owner Lock 被占用、租约未过期或有有效 Owner 时严禁清理。下一次备份检测到遗留维护状态时拒绝执行，并输出此修复命令。

### 10.3 保留策略

默认建议：

- 最近 7 个日常备份。
- 最近 4 个周备份。
- 升级前备份手动标记，不被自动清理。

由于是个人电脑部署，备份目录最好定期复制到另一个物理磁盘；同一磁盘上的备份不能防止磁盘故障。

## 11. 恢复

恢复是破坏性操作，只通过本地脚本/命令执行，不提供浏览器在线按钮直接覆盖当前数据库。

流程：

1. 验证备份校验和和 Manifest。
2. 对当前状态再做一次应急备份。
3. 停止 `web`、`api`、`worker`。
4. 启动并等待 PostgreSQL ready。
5. 清理并恢复数据库 dump。
6. 在事务中把备份快照里的 `system_state` 完整归一为 `maintenance_mode=false, phase=normal`，清除 owner、started/heartbeat/lease 时间，并依据已验证 Manifest 将当前 `backups` 记录从快照中的 `creating` 归一为 `ready`；保留恢复审计记录。
7. 恢复文件对象到临时目录并校验。
8. 原子切换文件目录。
9. 运行数据库迁移兼容检查。
10. 启动 API/Worker/Web。
11. 运行 Doctor：数据库引用文件存在、active 文档唯一、Job 状态可恢复、维护状态已清除。

当前实现只允许恢复到**空的** `app_data` 和新建数据库卷。保留备份目录，清空
目标 Compose 卷后，以显式确认变量执行：

```bash
docker compose --env-file deploy/.env -f deploy/compose.yaml down -v
docker compose --env-file deploy/.env -f deploy/compose.yaml up -d db --wait
RESTORE_BACKUP=backup-YYYYMMDDTHHMMSSZ RESTORE_CONFIRM=RESTORE \
  docker compose --env-file deploy/.env -f deploy/compose.yaml \
  --profile manual run --rm restore
docker compose --env-file deploy/.env -f deploy/compose.yaml up -d --wait
make docker-doctor
```

`RESTORE_CONFIRM=RESTORE` 和精确的 `RESTORE_BACKUP` 名称缺一不可；禁止默认选择“最新备份”后直接覆盖。`down -v` 只应在已确认该项目卷可被备份恢复覆盖时使用。

## 12. 旧数据导入

旧系统数据根 `legacy-data/` 只以只读方式挂载到一次性 `legacy-import` 服务；Importer 从 `/legacy/course` 读取课程、从 `/legacy/.tasks` 读取历史任务。先扫描，不写数据库和文件卷：

```bash
docker compose --env-file deploy/.env -f deploy/compose.yaml \
  --profile manual run --rm legacy-import \
  scan --source /legacy --report /reports/scan.json --fail-on-warning
```

人工检查 Manifest、冲突和 resolution 后再导入：

```bash
docker compose --env-file deploy/.env -f deploy/compose.yaml \
  --profile manual run --rm legacy-import \
  import --source /legacy --report /reports/import.json --resolution /reports/resolution.json
```

约束：

- `/legacy` 永远是 `:ro`；Importer 不修改源文件、时间戳或权限。
- `/var/lib/ai-study-tutor` 和 `/reports` 是明确的 `:rw` 目标；Importer 运行时仍使用非 root 用户和只读根文件系统。
- `scan` 不创建业务数据；`import` 使用 staging batch、幂等记录和最终 reconciliation gate。
- 最终切换前必须先停止旧 Python 写入并对 `LEGACY_SOURCE_DIR` 生成新的 Manifest。
- `resolution.json` 的 `schema_version=1`、`manifest_sha256` 和每条 `accepted_findings` 必须精确匹配 scan 报告中的 error finding；导入批次保存 resolution SHA-256。
- 同一 Manifest 与同一 resolution 重跑复用原批次；换 resolution 会生成新审计批次，已映射的 source hash 只会产生 `deduplicated` 记录。
- `needs_review`、`quarantined` 和 `skipped_by_policy` 都会写入 JSON 报告与数据库审计。报告只含相对路径、hash、稳定原因码和计数，不含学习正文、答案或旧 Prompt。

便捷命令：

```bash
make docker-legacy-scan
LEGACY_IMPORT_ARGS='import --source /legacy --report /reports/import.json --resolution /reports/resolution.json --fail-on-warning' \
  make docker-legacy-import
```

## 13. 升级与回滚

### 13.1 升级

1. 阅读 Release Notes 和迁移说明。
2. 运行完整备份。
3. 拉取或构建指定版本镜像。
4. `docker compose config` 验证配置。
5. 启动新版本；migration 一次性服务先执行。
6. 检查 `/readyz`、Worker 心跳、Claude 测试和关键业务流程。
7. 保留旧镜像和升级前备份，直到验收完成。

建议：

```bash
APP_VERSION=0.4.0 docker compose --env-file deploy/.env -f deploy/compose.yaml up -d
```

### 13.2 数据库迁移原则

- 优先 Expand/Contract：先新增兼容字段，再切换代码，最后在后续版本删除旧字段。
- 迁移脚本不可依赖应用内存状态。
- 大数据更新分批执行并可恢复。
- 不可逆迁移必须在 Release Notes 中声明，并强制要求升级前备份。

### 13.3 回滚

- 若数据库迁移向后兼容：切回旧镜像并重启。
- 若迁移不可逆：停止服务，恢复升级前备份，再切回旧镜像。
- 不允许只恢复数据库而不恢复对应文件卷版本。

## 14. 数据目录清理

Worker 定期清理：

- `tmp/` 中超过 24 小时且无运行 Job 的目录。
- 已成功下载且策略允许删除的中间音频。
- 过期导出文件。
- 孤立 `file_objects`。

不会自动清理：

- 用户上传的原始素材，除非用户明确删除。
- 当前和历史 Document 版本。
- 试卷、作答、反馈和薄弱点证据。
- 备份目录。

所有自动清理先写审计日志，并按数据库引用判断，不能只按文件名猜测。

## 15. 安全基线

- Web 端口绑定 `127.0.0.1`，不是 `0.0.0.0`。
- Caddy/API 只接受配置的本机 Host；写请求校验同源 Origin，跨域预检默认拒绝，不配置 `Access-Control-Allow-Origin: *`。
- 只有 Worker 获得 Anthropic secret 和出网网络。
- PostgreSQL 不映射宿主机端口。
- 容器以非 root 用户运行，`cap_drop: ALL`、`no-new-privileges`、只读根文件系统。
- 上传文件受大小和类型限制；URL 素材经协议与白名单校验、Worker 规范化，实际抓取由 Worker 容器内 Claude Code CLI 的 WebFetch 客户端工具完成（需出网访问白名单站点）。
- 所有 Markdown 在前端安全渲染。
- 日志输出请求元数据，不输出 API Key、数据库密码和完整敏感正文。
- 依赖镜像和二进制定期扫描，发布时生成 SBOM。
- Secret、`.env`、数据和备份不提交 Git。

如果未来需要局域网访问，必须先新增：

- 认证和会话管理。
- HTTPS。
- CSRF 和来源校验。
- 安全的密码/恢复机制。
- 局域网威胁模型和访问日志。

不能只把端口绑定改成 `0.0.0.0`。

## 16. 常见故障

### 16.1 Web 可打开但 API 不可用

检查：

```bash
docker compose --env-file deploy/.env -f deploy/compose.yaml ps
docker compose --env-file deploy/.env -f deploy/compose.yaml logs --tail=200 migrate api db
```

常见原因：数据库未健康、migration 失败、secret 文件权限/内容错误。

### 16.2 Claude 测试失败

检查 Worker 日志中的稳定错误码，不输出 Key：

- `claude_auth_failed`：Key 无效或权限不足。
- `claude_rate_limited`：等待后重试。
- `claude_timeout`：网络或超时配置。
- `claude_invalid_request`：模型标识或请求不支持；URL 导入的提取失败先查 `material_fetch_failed`/`claude_auth_failed`——确认 Worker 镜像内 `claude` CLI 可用（`nodejs`+npm 全局安装）、容器以可写 HOME（`/var/lib/ai-study-tutor`）运行、网关 keys.json 有启用 Key，且 Worker 能出网访问白名单站点。

确认 Worker 有 `egress` 网络，而 API 不需要出网。

### 16.3 Job 长期 running

- 检查 Worker 是否健康和有心跳。
- 检查 `lease_expires_at` 是否过期。
- Recovery Loop 应自动回收过期任务。
- 若任务涉及子进程，确认 ffmpeg/yt-dlp 是否响应取消。

不要手工将任务改为 succeeded。

### 16.4 磁盘空间不足

- 系统状态应提前显示 `degraded`。
- 暂停新上传和下载。
- 清理可重建的 `tmp/` 和过期导出。
- 不直接删除 `objects/` 下被数据库引用的文件。

### 16.5 SSE 不更新

- REST 查询 Job 确认任务是否仍变化。
- 检查 Caddy 是否缓冲响应。
- 检查浏览器是否重连并发送 Last-Event-ID。
- SSE 故障不应影响 Job 实际执行。

### 16.6 系统长期停留在维护模式

先检查是否有正在运行的 Backup 和 Maintenance Owner Lock。只有确认备份进程已退出后才运行：

```bash
docker compose --env-file deploy/.env -f deploy/compose.yaml \
  --profile manual run --rm maintenance-repair
```

命令因 Owner Lock 被占用、租约未过期或仍有有效 Job/file Owner 而拒绝时，不得手工 `UPDATE system_state` 绕过保护；应先检查 Backup/数据库日志和系统时间。修复成功后核对对应 Backup 已标为 `failed(maintenance_lease_expired)`、过期 Job 已回收、temporary 对象已 ready/quarantined/deleted，再运行普通 Doctor 和一次新备份。

## 17. 运维验收清单

- [ ] 只有 `127.0.0.1:8080` 暴露到宿主机。
- [ ] `docker compose config` 不包含 secret 内容。
- [ ] API 容器没有 Anthropic secret 和出网网络。
- [ ] Worker 能访问 Anthropic API 和 PostgreSQL。
- [ ] 数据库健康后才执行 migration。
- [ ] migration 成功后才启动 API/Worker。
- [ ] 重建全部镜像后课程数据仍存在。
- [ ] Worker 强制终止后任务可恢复。
- [ ] 备份可在空环境恢复并通过 Doctor。
- [ ] `docker compose down` 不丢数据。
- [ ] 文档明确警告 `down -v` 的风险。

## 18. 官方参考

- [Docker Compose secrets](https://docs.docker.com/compose/how-tos/use-secrets/)
- [Docker Compose startup order](https://docs.docker.com/compose/how-tos/startup-order/)
- [Docker Compose service reference](https://docs.docker.com/reference/compose-file/services/)
- [Claude Go Messages API](https://platform.claude.com/docs/en/api/go/messages)
- [Claude API errors](https://platform.claude.com/docs/en/api/errors)
