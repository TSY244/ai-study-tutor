# AI Study Tutor Web Rebuild

本仓库正在从 Python CLI/TUI 迁移到单用户、本机部署的 Web 应用：React + TypeScript + Vite 前端，Go + Gin API，PostgreSQL Job Queue 与独立 Worker，Docker Compose 部署。

当前可执行的重构设计、接口和部署蓝图位于 [docs/web-rebuild/README.md](./docs/web-rebuild/README.md)。前端实施可直接使用 [Codex 提示词](./docs/web-rebuild/08-frontend-codex-prompt.md)。

**部署**：全部组件以 Docker 容器运行，宿主机只需 Docker。详见 [docs/deployment.md](./docs/deployment.md)。

## 目录

```text
backend/           Go 后端（cmd/、internal/、testdata/、go.mod、后端 Dockerfile）
frontend/          React + TypeScript + Vite 前端
api/               前后端共享的 OpenAPI 发布契约与示例
deploy/            Docker Compose 拓扑与运维脚本
tools/             契约/文档校验脚本
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

## B 站视频导入

B 站链接使用 [bili-cli](https://github.com/kukemc/bili-cli)（`bili-agent==1.2.1`）提取完整字幕/已配置的语音转写、章节和 9 帧画面总览，再由 Claude 读取文字和图片生成画面要点与综合总结。完整文字和图文总结共同写入 `raw_text`，自动用于教师教纲、学生笔记及后续测验、辅导；支持链接中的 `?p=2` 分 P 参数。

在「设置 → B 站 Cookie」点击「扫码登录」，用哔哩哔哩 App 扫码并确认，系统自动保存登录状态；也可继续手动保存 Cookie。总结阶段还需配置支持图片输入的 Claude 网关模型。无字幕且未配置 ASR 时，会明确标注口述内容缺失，只依据采样画面总结；无法获取画面时任务失败，不会用页面简介冒充视频内容。已有素材需重新导入才能获得图文总结。

更新后执行 `docker compose -f deploy/compose.yaml --env-file deploy/.env up -d --build worker` 重建 Worker。其他站点沿用现有导入方式。

Worker 的链接提取和测验生成使用 **Pi SDK**；B 站先由 bili-cli 获取字幕与采样画面，再交给 Pi 总结并传入后续教纲、笔记和测验流程。设置页可将已保存的 Anthropic / OpenAI 兼容 Key 应用到 Pi，原有 Key 配置继续有效。详见 [部署说明](docs/deployment.md#pi-执行引擎)。

生成测验会先分析资料中的知识点，再围绕知识点出题。课程详情和课时测验页展示知识点解释、学习目标和可核对的原文依据；每道题显示考查知识点。点击「生成测验」可选择常规/复习测验、题量，并填写最多 3000 字的出题备注，指定覆盖重点、题型比例和难度偏好；备注随试卷保存。发布时校验知识点依据、题目关联、题量、重复题干和解析，无特别重点时检查核心知识点覆盖。知识点和试卷在同一事务发布，历史试卷保留当时的知识点快照。

课程及其子资源的新增、修改和删除接口，见 [课程内容管理 API](docs/course-content-api.md)。
