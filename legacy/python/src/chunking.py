"""Text cleaning and chapter slicing utilities."""

from __future__ import annotations

import re
from typing import Any


def clean_text(text: str) -> str:
    """Normalize whitespace and line breaks."""
    text = text.replace("\r\n", "\n").replace("\r", "\n")
    text = re.sub(r"[ \t]+", " ", text)
    text = re.sub(r"\n{3,}", "\n\n", text)
    return text.strip()


def chapter_slice(raw_text: str, outline: dict[str, Any], chapter_id: int) -> str:
    """Extract the text chunk most relevant to a given chapter."""
    chapters = outline.get("chapters", [])
    if not chapters:
        return raw_text
    titles = [str(ch.get("title", "")).strip() for ch in chapters]
    current_title = titles[chapter_id - 1] if 0 < chapter_id <= len(titles) else ""
    next_title = titles[chapter_id] if chapter_id < len(titles) else ""

    if current_title and current_title in raw_text:
        start = raw_text.find(current_title)
        end = raw_text.find(next_title, start + len(current_title)) if next_title else -1
        if end > start:
            return raw_text[start:end].strip()
        return raw_text[start:].strip()

    # Fallback: proportional split
    total = len(raw_text)
    count = max(len(chapters), 1)
    start = total * (chapter_id - 1) // count
    end = total * chapter_id // count
    padding = min(1200, total // count)
    return raw_text[max(0, start - padding) : min(total, end + padding)].strip()
