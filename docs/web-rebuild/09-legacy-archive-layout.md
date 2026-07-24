# 旧版 Python 归档布局

## 1. 目的

本次目录调整将当前 Python CLI/TUI 实现与新 Web 重构根目录分离。归档不是删除：旧版代码、测试和用户学习数据继续保留，供行为基线、legacy-import、回归核对和必要回退使用。

## 2. 目标布局

```text
ai-study-tutor/
├── docs/web-rebuild/            # Web 重构的唯一设计与交接文档
├── legacy/
│   └── python/                  # 旧 Python CLI/TUI 源码、测试与依赖定义
│       ├── main.py
│       ├── process_tasks.py
│       ├── tutor.sh
│       ├── Dockerfile
│       ├── .env.example
│       ├── .python-version
│       ├── src/
│       ├── tests/
│       ├── tools/
│       ├── pyproject.toml
│       ├── requirements.txt
│       ├── uv.lock
│       ├── config.example.json
│       └── README.md
├── legacy-data/                 # 旧系统的本地数据导入源，不作为新运行时数据
│   ├── course/
│   ├── .tasks/
│   ├── workspace/
│   └── materials/
│   └── private/python-config.json  # 本地忽略，不提交
├── apps/web/                    # 未来 React 前端
├── api/                         # 未来发布 OpenAPI 契约
├── cmd/ internal/ db/ deploy/   # 未来 Go 与 Docker 实现
├── .gitignore
└── README.md                    # 新 Web 重构入口
```

## 3. 迁移规则

- `main.py`、`process_tasks.py`、`tutor.sh`、`Dockerfile`、`.env.example`、`.python-version`、`src/`、`tests/`、Python 打包/锁定文件及旧 README 统一迁入 `legacy/python/`（一次性 `tmp_validate.py` 已删除，不再归档）。
- `legacy/python/config.example.json` 必须由固定、安全模板新建，不能读取或复制 `config.json`。根 `config.json` 移入被忽略的 `legacy-data/private/python-config.json`，保留本机旧版可用性但不进入提交；旧版运行时显式设置 `TUTOR_CONFIG=../../legacy-data/private/python-config.json`。
- `course/`、`.tasks/`、`workspace/`、`materials/` 迁入 `legacy-data/`。它们是迁移输入而非新系统运行目录；其中可能包含个人学习材料或历史任务，后续默认不提交新增内容。
- `.pytest_cache/`、`__pycache__/`、`ai_study_tutor.egg-info/` 等缓存/构建产物删除，不归档。
- `.claude/` 和 `.superpowers/` 是本地 Agent 工具状态，不归档到旧产品；保留或由 `.gitignore` 忽略。
- 不移动 `.git/`，不重写历史提交，不删除任何旧业务文件。

## 4. 新根目录约束

- 根 `README.md` 只描述 Web 重构、文档入口和旧版归档位置，不再给出 Python CLI 安装/运行说明。
- 根 `.gitignore` 忽略 `legacy-data/` 的后续本地内容、Python 缓存、`.env`、secrets 和新运行时数据；已跟踪旧数据的移动仍保留历史可追溯性。
- `docs/web-rebuild/05-deployment-operations.md` 的 `LEGACY_SOURCE_DIR` 默认改为 `../legacy-data`，以 `/legacy/course` 和 `/legacy/.tasks` 同时挂载给 Importer；`07-migration-roadmap.md` 明确旧导入源位于 `legacy-data/`。
- 旧 Python 可在 `legacy/python/` 内按其 README 运行；归档前把 TUI preflight 改为检查 `TUTOR_CONFIG`（默认 `config.json`），并把任务目录改为接受 `TUTOR_TASKS_DIR`（默认 `.tasks`）。旧版 README 要求显式设置：`TUTOR_CONFIG=../../legacy-data/private/python-config.json`、`TUTOR_TASKS_DIR=../../legacy-data/.tasks`、`TUTOR_COURSE_PATH=../../legacy-data/course`。这些变量避免在归档代码中提交私密配置或创建指向私密数据的符号链接。

## 5. 验收

1. 根目录不再保留 Python CLI/TUI 源码、测试、依赖文件、旧配置或旧学习数据目录。
2. `legacy/python/` 包含完整旧版源码和测试，`legacy-data/` 包含原学习数据和任务目录。
3. 不存在 Python 缓存或打包生成物。
4. 所有 Web 重构文档链接仍有效，OpenAPI 与 Compose 文档默认旧数据路径一致。
5. `git status` 明确显示移动与删除，不夹带缓存或 secret。
6. 对暂存内容与最近 Git 历史执行 secret scan；任何命中必须停止提交、确认是否为真实凭据，并在必要时轮换/撤销。
