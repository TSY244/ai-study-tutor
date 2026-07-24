# Compose 部署演练记录

## 范围

本记录覆盖个人电脑上的单用户 Compose 部署。演练使用独立项目名
`ai-study-tutor-drill`、回环端口 `127.0.0.1:18080`、一次性命名卷和临时备份
目录；不会操作常规 `ai-study-tutor` 项目的容器或卷。Provider 设为 `fake`，不
需要也不读取真实 Anthropic Key。

## 已验证结果

| 演练 | 结果 | 证据 |
| --- | --- | --- |
| 空环境启动 | 通过 | `db -> migrate -> api/worker -> web` 全部 healthy；`/healthz` 返回 `{"status":"ok"}`。 |
| Fake 闭环 | 通过 | `POST /api/v1/system/claude-tests` 返回 202，Worker 将 Job 推进到 `succeeded`。 |
| 重启持久化 | 通过 | 重启 API/Worker 后，`GET /api/v1/courses?limit=10` 仍返回 seed 课程。 |
| Worker 强杀 | 通过 | 在 Worker 容器内向 PID 1 发送 `SIGKILL`，Docker `unless-stopped` 策略重启进程并重新 healthy。 |
| 维护恢复 | 通过 | 注入过期 `system_state` 后，Doctor 返回异常；`maintenance-repair` 完成后 Doctor 返回正常。 |
| 备份恢复 | 通过 | 备份发布为 `backup-20260722T161525Z`；销毁隔离卷、启动空 DB、显式确认 restore 后，恢复完成且课程可读。 |
| Caddy/SSE | 通过 | `release-smoke.sh` 验证 REST、SPA 回退、`text/event-stream`、不可变静态资源缓存头。 |
| Secret 边界 | 通过 | `verify-secrets.sh` 断言只有 Worker 挂载 `anthropic_api_key`；API 容器中该路径不存在。 |
| 运行时权限 | 通过 | API、Worker、Tools 镜像均为 UID/GID `10001:10001`，服务使用只读根文件系统与 `cap_drop: ALL`。 |

恢复演练暴露并修复了一个工具调用问题：`pg_restore` 必须显式指定
`--dbname=<database>`；仅设置 `PGDATABASE` 会被解释为输出模式。该参数只包含
数据库名，认证仍由临时、权限为 0600 的 `PGPASSFILE` 提供，不会进入命令参数或
日志。恢复同时使用 `--single-transaction`，并拒绝符号链接形式的备份目录，避免
部分数据库恢复或以备份目录名跳出受控根路径。

## 可重复命令

```bash
make docker-init
make docker-config
make docker-up
make docker-doctor
make docker-backup

# 破坏性：只在允许清空该 Compose 项目卷时执行。
RESTORE_BACKUP=backup-YYYYMMDDTHHMMSSZ make docker-restore

# 使用 fake Provider 的隔离部署演练。
RUN_DEPLOYMENT_DRILLS=1 make docker-drill
```

`make docker-drill` 通过容器内 PID 1 的 `SIGKILL` 触发非预期进程退出；不要用
`docker compose kill` 替代该步骤，因为 Compose 将其视为显式操作，不能代表
restart policy 的恢复路径。

## 前端容器接口

- 只构建并提供 `frontend/dist`；不提供独立前端宿主机端口。
- 浏览器经 `http://127.0.0.1:${APP_PORT}` 访问同源 REST 和 SSE：
  `/api/v1/*`、`/healthz`、`/readyz`、`/api/v1/jobs/stream`。
- Caddy 代理 SSE 时已禁用缓冲；客户端应按 OpenAPI 使用 `Last-Event-ID` 重连。
- 前端容器不得连接 PostgreSQL、挂载 `app_data`/备份目录、挂载 Docker secret，
  也不得声明 Anthropic 相关环境变量。

## 残余风险

- `legacy-import` 已实现只读 scan、事务性 import 和 reconciliation；真实归档的预切换演练仍有 7 条待人工处理的隔离记录。此风险详见 [旧数据导入与发布审查](./16-legacy-import-release-review.md)，未完成 resolution/隔离决策前不可执行最终切换。
- restore 有意只允许空的 `app_data` 和新数据库卷，避免覆盖现有文件；生产恢复前
  必须保留备份目录并确认目标项目名。
- 本轮仅使用 Fake Provider；真实 Anthropic smoke 仍需用户显式设置有效 secret
  并单独执行，默认 CI 和部署演练不依赖真实 Key。
