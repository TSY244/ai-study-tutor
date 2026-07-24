"""Knowledge extraction via Claude Code Agent (backward-compatible re-exports).

This module now delegates to src.agent_runner instead of calling claude CLI directly.
Kept for backward compatibility — existing imports continue to work.
"""

from __future__ import annotations

import sys
from pathlib import Path

# Re-export from agent_runner
from .agent_runner import (
    build_extraction_prompt,
    verify_extraction_output,
    normalize_output,
    queue_extraction_task,
)

# Legacy function name alias
verify_output = verify_extraction_output


class ExtractionError(RuntimeError):
    """Raised when extraction fails."""


def run_extraction(
    prompt: str,
    output_dir: Path,
    model: str,
    api_key: str,
    base_url: str | None = None,
    timeout: int = 900,
) -> None:
    """Queue an extraction task for Claude Code Agent.

    This no longer runs claude CLI directly. Instead it writes a task file
    that Claude Code Agent will process.

    Args:
        prompt: Full prompt for Claude Code Agent.
        output_dir: Directory for output files.
        model: Model name (required).
        api_key: Not used (agent uses its own auth).
        base_url: Not used (agent uses its own config).
        timeout: Not used (agent processing is async).
    """
    if not model:
        raise ExtractionError("必须指定 --model 参数。")

    output_dir = Path(output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)
    (output_dir / "teacher_notes").mkdir(parents=True, exist_ok=True)
    (output_dir / "student_notes").mkdir(parents=True, exist_ok=True)

    # Extract subject/lesson/name from the prompt
    import re
    subject_match = re.search(r"科目[：:]\s*(.+)", prompt)
    lesson_match = re.search(r"课时编号[：:]\s*(\d+)", prompt)
    name_match = re.search(r"课时名称[：:]\s*(.+)", prompt)

    subject = subject_match.group(1).strip() if subject_match else ""
    lesson = int(lesson_match.group(1)) if lesson_match else 0
    name = name_match.group(1).strip() if name_match else ""

    task_id = queue_extraction_task(
        source_desc=prompt[:500],
        subject=subject,
        lesson=lesson,
        name=name,
        output_dir=output_dir,
        model=model,
    )

    print(f"\n✅ 提取任务已排队：{task_id}")
    print(f"   输出目录：{output_dir}")
    print()
    print("   ╔══════════════════════════════════════════════════════╗")
    print("   ║  📋 请在 Claude Code 中说：                         ║")
    print("   ║     「处理所有待处理的学习任务」                    ║")
    print("   ╚══════════════════════════════════════════════════════╝")
