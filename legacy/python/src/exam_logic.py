"""Exam logic — quiz generation, terminal answering, grading, wrong-question tracking.

Now integrates the question-generation policy from src/question_policy.py.
"""

from __future__ import annotations

import json
from datetime import datetime
from pathlib import Path
from typing import Any

from .ai_adapter.base_ai import BaseAI
from .config_loader import get_config
from .memory import (
    compute_chapter_mastery,
    course_weaknesses,
    get_consecutive_stats,
    reduce_weakness_severity,
    update_weaknesses,
)
from .models import (
    Answer,
    Attempt,
    CognitiveLevel,
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
from .question_policy import (
    DEFAULT_QUESTIONS,
    determine_cognitive_distribution,
    determine_difficulty_distribution,
    determine_question_count,
    should_stop_early,
)
from .storage import Storage, StorageError


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

QUIZ_EXIT_COMMANDS = {"0", "q", "quit", "exit", "back", "返回", "退出", "取消"}


class QuizAborted(RuntimeError):
    """Raised when the user exits an in-progress terminal quiz."""


def _is_quiz_exit_command(value: str) -> bool:
    return value.strip().lower() in QUIZ_EXIT_COMMANDS


def _abort_quiz() -> None:
    raise QuizAborted("已退出答题，未保存本次记录。")


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
        lines.append(f"- 认知层次：{q.cognitive_level.value if q.cognitive_level else '未标注'}")
        lines.append(f"- 难度：{q.difficulty}")
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
    question_count: int | None = None,
) -> Quiz:
    """Generate a quiz using policy-driven question count, cognitive distribution,
    and difficulty distribution.

    If question_count is None, the policy determines it automatically.
    """
    storage = storage or Storage()
    raw_text = _load_raw_text(subject, chapter_id, storage)
    student_notes_path = storage.student_notes_path(subject, chapter_id)
    lesson_text = storage.read_text(student_notes_path) if student_notes_path.exists() else ""
    weaknesses = course_weaknesses(storage, subject, chapter_id)

    # ── Compute mastery ──
    mastery = compute_chapter_mastery(storage, subject, chapter_id)

    # ── Get consecutive stats ──
    constats = get_consecutive_stats(storage, subject, chapter_id)

    # ── Determine question count (policy-driven) ──
    if question_count is None:
        # Detect content complexity from length and structure
        content_complexity = _detect_complexity(raw_text, lesson_text)
        question_count = determine_question_count(
            content_complexity=content_complexity,
            consecutive_errors=constats.get("consecutive_wrong", 0),
            consecutive_correct=constats.get("consecutive_correct", 0),
            accuracy=mastery,
            user_override=None,
        )

    # ── Early stop? ──
    if question_count == 0:
        raise StorageError(
            f"🎉 掌握度已达 {mastery:.0%}，连续正确 {constats.get('consecutive_correct', 0)} 题，无需再生成新题。"
            "可以通过 --exam-review 进行二次复习考试。"
        )

    # ── Determine cognitive distribution ──
    cognitive_dist = determine_cognitive_distribution(mastery, question_count)

    # ── Determine difficulty distribution ──
    difficulty_dist = determine_difficulty_distribution(question_count)

    # ── Build the prompt ──
    if retest:
        latest_quiz, latest_feedback = _latest_attempt_context(storage, subject, chapter_id)
        system, prompt = retest_prompt(
            raw_text, weaknesses, latest_quiz, latest_feedback,
            question_count=question_count,
            cognitive_dist={k.value: v for k, v in cognitive_dist.items()},
            difficulty_dist=difficulty_dist,
            mastery=mastery,
            consecutive_stats=constats,
        )
    else:
        system, prompt = quiz_prompt(
            raw_text, lesson_text, weaknesses,
            question_count=question_count,
            cognitive_dist={k.value: v for k, v in cognitive_dist.items()},
            difficulty_dist=difficulty_dist,
            mastery=mastery,
            consecutive_stats=constats,
        )

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

    # ── Validate policy compliance ──
    _validate_quiz_policy(quiz, cognitive_dist, difficulty_dist, question_count)

    save_quiz_files(storage, subject, quiz)

    # ── Print policy summary ──
    _print_policy_summary(quiz, mastery, cognitive_dist, difficulty_dist)

    return quiz


# ---------------------------------------------------------------------------
# CLI quiz
# ---------------------------------------------------------------------------

def run_cli_quiz(storage: Storage, course: str, quiz: Quiz) -> tuple[Path, float, list[dict[str, Any]]]:
    """Interactive terminal quiz. Returns (attempt_path, auto_score, wrong_answers).

    Now tracks consecutive-correct / consecutive-wrong stats for policy feedback.
    """
    answers: list[Answer] = []
    total_auto_score = 0.0
    wrong_answers: list[dict[str, Any]] = []

    print("提示：答题时输入 0/q/返回/退出 可退出答题，不保存本次记录。")

    for index, question in enumerate(quiz.questions, start=1):
        print(f"\n{'='*40}")
        print(f"  [{index}/{len(quiz.questions)}]")
        cognitive_label = f" [{_cognitive_emoji(question.cognitive_level)} {question.cognitive_level.value if question.cognitive_level else '?'}]"
        print(f"  {question.question}{cognitive_label}")
        if question.options:
            for key, value in question.options.items():
                print(f"    {key}. {value}")

        if question.type in (QuestionType.short_answer, QuestionType.case_analysis):
            print("请输入答案，单独一行 END 结束：")
            lines: list[str] = []
            while True:
                line = input()
                if _is_quiz_exit_command(line):
                    _abort_quiz()
                if line.strip().upper() == "END":
                    break
                lines.append(line)
            user_answer = "\n".join(lines).strip()
            print("[提示] 主观题答案已保存，交卷后会自动启动 AI 试卷检测。")
        else:
            hint = "A/B/C/D" if question.type == QuestionType.single_choice else "A,C,D 或 Y/N"
            user_answer = input(f"你的答案（{hint}）：").strip()
            if _is_quiz_exit_command(user_answer):
                _abort_quiz()
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
                    "difficulty": question.difficulty,
                    "cognitive_level": question.cognitive_level.value if question.cognitive_level else None,
                })
            print(f"  {question.explanation}")
        answers.append(Answer(
            question_id=question.id,
            user_answer=user_answer,
        ))

    # ── Compute consecutive stats ──
    consec_correct, consec_wrong = _compute_consecutive(answers, quiz)

    # Save attempt (with stats)
    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    exam_dir = storage.exam_dir(course, quiz.chapter_id)
    attempt_path = exam_dir / f"{timestamp}_answers.json"
    payload: dict[str, Any] = {
        "quiz_id": quiz.id,
        "course_id": quiz.course_id,
        "chapter_id": quiz.chapter_id,
        "auto_score": total_auto_score,
        "total_possible": sum(q.score for q in quiz.questions),
        "answers": [
            {
                "question_id": a.question_id,
                "user_answer": a.user_answer,
                "is_correct": _answer_is_correct(a, quiz),
            }
            for a in answers
        ],
        "consecutive_correct": consec_correct,
        "consecutive_wrong": consec_wrong,
        "created_at": timestamp,
        "note": "客观题已自动判分，交卷后会默认启动 AI 试卷检测。",
        "ai_grading": {
            "status": "pending",
            "retryable": True,
            "created_at": timestamp,
            "message": "等待 AI 试卷检测。",
        },
    }
    storage.save_json(attempt_path, payload)

    # Record wrong questions
    if wrong_answers:
        wrong_dir = storage.wrong_questions_dir(course, quiz.chapter_id)
        storage.save_json(wrong_dir / f"{timestamp}_wrong.json", wrong_answers)

    accuracy = total_auto_score / sum(q.score for q in quiz.questions) if quiz.questions else 0

    print(f"\n答题记录已保存：{attempt_path}")
    print(f"自动判分（客观题）：{total_auto_score} 分")
    print(f"连续正确：{consec_correct} 题 | 连续错误：{consec_wrong} 题")

    # ── Policy feedback ──
    if should_stop_early(accuracy, consec_correct):
        print(f"\n🎉 掌握度达标（{accuracy:.0%}），连续正确 {consec_correct} 题！")
        print("   建议：无需再次生成新题，可通过 --exam-review 进行挑战性复习。")
    elif consec_wrong >= 2:
        print(f"\n⚠️  连续错误 {consec_wrong} 题，下次生成将自动增加 3 道练习题。")

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
    answers_path = Path(answers_path)
    # Find quiz file
    qpath = _find_quiz_path(storage, course, quiz_id)
    quiz = storage.read_json(qpath)
    answers = _parse_answers_file(answers_path, storage)

    system, prompt = grade_prompt(quiz, [item.model_dump(mode="json") for item in answers])
    payload_text = ai.chat_json(prompt, system=system)
    payload = _extract_json(payload_text)

    return apply_grade_payload(course, quiz_id, answers_path, payload, storage)


def apply_grade_payload(
    course: str,
    quiz_id: str,
    answers_path: Path | str,
    payload: dict[str, Any],
    storage: Storage | None = None,
) -> Attempt:
    """Apply a grading JSON payload to a saved attempt."""
    storage = storage or Storage()
    answers_path = Path(answers_path)
    qpath = _find_quiz_path(storage, course, quiz_id)
    quiz = storage.read_json(qpath)
    answers = _parse_answers_file(answers_path, storage)
    timestamp = _attempt_timestamp(answers_path)
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

    # ── Update weaknesses ──
    update_weaknesses(
        storage,
        course,
        [item.model_dump(mode="json") for item in weak_concepts],
        chapter=chapter_id,
    )

    # ── Reduce severity for correctly-answered concepts ──
    for fb in attempt.question_feedback:
        if fb.is_correct:
            for concept in fb.weak_concepts:
                # If feedback says a concept is weak but answer is correct, reduce
                pass
        else:
            for concept in fb.weak_concepts:
                pass  # Already handled by update_weaknesses above

    # Reduce severity for concepts in correctly-answered questions
    for fb in attempt.question_feedback:
        if fb.is_correct and fb.weak_concepts:
            for concept in fb.weak_concepts:
                reduce_weakness_severity(storage, course, concept, chapter=chapter_id, amount=1)

    exam_dir = _find_quiz_dir(storage, course, quiz_id) or storage.exam_dir(course, chapter_id or 1)
    feedback_json_path = storage.save_json(exam_dir / f"{timestamp}_feedback.json", attempt)
    feedback_md_path = storage.save_text(exam_dir / f"{timestamp}_feedback.md", _feedback_to_markdown(attempt))
    _attach_feedback_to_answer_record(
        storage,
        answers_path,
        attempt,
        feedback_json_path,
        feedback_md_path,
    )
    return attempt


def grade_attempt_with_default_ai(
    course: str,
    quiz_id: str,
    answers_path: Path | str,
    storage: Storage | None = None,
    model_override: str | None = None,
) -> Attempt:
    """Grade a saved attempt using the configured OpenAI-compatible provider."""
    from .ai_adapter.claude_api import ClaudeAPI
    from .config_loader import get_config

    cfg = get_config()
    provider_cfg = cfg.provider_config(model_override=model_override)
    provider_cfg.setdefault("max_token", cfg.max_token)
    ai = ClaudeAPI(provider_cfg)
    return grade_answers(course, quiz_id, answers_path, ai, storage)


def grade_attempt_with_claude_cli(
    course: str,
    lesson: int,
    quiz_id: str,
    answers_path: Path | str,
    storage: Storage | None = None,
    model: str = "",
    agent_command: str = "",
    timeout: int = 600,
) -> bool:
    """Queue and process an attempt grading task via Claude Code CLI."""
    from .agent_runner import queue_grade_attempt_task, process_task_via_claude_cli

    storage = storage or Storage()
    task_kwargs = {"model": model}
    if agent_command:
        task_kwargs["agent_command"] = agent_command
    task_id = queue_grade_attempt_task(
        course,
        lesson,
        quiz_id,
        Path(answers_path),
        storage,
        **task_kwargs,
    )
    if agent_command:
        return process_task_via_claude_cli(
            task_id, timeout=timeout, agent_command=agent_command
        )
    return process_task_via_claude_cli(task_id, timeout=timeout)


def mark_grading_failed(storage: Storage, answers_path: Path | str, error: Exception | str) -> None:
    """Record that AI grading failed so the attempt can be retried later."""
    path = Path(answers_path)
    if not path.exists() or path.suffix.lower() != ".json":
        return
    data = storage.read_json(path)
    if not isinstance(data, dict):
        return
    data["ai_grading"] = {
        "status": "failed",
        "retryable": True,
        "error": str(error),
        "failed_at": datetime.now().astimezone().isoformat(timespec="seconds"),
    }
    storage.save_json(path, data)


def list_pending_grading_attempts(
    subject: str,
    lesson: int,
    storage: Storage | None = None,
    include_completed: bool = False,
) -> list[dict[str, Any]]:
    """List answer records that still need AI grading or had grading failures."""
    storage = storage or Storage()
    exam_dir = storage.exam_dir(subject, lesson)
    pending: list[dict[str, Any]] = []
    for path in sorted(exam_dir.glob("*_answers.json"), reverse=True):
        try:
            data = storage.read_json(path)
        except Exception:
            continue
        if not isinstance(data, dict):
            continue
        grading = data.get("ai_grading")
        status = grading.get("status") if isinstance(grading, dict) else "pending"
        feedback_path = path.with_name(path.name.replace("_answers.json", "_feedback.json"))
        if status == "completed" and feedback_path.exists() and not include_completed:
            continue
        pending.append({
            "quiz_id": data.get("quiz_id", ""),
            "course_id": data.get("course_id", subject),
            "chapter_id": data.get("chapter_id", lesson),
            "answers_path": str(path),
            "status": status or "pending",
            "error": grading.get("error", "") if isinstance(grading, dict) else "",
            "created_at": data.get("created_at", ""),
        })
    return pending


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


def _wrong_analysis_output_name(attempt_path: Path | str | None = None) -> str:
    if attempt_path:
        stem = Path(attempt_path).stem
        timestamp = stem[: -len("_answers")] if stem.endswith("_answers") else stem
        return f"{timestamp}_wrong_analysis.md"
    return "fix_wrong.md"


def analyze_wrong_answers_with_claude_cli(
    subject: str,
    lesson: int,
    wrong_answers: list[dict[str, Any]],
    storage: Storage | None = None,
    *,
    agent_command: str = "",
    attempt_path: Path | str | None = None,
    timeout: int = 600,
) -> Path | None:
    """Generate a local Markdown analysis file for the given wrong answers."""
    if not wrong_answers:
        return None

    from .agent_runner import queue_fix_wrong_task, process_task_via_claude_cli

    storage = storage or Storage()
    course_content = ""
    student_path = storage.student_notes_path(subject, lesson)
    if student_path.exists():
        course_content = storage.read_text(student_path)

    output_dir = storage.wrong_questions_dir(subject, lesson)
    output_name = _wrong_analysis_output_name(attempt_path)
    payload = {
        "scope": "本次考试错题" if attempt_path else "当前课时错题",
        "attempt_path": str(attempt_path) if attempt_path else "",
        "wrong_questions": wrong_answers,
    }
    task_id = queue_fix_wrong_task(
        subject,
        lesson,
        payload,
        course_content,
        output_dir,
        agent_command=agent_command,
        output_file=output_name,
    )
    output_path = output_dir / output_name
    success = process_task_via_claude_cli(
        task_id,
        timeout=timeout,
        agent_command=agent_command,
    )
    if success and output_path.exists():
        return output_path
    return None


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
# Quiz history — list, view, switch quizzes
# ---------------------------------------------------------------------------

def list_historical_quizzes(
    subject: str, lesson: int, storage: Storage | None = None
) -> list[dict[str, Any]]:
    """Return metadata for all quizzes of a subject/lesson, newest first.

    Each entry: {id, title, source, question_count, created_at, path}
    """
    storage = storage or Storage()
    exam_dir = storage.exam_dir(subject, lesson)
    quizzes: list[dict[str, Any]] = []

    for f in sorted(exam_dir.glob("*.json"), reverse=True):
        # Skip non-quiz files (answers, feedback, transient output, etc.)
        name = f.name
        if name.endswith("_answers.json") or name.endswith("_feedback.json"):
            continue
        if name == "quiz_result.json":
            continue  # transient output file, not a saved quiz
        if name.endswith("_student.md") or name.endswith("_teacher.md"):
            continue
        try:
            data = storage.read_json(f)
            if not isinstance(data, dict) or "questions" not in data:
                continue
            quizzes.append({
                "id": data.get("id", f.stem),
                "title": data.get("title", f.stem),
                "source": data.get("source", "regular"),
                "question_count": len(data.get("questions", [])),
                "created_at": data.get("created_at", ""),
                "chapter_id": data.get("chapter_id", lesson),
                "path": str(f),
            })
        except Exception:
            pass

    # Sort: newest first by created_at, then filename
    quizzes.sort(key=lambda q: q.get("created_at", ""), reverse=True)
    return quizzes


def load_quiz_by_id(
    subject: str, lesson: int, quiz_id: str, storage: Storage | None = None
) -> Quiz | None:
    """Load a specific quiz by its ID. Returns None if not found."""
    storage = storage or Storage()
    exam_dir = storage.exam_dir(subject, lesson)
    candidate = exam_dir / f"{quiz_id}.json"
    if not candidate.exists():
        # Search by id field inside files
        for f in sorted(exam_dir.glob("*.json")):
            name = f.name
            if name.endswith("_answers.json") or name.endswith("_feedback.json"):
                continue
            try:
                data = storage.read_json(f)
                if isinstance(data, dict) and data.get("id") == quiz_id:
                    candidate = f
                    break
            except Exception:
                pass
    if not candidate.exists():
        return None
    data = storage.read_json(candidate)
    return Quiz.model_validate(data)


def get_latest_quiz(
    subject: str, lesson: int, storage: Storage | None = None
) -> Quiz | None:
    """Return the most recent quiz for a lesson, or None."""
    quizzes = list_historical_quizzes(subject, lesson, storage)
    if not quizzes:
        return None
    return load_quiz_by_id(subject, lesson, quizzes[0]["id"], storage)


# ---------------------------------------------------------------------------
# Custom quiz assembly — pick questions from multiple quizzes
# ---------------------------------------------------------------------------

def list_all_questions(
    subject: str, lesson: int, storage: Storage | None = None
) -> list[dict[str, Any]]:
    """Return every question from every quiz for a subject/lesson.

    Each entry is a question dict with added `_quiz_id` and `_quiz_title`
    so the caller knows which quiz it came from.
    """
    storage = storage or Storage()
    quizzes = list_historical_quizzes(subject, lesson, storage)
    questions: list[dict[str, Any]] = []

    for qmeta in quizzes:
        quiz = load_quiz_by_id(subject, lesson, qmeta["id"], storage)
        if quiz is None:
            continue
        for q in quiz.questions:
            qd = q.model_dump(mode="json")
            qd["_quiz_id"] = quiz.id
            qd["_quiz_title"] = quiz.title
            qd["_quiz_source"] = quiz.source
            questions.append(qd)

    return questions


def assemble_quiz(
    subject: str,
    lesson: int,
    question_ids: list[str],
    title: str = "",
    storage: Storage | None = None,
) -> Quiz | None:
    """Create a new quiz from selected questions across historical quizzes.

    Parameters
    ----------
    question_ids : list of "<quiz_id>:<question_id>" pairs, e.g. ["quiz_chapter_01:q1", "retest_chapter_01_20260605:q3"]
    title : optional custom title
    storage : Storage instance

    Returns
    -------
    Quiz or None if no questions were matched.
    """
    from datetime import datetime

    storage = storage or Storage()
    all_qs = list_all_questions(subject, lesson, storage)

    # Build lookup: "<quiz_id>:<qid>" → question data
    lookup: dict[str, dict[str, Any]] = {}
    for q in all_qs:
        key = f"{q['_quiz_id']}:{q['id']}"
        lookup[key] = q

    # Collect matched questions
    matched: list[QuizQuestion] = []
    source_quizzes: set[str] = set()
    for qid_key in question_ids:
        qid_key = qid_key.strip()
        if qid_key in lookup:
            data = lookup[qid_key]
            source_quizzes.add(data["_quiz_id"])
            # Remove our metadata keys before constructing QuizQuestion
            qdata = {k: v for k, v in data.items() if not k.startswith("_")}
            matched.append(QuizQuestion.model_validate(qdata))
        else:
            # Try partial match: just question id (e.g. "q1")
            partial_matches = [v for k, v in lookup.items() if k.endswith(f":{qid_key}")]
            if len(partial_matches) == 1:
                data = partial_matches[0]
                source_quizzes.add(data["_quiz_id"])
                qdata = {k: v for k, v in data.items() if not k.startswith("_")}
                matched.append(QuizQuestion.model_validate(qdata))

    if not matched:
        return None

    # Renumber questions sequentially
    for i, q in enumerate(matched, start=1):
        q.id = f"q{i}"

    stamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    quiz = Quiz(
        id=f"assembled_chapter_{lesson:02d}_{stamp}",
        course_id=subject,
        chapter_id=lesson,
        title=title or f"组装试卷 — {subject} 第{lesson}课",
        questions=matched,
        mode="markdown",
        source="assembled",
        source_quizzes=sorted(source_quizzes),
    )

    save_quiz_files(storage, subject, quiz)
    return quiz


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


def _parse_answers_file(path: Path, storage: Storage) -> list[Answer]:
    if path.suffix.lower() == ".json":
        return _parse_answers_json(path, storage)
    return _parse_answers_markdown(path)


def _parse_answers_json(path: Path, storage: Storage) -> list[Answer]:
    if not path.exists():
        raise StorageError(f"答案文件不存在：{path}")
    data = storage.read_json(path)
    raw_answers = data.get("answers", []) if isinstance(data, dict) else []
    answers: list[Answer] = []
    for item in raw_answers:
        question_id = str(item.get("question_id", "")).strip()
        if not question_id:
            continue
        answers.append(Answer(
            question_id=question_id,
            user_answer=str(item.get("user_answer", "")).strip(),
        ))
    if not answers:
        raise StorageError("未能从 JSON 答案文件解析出答案。")
    return answers


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


def _attempt_timestamp(path: Path) -> str:
    stem = path.stem
    if stem.endswith("_answers"):
        return stem[: -len("_answers")]
    return datetime.now().strftime("%Y%m%d_%H%M%S")


def _attach_feedback_to_answer_record(
    storage: Storage,
    answers_path: Path,
    attempt: Attempt,
    feedback_json_path: Path,
    feedback_md_path: Path,
) -> None:
    if answers_path.suffix.lower() != ".json" or not answers_path.exists():
        return
    data = storage.read_json(answers_path)
    if not isinstance(data, dict):
        return
    feedback_by_question = {
        item.question_id: item for item in attempt.question_feedback
    }
    for answer in data.get("answers", []):
        if not isinstance(answer, dict):
            continue
        fb = feedback_by_question.get(str(answer.get("question_id", "")))
        if fb is None:
            continue
        answer["ai_is_correct"] = fb.is_correct
        answer["ai_score"] = fb.score
        answer["ai_feedback"] = fb.feedback
        answer["correct_understanding"] = fb.correct_understanding
        answer["weak_concepts"] = fb.weak_concepts
    data["ai_grading"] = {
        "status": "completed",
        "total_score": attempt.total_score,
        "feedback": attempt.feedback,
        "question_feedback": [
            item.model_dump(mode="json") for item in attempt.question_feedback
        ],
        "weak_concepts": [
            item.model_dump(mode="json") for item in attempt.weak_concepts
        ],
        "review_plan": attempt.review_plan,
        "retest_recommended": attempt.retest_recommended,
        "feedback_json_path": str(feedback_json_path),
        "feedback_md_path": str(feedback_md_path),
        "graded_at": datetime.now().astimezone().isoformat(timespec="seconds"),
    }
    storage.save_json(answers_path, data)


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


def _detect_complexity(raw_text: str, lesson_text: str) -> str:
    """Heuristic to classify content complexity.

    Returns "simple", "normal", or "complex".
    """
    combined = raw_text + "\n" + lesson_text
    length = len(combined)

    # Count indicators of complexity
    math_indicators = sum(1 for kw in ["推导", "证明", "公式", "定理", "计算", "微分", "积分"]
                          if kw in combined)
    step_indicators = sum(1 for kw in ["步骤", "流程", "方法", "步骤一", "步骤二", "首先", "然后", "接着"]
                          if kw in combined)
    concept_count = combined.count("概念") + combined.count("定义")

    score = math_indicators * 2 + step_indicators + concept_count

    if length < 3000 and score < 5:
        return "simple"
    elif score >= 8 or length > 15000:
        return "complex"
    else:
        return "normal"


def _validate_quiz_policy(
    quiz: Quiz,
    cognitive_dist: dict[CognitiveLevel, int],
    difficulty_dist: dict[str, int],
    question_count: int,
) -> None:
    """Soft-validate that the generated quiz roughly matches the policy.

    Prints warnings (does not fail) since AI generation is inherently
    approximate.
    """
    actual_count = len(quiz.questions)

    # Count cognitive levels
    level_counts: dict[str, int] = {}
    for q in quiz.questions:
        if q.cognitive_level:
            level_counts[q.cognitive_level.value] = level_counts.get(q.cognitive_level.value, 0) + 1

    # Count difficulties
    diff_counts: dict[str, int] = {}
    for q in quiz.questions:
        diff_counts[q.difficulty] = diff_counts.get(q.difficulty, 0) + 1

    if actual_count != question_count:
        print(f"[策略校验] ⚠️ 题目数量：期望 {question_count}，实际 {actual_count}")


def _print_policy_summary(
    quiz: Quiz,
    mastery: float,
    cognitive_dist: dict[CognitiveLevel, int],
    difficulty_dist: dict[str, int],
) -> None:
    """Print a human-readable summary of the applied policy."""
    print(f"\n{'─'*50}")
    print(f"📋 题目生成策略摘要")
    print(f"{'─'*50}")
    tier = "高（>85%）" if mastery > 0.85 else "中（60%-85%）" if mastery >= 0.60 else "低（<60%）" if mastery > 0 else "首次"
    print(f"  掌握度：{mastery:.0%}（{tier}）")
    print(f"  共 {len(quiz.questions)} 题")
    print(f"  认知层次：{' | '.join(f'{lv.value}={ct}' for lv, ct in cognitive_dist.items() if ct > 0)}")
    print(f"  难度分布：{' | '.join(f'{k}={v}' for k, v in difficulty_dist.items() if v > 0)}")
    print(f"{'─'*50}\n")


def _cognitive_emoji(level: CognitiveLevel | None) -> str:
    if level is None:
        return "📝"
    emoji_map = {
        CognitiveLevel.recall: "🧠",
        CognitiveLevel.explain: "💡",
        CognitiveLevel.apply: "🔧",
        CognitiveLevel.transfer: "🚀",
        CognitiveLevel.reflect: "🤔",
    }
    return emoji_map.get(level, "📝")


def _answer_is_correct(answer: Answer, quiz: Quiz) -> bool | None:
    """Determine if an answer is correct based on quiz question type.

    Returns None for subjective questions (needs AI grading).
    """
    question = next((q for q in quiz.questions if q.id == answer.question_id), None)
    if question is None:
        return None
    if question.type in (QuestionType.short_answer, QuestionType.case_analysis):
        return None  # Needs AI grading
    return is_objective_correct(question, answer.user_answer)


def _compute_consecutive(answers: list[Answer], quiz: Quiz) -> tuple[int, int]:
    """Compute consecutive correct and wrong from the end of the quiz."""
    consec_correct = 0
    consec_wrong = 0

    for ans in reversed(answers):
        correct = _answer_is_correct(ans, quiz)
        if correct is True:
            consec_correct += 1
            if consec_wrong > 0:
                break
        elif correct is False:
            consec_wrong += 1
            if consec_correct > 0:
                break
        # None (subjective) breaks the streak
        else:
            break

    return consec_correct, consec_wrong
