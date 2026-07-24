"""File-based storage layer for courses, lessons, exams."""

from __future__ import annotations

import json
import re
from pathlib import Path
from typing import Any

from pydantic import BaseModel


T = type


def slugify(name: str) -> str:
    """Normalize a name for use as a directory/filename slug."""
    text = name.strip().lower()
    text = re.sub(r"\s+", "-", text)
    text = re.sub(r"[^\w\-一-鿿]+", "", text)
    text = re.sub(r"-{2,}", "-", text).strip("-_")
    return text or "course"


class StorageError(RuntimeError):
    pass


class Storage:
    """Manages persistence for courses, lessons, quizzes, and attempts."""

    def __init__(self, course_root: Path | str = "./course") -> None:
        self.course_root = Path(course_root)
        self.course_root.mkdir(parents=True, exist_ok=True)

    # --- Paths ---

    def subject_dir(self, subject: str) -> Path:
        return self.course_root / slugify(subject)

    def lesson_dir(self, subject: str, lesson: int) -> Path:
        return self.subject_dir(subject) / f"lesson_{lesson:02d}"

    def teacher_notes_path(self, subject: str, lesson: int) -> Path:
        p = self.lesson_dir(subject, lesson) / "teacher_notes" / "content.md"
        if p.exists():
            return p
        # Fallback: claude may have written to output/ subdir
        fallback = self.lesson_dir(subject, lesson) / "output" / "teacher_notes" / "content.md"
        return fallback if fallback.exists() else p

    def student_notes_path(self, subject: str, lesson: int) -> Path:
        p = self.lesson_dir(subject, lesson) / "student_notes" / "content.md"
        if p.exists():
            return p
        fallback = self.lesson_dir(subject, lesson) / "output" / "student_notes" / "content.md"
        return fallback if fallback.exists() else p

    def info_path(self, subject: str, lesson: int) -> Path:
        return self.lesson_dir(subject, lesson) / "info.json"

    def exam_dir(self, subject: str, lesson: int) -> Path:
        p = self.lesson_dir(subject, lesson) / "exam"
        p.mkdir(parents=True, exist_ok=True)
        return p

    def wrong_questions_dir(self, subject: str, lesson: int) -> Path:
        p = self.lesson_dir(subject, lesson) / "wrong_questions"
        p.mkdir(parents=True, exist_ok=True)
        return p

    def raw_dir(self, subject: str, lesson: int) -> Path:
        p = self.lesson_dir(subject, lesson) / "raw"
        p.mkdir(parents=True, exist_ok=True)
        return p

    def raw_text_path(self, subject: str, lesson: int) -> Path:
        return self.raw_dir(subject, lesson) / "raw_text.md"

    def weaknesses_path(self) -> Path:
        return self.course_root / "weaknesses.json"

    def scores_path(self) -> Path:
        return self.course_root / "scores.json"

    # --- I/O helpers ---

    def save_text(self, path: Path, content: str) -> Path:
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content, encoding="utf-8")
        return path

    def read_text(self, path: Path) -> str:
        if not path.exists():
            raise StorageError(f"文件不存在：{path}")
        return path.read_text(encoding="utf-8")

    def save_json(self, path: Path, data: Any) -> Path:
        path.parent.mkdir(parents=True, exist_ok=True)
        if isinstance(data, BaseModel):
            payload = data.model_dump(mode="json")
        elif isinstance(data, list):
            payload = [
                item.model_dump(mode="json") if isinstance(item, BaseModel) else item
                for item in data
            ]
        else:
            payload = data
        path.write_text(
            json.dumps(payload, ensure_ascii=False, indent=2),
            encoding="utf-8",
        )
        return path

    def read_json(self, path: Path) -> Any:
        if not path.exists():
            raise StorageError(f"JSON 文件不存在：{path}")
        return json.loads(path.read_text(encoding="utf-8"))

    # --- Subject / lesson queries ---

    def list_subjects(self) -> list[str]:
        """Return all subject names found under course_root.

        A subject is any directory that either has lessons or a .subject marker.
        """
        if not self.course_root.exists():
            return []
        result: list[str] = []
        for d in sorted(self.course_root.iterdir()):
            if not d.is_dir() or d.name.startswith("."):
                continue
            if d.name in ("weaknesses.json", "scores.json"):
                continue
            # Has lessons OR marker file
            has_lessons = any(
                sub.is_dir() and sub.name.startswith("lesson_")
                for sub in d.iterdir()
            )
            if has_lessons or (d / ".subject").exists():
                result.append(d.name)
        return result

    def create_subject(self, subject: str) -> Path:
        """Create a subject directory with a marker file so it persists even without lessons."""
        sd = self.subject_dir(subject)
        sd.mkdir(parents=True, exist_ok=True)
        (sd / ".subject").touch()
        return sd

    def list_lessons(self, subject: str) -> list[int]:
        """Return all lesson numbers for a subject."""
        sd = self.subject_dir(subject)
        if not sd.exists():
            return []
        result: list[int] = []
        for d in sorted(sd.iterdir()):
            if d.is_dir() and d.name.startswith("lesson_"):
                try:
                    result.append(int(d.name.split("_")[1]))
                except (IndexError, ValueError):
                    pass
        return result

    def lesson_exists(self, subject: str, lesson: int) -> bool:
        ld = self.lesson_dir(subject, lesson)
        return ld.exists() and any(ld.iterdir())

    def delete_subject(self, subject: str) -> bool:
        """Delete an entire subject directory. Returns True if deleted."""
        sd = self.subject_dir(subject)
        if not sd.exists():
            return False
        import shutil
        shutil.rmtree(sd)
        return True

    def delete_lesson(self, subject: str, lesson: int) -> bool:
        """Delete a single lesson directory. Returns True if deleted."""
        ld = self.lesson_dir(subject, lesson)
        if not ld.exists():
            return False
        import shutil
        shutil.rmtree(ld)
        # Clean up empty subject dir
        sd = self.subject_dir(subject)
        if sd.exists() and not any(sd.iterdir()):
            shutil.rmtree(sd)
        return True

    def ensure_subject_dir(self, subject: str) -> Path:
        p = self.subject_dir(subject)
        p.mkdir(parents=True, exist_ok=True)
        return p

    def ensure_lesson_dir(self, subject: str, lesson: int) -> Path:
        p = self.lesson_dir(subject, lesson)
        p.mkdir(parents=True, exist_ok=True)
        return p
