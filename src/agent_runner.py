"""Claude Code Agent task delegation system.

Replaces both:
  - extraction_runner.py  (claude -p subprocess)
  - ai_adapter/claude_api.py  (OpenAI-compatible API calls)

All AI work is delegated to Claude Code Agent via structured JSON task files.
Python writes tasks → Claude Code Agent processes them → Python verifies results.

Task file format (.tasks/pending/<task_id>.json):
{
  "task_id": "task_20260605_120000_extract",
  "task_type": "extraction | quiz_gen | exam_review | fix_wrong | summarize | regenerate",
  "subject": "科目名",
  "lesson": 1,
  "name": "课时名",
  "prompt": "完整的执行提示词...",
  "output_dir": "/abs/path/to/lesson_01",
  "output_files": ["teacher_notes/content.md", ...],
  "output_format": "markdown | json",
  "model": "deepseek-v4-pro",
  "status": "pending | processing | completed | failed",
  "created_at": "ISO timestamp",
  "completed_at": null
}
"""

from __future__ import annotations

import json
import os
import re
import sys
import uuid
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

PROJECT_ROOT = Path(__file__).resolve().parent.parent
TASKS_DIR = PROJECT_ROOT / ".tasks"
PENDING_DIR = TASKS_DIR / "pending"
COMPLETED_DIR = TASKS_DIR / "completed"


# ---------------------------------------------------------------------------
# Task type constants
# ---------------------------------------------------------------------------

class TaskType:
    EXTRACTION = "extraction"       # Import lesson → generate teacher + student notes
    QUIZ_GEN = "quiz_gen"           # Generate exam questions
    EXAM_REVIEW = "exam_review"     # Mixed retest with old wrong questions
    FIX_WRONG = "fix_wrong"         # AI analysis of wrong answers
    SUMMARIZE = "summarize"         # Evaluate student self-summary
    REGENERATE = "regenerate"       # Re-generate notes from raw text
    CUSTOM = "custom"               # Generic one-shot prompt


# ---------------------------------------------------------------------------
# Task creation
# ---------------------------------------------------------------------------

def _ensure_dirs() -> None:
    PENDING_DIR.mkdir(parents=True, exist_ok=True)
    COMPLETED_DIR.mkdir(parents=True, exist_ok=True)


def _make_task_id(task_type: str) -> str:
    stamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    short = uuid.uuid4().hex[:6]
    return f"task_{stamp}_{task_type}_{short}"


def create_task(
    task_type: str,
    prompt: str,
    output_dir: Path | str,
    *,
    subject: str = "",
    lesson: int = 0,
    name: str = "",
    output_files: list[str] | None = None,
    output_format: str = "markdown",
    model: str = "",
    system_prompt: str = "",
    source_text: str = "",
    extra: dict[str, Any] | None = None,
) -> dict[str, Any]:
    """Create a task file for Claude Code Agent to process.

    Returns the task dict (also written to .tasks/pending/<task_id>.json).
    """
    _ensure_dirs()

    task_id = _make_task_id(task_type)
    task: dict[str, Any] = {
        "task_id": task_id,
        "task_type": task_type,
        "subject": subject,
        "lesson": lesson,
        "name": name,
        "prompt": prompt,
        "system_prompt": system_prompt,
        "source_text": source_text,
        "output_dir": str(Path(output_dir).resolve()),
        "output_files": output_files or [],
        "output_format": output_format,
        "model": model,
        "status": "pending",
        "created_at": datetime.now(timezone.utc).isoformat(),
        "completed_at": None,
        "error": None,
        "extra": extra or {},
    }

    task_path = PENDING_DIR / f"{task_id}.json"
    task_path.write_text(
        json.dumps(task, ensure_ascii=False, indent=2),
        encoding="utf-8",
    )
    return task


# ---------------------------------------------------------------------------
# Task queries
# ---------------------------------------------------------------------------

def list_pending_tasks() -> list[dict[str, Any]]:
    """Return all pending tasks, oldest first."""
    _ensure_dirs()
    tasks: list[dict[str, Any]] = []
    for f in sorted(PENDING_DIR.glob("*.json")):
        try:
            tasks.append(json.loads(f.read_text(encoding="utf-8")))
        except (json.JSONDecodeError, OSError):
            pass
    return tasks


def list_completed_tasks() -> list[dict[str, Any]]:
    """Return all completed tasks, most recent first."""
    _ensure_dirs()
    tasks: list[dict[str, Any]] = []
    for f in sorted(COMPLETED_DIR.glob("*.json"), reverse=True):
        try:
            tasks.append(json.loads(f.read_text(encoding="utf-8")))
        except (json.JSONDecodeError, OSError):
            pass
    return tasks


def get_pending_count() -> int:
    _ensure_dirs()
    return len(list(PENDING_DIR.glob("*.json")))


def mark_completed(task_id: str, error: str | None = None) -> None:
    """Move a task from pending to completed."""
    task_path = PENDING_DIR / f"{task_id}.json"
    if not task_path.exists():
        return
    task = json.loads(task_path.read_text(encoding="utf-8"))
    task["status"] = "failed" if error else "completed"
    task["completed_at"] = datetime.now(timezone.utc).isoformat()
    task["error"] = error
    completed_path = COMPLETED_DIR / f"{task_id}.json"
    completed_path.parent.mkdir(parents=True, exist_ok=True)
    completed_path.write_text(
        json.dumps(task, ensure_ascii=False, indent=2),
        encoding="utf-8",
    )
    task_path.unlink()


# ---------------------------------------------------------------------------
# Convenience: build full extraction prompts
# ---------------------------------------------------------------------------

def build_extraction_prompt(
    source_description: str,
    subject: str,
    lesson: int,
    name: str,
    output_dir: Path | str | None = None,
) -> str:
    """Build the extraction prompt for lesson import.

    This is the same template previously in extraction_runner.py, now
    adapted for Claude Code Agent.

    If output_dir is None, uses /output/ as placeholder (backward compatible).
    """
    if output_dir is not None:
        out = str(Path(output_dir).resolve())
    else:
        out = "/output"
    teacher_dir = f"{out}/teacher_notes"
    student_dir = f"{out}/student_notes"

    return f"""你需要完成以下知识提取任务。

## 任务信息
- 科目：{subject}
- 课时编号：{lesson}
- 课时名称：{name}

## 素材获取
{source_description}

## 执行步骤

### 步骤 1：获取素材文本
如果是 YouTube/B站 URL：
- 使用 yt-dlp 下载字幕（中文字幕优先）
- 命令：yt-dlp --write-subs --write-auto-subs --sub-langs "zh-Hans,zh,en,zh-CN" --skip-download --convert-subs srt -o "{out}/raw_src" "<URL>"
- 如果没有字幕，使用 WebFetch + WebSearch 获取视频信息和第三方分析

如果是本地文件：直接 Read 读取内容。
如果是纯文本：Read {out}/raw/raw_text.md。

### 步骤 2：生成教师教纲（teacher_notes）
整理全部文本内容，生成教师教纲（Markdown），用 Write 写入 {teacher_dir}/content.md

教纲结构：
# 教师教纲：{name}
## 课程信息
## 教学目标（3-5条具体目标）
## 知识点体系（层级列表，标注重难点）
## 教学流程（导入→核心讲解→练习→小结，每段标注时长）
## 板书设计
## 课后作业建议
## 教学反思要点

### 步骤 3：生成学生学习笔记（student_notes）
基于教纲生成学生笔记（Markdown），用 Write 写入 {student_dir}/content.md

笔记结构：
# {name} — 学习笔记
## 本课目标
## 前置知识
## 核心知识点（定义、原理、举例、易错点）
## 重点标注（> ⚠️ 标注）
## 常见误区
## 记忆技巧（口诀、类比、图表）
## 自测清单（- [ ] ...）
## 课后练习建议

### 步骤 4：生成课程元信息
用 Write 写入 {out}/info.json：
{{"subject": "{subject}", "lesson": {lesson}, "name": "{name}"}}

## 重要规则
- 所有文件用 Write 工具写入指定路径
- 确保目录存在（用 Bash mkdir -p）
- 内容只基于素材，不编造
- 中文表达清晰，适合初学者
- 完成后报告 EXTRACTION_COMPLETE"""


def build_quiz_gen_prompt(
    subject: str,
    lesson: int,
    lesson_content: str,
    question_count: int = 10,
    weaknesses: list[dict[str, Any]] | None = None,
) -> str:
    """Build prompt for quiz generation task."""
    weaknesses_str = json.dumps(weaknesses or [], ensure_ascii=False, indent=2)
    schema = {
        "quiz_title": "章节测试标题",
        "questions": [
            {
                "id": "q1",
                "type": "single_choice",
                "question": "题干",
                "options": {"A": "选项A", "B": "选项B", "C": "选项C", "D": "选项D"},
                "correct_answer": "A",
                "explanation": "解析",
                "tested_concepts": ["概念"],
                "difficulty": "easy",
                "score": 5,
            }
        ],
    }
    return f"""请生成 {question_count} 道章节考试题。

题型需覆盖 single_choice、multiple_choice、true_false、short_answer、case_analysis。
每题必须有 tested_concepts 和 score。

必须输出严格 JSON（不要 Markdown 代码块，不要额外解释）。

JSON Schema：
{json.dumps(schema, ensure_ascii=False, indent=2)}

历史薄弱点：
{weaknesses_str}

课程内容：
{lesson_content[:24000]}"""


def build_summarize_prompt(
    subject: str,
    lesson: int,
    student_input: str,
    course_content: str,
) -> str:
    """Build prompt for summary evaluation task."""
    schema = {
        "accuracy_score": 85,
        "completeness_score": 70,
        "overall_assessment": "总体评价",
        "key_misunderstandings": ["理解偏差点"],
        "missing_concepts": ["遗漏的重要概念"],
        "strengths": ["理解较好的部分"],
        "suggestions": ["改进建议"],
    }
    return f"""请评估学生对以下学习内容的自我总结。比对原始资料，评估理解准确度和完整度。

科目：{subject}

原始资料：
{course_content[:12000]}

学生总结：
{student_input}

请按以下 JSON schema 输出（必须严格 JSON，不要 Markdown 代码块）：
{json.dumps(schema, ensure_ascii=False, indent=2)}"""


def build_fix_wrong_prompt(
    wrong_question: dict[str, Any],
    course_content: str,
) -> str:
    """Build prompt for wrong-question analysis task."""
    return f"""请分析以下错题，给出：
1. 错因分析
2. 正确解题思路
3. 相关知识点讲解
4. 一道类似练习题

课程内容参考：
{course_content[:8000]}

错题信息：
{json.dumps(wrong_question, ensure_ascii=False, indent=2)}

输出 Markdown 格式。"""


# ---------------------------------------------------------------------------
# High-level convenience functions (used by course_service and main.py)
# ---------------------------------------------------------------------------

def queue_extraction_task(
    source_desc: str,
    subject: str,
    lesson: int,
    name: str,
    output_dir: Path,
    model: str = "",
    source_text: str = "",
) -> str:
    """Queue a lesson extraction task. Returns task_id."""
    prompt = build_extraction_prompt(source_desc, subject, lesson, name, output_dir)
    task = create_task(
        task_type=TaskType.EXTRACTION,
        prompt=prompt,
        output_dir=output_dir,
        subject=subject,
        lesson=lesson,
        name=name,
        output_files=["teacher_notes/content.md", "student_notes/content.md", "info.json"],
        output_format="markdown",
        model=model,
        source_text=source_text,
    )
    return task["task_id"]


def queue_quiz_gen_task(
    subject: str,
    lesson: int,
    lesson_content: str,
    output_dir: Path,
    question_count: int = 10,
    weaknesses: list[dict[str, Any]] | None = None,
    model: str = "",
    retest: bool = False,
) -> str:
    """Queue a quiz generation task. Returns task_id."""
    prompt = build_quiz_gen_prompt(subject, lesson, lesson_content, question_count, weaknesses)
    task = create_task(
        task_type=TaskType.EXAM_REVIEW if retest else TaskType.QUIZ_GEN,
        prompt=prompt,
        output_dir=output_dir,
        subject=subject,
        lesson=lesson,
        output_files=["quiz_result.json"],
        output_format="json",
        model=model,
        system_prompt="你必须返回严格 JSON，不要 Markdown 代码块，不要额外解释。只能基于用户提供的学习资料生成内容。",
    )
    return task["task_id"]


def queue_summarize_task(
    subject: str,
    lesson: int,
    student_input: str,
    course_content: str,
    output_dir: Path,
    model: str = "",
) -> str:
    """Queue a summary evaluation task. Returns task_id."""
    prompt = build_summarize_prompt(subject, lesson, student_input, course_content)
    task = create_task(
        task_type=TaskType.SUMMARIZE,
        prompt=prompt,
        output_dir=output_dir,
        subject=subject,
        lesson=lesson,
        output_files=["summary_result.json"],
        output_format="json",
        model=model,
        system_prompt="你必须返回严格 JSON，不要 Markdown 代码块，不要额外解释。",
    )
    return task["task_id"]


def queue_fix_wrong_task(
    subject: str,
    lesson: int,
    wrong_question: dict[str, Any],
    course_content: str,
    output_dir: Path,
    model: str = "",
) -> str:
    """Queue a wrong-question analysis task. Returns task_id."""
    prompt = build_fix_wrong_prompt(wrong_question, course_content)
    task = create_task(
        task_type=TaskType.FIX_WRONG,
        prompt=prompt,
        output_dir=output_dir,
        subject=subject,
        lesson=lesson,
        output_files=["fix_analysis.md"],
        output_format="markdown",
        model=model,
    )
    return task["task_id"]


def queue_regenerate_task(
    subject: str,
    lesson: int,
    name: str,
    source_text: str,
    output_dir: Path,
    model: str = "",
) -> str:
    """Queue a note regeneration task. Returns task_id."""
    prompt = build_extraction_prompt(
        f"从以下已有文本重新生成笔记：\n\n{source_text[:20000]}",
        subject, lesson, name, output_dir,
    )
    task = create_task(
        task_type=TaskType.REGENERATE,
        prompt=prompt,
        output_dir=output_dir,
        subject=subject,
        lesson=lesson,
        name=name,
        output_files=["teacher_notes/content.md", "student_notes/content.md"],
        output_format="markdown",
        model=model,
    )
    return task["task_id"]


# ---------------------------------------------------------------------------
# Output verification (called after task completion)
# ---------------------------------------------------------------------------

def verify_extraction_output(output_dir: Path) -> dict[str, bool]:
    """Check that extraction output files exist."""
    return {
        "teacher_notes": (output_dir / "teacher_notes" / "content.md").exists(),
        "student_notes": (output_dir / "student_notes" / "content.md").exists(),
        "info_json": (output_dir / "info.json").exists(),
    }


def normalize_output(output_dir: Path) -> None:
    """If Claude Code Agent wrote to output/ subdir, move files up."""
    nested = output_dir / "output"
    if not nested.exists() or not nested.is_dir():
        return
    import shutil
    for item in nested.iterdir():
        dest = output_dir / item.name
        if dest.exists():
            if dest.is_dir():
                shutil.rmtree(dest)
            else:
                dest.unlink()
        shutil.move(str(item), str(dest))
    try:
        nested.rmdir()
    except OSError:
        pass


# ---------------------------------------------------------------------------
# Task processing guide (printed for Claude Code Agent)
# ---------------------------------------------------------------------------

TASK_PROCESSING_GUIDE = """
╔══════════════════════════════════════════════════════════════╗
║  📋 待处理任务                                               ║
║                                                              ║
║  运行以下命令查看和处理任务：                                ║
║    python process_tasks.py --list                            ║
║    python process_tasks.py --process <task_id>               ║
║    python process_tasks.py --process-all                     ║
║                                                              ║
║  或在 Claude Code 中说：                                     ║
║    "处理所有待处理的学习任务"                                ║
╚══════════════════════════════════════════════════════════════╝
"""


def print_pending_summary() -> None:
    """Print a summary of pending tasks to stdout."""
    pending = list_pending_tasks()
    if not pending:
        print("\n✅ 没有待处理的任务。")
        return
    print(f"\n📋 待处理任务：{len(pending)} 个\n")
    for i, task in enumerate(pending, 1):
        ttype = task.get("task_type", "?")
        subject = task.get("subject", "")
        name = task.get("name", "")
        task_id = task["task_id"]
        print(f"  {i}. [{ttype}] {subject} · {name}")
        print(f"     ID: {task_id}")
        print(f"     输出: {task.get('output_dir', '')}")
    print()
    print("  在 Claude Code 中说「处理所有待处理的学习任务」即可开始。")
