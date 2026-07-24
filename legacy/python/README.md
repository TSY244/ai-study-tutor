# AI 学习助教（归档 Python CLI/TUI）

> 此目录保留 Web 重构前的 Python 版本，用于行为基线、旧数据导入和必要回退；新开发请阅读仓库根目录 `docs/web-rebuild/`。

旧运行数据已迁至 `../../legacy-data/`。使用本目录的启动脚本时会自动设置以下默认路径，用户传入的环境变量优先：

```bash
export TUTOR_CONFIG=../../legacy-data/private/python-config.json
export TUTOR_TASKS_DIR=../../legacy-data/.tasks
export TUTOR_COURSE_PATH=../../legacy-data/course
./tutor.sh
```

私密配置不再位于本目录，也不会提交。需要创建本地配置时，从 `config.example.json` 新建私密文件并写入 `legacy-data/private/python-config.json`；不要把真实 Key 写入 Git。

[![Python](https://img.shields.io/badge/python-3.11+-blue)](https://www.python.org/)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

AI 驱动的全链路学习工具。支持 YouTube / B站 视频链接和本地文件导入，一键生成教师教纲 + 学生笔记，附带 AI 出题、终端答题、自动判分、错题归档、成绩统计、AI 总结测评。**导入 → 笔记 → 出题 → 考试 → 批改 → 错题复习，全自动闭环。**

## 安装

```bash
git clone <this-repo>
cd ai-study-tutor
uv sync --extra dev
```

需要 Python 3.11+，推荐使用 [uv](https://docs.astral.sh/uv/) 管理依赖。

## 配置

编辑 `config.json`，填入 API Key：

```json
{
  "agent_command": "codex exec --sandbox workspace-write --ask-for-approval never",
  "ai_config": {
    "default_provider": "deepseek",
    "providers": {
      "deepseek": {
        "api_key": "你的 API Key",
        "base_url": "https://api.deepseek.com",
        "model": "deepseek-v4-pro"
      }
    }
  }
}
```

`agent_command` 默认使用 Codex 非交互式执行。要临时切回原来的 Claude Code，可把它设为预设名：

```json
{
  "agent_command": "claude"
}
```

或通过环境变量：

```bash
export OPENAI_API_KEY="你的 Key"
export OPENAI_BASE_URL="https://api.deepseek.com"
export OPENAI_MODEL="deepseek-v4-pro"
```

## 快速开始

```bash
./tutor.sh          # 启动交互界面
```

### 交互菜单

```
主菜单                      科目菜单（进入后）
───────                    ─────────────────
  1  高数 (3 课时)            1  极限基础  📖📝📋
  N  新建科目                 2  导数      📖📝
  D  删除科目                 🎓 学生区
  0  退出                      VS  查看学生笔记
                               ES  进考场
                               SS  AI 评估总结
                              📖 教师区
                               VT  查看教纲
                               ET  管理考试
                               A   导入新课時
```

操作：输入 `1VS` 查看第1课学生笔记，`1ET` 管理第1课考试。

### CLI 模式

```bash
# 导入视频 → AI 生成笔记
python main.py --load-url "https://youtu.be/xxx" \
    --subject "高数" --lesson 1 --name "极限基础"

# 临时使用 Claude Code agent
python main.py --load-url "https://youtu.be/xxx" \
    --subject "高数" --lesson 1 --name "极限基础" --agent-command claude

# 查看笔记
python main.py --learn --subject "高数" --lesson 1

# 出题 + 答题
python main.py --exam-new --subject "高数" --lesson 1 --num 10
python main.py --exam --subject "高数" --lesson 1

# 错题管理
python main.py --wrong-list --subject "高数" --lesson 1
python main.py --fix-wrong --subject "高数" --lesson 1

# 查看成绩
python main.py --score --subject "高数" --lesson 1

# AI 评估总结
python main.py --summarize --subject "高数" --lesson 1 --input "极限就是..."

# 导出笔记
python main.py --export-md --subject "高数" --lesson 1
```

## 输出结构

```
course/
└── {科目}/
    ├── .subject              # 科目标记
    └── lesson_01/
        ├── info.json         # 课时元信息
        ├── teacher_notes/content.md   # 教师教纲
        ├── student_notes/content.md   # 学生笔记
        ├── exam/             # 试卷 & 答题记录
        └── export.md         # 合并导出
```

## 架构

```
./tutor.sh                     # 入口脚本
main.py                        # CLI 入口
src/
├── interactive.py             # 前端：多级菜单
├── course_service.py          # 后端：业务逻辑
├── storage.py                 # 文件存储
├── extraction_runner.py       # claude CLI 调用
├── exam_logic.py              # 考试 / 判分 / 错题
├── prompts.py                 # AI 提示词模板
├── models.py                  # 数据模型
├── config_loader.py           # 配置加载
├── memory.py                  # 薄弱点追踪
├── chunking.py                # 文本处理
├── summary_check.py           # AI 总结评估
├── ai_adapter/                # AI API 适配
└── material_parse/            # 素材解析
```

## 开发

```bash
uv run python -m pytest        # 运行测试
```
