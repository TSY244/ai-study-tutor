"""Exam logic — quiz generation, terminal answering, grading, wrong-question tracking."""

from __future__ import annotations

import json
from datetime import datetime
from pathlib import Path
from typing import Any

from .ai_adapter.base_ai import BaseAI
from .config_loader import get_config
from .memory import course_weaknesses, update_weaknesses
from .models import (
    Answer,
    Attempt,
    QuestionFeedback,
    QuestionType,
    Quiz,
    QuizQuestion,
    WeakConcept,
)
from .prompts import (
    fix_wrong_prompt,
    grade_prompt,
    quiz_prompt,
    retest_prompt,
)
from .storage import Storage, StorageError


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _load_raw_text(subject: str, lesson: int, storage: Storage) -> str:
    """Load raw text from any source in the lesson directory."""
    lesson_dir = storage.lesson_dir(subject, lesson)

    # 1. Try the canonical raw_text.md
    raw_path = storage.raw_text_path(subject, lesson)
    if raw_path.exists():
        return storage.read_text(raw_path)

    # 2. Try any file in lesson root containing subtitle/raw content
    for pattern in ["*.srt", "*.vtt", "*.txt", "*.md"]:
        for f in sorted(lesson_dir.glob(pattern)):
            text = storage.read_text(f)
            if len(text) > 100:  # Skip tiny/marker files
                return text

    # 3. Try output/ subdirectory
    output_dir = lesson_dir / "output"
    if output_dir.exists():
        for pattern in ["*.srt", "*.vtt", "*.txt", "*.md"]:
            for f in sorted(output_dir.glob(pattern)):
                text = storage.read_text(f)
                if len(text) > 100:
                    return text

    # 4. Fall back to student notes as "raw" content
    student_path = storage.student_notes_path(subject, lesson)
    if student_path.exists():
        return storage.read_text(student_path)

    raise FileNotFoundError(f"未找到可用素材：{lesson_dir}")

def quiz_id_for(chapter_id: int, retest: bool = False) -> str:
    if retest:
        stamp = datetime.now().strftime("%Y%m%d_%H%M%S")
        return f"retest_chapter_{chapter_id:02d}_{stamp}"
    return f"quiz_chapter_{chapter_id:02d}"


def normalize_objective_answer(value: str) -> str:
    return ",".join(
        part.strip().upper()
        for part in value.replace("，", ",").split(",")
        if part.strip()
    )


def is_objective_correct(question: QuizQuestion, user_answer: str) -> bool:
    if question.type in (QuestionType.short_answer, QuestionType.case_analysis):
        return False  # Needs AI grading
    normalized_user = normalize_objective_answer(user_answer)
    if question.type == QuestionType.true_false:
        mapping = {
            "Y": "TRUE", "N": "FALSE",
            "YES": "TRUE", "NO": "FALSE",
            "对": "TRUE", "错": "FALSE",
        }
        normalized_user = mapping.get(normalized_user, normalized_user)
    correct = question.correct_answer
    if isinstance(correct, list):
        normalized_correct = normalize_objective_answer(
            ",".join(str(item) for item in correct)
        )
    else:
        normalized_correct = normalize_objective_answer(str(correct))
        if question.type == QuestionType.true_false:
            mapping = {"Y": "TRUE", "N": "FALSE", "对": "TRUE", "错": "FALSE"}
            normalized_correct = mapping.get(normalized_correct, normalized_correct)
    return normalized_user == normalized_correct


# ---------------------------------------------------------------------------
# Quiz I/O
# ---------------------------------------------------------------------------

def quiz_to_markdown(quiz: Quiz, teacher: bool = False) -> str:
    lines = [f"# {quiz.title}", "", f"- 课程 ID：{quiz.course_id}", f"- 章节：{quiz.chapter_id}", ""]
    for index, q in enumerate(quiz.questions, start=1):
        lines.append(f"## {index}. {q.question}")
        lines.append("")
        lines.append(f"- 题型：{q.type.value}")
        lines.append(f"- 分值：{q.score}")
        lines.append(f"- 考察概念：{'、'.join(q.tested_concepts)}")
        if q.options:
            lines.append("")
            for key, value in q.options.items():
                lines.append(f"{key}. {value}")
        lines.append("")
        lines.append("答：")
        lines.append("")
        if teacher:
            lines.append(f"**标准答案**：{q.correct_answer}")
            lines.append("")
            lines.append(f"**解析**：{q.explanation}")
            lines.append("")
    return "\n".join(lines).strip() + "\n"


def save_quiz_files(storage: Storage, course: str, quiz: Quiz) -> None:
    qdir = storage.exam_dir(course, quiz.chapter_id)
    storage.save_json(qdir / f"{quiz.id}.json", quiz)
    storage.save_text(qdir / f"{quiz.id}_student.md", quiz_to_markdown(quiz, teacher=False))
    storage.save_text(qdir / f"{quiz.id}_teacher.md", quiz_to_markdown(quiz, teacher=True))


# ---------------------------------------------------------------------------
# Quiz generation
# ---------------------------------------------------------------------------

def build_quiz(
    subject: str,
    chapter_id: int,
    mode: str,
    ai: BaseAI,
    storage: Storage | None = None,
    retest: bool = False,
    question_count: int = 10,
) -> Quiz:
    storage = storage or Storage()
    raw_text = _load_raw_text(subject, chapter_id, storage)
    student_notes_path = storage.student_notes_path(subject, chapter_id)
    lesson_text = storage.read_text(student_notes_path) if student_notes_path.exists() else ""
    weaknesses = course_weaknesses(storage, subject, chapter_id)

    if retest:
        latest_quiz, latest_feedback = _latest_attempt_context(storage, subject, chapter_id)
        system, prompt = retest_prompt(
            raw_text, weaknesses, latest_quiz, latest_feedback, question_count,
        )
    else:
        system, prompt = quiz_prompt(raw_text, lesson_text, weaknesses, question_count)

    payload_text = ai.chat_json(prompt, system=system)
    payload = _extract_json(payload_text)

    quiz = Quiz(
        id=quiz_id_for(chapter_id, retest=retest),
        course_id=subject,
        chapter_id=chapter_id,
        title=str(payload.get("quiz_title") or f"第 {chapter_id} 课测试"),
        questions=[QuizQuestion.model_validate(item) for item in payload.get("questions", [])],
        mode=mode,
        source="retest" if retest else "regular",
    )
    if not quiz.questions:
        raise StorageError("AI 未生成任何题目，请检查资料或重试。")
    save_quiz_files(storage, subject, quiz)
    return quiz


# ---------------------------------------------------------------------------
# CLI quiz
# ---------------------------------------------------------------------------

def run_cli_quiz(storage: Storage, course: str, quiz: Quiz) -> tuple[Path, float, list[dict[str, Any]]]:
    """Interactive terminal quiz. Returns (attempt_path, auto_score, wrong_answers)."""
    answers: list[Answer] = []
    total_auto_score = 0.0
    wrong_answers: list[dict[str, Any]] = []

    for index, question in enumerate(quiz.questions, start=1):
        print(f"\n{'='*40}")
        print(f"  [{index}/{len(quiz.questions)}]")
        print(f"  {question.question}")
        if question.options:
            for key, value in question.options.items():
                print(f"    {key}. {value}")

        if question.type in (QuestionType.short_answer, QuestionType.case_analysis):
            print("请输入答案，单独一行 END 结束：")
            lines: list[str] = []
            while True:
                line = input()
                if line.strip().upper() == "END":
                    break
                lines.append(line)
            user_answer = "\n".join(lines).strip()
            print("[提示] 主观题已保存，稍后请运行 --exam-review 进行 AI 批阅。")
        else:
            hint = "A/B/C/D" if question.type == QuestionType.single_choice else "A,C,D 或 Y/N"
            user_answer = input(f"你的答案（{hint}）：").strip()
            correct = is_objective_correct(question, user_answer)
            if correct:
                total_auto_score += question.score
                print("✓ 正确")
            else:
                print(f"✗ 错误  标准答案：{question.correct_answer}")
                wrong_answers.append({
                    "question_id": question.id,
                    "question": question.question,
                    "user_answer": user_answer,
                    "correct_answer": question.correct_answer,
                    "explanation": question.explanation,
                    "tested_concepts": question.tested_concepts,
                    "score": question.score,
                })
            print(f"  {question.explanation}")
        answers.append(Answer(question_id=question.id, user_answer=user_answer))

    # Save attempt
    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    exam_dir = storage.exam_dir(course, quiz.chapter_id)
    attempt_path = exam_dir / f"{timestamp}_answers.json"
    payload: dict[str, Any] = {
        "quiz_id": quiz.id,
        "course_id": quiz.course_id,
        "chapter_id": quiz.chapter_id,
        "auto_score": total_auto_score,
        "answers": [item.model_dump(mode="json") for item in answers],
        "created_at": timestamp,
        "note": "客观题已自动判分，主观题请运行 --exam-review 进行 AI 批阅。",
    }
    storage.save_json(attempt_path, payload)

    # Record wrong questions
    if wrong_answers:
        wrong_dir = storage.wrong_questions_dir(course, quiz.chapter_id)
        storage.save_json(wrong_dir / f"{timestamp}_wrong.json", wrong_answers)

    print(f"\n答题记录已保存：{attempt_path}")
    print(f"自动判分（客观题）：{total_auto_score} 分")
    return attempt_path, total_auto_score, wrong_answers


# ---------------------------------------------------------------------------
# Grading
# ---------------------------------------------------------------------------

def grade_answers(
    course: str,
    quiz_id: str,
    answers_path: Path | str,
    ai: BaseAI,
    storage: Storage | None = None,
) -> Attempt:
    storage = storage or Storage()
    # Find quiz file
    exam_dir = storage.exam_dir(course, 1)  # Try all lesson dirs
    # Actually, we need to find the correct exam dir. Let's search.
    qpath = _find_quiz_path(storage, course, quiz_id)
    quiz = storage.read_json(qpath)
    answers = _parse_answers_markdown(Path(answers_path))

    system, prompt = grade_prompt(quiz, [item.model_dump(mode="json") for item in answers])
    payload_text = ai.chat_json(prompt, system=system)
    payload = _extract_json(payload_text)

    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    weak_concepts = [WeakConcept.model_validate(item) for item in payload.get("weak_concepts", [])]
    attempt = Attempt(
        id=timestamp,
        course_id=storage.subject_dir(course).name,
        quiz_id=quiz_id,
        answers=answers,
        total_score=float(payload.get("total_score", 0)),
        question_feedback=[
            QuestionFeedback.model_validate(item) for item in payload.get("question_feedback", [])
        ],
        feedback=str(payload.get("feedback", "")),
        weak_concepts=weak_concepts,
        review_plan=[str(item) for item in payload.get("review_plan", [])],
        retest_recommended=bool(payload.get("retest_recommended", False)),
    )

    chapter_id = int(quiz.get("chapter_id", 0) or 0) or None
    update_weaknesses(
        storage,
        course,
        [item.model_dump(mode="json") for item in weak_concepts],
        chapter=chapter_id,
    )

    exam_dir = _find_quiz_dir(storage, course, quiz_id) or storage.exam_dir(course, chapter_id or 1)
    storage.save_json(exam_dir / f"{timestamp}_feedback.json", attempt)
    storage.save_text(exam_dir / f"{timestamp}_feedback.md", _feedback_to_markdown(attempt))
    return attempt


# ---------------------------------------------------------------------------
# Wrong questions
# ---------------------------------------------------------------------------

def list_wrong_questions(
    subject: str, lesson: int, storage: Storage | None = None
) -> list[dict[str, Any]]:
    storage = storage or Storage()
    wdir = storage.wrong_questions_dir(subject, lesson)
    all_wrong: list[dict[str, Any]] = []
    for f in sorted(wdir.glob("*_wrong.json")):
        data = storage.read_json(f)
        if isinstance(data, list):
            all_wrong.extend(data)
    return all_wrong


def fix_wrong_analysis(
    subject: str,
    lesson: int,
    ai: BaseAI,
    storage: Storage | None = None,
) -> str:
    """Generate AI analysis for all wrong questions of a lesson."""
    storage = storage or Storage()
    wrong_list = list_wrong_questions(subject, lesson, storage)
    if not wrong_list:
        return "该课时暂无错题，继续保持！"

    raw_text = ""
    raw_path = storage.raw_text_path(subject, lesson)
    if raw_path.exists():
        raw_text = storage.read_text(raw_path)

    results: list[str] = [f"# 错题解析：{subject} 第{lesson}课\n"]
    for i, wq in enumerate(wrong_list, start=1):
        system, prompt = fix_wrong_prompt(wq, raw_text)
        analysis = ai.chat(prompt, system=system)
        results.append(f"\n## 错题 {i}\n")
        results.append(f"**原题**：{wq.get('question', '')}\n")
        results.append(f"**你的答案**：{wq.get('user_answer', '')}\n")
        results.append(analysis)

    full_md = "\n".join(results)
    wdir = storage.wrong_questions_dir(subject, lesson)
    storage.save_text(wdir / "fix_wrong.md", full_md)
    return full_md


# ---------------------------------------------------------------------------
# Scores
# ---------------------------------------------------------------------------

def get_scores(subject: str, lesson: int, storage: Storage | None = None) -> list[dict[str, Any]]:
    storage = storage or Storage()
    scores_path = storage.scores_path()
    if not scores_path.exists():
        return []
    all_scores = storage.read_json(scores_path)
    return [
        s for s in all_scores
        if s.get("subject") == subject and s.get("lesson") == lesson
    ]


def record_score(
    subject: str, lesson: int, score: float, total: float, storage: Storage | None = None
) -> None:
    storage = storage or Storage()
    scores_path = storage.scores_path()
    all_scores: list[dict[str, Any]] = []
    if scores_path.exists():
        all_scores = storage.read_json(scores_path)
    all_scores.append({
        "subject": subject,
        "lesson": lesson,
        "score": score,
        "total": total,
        "timestamp": datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
    })
    storage.save_json(scores_path, all_scores)


def export_merged_md(subject: str, lesson: int, storage: Storage | None = None) -> str:
    """Export merged teacher + student notes as single MD file."""
    storage = storage or Storage()
    parts: list[str] = []
    teacher_path = storage.teacher_notes_path(subject, lesson)
    if teacher_path.exists():
        parts.append(storage.read_text(teacher_path))
    student_path = storage.student_notes_path(subject, lesson)
    if student_path.exists():
        parts.append("\n\n---\n\n")
        parts.append(storage.read_text(student_path))
    merged = "\n".join(parts)
    export_path = storage.lesson_dir(subject, lesson) / "export.md"
    storage.save_text(export_path, merged)
    return merged


# ---------------------------------------------------------------------------
# Internal helpers
# ---------------------------------------------------------------------------

def _extract_json(text: str) -> Any:
    """Best-effort JSON extraction from AI output."""
    import re as _re

    cleaned = text.strip()
    fence = _re.search(r"```(?:json)?\s*(.*?)```", cleaned, _re.S | _re.I)
    if fence:
        cleaned = fence.group(1).strip()
    try:
        return json.loads(cleaned)
    except json.JSONDecodeError:
        starts = [i for i in [cleaned.find("{"), cleaned.find("[")] if i >= 0]
        if not starts:
            raise ValueError(f"AI 输出中未找到 JSON 对象。内容：{text[:200]}")
        start = min(starts)
        end = max(cleaned.rfind("}"), cleaned.rfind("]"))
        if end < start:
            raise ValueError("AI 输出中的 JSON 不完整。")
        return json.loads(cleaned[start : end + 1])


def _parse_answers_markdown(path: Path) -> list[Answer]:
    if not path.exists():
        raise StorageError(f"答案文件不存在：{path}")
    text = path.read_text(encoding="utf-8")
    answers: list[Answer] = []
    current_id: str | None = None
    current_lines: list[str] = []
    for line in text.splitlines():
        stripped = line.strip()
        if stripped.lower().startswith(("q", "question")) and ":" in stripped:
            if current_id:
                answers.append(Answer(question_id=current_id, user_answer="\n".join(current_lines).strip()))
            key, value = stripped.split(":", 1)
            current_id = key.strip()
            current_lines = [value.strip()] if value.strip() else []
        elif current_id:
            current_lines.append(line)
    if current_id:
        answers.append(Answer(question_id=current_id, user_answer="\n".join(current_lines).strip()))
    if not answers:
        raise StorageError("未能从答案文件解析出答案。请使用格式：q1: 你的答案")
    return answers


def _feedback_to_markdown(attempt: Attempt) -> str:
    lines = [
        f"# 批阅反馈：{attempt.quiz_id}",
        "",
        f"- 总分：{attempt.total_score}",
        f"- 是否建议二次考试：{'是' if attempt.retest_recommended else '否'}",
        "",
        "## 逐题反馈", "",
    ]
    for item in attempt.question_feedback:
        lines.extend([
            f"### {item.question_id}", "",
            f"- 得分：{item.score}",
            f"- 是否正确：{'是' if item.is_correct else '否'}",
            f"- 反馈：{item.feedback}",
            f"- 正确理解：{item.correct_understanding}",
            f"- 薄弱概念：{'、'.join(item.weak_concepts) or '无'}",
            "",
        ])
    lines.extend(["## 薄弱点", ""])
    for weak in attempt.weak_concepts:
        lines.append(f"- **{weak.concept}**（严重度 {weak.severity}）：{weak.weakness_description}。证据：{weak.evidence}")
    lines.extend(["", "## 复习建议", ""])
    for plan in attempt.review_plan:
        lines.append(f"- {plan}")
    return "\n".join(lines).strip() + "\n"


def _latest_attempt_context(
    storage: Storage, course: str, chapter_id: int
) -> tuple[dict[str, Any] | None, dict[str, Any] | None]:
    exam_dir = storage.exam_dir(course, chapter_id)
    quiz_files = sorted(exam_dir.glob(f"*chapter_{chapter_id:02d}*.json"), reverse=True)
    latest_quiz = storage.read_json(quiz_files[0]) if quiz_files else None

    feedback_files = sorted(exam_dir.glob("*_feedback.json"), reverse=True)
    latest_feedback = storage.read_json(feedback_files[0]) if feedback_files else None
    return latest_quiz, latest_feedback


def _find_quiz_path(storage: Storage, course: str, quiz_id: str) -> Path:
    """Search for a quiz JSON file across all lessons."""
    subject_dir = storage.subject_dir(course)
    if not subject_dir.exists():
        raise StorageError(f"未找到课程：{course}")
    for lesson_dir in sorted(subject_dir.glob("lesson_*")):
        exam_dir = lesson_dir / "exam"
        candidate = exam_dir / f"{quiz_id}.json"
        if candidate.exists():
            return candidate
    # Fallback
    raise StorageError(f"未找到试题文件：{quiz_id}.json")


def _find_quiz_dir(storage: Storage, course: str, quiz_id: str) -> Path | None:
    subject_dir = storage.subject_dir(course)
    if not subject_dir.exists():
        return None
    for lesson_dir in sorted(subject_dir.glob("lesson_*")):
        exam_dir = lesson_dir / "exam"
        candidate = exam_dir / f"{quiz_id}.json"
        if candidate.exists():
            return exam_dir
    return None
