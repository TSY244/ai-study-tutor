"""Summary checker — AI evaluates student's self-summary against course material."""

from __future__ import annotations

import json as _json
from typing import Any

from .ai_adapter.base_ai import BaseAI
from .prompts import summary_check_prompt
from .storage import Storage


def evaluate_summary(
    subject: str,
    lesson: int,
    student_input: str,
    ai: BaseAI,
    storage: Storage | None = None,
) -> dict[str, Any]:
    """Evaluate a student's self-summary against the course material.

    Args:
        subject: Course subject name.
        lesson: Lesson number.
        student_input: The student's summary text.
        ai: AI adapter.
        storage: Storage instance.

    Returns:
        A dict with accuracy_score, completeness_score, overall_assessment,
        key_misunderstandings, missing_concepts, strengths, suggestions.
    """
    storage = storage or Storage()
    raw_path = storage.raw_text_path(subject, lesson)
    if not raw_path.exists():
        raise FileNotFoundError(f"未找到课程内容：{raw_path}。请先导入素材。")
    raw_text = storage.read_text(raw_path)

    system, prompt = summary_check_prompt(raw_text, student_input, subject)
    response = ai.chat_json(prompt, system=system)
    return _extract_json(response)


def _extract_json(text: str) -> Any:
    """Best-effort JSON extraction."""
    import re as _re

    cleaned = text.strip()
    fence = _re.search(r"```(?:json)?\s*(.*?)```", cleaned, _re.S | _re.I)
    if fence:
        cleaned = fence.group(1).strip()
    try:
        return _json.loads(cleaned)
    except _json.JSONDecodeError:
        starts = [i for i in [cleaned.find("{"), cleaned.find("[")] if i >= 0]
        if not starts:
            raise ValueError(f"AI 输出中未找到 JSON。内容：{text[:200]}")
        start = min(starts)
        end = max(cleaned.rfind("}"), cleaned.rfind("]"))
        if end < start:
            raise ValueError("AI JSON 不完整。")
        return _json.loads(cleaned[start : end + 1])
