"""Claude Code Agent task delegation system.

Replaces both:
  - extraction_runner.py  (claude -p subprocess)
  - ai_adapter/claude_api.py  (OpenAI-compatible API calls)

All AI work is delegated to Claude Code Agent via structured JSON task files.
Python writes tasks → Claude Code Agent processes them → Python verifies results.

Task file format ($TUTOR_TASKS_DIR/pending/<task_id>.json, default .tasks/):
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
import shlex
import sys
import uuid
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

from .config_loader import DEFAULT_AGENT_COMMAND, get_config, resolve_agent_command

PROJECT_ROOT = Path(__file__).resolve().parent.parent
TASKS_DIR = Path(os.getenv("TUTOR_TASKS_DIR", str(PROJECT_ROOT / ".tasks")))
PENDING_DIR = TASKS_DIR / "pending"
COMPLETED_DIR = TASKS_DIR / "completed"


def _configured_agent_command() -> str:
    try:
        return get_config().agent_command
    except Exception:
        return DEFAULT_AGENT_COMMAND


def build_agent_command(instruction: str, agent_command: str | None = None) -> list[str]:
    command = resolve_agent_command(agent_command or _configured_agent_command())
    try:
        parts = shlex.split(command)
    except ValueError as exc:
        raise ValueError(f"AI 命令配置无效：{exc}") from exc
    if not parts:
        raise ValueError("AI 命令配置不能为空。")
    return [*parts, instruction]


def _task_agent_command(task: dict[str, Any], override: str | None = None) -> str:
    if override and override.strip():
        return override.strip()
    command = task.get("command") or task.get("agent_command") or ""
    return command.strip() if isinstance(command, str) else ""


def _shared_agent_command(tasks: list[dict[str, Any]], override: str | None = None) -> str:
    if override and override.strip():
        return override.strip()
    commands = {
        command
        for task in tasks
        if (command := _task_agent_command(task))
    }
    if len(commands) == 1:
        return commands.pop()
    return ""


# ---------------------------------------------------------------------------
# Task type constants
# ---------------------------------------------------------------------------

class TaskType:
    EXTRACTION = "extraction"       # Import lesson → generate teacher + student notes
    QUIZ_GEN = "quiz_gen"           # Generate exam questions
    EXAM_REVIEW = "exam_review"     # Mixed retest with old wrong questions
    GRADE_ATTEMPT = "grade_attempt" # Grade a saved student attempt
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
    command: str = "",
    system_prompt: str = "",
    source_text: str = "",
    extra: dict[str, Any] | None = None,
) -> dict[str, Any]:
    """Create a task file for Claude Code Agent to process.

    Returns the task dict (also written to $TUTOR_TASKS_DIR/pending/<task_id>.json).
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
        "command": command,
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
    question_count: int = 8,
    weaknesses: list[dict[str, Any]] | None = None,
    *,
    cognitive_distribution: dict[str, int] | None = None,
    difficulty_distribution: dict[str, int] | None = None,
    mastery: float = 0.0,
    consecutive_stats: dict[str, int] | None = None,
    retest: bool = False,
) -> str:
    """Build prompt for quiz generation task.

    Includes the full question-generation policy from question_policy.py.
    """
    from .models import CognitiveLevel
    from .question_policy import build_policy_context, determine_difficulty_distribution

    # ── Resolve difficulty distribution ──
    diff_dist = difficulty_distribution
    if diff_dist is None:
        diff_dist = determine_difficulty_distribution(question_count)
    diff_sum = sum(diff_dist.values())
    if diff_sum != question_count:
        diff_dist = determine_difficulty_distribution(question_count)

    # ── Resolve cognitive distribution ──
    level_dist: dict[CognitiveLevel, int] = {}
    if cognitive_distribution:
        for k, v in cognitive_distribution.items():
            try:
                level_dist[CognitiveLevel(k)] = v
            except ValueError:
                pass

    # ── Build policy text ──
    policy_text = build_policy_context(
        cognitive_dist=level_dist,
        difficulty_dist=diff_dist,
        question_count=question_count,
        mastery=mastery,
        weaknesses=weaknesses,
    )

    # ── Stats text ──
    stats_text = ""
    if consecutive_stats:
        cc = consecutive_stats.get("consecutive_correct", 0)
        ce = consecutive_stats.get("consecutive_wrong", 0)
        stats_text = f"\n上次答题统计：连续正确 {cc} 题，连续错误 {ce} 题。\n"

    # ── Schema ──
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
                "cognitive_level": "recall",
            }
        ],
    }

    mode = "二次考试（retest）" if retest else "章节测试"
    retest_note = (
        "\n特殊要求：不重复原题，换角度考察同一概念，重点围绕历史薄弱点，增加案例题和解释题。"
        if retest else ""
    )

    weaknesses_str = json.dumps(weaknesses or [], ensure_ascii=False, indent=2)

    return f"""请严格按照下面的题目生成策略，生成 {question_count} 道{mode}题。
{retest_note}

要求：
1. 认知层次分布严格按策略表执行（每道题标注 cognitive_level）
2. 难度分布严格按策略表执行，顺序递增
3. 题型覆盖 single_choice、multiple_choice、true_false、short_answer、case_analysis
4. 每题必须有 tested_concepts、score、difficulty、cognitive_level
5. 不直接复制资料原文

必须输出严格 JSON（不要 Markdown 代码块，不要额外解释）。

{policy_text}

JSON Schema：
{json.dumps(schema, ensure_ascii=False, indent=2)}

历史薄弱点：
{weaknesses_str}
{stats_text}
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


def build_grade_attempt_prompt(
    subject: str,
    lesson: int,
    quiz_id: str,
    quiz_path: Path,
    answers_path: Path,
    teacher_notes_path: Path,
    student_notes_path: Path,
    output_path: Path,
) -> str:
    """Build prompt for grading a completed attempt task."""
    schema = {
        "total_score": 0,
        "question_feedback": [
            {
                "question_id": "q1",
                "score": 0,
                "is_correct": False,
                "feedback": "批阅意见",
                "correct_understanding": "正确理解",
                "weak_concepts": ["薄弱概念"],
            }
        ],
        "feedback": "整体点评",
        "weak_concepts": [
            {
                "concept": "概念",
                "weakness_description": "薄弱描述",
                "severity": 4,
                "evidence": "证据",
            }
        ],
        "review_plan": ["复习建议"],
        "retest_recommended": True,
    }
    return f"""你是严谨的学习测评批阅 agent。请基于课程资料批阅学生已完成的试卷。

科目：{subject}
课时：第{lesson}课
试卷 ID：{quiz_id}

必须读取并综合以下文件：
- 教师教纲：{teacher_notes_path}
- 学生笔记：{student_notes_path}
- 试卷 JSON：{quiz_path}
- 学生答题记录 JSON：{answers_path}

批阅要求：
1. 客观题按试卷 correct_answer 判定。
2. 主观题必须结合教师教纲、学生笔记、标准答案和题目解析判断正误。
3. 对每道题给出 score、is_correct、feedback、correct_understanding、weak_concepts。
4. 汇总 total_score、整体 feedback、weak_concepts、review_plan、retest_recommended。
5. 薄弱点必须有 concept、weakness_description、severity、evidence。
6. 只基于上述文件判断，不要编造课程资料外内容。

请将严格 JSON 写入下面路径，不要输出 Markdown 代码块：
{output_path}

JSON Schema：
{json.dumps(schema, ensure_ascii=False, indent=2)}
"""


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
    agent_command: str = "",
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
        command=agent_command,
        source_text=source_text,
    )
    return task["task_id"]


def queue_quiz_gen_task(
    subject: str,
    lesson: int,
    lesson_content: str,
    output_dir: Path,
    question_count: int = 8,
    weaknesses: list[dict[str, Any]] | None = None,
    model: str = "",
    agent_command: str = "",
    retest: bool = False,
    *,
    cognitive_distribution: dict[str, int] | None = None,
    difficulty_distribution: dict[str, int] | None = None,
    mastery: float = 0.0,
    consecutive_stats: dict[str, int] | None = None,
) -> str:
    """Queue a quiz generation task. Returns task_id."""
    prompt = build_quiz_gen_prompt(
        subject, lesson, lesson_content, question_count, weaknesses,
        cognitive_distribution=cognitive_distribution,
        difficulty_distribution=difficulty_distribution,
        mastery=mastery,
        consecutive_stats=consecutive_stats,
        retest=retest,
    )
    task = create_task(
        task_type=TaskType.EXAM_REVIEW if retest else TaskType.QUIZ_GEN,
        prompt=prompt,
        output_dir=output_dir,
        subject=subject,
        lesson=lesson,
        output_files=["quiz_result.json"],
        output_format="json",
        model=model,
        command=agent_command,
        system_prompt=(
            "你必须返回严格 JSON，不要 Markdown 代码块，不要额外解释。"
            "只能基于用户提供的学习资料生成内容。"
            "注意：JSON 中所有中文引号必须使用弯引号 “（U+201C）和 ”（U+201D），"
            "严禁在中文文本中使用 ASCII 直双引号，否则 JSON 会解析失败。"
        ),
    )
    return task["task_id"]


def queue_summarize_task(
    subject: str,
    lesson: int,
    student_input: str,
    course_content: str,
    output_dir: Path,
    model: str = "",
    agent_command: str = "",
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
        command=agent_command,
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
    agent_command: str = "",
    output_file: str = "fix_analysis.md",
) -> str:
    """Queue a wrong-question analysis task. Returns task_id."""
    prompt = build_fix_wrong_prompt(wrong_question, course_content)
    task = create_task(
        task_type=TaskType.FIX_WRONG,
        prompt=prompt,
        output_dir=output_dir,
        subject=subject,
        lesson=lesson,
        output_files=[output_file],
        output_format="markdown",
        model=model,
        command=agent_command,
    )
    return task["task_id"]


def queue_grade_attempt_task(
    subject: str,
    lesson: int,
    quiz_id: str,
    answers_path: Path | str,
    storage,
    model: str = "",
    agent_command: str = "",
) -> str:
    """Queue a completed attempt grading task. Returns task_id."""
    from .exam_logic import _find_quiz_path

    answers_path = Path(answers_path)
    exam_dir = storage.exam_dir(subject, lesson)
    stem = answers_path.stem
    timestamp = stem[: -len("_answers")] if stem.endswith("_answers") else stem
    grade_result_name = f"{timestamp}_grade_result.json"
    output_path = exam_dir / grade_result_name
    quiz_path = _find_quiz_path(storage, subject, quiz_id)
    prompt = build_grade_attempt_prompt(
        subject=subject,
        lesson=lesson,
        quiz_id=quiz_id,
        quiz_path=quiz_path,
        answers_path=answers_path,
        teacher_notes_path=storage.teacher_notes_path(subject, lesson),
        student_notes_path=storage.student_notes_path(subject, lesson),
        output_path=output_path,
    )
    task = create_task(
        task_type=TaskType.GRADE_ATTEMPT,
        prompt=prompt,
        output_dir=exam_dir,
        subject=subject,
        lesson=lesson,
        name=f"{subject} 第{lesson}课试卷检测",
        output_files=[grade_result_name],
        output_format="json",
        model=model,
        command=agent_command,
        system_prompt="你必须返回严格 JSON，不要 Markdown 代码块，不要额外解释。",
        extra={
            "quiz_id": quiz_id,
            "answers_path": str(answers_path),
            "grade_result_file": grade_result_name,
            "course_root": str(storage.course_root),
        },
    )
    return task["task_id"]


def queue_regenerate_task(
    subject: str,
    lesson: int,
    name: str,
    source_text: str,
    output_dir: Path,
    model: str = "",
    agent_command: str = "",
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
        command=agent_command,
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


# ---------------------------------------------------------------------------
# JSON repair — Claude Code sometimes uses ASCII " in Chinese text
# ---------------------------------------------------------------------------

def _repair_json_chinese_quotes(text: str) -> str:
    """Replace ASCII double-quotes used as Chinese quotation marks with curly quotes.

    Scans for "..." patterns where the content contains CJK characters and
    replaces the surrounding ASCII quotes with “ (U+201C) and ” (U+201D).
    """
    import re as _re

    result = list(text)
    i = 0
    while i < len(text):
        if text[i] != '"':
            i += 1
            continue

        # Skip JSON structural quotes (adjacent to JSON syntax chars)
        before = text[i - 1] if i > 0 else ' '
        after = text[i + 1] if i + 1 < len(text) else ' '

        if before in ':{}[] \t\r\n' or after in ':{}[], \t\r\n':
            i += 1
            continue

        # Find matching close quote
        j = i + 1
        while j < len(text) and text[j] != '"':
            j += 1

        if j >= len(text):
            i += 1
            continue

        inner = text[i + 1:j]
        has_cjk = bool(_re.search(r'[一-鿿　-〿＀-￯]', inner))

        if has_cjk:
            result[i] = '“'  # "
            result[j] = '”'  # "

        i = j + 1

    return ''.join(result)


def _repair_and_validate_json(path: str) -> bool:
    """Try to repair Chinese quotes in a JSON file. Returns True if valid after repair."""
    import json as _json

    p = Path(path)
    if not p.exists():
        return False

    raw = p.read_text(encoding="utf-8")
    try:
        _json.loads(raw)
        return True  # Already valid
    except _json.JSONDecodeError:
        pass

    # Try repairing
    repaired = _repair_json_chinese_quotes(raw)
    if repaired == raw:
        return False  # No changes made, still broken

    try:
        _json.loads(repaired)
        p.write_text(repaired, encoding="utf-8")
        return True
    except _json.JSONDecodeError:
        return False


# ---------------------------------------------------------------------------
# Automatic task processing via Claude Code CLI (no manual intervention)
# ---------------------------------------------------------------------------

def process_task_via_claude_cli(
    task_id: str,
    timeout: int = 300,
    agent_command: str | None = None,
) -> bool:
    """Process a pending task by invoking Claude Code CLI as a subprocess.

    This is the primary automated path — it calls `claude -p` to process the
    task file, showing progress to the user while waiting.  No manual
    "处理所有待处理的学习任务" step is needed.

    Returns True if the task was processed successfully.
    """
    import subprocess
    import threading
    import time

    task_path = PENDING_DIR / f"{task_id}.json"
    if not task_path.exists():
        print(f"   ⚠️  任务文件不存在：{task_path}")
        return False

    task = json.loads(task_path.read_text(encoding="utf-8"))
    subject = task.get("subject", "")
    lesson = task.get("lesson", 1)
    output_dir = task.get("output_dir", "")
    task_type = task.get("task_type", "")
    output_dir_path = Path(output_dir)

    # Build instruction for Claude Code
    if task_type == TaskType.GRADE_ATTEMPT:
        grade_result_file = task.get("extra", {}).get("grade_result_file")
        if not grade_result_file:
            grade_result_file = (task.get("output_files") or ["grade_result.json"])[0]
        grade_result_path = output_dir_path / grade_result_file
        instruction = (
            f"请处理任务文件 {task_path}。"
            f"读取该 JSON 文件，按照其中的 prompt 字段进行试卷检测和批阅。"
            f"必须基于 prompt 中列出的教师教纲、学生笔记、试卷 JSON、学生答题记录 JSON 来判断。"
            f"将严格 JSON 批阅结果保存到 {grade_result_path}。"
            f"不要输出 Markdown 代码块，不要只在 stdout 输出，必须写入文件。"
        )
        progress_title = "Claude Code 进行试卷检测"
        progress_verb = "试卷检测中"
    elif task_type == TaskType.FIX_WRONG:
        output_file = (task.get("output_files") or ["fix_analysis.md"])[0]
        analysis_path = output_dir_path / output_file
        instruction = (
            f"请处理任务文件 {task_path}。"
            f"读取该 JSON 文件，按照其中的 prompt 字段生成错题解析。"
            f"将 Markdown 解析结果保存到 {analysis_path}。"
            f"内容要包含错因分析、正确思路、知识点补强、下次避免方式。"
            f"不要只在 stdout 输出，必须写入文件。"
        )
        progress_title = "Claude Code 生成错题解析"
        progress_verb = "错题解析生成中"
    else:
        quiz_id = f"quiz_chapter_{lesson:02d}_{task_id[-6:]}"
        instruction = (
            f"请处理任务文件 {task_path}。"
            f"读取该 JSON 文件，按照其中的 prompt 字段生成试卷题目。"
            f"将生成的试卷 JSON 保存到 {output_dir}/{quiz_id}.json。"
            f"试卷 JSON 必须包含字段：\"id\": \"{quiz_id}\", \"course_id\": \"{subject}\", "
            f"\"chapter_id\": {lesson}, \"title\": \"{subject}·第{lesson}课 章节测试\"。"
            f"必须严格按照 prompt 中的策略（认知层次、难度分布、题型分布）执行。"
            f"输出严格 JSON，不使用 Markdown 代码块。"
            f"中文引号必须使用弯引号 “ 和 ”，严禁使用 ASCII 直双引号 \"。"
        )
        progress_title = "Claude Code 生成试卷"
        progress_verb = "生成中"

    try:
        command = build_agent_command(instruction, _task_agent_command(task, agent_command))
    except ValueError as exc:
        print(f"   ⚠️  {exc}")
        return False

    # Progress display
    print(f"\n   🤖 正在启动 {progress_title}...")
    print(f"   科目：{subject}  课时：第{lesson}课")
    print(f"   ⏳", end="", flush=True)

    stop_spinner = threading.Event()
    start_time = time.time()

    def spinner():
        chars = ["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"]
        i = 0
        while not stop_spinner.is_set():
            elapsed = int(time.time() - start_time)
            print(f"\r   ⏳ {chars[i % len(chars)]}  {progress_verb}...（{elapsed} 秒）", end="", flush=True)
            i += 1
            time.sleep(0.15)
        # Clear spinner line on stop
        elapsed = int(time.time() - start_time)
        print(f"\r   {' ' * 40}", end="\r")

    spinner_thread = threading.Thread(target=spinner, daemon=True)
    spinner_thread.start()

    try:
        result = subprocess.run(
            command,
            capture_output=True,
            text=True,
            timeout=timeout,
            cwd=str(PROJECT_ROOT),
            env={**dict(os.environ), "NO_COLOR": "1"},
        )
    except subprocess.TimeoutExpired:
        stop_spinner.set()
        spinner_thread.join(timeout=0.5)
        print(f"\r   ⚠️  生成超时（{timeout} 秒），任务仍在排队。")
        return False
    finally:
        stop_spinner.set()
        spinner_thread.join(timeout=0.5)

    elapsed = int(time.time() - start_time)
    print(f"\r   ⏱️  用时 {elapsed} 秒。", flush=True)

    if result.returncode != 0:
        print(f"   ⚠️  Claude Code 返回错误（exit code: {result.returncode}）")
        if result.stderr:
            print(f"   {result.stderr[:200]}")
        return False

    # Check if output was generated
    if task_type == TaskType.GRADE_ATTEMPT:
        grade_result_file = task.get("extra", {}).get("grade_result_file")
        if not grade_result_file:
            grade_result_file = (task.get("output_files") or ["grade_result.json"])[0]
        result_path = output_dir_path / grade_result_file
        if result_path.exists():
            repaired = _repair_and_validate_json(str(result_path))
            if not repaired:
                print("   ⚠️  试卷检测 JSON 格式有问题，请手动检查。")
                return False
            try:
                payload = json.loads(result_path.read_text(encoding="utf-8"))
                from .storage import Storage
                from .exam_logic import apply_grade_payload, mark_grading_failed
                extra = task.get("extra", {})
                storage = Storage(extra.get("course_root", "./course"))
                attempt = apply_grade_payload(
                    subject,
                    extra.get("quiz_id", ""),
                    extra.get("answers_path", ""),
                    payload,
                    storage,
                )
                feedback_md = output_dir_path / f"{attempt.id}_feedback.md"
                print(f"   ✅ 试卷检测完成，总分：{attempt.total_score}")
                print(f"   📄 点评与薄弱点：{feedback_md}")
                mark_completed(task_id)
                return True
            except Exception as e:
                try:
                    from .storage import Storage
                    from .exam_logic import mark_grading_failed
                    extra = task.get("extra", {})
                    mark_grading_failed(
                        Storage(extra.get("course_root", "./course")),
                        extra.get("answers_path", ""),
                        e,
                    )
                except Exception:
                    pass
                print(f"   ⚠️  试卷检测结果落库失败：{e}")
                return False

        print(f"   ⚠️  Claude Code 执行完成，但未找到试卷检测结果：{result_path}")
        print(f"   输出内容：{result.stdout[:300] if result.stdout else '(空)'}")
        return False

    if task_type == TaskType.FIX_WRONG:
        output_file = (task.get("output_files") or ["fix_analysis.md"])[0]
        analysis_path = output_dir_path / output_file
        if analysis_path.exists():
            print(f"   ✅ 错题解析已保存：{analysis_path}")
            mark_completed(task_id)
            return True

        print(f"   ⚠️  Claude Code 执行完成，但未找到错题解析：{analysis_path}")
        print(f"   输出内容：{result.stdout[:300] if result.stdout else '(空)'}")
        return False

    quiz_id = f"quiz_chapter_{lesson:02d}_{task_id[-6:]}"
    result_path = output_dir_path / f"{quiz_id}.json"

    # Also check quiz_result.json (fallback name)
    fallback_path = output_dir_path / "quiz_result.json"
    if not result_path.exists() and fallback_path.exists():
        # Rename fallback to proper name
        fallback_path.rename(result_path)
        print(f"   📄 已重命名 quiz_result.json → {quiz_id}.json")

    if result_path.exists():
        # Repair Chinese ASCII quotes if needed (common Claude Code issue)
        repaired = _repair_and_validate_json(str(result_path))
        if repaired:
            # Ensure the quiz has proper id field
            try:
                data = json.loads(result_path.read_text(encoding="utf-8"))
                if "id" not in data or data.get("id") != quiz_id:
                    data["id"] = quiz_id
                    data["course_id"] = data.get("course_id", subject)
                    data["chapter_id"] = data.get("chapter_id", lesson)
                    if "title" not in data:
                        data["title"] = f"{subject}·第{lesson}课 章节测试"
                    result_path.write_text(
                        json.dumps(data, ensure_ascii=False, indent=2),
                        encoding="utf-8",
                    )
                    print(f"   📝 已修正试卷元数据")
                q_count = len(data.get("questions", []))
                print(f"   ✅ 试卷已生成：{quiz_id}（{q_count} 题）")
            except Exception as e:
                print(f"   ⚠️  试卷元数据修正失败：{e}")
        else:
            print(f"   ⚠️  试卷 JSON 格式有问题，但文件已保存。请手动检查。")

        # Also repair any sibling quiz files
        for sibling in output_dir_path.glob(f"quiz_chapter_{lesson:02d}_*.json"):
            _repair_and_validate_json(str(sibling))

        mark_completed(task_id)
        return True

    print(f"   ⚠️  Claude Code 执行完成，但未找到输出文件。")
    print(f"   输出内容：{result.stdout[:300] if result.stdout else '(空)'}")
    return False


def process_all_pending_via_claude(
    timeout: int = 600,
    agent_command: str | None = None,
) -> tuple[int, int]:
    """Process ALL pending tasks by invoking Claude Code CLI.

    Used after queuing fix_wrong, summarize, regenerate, or extraction tasks.
    Returns (succeeded, failed).
    """
    import subprocess
    import threading
    import time

    pending = list_pending_tasks()
    if not pending:
        return (0, 0)

    task_list = "\n".join(
        f"  - {t['task_id']} [{t.get('task_type', '?')}] {t.get('subject', '')} 第{t.get('lesson', 0)}课"
        for t in pending
    )

    instruction = (
        f"请处理以下 {len(pending)} 个待处理任务。\n\n"
        f"{task_list}\n\n"
        f"任务文件在 {PENDING_DIR} 目录下。对每个任务：\n"
        f"1. 读取任务 JSON 文件\n"
        f"2. 按照其中的 prompt 和 system_prompt 执行\n"
        f"3. 将结果保存到 output_dir 指定的路径\n"
        f"4. 完成后用 Write 工具标记完成状态\n\n"
        f"注意：所有 JSON 输出中，中文引号必须使用弯引号 \" 和 \"，"
        f"严禁使用 ASCII 直双引号，否则 JSON 会解析失败。"
    )

    try:
        command = build_agent_command(instruction, _shared_agent_command(pending, agent_command))
    except ValueError as exc:
        print(f"   ⚠️  {exc}")
        return (0, len(pending))

    print(f"\n   🤖 正在调用 Claude Code 处理 {len(pending)} 个任务...")
    for t in pending:
        ttype = t.get("task_type", "?")
        subj = t.get("subject", "")
        ln = t.get("lesson", 0)
        print(f"      [{ttype}] {subj} 第{ln}课")
    print(f"   ⏳", end="", flush=True)

    # Spinner with elapsed time
    stop_spinner = threading.Event()
    start_time = time.time()

    def spinner():
        chars = ["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"]
        i = 0
        while not stop_spinner.is_set():
            elapsed = int(time.time() - start_time)
            print(f"\r   ⏳ {chars[i % len(chars)]}  处理中...（{elapsed} 秒）", end="", flush=True)
            i += 1
            time.sleep(0.15)
        elapsed = int(time.time() - start_time)
        print(f"\r   {' ' * 40}", end="\r")

    spinner_thread = threading.Thread(target=spinner, daemon=True)
    spinner_thread.start()

    try:
        result = subprocess.run(
            command,
            capture_output=True,
            text=True,
            timeout=timeout,
            cwd=str(PROJECT_ROOT),
            env={**dict(os.environ), "NO_COLOR": "1"},
        )
    except subprocess.TimeoutExpired:
        stop_spinner.set()
        spinner_thread.join(timeout=0.5)
        elapsed = int(time.time() - start_time)
        print(f"\r   ⚠️  处理超时（{elapsed} 秒），部分任务可能未完成。")
        return (0, len(pending))
    finally:
        stop_spinner.set()
        spinner_thread.join(timeout=0.5)

    elapsed = int(time.time() - start_time)
    print(f"\r   ⏱️  用时 {elapsed} 秒。", flush=True)

    # Check results and mark completed tasks
    succeeded = 0
    failed = 0
    still_pending = list_pending_tasks()

    for task in still_pending:
        ttype = task.get("task_type", "")
        output_dir = Path(task.get("output_dir", ""))
        output_files = task.get("output_files", [])

        # Check if output was generated
        any_output = False
        for of in output_files:
            if (output_dir / of).exists():
                any_output = True
                break

        # For quiz_gen, check quiz files
        if not any_output and ttype in ("quiz_gen", "exam_review"):
            lesson = task.get("lesson", 1)
            for candidate in output_dir.glob(f"quiz_chapter_{lesson:02d}_*.json"):
                if candidate.name != "quiz_result.json":
                    _repair_and_validate_json(str(candidate))
                    any_output = True

        # For fix_wrong, check markdown output
        if not any_output and ttype == "fix_wrong":
            if (output_dir / "fix_analysis.md").exists():
                any_output = True

        if any_output:
            mark_completed(task["task_id"])
            succeeded += 1
        else:
            failed += 1

    print(f"   ✅ 成功 {succeeded} 个，失败 {failed} 个")
    return (succeeded, failed)


# ---------------------------------------------------------------------------
# Backward-compatible task processing wrappers
# ---------------------------------------------------------------------------

def process_pending_tasks_now(
    storage=None,
    ai_provider: str | None = None,
    model: str | None = None,
) -> tuple[int, int]:
    """Process ALL pending tasks through Claude Code CLI agent."""
    return process_all_pending_via_claude()


def process_task_now(
    task_id_or_type: str,
    storage=None,
    ai_provider: str | None = None,
    model: str | None = None,
) -> bool:
    """Process a single pending task through Claude Code CLI agent.

    Parameters
    ----------
    task_id_or_type : task_id (full) or task_type (e.g. "quiz_gen") to
                      process the first pending task of that type.

    Returns True on success, False on failure.
    """
    pending = list_pending_tasks()
    task = None

    # Match by exact ID first
    for t in pending:
        if t["task_id"] == task_id_or_type or t["task_id"].startswith(task_id_or_type):
            task = t
            break

    # Match by task type
    if task is None:
        for t in pending:
            if t.get("task_type") == task_id_or_type:
                task = t
                break

    if task is None:
        return False

    return process_task_via_claude_cli(task["task_id"])


def _process_single_task(
    task: dict[str, Any],
    cfg,
    ai_provider: str | None = None,
    model: str | None = None,
) -> None:
    """Process one task using the direct AI API."""
    from .ai_adapter.factory import get_ai

    ttype = task.get("task_type", "")
    output_dir = Path(task.get("output_dir", "."))

    provider = ai_provider or cfg.default_provider
    # get_ai expects the full ai_config (with "providers" key), not provider-level cfg
    ai = get_ai(provider, cfg.ai_config)

    if ttype in (TaskType.QUIZ_GEN, TaskType.EXAM_REVIEW):
        _process_quiz_gen_task(task, ai, output_dir)
    elif ttype == TaskType.FIX_WRONG:
        _process_fix_wrong_task(task, ai, output_dir)
    elif ttype == TaskType.SUMMARIZE:
        _process_summarize_task(task, ai, output_dir)
    elif ttype == TaskType.EXTRACTION:
        _process_extraction_task(task, ai, output_dir)
    elif ttype == TaskType.REGENERATE:
        _process_extraction_task(task, ai, output_dir)
    elif ttype == TaskType.CUSTOM:
        _process_custom_task(task, ai, output_dir)
    else:
        raise ValueError(f"不支持的任务类型：{ttype}")


def _process_quiz_gen_task(task: dict[str, Any], ai, output_dir: Path) -> None:
    """Process a quiz_gen / exam_review task — call AI, parse JSON, save files."""
    from .prompts import build_policy_context
    from .question_policy import (
        determine_cognitive_distribution,
        determine_difficulty_distribution,
    )
    from .models import Quiz, QuizQuestion

    prompt = task.get("prompt", "")
    system_prompt = task.get("system_prompt", "你必须返回严格 JSON，不要 Markdown 代码块，不要额外解释。")
    lesson = task.get("lesson", 1)

    print(f"   🤖 正在向 AI 发送请求（{task.get('subject', '')} 第{lesson}课）...")
    payload_text = ai.chat_json(prompt, system=system_prompt)
    print(f"   📥 AI 响应已收到，正在解析试卷...")

    # Extract JSON
    import re as _re
    cleaned = payload_text.strip()
    fence = _re.search(r"```(?:json)?\s*(.*?)```", cleaned, _re.S | _re.I)
    if fence:
        cleaned = fence.group(1).strip()
    try:
        payload = json.loads(cleaned)
    except json.JSONDecodeError:
        starts = [i for i in [cleaned.find("{"), cleaned.find("[")] if i >= 0]
        if not starts:
            raise ValueError(f"AI 输出中未找到 JSON。内容：{payload_text[:200]}")
        start = min(starts)
        end = max(cleaned.rfind("}"), cleaned.rfind("]"))
        if end < start:
            raise ValueError("AI 输出中的 JSON 不完整。")
        payload = json.loads(cleaned[start : end + 1])

    quiz = Quiz(
        id=f"quiz_chapter_{lesson:02d}",
        course_id=task.get("subject", ""),
        chapter_id=lesson,
        title=str(payload.get("quiz_title") or f"第 {lesson} 课测试"),
        questions=[QuizQuestion.model_validate(item) for item in payload.get("questions", [])],
        mode="markdown",
        source="regular",
    )

    if not quiz.questions:
        raise ValueError("AI 未生成任何题目。")

    # Save quiz files
    from .storage import Storage
    from .exam_logic import save_quiz_files, quiz_to_markdown

    storage = Storage()
    save_quiz_files(storage, task.get("subject", ""), quiz)

    # Also save to the expected task output path
    result_path = output_dir / "quiz_result.json"
    result_path.parent.mkdir(parents=True, exist_ok=True)
    result_path.write_text(
        json.dumps(quiz.model_dump(mode="json"), ensure_ascii=False, indent=2),
        encoding="utf-8",
    )

    print(f"✅ 试卷已生成：{quiz.id}（{len(quiz.questions)} 题）")


def _process_fix_wrong_task(task: dict[str, Any], ai, output_dir: Path) -> None:
    """Process a fix_wrong task."""
    prompt = task.get("prompt", "")
    system_prompt = task.get("system_prompt", "")
    analysis = ai.chat(prompt, system=system_prompt)

    output_file = (task.get("output_files") or ["fix_analysis.md"])[0]
    out_path = output_dir / output_file
    out_path.parent.mkdir(parents=True, exist_ok=True)
    out_path.write_text(analysis, encoding="utf-8")
    print(f"✅ 错题解析已保存：{out_path}")


def _process_summarize_task(task: dict[str, Any], ai, output_dir: Path) -> None:
    """Process a summarize task."""
    import re as _re

    prompt = task.get("prompt", "")
    system_prompt = task.get("system_prompt", "你必须返回严格 JSON。")
    payload_text = ai.chat_json(prompt, system=system_prompt)

    cleaned = payload_text.strip()
    fence = _re.search(r"```(?:json)?\s*(.*?)```", cleaned, _re.S | _re.I)
    if fence:
        cleaned = fence.group(1).strip()

    try:
        payload = json.loads(cleaned)
    except json.JSONDecodeError:
        start = min(cleaned.find("{"), cleaned.find("["))
        end = max(cleaned.rfind("}"), cleaned.rfind("]"))
        payload = json.loads(cleaned[start : end + 1])

    out_path = output_dir / "summary_result.json"
    out_path.parent.mkdir(parents=True, exist_ok=True)
    out_path.write_text(
        json.dumps(payload, ensure_ascii=False, indent=2),
        encoding="utf-8",
    )
    print(f"✅ 评估结果已保存：{out_path}")


def _process_extraction_task(task: dict[str, Any], ai, output_dir: Path) -> None:
    """Process an extraction task — this is complex and best left to Claude Code Agent.

    Falls back gracefully with a message.
    """
    raise ValueError(
        "提取任务需要 Claude Code Agent 的多工具能力（Read/Write/Bash/WebFetch），"
        "请说「处理所有待处理的学习任务」来手动触发。"
    )


def _process_custom_task(task: dict[str, Any], ai, output_dir: Path) -> None:
    """Process a generic custom task."""
    prompt = task.get("prompt", "")
    system_prompt = task.get("system_prompt", "")
    response = ai.chat(prompt, system=system_prompt)

    out_path = output_dir / "custom_result.md"
    out_path.parent.mkdir(parents=True, exist_ok=True)
    out_path.write_text(response, encoding="utf-8")
    print(f"✅ 结果已保存：{out_path}")
