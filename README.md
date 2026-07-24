# AI Study Tutor Web Rebuild

本仓库正在从 Python CLI/TUI 迁移到单用户、本机部署的 Web 应用：React + TypeScript + Vite 前端，Go + Gin API，PostgreSQL Job Queue 与独立 Worker，Docker Compose 部署。

当前可执行的重构设计、接口和部署蓝图位于 [docs/web-rebuild/README.md](./docs/web-rebuild/README.md)。前端实施可直接使用 [Codex 提示词](./docs/web-rebuild/08-frontend-codex-prompt.md)。

## 目录

```text
docs/web-rebuild/  Web 重构产品、前后端、OpenAPI、Docker 与迁移文档
legacy/python/     归档的旧 Python CLI/TUI 源码与测试
legacy-data/       旧课程、任务、工作区和本地私密配置的导入源（忽略）
```

旧系统仅用于行为基线和 `legacy-import`。如需运行归档版本，请进入 `legacy/python/` 并阅读其中的 README；不要将 API Key 或个人学习数据提交到仓库。

## 当前状态

Go API/Worker、React Web、发布 OpenAPI 与本机 Docker Compose 拓扑均已实现。首次启动：

```bash
make docker-up
make docker-seed
make docker-smoke
```

浏览器访问 `http://127.0.0.1:8080`。未配置 Claude Key 时系统以降级模式启动；AI 功能需要把 Key 写入 `deploy/secrets/anthropic_api_key.txt`，该 secret 只挂载到 Worker。旧数据默认由 `legacy-data/` 提供给 Importer，运行时不做 Python/Go 双写。
