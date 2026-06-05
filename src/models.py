"""Pydantic data models for courses, quizzes, exams, attempts."""

from __future__ import annotations

from datetime import datetime
from enum import Enum
from typing import Any, Literal

from pydantic import BaseModel, Field


def now_iso() -> str:
    return datetime.now().astimezone().isoformat(timespec="seconds")


class Course(BaseModel):
    id: str
    title: str
    created_at: str = Field(default_factory=now_iso)
    source_files: list[str] = Field(default_factory=list)
    raw_text_path: str
    outline_path: str | None = None


class Chapter(BaseModel):
    chapter_id: int
    title: str
    learning_goals: list[str] = Field(default_factory=list)
    key_concepts: list[str] = Field(default_factory=list)
    difficulty: Literal["beginner", "intermediate", "advanced"] = "beginner"
    estimated_minutes: int = 30


class CourseOutline(BaseModel):
    course_title: str
    target_audience: str = "初学者"
    chapters: list[Chapter]


class QuestionType(str, Enum):
    single_choice = "single_choice"
    multiple_choice = "multiple_choice"
    true_false = "true_false"
    short_answer = "short_answer"
    case_analysis = "case_analysis"


class QuizQuestion(BaseModel):
    id: str
    type: QuestionType
    question: str
    options: dict[str, str] | None = None
    correct_answer: Any
    explanation: str
    tested_concepts: list[str] = Field(default_factory=list)
    difficulty: str = "easy"
    score: float = 5


class Quiz(BaseModel):
    id: str
    course_id: str
    chapter_id: int
    title: str
    questions: list[QuizQuestion]
    created_at: str = Field(default_factory=now_iso)
    mode: str = "markdown"
    source: Literal["regular", "retest"] = "regular"


class Answer(BaseModel):
    question_id: str
    user_answer: str


class QuestionFeedback(BaseModel):
    question_id: str
    score: float = 0
    is_correct: bool = False
    feedback: str
    correct_understanding: str
    weak_concepts: list[str] = Field(default_factory=list)


class WeakConcept(BaseModel):
    concept: str
    weakness_description: str
    severity: int = Field(default=3, ge=1, le=5)
    evidence: str = ""


class Attempt(BaseModel):
    id: str
    course_id: str
    quiz_id: str
    answers: list[Answer]
    total_score: float = 0
    question_feedback: list[QuestionFeedback] = Field(default_factory=list)
    feedback: str = ""
    weak_concepts: list[WeakConcept] = Field(default_factory=list)
    review_plan: list[str] = Field(default_factory=list)
    retest_recommended: bool = False
    created_at: str = Field(default_factory=now_iso)


class Weakness(BaseModel):
    id: str
    course: str
    chapter: int | None = None
    concept: str
    weakness_description: str
    evidence: str
    severity: int = Field(default=3, ge=1, le=5)
    first_seen: str = Field(default_factory=now_iso)
    last_seen: str = Field(default_factory=now_iso)
    review_count: int = 1
    mastered: bool = False


# --- Course lesson info ---

class LessonInfo(BaseModel):
    """Metadata for a single lesson/course instance."""
    subject: str
    lesson: int
    name: str
    created_at: str = Field(default_factory=now_iso)
    source_files: list[str] = Field(default_factory=list)
    source_url: str | None = None
    raw_text: str = ""
