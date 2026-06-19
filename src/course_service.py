"""Backend service layer — all business logic, no user I/O.

The frontend (interactive.py / main.py CLI) calls these functions.
AI tasks are delegated to Claude Code Agent via src/agent_runner.py.

Question-generation now routes through the policy engine
(src/question_policy.py) to ensure learning-science-backed defaults.
"""

from __future__ import annotations

from pathlib import Path
from typing import Any

from .config_loader import get_config
from .storage import Storage, StorageError


# ── Lazy singleton ──

_svc = None

def svc() -> "CourseService":
    global _svc
    if _svc is None:
        _svc = CourseService()
    return _svc


class CourseService:
    """Stateless service that wraps Storage + agent task delegation."""

    def __init__(self) -> None:
        cfg = get_config()
        self.storage = Storage(cfg.course_path)
        self.cfg = cfg

    # ── Subject CRUD ──

    def list_subjects(self) -> list[str]:
        return self.storage.list_subjects()

    def create_subject(self, name: str) -> Path:
        if not name.strip():
            raise ValueError("科目名称不能为空。")
        return self.storage.create_subject(name.strip())

    def delete_subject(self, name: str) -> bool:
        return self.storage.delete_subject(name)

    # ── Lesson CRUD ──

    def list_lessons(self, subject: str) -> list[int]:
        return sorted(self.storage.list_lessons(subject))

    def lesson_info(self, subject: str, lesson: int) -> dict[str, Any] | None:
        p = self.storage.info_path(subject, lesson)
        if p.exists():
            return self.storage.read_json(p)
        return None

    def lesson_name(self, subject: str, lesson: int) -> str:
        info = self.lesson_info(subject, lesson)
        if info:
            return info.get("name", f"第{lesson}课")
        return f"第{lesson}课"

    def lesson_status(self, subject: str, lesson: int) -> dict[str, bool]:
        """Return {teacher, student, quiz} — whether each exists."""
        s = self.storage
        return {
            "teacher": s.teacher_notes_path(subject, lesson).exists(),
            "student": s.student_notes_path(subject, lesson).exists(),
            "quiz": bool(list(s.exam_dir(subject, lesson).glob("quiz_chapter_*.json"))),
        }

    def delete_lesson(self, subject: str, lesson: int) -> bool:
        return self.storage.delete_lesson(subject, lesson)

    # ── Import (via Claude Code Agent) ──

    def queue_import_lesson(
        self,
        subject: str,
        lesson: int,
        name: str,
        source_flag: str,  # "--load-url" / "--load-video" / "--source-text"
        source_value: str,
        model: str = "deepseek-v4-pro",
    ) -> str:
        """Queue a lesson extraction task for Claude Code Agent. Returns task_id.

        The task will be processed by Claude Code Agent (the current session).
        After queuing, the caller should tell the user to ask Claude Code to
        process pending tasks.
        """
        from .agent_runner import (
            queue_extraction_task,
            print_pending_summary,
        )

        # Ensure output dirs exist
        lesson_dir = self.storage.ensure_lesson_dir(subject, lesson)
        (lesson_dir / "teacher_notes").mkdir(parents=True, exist_ok=True)
        (lesson_dir / "student_notes").mkdir(parents=True, exist_ok=True)

        # Build source description
        if source_flag == "--load-url":
            source_desc = (
                f"从视频链接提取知识：{source_value}\n\n"
                f"先用 yt-dlp 获取该视频的中文字幕。如果视频有中文字幕（人工/AI），下载字幕文本；"
                f"如果没有中文字幕，尝试获取英文字幕然后翻译；"
                f"如果完全没有字幕，尝试下载视频的元数据和描述信息，结合 WebFetch 访问视频页面获取简介。"
            )
        elif source_flag == "--load-video":
            source_desc = f"从本地文件提取知识：\n  - 素材文件：{source_value}\n  请使用 Read 工具读取文件内容。如果是视频/音频，使用 ffmpeg 或 yt-dlp 提取音频/字幕。"
        elif source_flag == "--source-text":
            raw_path = self.storage.raw_text_path(subject, lesson)
            self.storage.save_text(raw_path, source_value)
            source_desc = f"文本已保存在 {raw_path}，请使用 Read 工具读取后生成笔记。"
        else:
            source_desc = f"素材：{source_value}"

        source_text = source_value if source_flag == "--source-text" else ""

        task_id = queue_extraction_task(
            source_desc=source_desc,
            subject=subject,
            lesson=lesson,
            name=name,
            output_dir=lesson_dir,
            model=model,
            source_text=source_text,
        )

        return task_id

    def verify_import(self, subject: str, lesson: int) -> dict[str, bool]:
        """Check if lesson import completed successfully."""
        from .agent_runner import verify_extraction_output, normalize_output

        lesson_dir = self.storage.lesson_dir(subject, lesson)
        normalize_output(lesson_dir)
        return verify_extraction_output(lesson_dir)

    def has_pending_tasks(self) -> bool:
        from .agent_runner import get_pending_count
        return get_pending_count() > 0

    # ── Exam ──

    def queue_exam_generation(
        self, subject: str, lesson: int, num: int | None = None, model: str = ""
    ) -> str:
        """Queue a quiz generation task. Returns task_id.

        If `num` is None, the policy engine determines the question count
        automatically based on mastery and content complexity.
        """
        from .agent_runner import queue_quiz_gen_task
        from .memory import compute_chapter_mastery, course_weaknesses, get_consecutive_stats
        from .question_policy import (
            determine_cognitive_distribution,
            determine_difficulty_distribution,
            determine_question_count,
        )

        lesson_dir = self.storage.lesson_dir(subject, lesson)
        exam_dir = self.storage.exam_dir(subject, lesson)

        # Gather lesson content
        lesson_content = ""
        student_path = self.storage.student_notes_path(subject, lesson)
        if student_path.exists():
            lesson_content = self.storage.read_text(student_path)
        else:
            raw_path = self.storage.raw_text_path(subject, lesson)
            if raw_path.exists():
                lesson_content = self.storage.read_text(raw_path)

        # Gather weaknesses
        weaknesses: list[dict[str, Any]] = []
        try:
            weaknesses = course_weaknesses(self.storage, subject, lesson)
        except Exception:
            pass

        # ── Compute policy parameters ──
        mastery = 0.0
        try:
            mastery = compute_chapter_mastery(self.storage, subject, lesson)
        except Exception:
            pass

        constats: dict[str, int] = {"consecutive_correct": 0, "consecutive_wrong": 0, "total_answered": 0}
        try:
            constats = get_consecutive_stats(self.storage, subject, lesson)
        except Exception:
            pass

        # Determine question count
        if num is not None:
            question_count = num
        else:
            question_count = determine_question_count(
                content_complexity=_detect_lesson_complexity(lesson_content),
                consecutive_errors=constats.get("consecutive_wrong", 0),
                consecutive_correct=constats.get("consecutive_correct", 0),
                accuracy=mastery,
            )

        # Determine distributions
        cognitive_dist = determine_cognitive_distribution(mastery, question_count)
        difficulty_dist = determine_difficulty_distribution(question_count)

        # ── Print policy summary for user ──
        self._print_policy_info(
            subject, lesson, question_count, mastery,
            cognitive_dist, difficulty_dist, constats,
        )

        task_id = queue_quiz_gen_task(
            subject=subject,
            lesson=lesson,
            lesson_content=lesson_content,
            output_dir=exam_dir,
            question_count=question_count,
            weaknesses=weaknesses,
            model=model,
            cognitive_distribution={k.value: v for k, v in cognitive_dist.items()},
            difficulty_distribution=difficulty_dist,
            mastery=mastery,
            consecutive_stats=constats,
        )
        return task_id

    # ── Wrong questions ──

    def wrong_count(self, subject: str, lesson: int) -> int:
        try:
            from .exam_logic import list_wrong_questions
            return len(list_wrong_questions(subject, lesson, self.storage))
        except Exception:
            return 0

    # ── Policy info ──

    def _print_policy_info(
        self,
        subject: str,
        lesson: int,
        question_count: int,
        mastery: float,
        cognitive_dist,
        difficulty_dist,
        constats: dict[str, int],
    ) -> None:
        """Print a summary of the applied policy to the user."""
        tier = (
            "高（>85%）" if mastery > 0.85
            else "中（60%-85%）" if mastery >= 0.60
            else "低（<60%）" if mastery > 0
            else "首次练习"
        )
        print(f"\n{'─'*50}")
        print(f"📋 题目生成策略：{subject} 第{lesson}课")
        print(f"{'─'*50}")
        print(f"  掌握度：{mastery:.0%}（{tier}）")
        print(f"  上次统计：连续正确 {constats.get('consecutive_correct', 0)} 题，"
              f"连续错误 {constats.get('consecutive_wrong', 0)} 题")
        print(f"  本次生成：{question_count} 题")
        print(f"  认知层次：{' | '.join(f'{lv.value}={ct}' for lv, ct in cognitive_dist.items() if ct > 0)}")
        print(f"  难度分布：{' | '.join(f'{k}={v}' for k, v in difficulty_dist.items() if v > 0)}")
        print(f"{'─'*50}")


def _detect_lesson_complexity(content: str) -> str:
    """Quick heuristic for content complexity level."""
    if not content:
        return "normal"
    length = len(content)
    math_kw = sum(1 for kw in ["推导", "证明", "公式", "定理", "计算", "微分", "积分"]
                  if kw in content)
    step_kw = sum(1 for kw in ["步骤", "流程", "方法"]
                  if kw in content)
    concept_kw = content.count("概念") + content.count("定义")
    score = math_kw * 2 + step_kw + concept_kw
    if length < 3000 and score < 5:
        return "simple"
    elif score >= 8 or length > 15000:
        return "complex"
    return "normal"
