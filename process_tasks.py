#!/usr/bin/env python3
"""Claude Code Agent task processor.

Lists and manages pending AI tasks from .tasks/pending/.
The actual processing is done by Claude Code Agent (the current session).

Usage:
  python process_tasks.py --list           # List all pending tasks
  python process_tasks.py --task <id>      # Show task details (for Claude Code to read)
  python process_tasks.py --clean          # Clear all completed tasks
  python process_tasks.py --retry <id>     # Move a failed task back to pending
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

PROJECT_ROOT = Path(__file__).resolve().parent
sys.path.insert(0, str(PROJECT_ROOT))

from src.agent_runner import (
    list_pending_tasks,
    list_completed_tasks,
    mark_completed,
    get_pending_count,
    PENDING_DIR,
    COMPLETED_DIR,
    TASKS_DIR,
)


def cmd_list(args):
    """List all pending and recent completed tasks."""
    pending = list_pending_tasks()
    completed = list_completed_tasks()

    if not pending and not completed:
        print("✅ 没有任务记录。")
        return

    if pending:
        print(f"\n📋 待处理任务：{len(pending)} 个\n")
        for i, task in enumerate(pending, 1):
            ttype = task.get("task_type", "?")
            subject = task.get("subject", "")
            name = task.get("name", "")
            task_id = task["task_id"]
            created = task.get("created_at", "")[:19]
            print(f"  {i}. [{ttype}] {subject} · {name}")
            print(f"     ID: {task_id}")
            print(f"     创建: {created}")
            print(f"     输出: {task.get('output_dir', '')}")
            print()

        print("  → 在 Claude Code 中说「处理所有待处理的学习任务」即可开始。\n")

    if completed:
        print(f"✅ 最近完成的任务：{len(completed)} 个\n")
        for task in completed[:5]:
            ttype = task.get("task_type", "?")
            subject = task.get("subject", "")
            name = task.get("name", "")
            status = task.get("status", "?")
            err = task.get("error")
            print(f"  [{status}] [{ttype}] {subject} · {name}")
            if err:
                print(f"         错误: {err}")
        print()


def cmd_task(args):
    """Print full task details for Claude Code Agent to read and execute."""
    pending = list_pending_tasks()
    target = None
    for task in pending:
        if task["task_id"] == args.task or task["task_id"].startswith(args.task):
            target = task
            break

    if not target:
        # Check completed
        for task in list_completed_tasks():
            if task["task_id"] == args.task or task["task_id"].startswith(args.task):
                target = task
                break

    if not target:
        print(f"未找到任务：{args.task}")
        sys.exit(1)

    import json
    print(json.dumps(target, ensure_ascii=False, indent=2))


def cmd_clean(args):
    """Remove all completed tasks."""
    import shutil
    count = 0
    if COMPLETED_DIR.exists():
        for f in COMPLETED_DIR.glob("*.json"):
            f.unlink()
            count += 1
    print(f"✅ 已清理 {count} 个已完成任务。")


def cmd_retry(args):
    """Move a failed task back to pending."""
    completed = list_completed_tasks()
    target = None
    for task in completed:
        if task["task_id"] == args.retry or task["task_id"].startswith(args.retry):
            target = task
            break

    if not target:
        print(f"未在已完成任务中找到：{args.retry}")
        sys.exit(1)

    import json
    target["status"] = "pending"
    target["error"] = None
    pending_path = PENDING_DIR / f"{target['task_id']}.json"
    pending_path.write_text(json.dumps(target, ensure_ascii=False, indent=2), encoding="utf-8")

    completed_path = COMPLETED_DIR / f"{target['task_id']}.json"
    if completed_path.exists():
        completed_path.unlink()

    print(f"✅ 已重新排队：{target['task_id']}")


def main():
    parser = argparse.ArgumentParser(
        description="Claude Code Agent 任务管理器",
    )
    parser.add_argument("--list", action="store_true", help="列出所有任务")
    parser.add_argument("--task", type=str, help="查看任务详情")
    parser.add_argument("--clean", action="store_true", help="清理已完成任务")
    parser.add_argument("--retry", type=str, help="重试失败的任务")

    args = parser.parse_args()

    if args.list:
        cmd_list(args)
    elif args.task:
        cmd_task(args)
    elif args.clean:
        cmd_clean(args)
    elif args.retry:
        cmd_retry(args)
    else:
        # Default: list pending
        cmd_list(args)


if __name__ == "__main__":
    main()
