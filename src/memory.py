"""Weakness/concept tracking — remembers what the student got wrong."""

from __future__ import annotations

import hashlib
from typing import Any

from .models import Weakness, now_iso
from .storage import Storage


def weakness_id(course: str, chapter: int | None, concept: str) -> str:
    raw = f"{course}|{chapter or ''}|{concept.strip().lower()}"
    return hashlib.sha1(raw.encode("utf-8")).hexdigest()[:12]


def load_weaknesses(storage: Storage | None = None) -> list[Weakness]:
    storage = storage or Storage()
    path = storage.weaknesses_path()
    if not path.exists():
        return []
    data = storage.read_json(path)
    return [Weakness.model_validate(item) for item in data]


def update_weaknesses(
    storage: Storage,
    course: str,
    weak_concepts: list[dict[str, Any]],
    chapter: int | None = None,
) -> list[Weakness]:
    existing = load_weaknesses(storage)
    by_id = {item.id: item for item in existing}
    now = now_iso()

    for item in weak_concepts:
        concept = str(item.get("concept", "")).strip()
        if not concept:
            continue
        wid = weakness_id(course, chapter, concept)
        severity = int(item.get("severity", 3) or 3)
        severity = min(5, max(1, severity))
        evidence = str(item.get("evidence", "")).strip()
        description = str(item.get("weakness_description", "")).strip()

        if wid in by_id:
            current = by_id[wid]
            current.last_seen = now
            current.review_count += 1
            current.severity = max(current.severity, severity)
            if evidence and evidence not in current.evidence:
                current.evidence = f"{current.evidence}\n{evidence}".strip()
            if description:
                current.weakness_description = description
            current.mastered = False
        else:
            by_id[wid] = Weakness(
                id=wid,
                course=course,
                chapter=chapter,
                concept=concept,
                weakness_description=description or "需要复习该概念",
                evidence=evidence,
                severity=severity,
                first_seen=now,
                last_seen=now,
            )

    result = sorted(by_id.values(), key=lambda w: (w.course, w.chapter or 0, w.concept))
    storage.save_json(storage.weaknesses_path(), result)
    return result


def course_weaknesses(
    storage: Storage,
    course: str,
    chapter: int | None = None,
    include_mastered: bool = False,
) -> list[dict[str, Any]]:
    result: list[dict[str, Any]] = []
    for weakness in load_weaknesses(storage):
        if weakness.course != course:
            continue
        if chapter is not None and weakness.chapter not in {None, chapter}:
            continue
        if weakness.mastered and not include_mastered:
            continue
        result.append(weakness.model_dump(mode="json"))
    return result
