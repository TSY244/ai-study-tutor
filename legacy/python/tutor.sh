#!/usr/bin/env bash
# AI 学习助教 — 启动脚本
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

# 归档后的旧数据位于仓库根目录 legacy-data/；用户显式传入的值优先。
export TUTOR_CONFIG="${TUTOR_CONFIG:-$SCRIPT_DIR/../../legacy-data/private/python-config.json}"
export TUTOR_TASKS_DIR="${TUTOR_TASKS_DIR:-$SCRIPT_DIR/../../legacy-data/.tasks}"
export TUTOR_COURSE_PATH="${TUTOR_COURSE_PATH:-$SCRIPT_DIR/../../legacy-data/course}"

# 如果有参数且以 -- 开头，透传给 python main.py（兼容旧用法）
if [[ "${1:-}" == --* ]]; then
    exec uv run python main.py "$@"
fi

# 否则启动 Python 交互界面
exec uv run python -m src.interactive "$@"
