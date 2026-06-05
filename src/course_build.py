"""Course builder — generates teacher notes, student notes, and lesson metadata."""

from __future__ import annotations

from .ai_adapter.base_ai import BaseAI
from .models import LessonInfo, now_iso
from .prompts import student_notes_prompt, teacher_notes_prompt
from .storage import Storage


def build_doc(
    raw_text: str,
    subject: str,
    lesson: int,
    name: str,
    ai: BaseAI,
    storage: Storage | None = None,
) -> tuple[str, str]:
    """Generate teacher notes and student notes via AI.

    Args:
        raw_text: The source material text.
        subject: Subject name.
        lesson: Lesson number.
        name: Lesson title/name.
        ai: AI adapter instance.
        storage: Storage instance.

    Returns:
        (teacher_notes_markdown, student_notes_markdown)
    """
    storage = storage or Storage()
    lesson_dir = storage.ensure_lesson_dir(subject, lesson)

    # 1. Save raw text
    storage.save_text(storage.raw_text_path(subject, lesson), raw_text)

    # 2. Generate teacher notes
    t_system, t_prompt = teacher_notes_prompt(raw_text, subject, name)
    teacher_md = ai.chat(t_prompt, system=t_system)
    storage.save_text(storage.teacher_notes_path(subject, lesson), teacher_md)

    # 3. Generate student notes from teacher notes
    s_system, s_prompt = student_notes_prompt(teacher_md)
    student_md = ai.chat(s_prompt, system=s_system)
    storage.save_text(storage.student_notes_path(subject, lesson), student_md)

    # 4. Save lesson info metadata
    info = LessonInfo(
        subject=subject,
        lesson=lesson,
        name=name,
        source_files=[],
        raw_text=raw_text,
    )
    storage.save_json(storage.info_path(subject, lesson), info)

    return teacher_md, student_md


def regenerate_notes(
    subject: str,
    lesson: int,
    ai: BaseAI,
    storage: Storage | None = None,
) -> tuple[str, str]:
    """Re-generate teacher and student notes from existing raw text (--fix-doc)."""
    storage = storage or Storage()
    raw_path = storage.raw_text_path(subject, lesson)
    if not raw_path.exists():
        raise FileNotFoundError(f"未找到源文件：{raw_path}")
    raw_text = storage.read_text(raw_path)

    info_path = storage.info_path(subject, lesson)
    name = f"第{lesson}课"
    if info_path.exists():
        info_data = storage.read_json(info_path)
        name = info_data.get("name", name)

    return build_doc(raw_text, subject, lesson, name, ai, storage)
