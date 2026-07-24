"""Golden-fixture driven regression tests for the legacy deterministic rules.

Fixtures live at the backend/testdata/legacy-fixtures/ and freeze
the CURRENT behaviour of question policy, objective grading, weakness
tracking and mastery. The same JSON files are the parity target for the Go
migration (B4), so expectations are only changed intentionally — see
backend/testdata/legacy-fixtures/README.md.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from src import exam_logic, memory, question_policy  # noqa: E402
from src.models import Attempt, CognitiveLevel, LessonInfo, Quiz, QuizQuestion, Weakness  # noqa: E402
from src.storage import Storage, slugify  # noqa: E402

FIXTURE_ROOT = Path(__file__).resolve().parents[3] / "backend" / "testdata" / "legacy-fixtures"
COURSE_FIXTURE = FIXTURE_ROOT / "course-minimal"


def load_golden(name: str) -> dict:
    path = FIXTURE_ROOT / name
    assert path.exists(), f"缺少 golden fixture：{path}"
    return json.loads(path.read_text(encoding="utf-8"))


def strip_timestamps(records: list[dict]) -> list[dict]:
    return [{k: v for k, v in r.items() if k not in ("first_seen", "last_seen")} for r in records]


# ---------------------------------------------------------------------------
# question-policy.golden.json
# ---------------------------------------------------------------------------

policy = load_golden("question-policy.golden.json")


@pytest.mark.parametrize("case", policy["question_count"]["cases"], ids=lambda c: c["name"])
def test_question_count(case):
    assert question_policy.determine_question_count(**case["input"]) == case["expect"]


@pytest.mark.parametrize("case", policy["cognitive_distribution"]["cases"], ids=lambda c: c["name"])
def test_cognitive_distribution(case):
    dist = question_policy.determine_cognitive_distribution(**case["input"])
    actual = {lv: dist.get(CognitiveLevel(lv), 0) for lv in policy["cognitive_distribution"]["levels"]}
    assert actual == case["expect"]
    assert sum(actual.values()) == case["input"]["question_count"]


@pytest.mark.parametrize("case", policy["difficulty_distribution"]["cases"], ids=lambda c: c["name"])
def test_difficulty_distribution(case):
    dist = question_policy.determine_difficulty_distribution(case["input"]["question_count"])
    assert dist == case["expect"]
    assert sum(dist.values()) == case["input"]["question_count"]


@pytest.mark.parametrize("case", policy["weighted_mastery"]["cases"], ids=lambda c: c["name"])
def test_weighted_mastery(case):
    assert question_policy.compute_mastery(case["input"]["scores"]) == case["expect"]


@pytest.mark.parametrize("case", policy["detect_complexity"]["cases"], ids=lambda c: c["name"])
def test_detect_complexity(case):
    assert exam_logic._detect_complexity(**case["input"]) == case["expect"]


# ---------------------------------------------------------------------------
# objective-grading.golden.json
# ---------------------------------------------------------------------------

grading = load_golden("objective-grading.golden.json")


@pytest.mark.parametrize("case", grading["normalize"]["cases"], ids=lambda c: c["name"])
def test_normalize_objective_answer(case):
    assert exam_logic.normalize_objective_answer(case["input"]) == case["expect"]


@pytest.mark.parametrize("case", grading["is_objective_correct"]["cases"], ids=lambda c: c["name"])
def test_is_objective_correct(case):
    q = case["input"]["question"]
    question = QuizQuestion.model_validate({
        "id": "q-fixture",
        "type": q["type"],
        "question": "fixture 题干",
        "options": q["options"],
        "correct_answer": q["correct_answer"],
        "explanation": "fixture 解析",
    })
    assert exam_logic.is_objective_correct(question, case["input"]["user_answer"]) == case["expect"]


# ---------------------------------------------------------------------------
# weakness.golden.json
# ---------------------------------------------------------------------------

weakness = load_golden("weakness.golden.json")


@pytest.mark.parametrize("case", weakness["id"]["cases"], ids=lambda c: c["name"])
def test_weakness_id(case):
    assert memory.weakness_id(**case["input"]) == case["expect"]


def _seed(storage: Storage, records: list[dict]) -> None:
    if records:
        storage.save_json(storage.weaknesses_path(), records)


@pytest.mark.parametrize("case", weakness["update"]["cases"], ids=lambda c: c["name"])
def test_update_weaknesses(case, tmp_path):
    storage = Storage(tmp_path)
    _seed(storage, case["input"]["initial_weaknesses"])
    defaults = weakness["update"]["batch_defaults"]
    result = []
    for batch in case["input"]["batches"]:
        result = memory.update_weaknesses(
            storage,
            batch.get("course", defaults["course"]),
            batch["concepts"],
            chapter=batch.get("chapter", defaults["chapter"]),
        )
    assert strip_timestamps([w.model_dump(mode="json") for w in result]) == case["expect"]


@pytest.mark.parametrize("case", weakness["reduce"]["cases"], ids=lambda c: c["name"])
def test_reduce_weakness_severity(case, tmp_path):
    storage = Storage(tmp_path)
    _seed(storage, case["input"]["initial_weaknesses"])
    defaults = weakness["reduce"]["defaults"]
    returns = [
        memory.reduce_weakness_severity(
            storage,
            defaults["course"],
            op["concept"],
            chapter=defaults["chapter"],
            amount=op.get("amount", defaults["amount"]),
        )
        for op in case["input"]["ops"]
    ]
    final = json.loads(storage.weaknesses_path().read_text(encoding="utf-8"))
    assert returns == case["expect"]["returns"]
    assert strip_timestamps(final) == case["expect"]["weaknesses"]


@pytest.mark.parametrize("case", weakness["resolve"]["cases"], ids=lambda c: c["name"])
def test_mark_weakness_resolved(case, tmp_path):
    storage = Storage(tmp_path)
    _seed(storage, case["input"]["initial_weaknesses"])
    defaults = weakness["resolve"]["defaults"]
    ret = memory.mark_weakness_resolved(
        storage, defaults["course"], case["input"]["concept"], chapter=case["input"]["chapter"],
    )
    final = json.loads(storage.weaknesses_path().read_text(encoding="utf-8"))
    assert ret == case["expect"]["return"]
    assert strip_timestamps(final) == case["expect"]["weaknesses"]


# ---------------------------------------------------------------------------
# mastery.golden.json
# ---------------------------------------------------------------------------

mastery = load_golden("mastery.golden.json")


@pytest.mark.parametrize("case", mastery["chapter_mastery"]["cases"], ids=lambda c: c["name"])
def test_chapter_mastery(case, tmp_path):
    storage = Storage(tmp_path)
    if case["input"]["scores"]:
        storage.save_json(storage.scores_path(), case["input"]["scores"])
    if case["input"]["weaknesses"]:
        storage.save_json(storage.weaknesses_path(), case["input"]["weaknesses"])
    actual = memory.compute_chapter_mastery(storage, case["input"]["course"], case["input"]["chapter"])
    assert actual == case["expect"]


@pytest.mark.parametrize("case", mastery["consecutive_stats"]["cases"], ids=lambda c: c["name"])
def test_consecutive_stats(case, tmp_path):
    storage = Storage(tmp_path)
    exam_dir = storage.exam_dir(case["input"]["course"], case["input"]["chapter"])
    for item in case["input"]["attempt_files"]:
        storage.save_json(exam_dir / item["name"], item["data"])
    actual = memory.get_consecutive_stats(storage, case["input"]["course"], case["input"]["chapter"])
    assert actual == case["expect"]


# ---------------------------------------------------------------------------
# course-minimal fixture: schema faithfulness + privacy guards
# ---------------------------------------------------------------------------

LESSON01 = COURSE_FIXTURE / "course" / "demo-calculus" / "lesson_01"


def test_course_fixture_quiz_matches_schema():
    quiz = Quiz.model_validate(json.loads((LESSON01 / "exam" / "quiz_chapter_01.json").read_text(encoding="utf-8")))
    assert [q.id for q in quiz.questions] == ["q1", "q2", "q3", "q4"]
    assert quiz.source == "regular"


def test_course_fixture_answers_parse_and_grading_status_known():
    storage = Storage(COURSE_FIXTURE / "course")
    completed = exam_logic._parse_answers_json(LESSON01 / "exam" / "20260601_100000_answers.json", storage)
    failed = exam_logic._parse_answers_json(LESSON01 / "exam" / "20260603_100000_answers.json", storage)
    assert len(completed) == len(failed) == 4
    for name, status in [("20260601_100000", "completed"), ("20260603_100000", "failed")]:
        data = json.loads((LESSON01 / "exam" / f"{name}_answers.json").read_text(encoding="utf-8"))
        assert data["ai_grading"]["status"] == status


def test_course_fixture_feedback_is_consistent():
    quiz = Quiz.model_validate(json.loads((LESSON01 / "exam" / "quiz_chapter_01.json").read_text(encoding="utf-8")))
    attempt = Attempt.model_validate(
        json.loads((LESSON01 / "exam" / "20260601_100000_feedback.json").read_text(encoding="utf-8"))
    )
    quiz_qids = {q.id for q in quiz.questions}
    assert {fb.question_id for fb in attempt.question_feedback} <= quiz_qids
    assert {a.question_id for a in attempt.answers} <= quiz_qids
    per_question = sum(fb.score for fb in attempt.question_feedback)
    assert per_question == pytest.approx(attempt.total_score)


def test_course_fixture_weaknesses_self_consistent():
    records = [Weakness.model_validate(item) for item in json.loads(
        (COURSE_FIXTURE / "course" / "weaknesses.json").read_text(encoding="utf-8")
    )]
    assert records, "weaknesses.json 不应为空"
    for w in records:
        assert w.id == memory.weakness_id(w.course, w.chapter, w.concept)
        assert 1 <= w.severity <= 5


def test_course_fixture_scores_and_info_match_schema():
    scores = json.loads((COURSE_FIXTURE / "course" / "scores.json").read_text(encoding="utf-8"))
    for s in scores:
        assert {"subject", "lesson", "score", "total", "timestamp"} <= set(s)
    for lesson in (1, 2):
        info = LessonInfo.model_validate(json.loads(
            (COURSE_FIXTURE / "course" / "demo-calculus" / f"lesson_{lesson:02d}" / "info.json").read_text(encoding="utf-8")
        ))
        assert slugify(info.subject) == "demo-calculus"
        assert info.lesson == lesson


def test_course_fixture_wrong_questions_reference_quiz():
    quiz = Quiz.model_validate(json.loads((LESSON01 / "exam" / "quiz_chapter_01.json").read_text(encoding="utf-8")))
    wrong = json.loads((LESSON01 / "wrong_questions" / "20260603_100000_wrong.json").read_text(encoding="utf-8"))
    assert isinstance(wrong, list) and wrong
    assert {item["question_id"] for item in wrong} <= {q.id for q in quiz.questions}


def test_course_fixture_tasks_match_agent_format_without_host_paths():
    tasks = sorted((COURSE_FIXTURE / ".tasks").glob("*/*.json"))
    assert tasks, ".tasks fixture 不应为空"
    required = {"task_id", "task_type", "subject", "lesson", "status", "created_at", "completed_at", "error"}
    for path in tasks:
        task = json.loads(path.read_text(encoding="utf-8"))
        assert required <= set(task), f"{path.name} 缺少任务字段"
        assert task["status"] in {"pending", "processing", "completed", "failed"}
        assert not Path(task["output_dir"]).is_absolute(), f"{path.name} 不得包含宿主机绝对路径"
        assert path.stem == task["task_id"]


def test_fixtures_contain_no_secrets_or_host_paths():
    forbidden = ["sk-ant-", "ANTHROPIC_API_KEY", "/Users/", "PRIVATE KEY", "postgres_password"]
    for path in FIXTURE_ROOT.rglob("*"):
        if not path.is_file():
            continue
        text = path.read_text(encoding="utf-8")
        for token in forbidden:
            assert token not in text, f"{path.relative_to(FIXTURE_ROOT)} 含敏感内容：{token}"
