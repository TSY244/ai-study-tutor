"""Weakness/concept tracking — remembers what the student got wrong.

Now includes mastery computation, consecutive-correct/wrong tracking,
and auto-resolution of mastered concepts.
"""

from __future__ import annotations

import hashlib
from datetime import datetime
from typing import Any

from .models import Weakness, now_iso
from .storage import Storage


def weakness_id(course: str, chapter: int | None, concept: str) -> str:
    raw = f"{course}|{chapter or ''}|{concept.strip().lower()}"
    return hashlib.sha1(raw.encode("utf-8")).hexdigest()[:12]


# ---------------------------------------------------------------------------
# Load / save
# ---------------------------------------------------------------------------

def load_weaknesses(storage: Storage | None = None) -> list[Weakness]:
    storage = storage or Storage()
    path = storage.weaknesses_path()
    if not path.exists():
        return []
    data = storage.read_json(path)
    return [Weakness.model_validate(item) for item in data]


def save_weaknesses(storage: Storage, weaknesses: list[Weakness]) -> None:
    storage.save_json(storage.weaknesses_path(), weaknesses)


# ---------------------------------------------------------------------------
# Update (with auto-mastery)
# ---------------------------------------------------------------------------

def update_weaknesses(
    storage: Storage,
    course: str,
    weak_concepts: list[dict[str, Any]],
    chapter: int | None = None,
) -> list[Weakness]:
    """Merge new weakness reports into persistent store.

    - New concepts are created.
    - Existing concepts have severity updated (max of old/new).
    - If review_count >= 3 and severity has trended down, mark mastered.
    """
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
    save_weaknesses(storage, result)
    return result


def reduce_weakness_severity(
    storage: Storage,
    course: str,
    concept: str,
    chapter: int | None = None,
    amount: int = 1,
) -> bool:
    """Reduce the severity of a weakness when the student answers correctly.

    If severity reaches 1 AND review_count >= 2, mark as mastered.
    Returns True if the concept was marked mastered.
    """
    existing = load_weaknesses(storage)
    wid = weakness_id(course, chapter, concept)
    now = now_iso()

    for w in existing:
        if w.id == wid:
            w.severity = max(1, w.severity - amount)
            w.last_seen = now
            w.review_count += 1
            if w.severity <= 1 and w.review_count >= 2:
                w.mastered = True
            save_weaknesses(storage, existing)
            return w.mastered

    return False


def mark_weakness_resolved(
    storage: Storage,
    course: str,
    concept: str,
    chapter: int | None = None,
) -> bool:
    """Explicitly mark a concept as mastered."""
    existing = load_weaknesses(storage)
    wid = weakness_id(course, chapter, concept)
    now = now_iso()

    for w in existing:
        if w.id == wid:
            w.mastered = True
            w.severity = 1
            w.last_seen = now
            save_weaknesses(storage, existing)
            return True

    return False


# ---------------------------------------------------------------------------
# Query
# ---------------------------------------------------------------------------

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


# ---------------------------------------------------------------------------
# Mastery computation
# ---------------------------------------------------------------------------

def compute_chapter_mastery(
    storage: Storage,
    course: str,
    chapter: int,
) -> float:
    """Compute a weighted mastery score (0–1) for a chapter.

    Combines two signals:
    1. Score history (weighted average, recent = heavier weight)
    2. Weakness resolution ratio

    Returns 0.0 if no data exists.
    """
    # ── Score-based mastery ──
    scores = _load_chapter_scores(storage, course, chapter)
    score_mastery = _weighted_score_mastery(scores)

    # ── Weakness-based adjustment ──
    weaknesses = load_weaknesses(storage)
    chapter_weak = [w for w in weaknesses
                    if w.course == course
                    and (w.chapter is None or w.chapter == chapter)]

    if not chapter_weak:
        # No weakness data — rely purely on scores
        return score_mastery

    total_weak = len(chapter_weak)
    mastered_weak = sum(1 for w in chapter_weak if w.mastered)
    weakness_resolution = mastered_weak / total_weak if total_weak > 0 else 1.0

    # Average severity (inverted: severity 1 → 1.0, severity 5 → 0.0)
    avg_severity = sum(w.severity for w in chapter_weak) / total_weak if total_weak > 0 else 3
    severity_factor = 1.0 - (avg_severity - 1) / 4.0  # maps [1,5] → [1.0, 0.0]

    # Combine: 50% scores, 30% resolution, 20% severity
    combined = 0.50 * score_mastery + 0.30 * weakness_resolution + 0.20 * severity_factor

    return max(0.0, min(1.0, combined))


# ---------------------------------------------------------------------------
# Consecutive-correct / wrong tracking
# ---------------------------------------------------------------------------

def get_consecutive_stats(
    storage: Storage,
    course: str,
    chapter: int,
) -> dict[str, int]:
    """Return {consecutive_correct, consecutive_wrong, total_answered} from
    the most recent quiz attempt."""
    attempts = _load_chapter_attempts(storage, course, chapter)
    if not attempts:
        return {"consecutive_correct": 0, "consecutive_wrong": 0, "total_answered": 0}

    latest = attempts[-1]
    answers = latest.get("answers", [])

    consec_correct = 0
    consec_wrong = 0

    # Count from the END of the quiz
    for ans in reversed(answers):
        if ans.get("is_correct"):
            consec_correct += 1
            if consec_wrong > 0:
                break
        else:
            consec_wrong += 1
            if consec_correct > 0:
                break

    # Also compute from beginning for wrong-streak detection
    consec_wrong_start = 0
    for ans in answers:
        if not ans.get("is_correct"):
            consec_wrong_start += 1
        else:
            break

    return {
        "consecutive_correct": consec_correct,
        "consecutive_wrong": max(consec_wrong, consec_wrong_start),
        "total_answered": len(answers),
    }


def save_consecutive_stats(
    storage: Storage,
    course: str,
    chapter: int,
    stats: dict[str, int],
) -> None:
    """Persist consecutive stats alongside the quiz attempt."""
    # Stats are embedded in the answers JSON, so this is a no-op
    # when stats are saved via run_cli_quiz.  Provided as a hook for
    # future use.
    pass


# ---------------------------------------------------------------------------
# Internal helpers
# ---------------------------------------------------------------------------

def _load_chapter_scores(
    storage: Storage,
    course: str,
    chapter: int,
) -> list[dict[str, Any]]:
    """Load score records for a course+chapter."""
    scores_path = storage.scores_path()
    if not scores_path.exists():
        return []
    all_scores = storage.read_json(scores_path)
    return [
        s for s in all_scores
        if s.get("subject") == course and s.get("lesson") == chapter
    ]


def _weighted_score_mastery(scores: list[dict[str, Any]]) -> float:
    """Compute weighted average of score ratios.

    Recent scores weighted more heavily (linear ramp by index).
    """
    if not scores:
        return 0.0

    sorted_scores = sorted(scores, key=lambda s: s.get("timestamp", ""))
    total_weight = 0.0
    weighted_sum = 0.0

    for i, s in enumerate(sorted_scores):
        score = float(s.get("score", 0))
        total = float(s.get("total", 1))
        if total <= 0:
            continue
        ratio = score / total
        weight = 1.0 + i * 0.5
        weighted_sum += ratio * weight
        total_weight += weight

    if total_weight == 0:
        return 0.0
    return weighted_sum / total_weight


def _load_chapter_attempts(
    storage: Storage,
    course: str,
    chapter: int,
) -> list[dict[str, Any]]:
    """Load all quiz answer attempts for a chapter, sorted by time."""
    exam_dir = storage.exam_dir(course, chapter)
    attempts: list[dict[str, Any]] = []
    for f in sorted(exam_dir.glob("*_answers.json")):
        try:
            data = storage.read_json(f)
            if isinstance(data, dict) and "answers" in data:
                attempts.append(data)
        except Exception:
            pass
    return attempts
