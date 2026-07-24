"""Generate the frozen golden fixtures under backend/testdata/legacy-fixtures/.

The golden JSON files freeze the CURRENT deterministic behaviour of the
legacy Python implementation (question policy, objective grading, weakness
tracking, mastery). They are the shared behaviour baseline consumed by:

- legacy/python/tests/test_golden_fixtures.py  (regression gate, this repo)
- the future Go implementation in internal/quiz/policy etc. (B4 parity)

Re-run this script ONLY when a behaviour change is intentional and reviewed
(see backend/testdata/legacy-fixtures/README.md). Never read legacy-data/ here:
every case below is synthetic and desensitized.

Usage (from legacy/python/):
    uv run --extra dev python tools/generate_golden_fixtures.py
"""

from __future__ import annotations

import json
import sys
import tempfile
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from src import exam_logic, memory, question_policy  # noqa: E402
from src.models import CognitiveLevel  # noqa: E402
from src.storage import Storage  # noqa: E402

FIXTURE_ROOT = Path(__file__).resolve().parents[3] / "backend" / "testdata" / "legacy-fixtures"

COURSE = "demo-calculus"
CHAPTER = 1

LEVELS = ["recall", "explain", "apply", "transfer", "reflect"]
DIFFICULTIES = ["easy", "medium", "hard", "challenge"]


def dump(name: str, payload: dict) -> None:
    path = FIXTURE_ROOT / name
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(
        json.dumps(payload, ensure_ascii=False, indent=2, sort_keys=False) + "\n",
        encoding="utf-8",
    )
    print(f"wrote {path.relative_to(FIXTURE_ROOT.parents[1])}")


# ---------------------------------------------------------------------------
# question-policy.golden.json
# ---------------------------------------------------------------------------

def gen_question_policy() -> dict:
    count_cases = [
        ("default_normal", {}),
        ("simple_content", {"content_complexity": "simple"}),
        ("complex_content", {"content_complexity": "complex"}),
        ("unknown_complexity_falls_back_to_default", {"content_complexity": "weird"}),
        ("consecutive_errors_add_3", {"consecutive_errors": 2}),
        ("consecutive_errors_add_3_clamped_to_max", {"content_complexity": "complex", "consecutive_errors": 5}),
        ("min_clamp_simple", {"content_complexity": "simple"}),
        ("early_stop_when_mastered_returns_0", {"accuracy": 0.85, "consecutive_correct": 3}),
        ("early_stop_boundary_accuracy_below", {"accuracy": 0.84, "consecutive_correct": 3}),
        ("early_stop_boundary_streak_below", {"accuracy": 0.85, "consecutive_correct": 2}),
        ("early_stop_ignores_consecutive_errors", {"accuracy": 0.90, "consecutive_correct": 3, "consecutive_errors": 2}),
        ("override_within_range", {"user_override": 12}),
        ("override_clamped_to_min", {"user_override": 3}),
        ("override_clamped_to_max", {"user_override": 20}),
        ("override_beats_early_stop", {"user_override": 7, "accuracy": 0.95, "consecutive_correct": 5}),
    ]
    question_count = [
        {
            "name": name,
            "input": {
                "content_complexity": kw.get("content_complexity", "normal"),
                "consecutive_errors": kw.get("consecutive_errors", 0),
                "consecutive_correct": kw.get("consecutive_correct", 0),
                "accuracy": kw.get("accuracy", 0.0),
                "user_override": kw.get("user_override"),
            },
            "expect": question_policy.determine_question_count(**kw),
        }
        for name, kw in count_cases
    ]

    cognitive_cases = []
    for mastery in [0.0, 0.40, 0.59, 0.60, 0.75, 0.85, 0.86, 1.0]:
        for count in [5, 8, 10, 15]:
            dist = question_policy.determine_cognitive_distribution(mastery, count)
            cognitive_cases.append({
                "name": f"mastery_{mastery:.2f}_count_{count}",
                "input": {"mastery": mastery, "question_count": count},
                "expect": {lv: dist.get(CognitiveLevel(lv), 0) for lv in LEVELS},
            })

    difficulty_cases = [
        {
            "name": f"count_{count}",
            "input": {"question_count": count},
            "expect": question_policy.determine_difficulty_distribution(count),
        }
        for count in range(5, 16)
    ]

    mastery_inputs = [
        ("empty_history", []),
        ("single_perfect_score", [{"score": 10, "total": 10, "timestamp": "2026-01-01"}]),
        ("single_zero_score", [{"score": 0, "total": 10, "timestamp": "2026-01-01"}]),
        ("recent_scores_weighted_heavier", [
            {"score": 0, "total": 10, "timestamp": "2026-01-01"},
            {"score": 10, "total": 10, "timestamp": "2026-01-03"},
            {"score": 10, "total": 10, "timestamp": "2026-01-02"},
        ]),
        ("zero_total_records_skipped", [
            {"score": 5, "total": 0, "timestamp": "2026-01-01"},
            {"score": 10, "total": 10, "timestamp": "2026-01-02"},
        ]),
        ("all_zero_totals_yield_zero", [
            {"score": 5, "total": 0, "timestamp": "2026-01-01"},
            {"score": 3, "total": 0, "timestamp": "2026-01-02"},
        ]),
        ("missing_timestamp_sorts_first", [
            {"score": 6, "total": 10},
            {"score": 8, "total": 10, "timestamp": "2026-01-01"},
        ]),
        ("fractional_scores", [
            {"score": 7.5, "total": 10, "timestamp": "2026-01-01"},
            {"score": 8.5, "total": 12, "timestamp": "2026-01-02"},
            {"score": 3, "total": 6, "timestamp": "2026-01-03"},
        ]),
    ]
    mastery_cases = [
        {"name": name, "input": {"scores": scores}, "expect": question_policy.compute_mastery(scores)}
        for name, scores in mastery_inputs
    ]

    complexity_inputs = [
        (
            "simple_short_plain_text",
            "本课介绍函数极限的直观含义：当自变量接近某个值时，函数值呈现的变化趋势。",
            "极限描述趋势。",
        ),
        (
            "normal_method_and_concept_text",
            "本课讲解求极限的常用方法与步骤。首先介绍代入法，然后介绍因式分解法，接着讨论有理化方法。",
            "极限的概念与连续的定义是本课重点。",
        ),
        (
            "complex_proof_and_math_text",
            "本课给出极限的严格定义，包含 ε-δ 推导与证明。涉及重要公式与定理，需要计算与微分、积分基础。",
            "定理的证明与公式的推导是难点。",
        ),
    ]
    complexity_cases = [
        {
            "name": name,
            "input": {"raw_text": raw, "lesson_text": lesson},
            "expect": exam_logic._detect_complexity(raw, lesson),
        }
        for name, raw, lesson in complexity_inputs
    ]

    return {
        "schema_version": 1,
        "domain": "question-policy",
        "source_module": "legacy/python/src/question_policy.py (+ exam_logic._detect_complexity)",
        "notes": [
            "determine_question_count 返回 0 表示策略判定无需继续练习(practice_not_needed)。",
            "认知层次/难度分布使用最大余数法，expect 为精确整数分配，总和恒等于 question_count。",
        ],
        "question_count": {
            "function": "determine_question_count(content_complexity, consecutive_errors, consecutive_correct, accuracy, user_override)",
            "cases": question_count,
        },
        "cognitive_distribution": {
            "function": "determine_cognitive_distribution(mastery, question_count)",
            "levels": LEVELS,
            "cases": cognitive_cases,
        },
        "difficulty_distribution": {
            "function": "determine_difficulty_distribution(question_count)",
            "difficulties": DIFFICULTIES,
            "cases": difficulty_cases,
        },
        "weighted_mastery": {
            "function": "compute_mastery(scores)",
            "float_compare": "JSON 保存完整双精度；跨语言比较建议使用 1e-12 绝对容差。",
            "cases": mastery_cases,
        },
        "detect_complexity": {
            "function": "exam_logic._detect_complexity(raw_text, lesson_text)",
            "cases": complexity_cases,
        },
    }


# ---------------------------------------------------------------------------
# objective-grading.golden.json
# ---------------------------------------------------------------------------

def gen_objective_grading() -> dict:
    normalize_inputs = [
        ("single_letter_upper", "a"),
        ("surrounding_whitespace", "  b  "),
        ("chinese_comma", "a，b"),
        ("mixed_separators_and_case", "a, b ,C"),
        ("empty_string", ""),
        ("only_separators", "，, ，"),
        ("multi_letter_token", "ab"),
        ("empty_segments_dropped", "a,,b,"),
        ("chinese_boolean_untouched", "对"),
        ("already_normalized", "A,B,C"),
    ]
    normalize_cases = [
        {"name": name, "input": value, "expect": exam_logic.normalize_objective_answer(value)}
        for name, value in normalize_inputs
    ]

    grading_inputs = [
        # single_choice
        ("single_choice_case_insensitive", "single_choice", "B", "b"),
        ("single_choice_wrong_option", "single_choice", "B", "A"),
        ("single_choice_whitespace_tolerated", "single_choice", "B", "  B "),
        # multiple_choice
        ("multiple_choice_exact_order", "multiple_choice", ["A", "C"], "A,C"),
        ("multiple_choice_reordered_is_wrong_legacy_quirk", "multiple_choice", ["A", "C"], "C,A"),
        ("multiple_choice_lowercase", "multiple_choice", ["A", "C"], "a,c"),
        ("multiple_choice_trailing_comma", "multiple_choice", ["A", "C"], "A,C,"),
        ("multiple_choice_duplicate_is_wrong_legacy_quirk", "multiple_choice", ["A", "C"], "A,A,C"),
        ("multiple_choice_partial_is_wrong", "multiple_choice", ["A", "C"], "A"),
        ("multiple_choice_correct_as_joined_string", "multiple_choice", "A,C", "A，C"),
        # true_false
        ("true_false_chinese_pair", "true_false", "对", "对"),
        ("true_false_user_yes_maps_to_chinese_correct", "true_false", "对", "Y"),
        ("true_false_user_no_maps_to_chinese_correct", "true_false", "错", "n"),
        ("true_false_english_correct_user_yes", "true_false", "TRUE", "yes"),
        ("true_false_mismatch", "true_false", "FALSE", "对"),
        ("true_false_unmapped_token_passes_through", "true_false", "T", "t"),
        # subjective types are never auto-graded
        ("short_answer_needs_ai", "short_answer", "极限是趋势", "极限是趋势"),
        ("case_analysis_needs_ai", "case_analysis", "见解析", "见解析"),
    ]
    grading_cases = []
    for name, qtype, correct, user in grading_inputs:
        question = {
            "id": "q-fixture",
            "type": qtype,
            "question": "fixture 题干",
            "options": {"A": "甲", "B": "乙", "C": "丙"} if "choice" in qtype else None,
            "correct_answer": correct,
            "explanation": "fixture 解析",
        }
        from src.models import QuizQuestion

        result = exam_logic.is_objective_correct(QuizQuestion.model_validate(question), user)
        grading_cases.append({
            "name": name,
            "input": {
                "question": {
                    "type": qtype,
                    "correct_answer": correct,
                    "options": question["options"],
                },
                "user_answer": user,
            },
            "expect": result,
        })

    return {
        "schema_version": 1,
        "domain": "objective-grading",
        "source_module": "legacy/python/src/exam_logic.py",
        "notes": [
            "多选按规范化后的字符串逐项比较：不去重、不排序，顺序或重复不同即判错（legacy 现状，golden 如实冻结）。",
            "03-backend-architecture.md §10.2 的目标行为（多选集合比较）与旧实现不同，B4 迁移时必须显式决策并记录。",
            "short_answer/case_analysis 恒为 False：需要 AI 批阅，本地不判分。",
        ],
        "normalize": {
            "function": "normalize_objective_answer(value)",
            "cases": normalize_cases,
        },
        "is_objective_correct": {
            "function": "is_objective_correct(question, user_answer)",
            "cases": grading_cases,
        },
    }


# ---------------------------------------------------------------------------
# weakness.golden.json
# ---------------------------------------------------------------------------

def _weakness_record(w) -> dict:
    data = w.model_dump(mode="json")
    del data["first_seen"]
    del data["last_seen"]
    return data


def gen_weakness() -> dict:
    id_inputs = [
        ("basic", COURSE, 1, "极限"),
        ("chapter_none", COURSE, None, "极限"),
        ("chapter_zero_same_as_none_legacy_quirk", COURSE, 0, "极限"),
        ("concept_case_and_whitespace_insensitive", COURSE, 1, "  极限  "),
        ("different_concept", COURSE, 1, "连续性"),
        ("different_course", "demo-linear-algebra", 1, "极限"),
    ]
    id_cases = [
        {
            "name": name,
            "input": {"course": course, "chapter": chapter, "concept": concept},
            "expect": memory.weakness_id(course, chapter, concept),
        }
        for name, course, chapter, concept in id_inputs
    ]

    tmp = Path(tempfile.mkdtemp(prefix="golden-weakness-"))

    def seed(storage: Storage, records: list[dict]) -> None:
        if records:
            storage.save_json(storage.weaknesses_path(), records)

    base_record = {
        "id": memory.weakness_id(COURSE, 1, "极限"),
        "course": COURSE,
        "chapter": 1,
        "concept": "极限",
        "weakness_description": "不理解 ε-δ 定义",
        "evidence": "答错选择题",
        "severity": 4,
        "first_seen": "2026-01-01T09:00:00+08:00",
        "last_seen": "2026-01-01T09:00:00+08:00",
        "review_count": 1,
        "mastered": False,
    }

    update_specs = [
        (
            "create_new_concept",
            [],
            [{"concepts": [{"concept": "极限", "severity": 4, "evidence": "答错选择题", "weakness_description": "不理解 ε-δ 定义"}]}],
        ),
        (
            "merge_keeps_max_severity_and_appends_evidence",
            [base_record],
            [{"concepts": [{"concept": "极限", "severity": 2, "evidence": "答错判断题", "weakness_description": "对 ε-δ 仍不熟练"}]}],
        ),
        (
            "merge_resets_mastered_flag",
            [dict(base_record, mastered=True, severity=1, review_count=5)],
            [{"concepts": [{"concept": "极限", "severity": 2, "evidence": "再次答错", "weakness_description": ""}]}],
        ),
        (
            "severity_clamped_to_5",
            [],
            [{"concepts": [{"concept": "极限", "severity": 9}]}],
        ),
        (
            "severity_zero_becomes_default_3_legacy_quirk",
            [],
            [{"concepts": [{"concept": "极限", "severity": 0}]}],
        ),
        (
            "severity_string_coerced",
            [],
            [{"concepts": [{"concept": "极限", "severity": "4"}]}],
        ),
        (
            "empty_concept_skipped",
            [],
            [{"concepts": [{"concept": "   ", "severity": 4}, {"concept": "连续性", "severity": 2}]}],
        ),
        (
            "result_sorted_by_course_chapter_concept",
            [],
            [{
                "concepts": [
                    {"concept": "导数", "severity": 2},
                    {"concept": "极限", "severity": 3},
                ],
                "chapter": 2,
            }, {
                "concepts": [{"concept": "矩阵", "severity": 1}],
                "course": "demo-linear-algebra",
                "chapter": None,
            }],
        ),
    ]
    update_cases = []
    for name, initial, batches in update_specs:
        storage = Storage(tmp / f"upd-{name}")
        seed(storage, initial)
        for batch in batches:
            result = memory.update_weaknesses(
                storage,
                batch.get("course", COURSE),
                batch["concepts"],
                chapter=batch.get("chapter", 1) if "chapter" in batch or "course" not in batch else batch.get("chapter"),
            )
        update_cases.append({
            "name": name,
            "input": {"initial_weaknesses": initial, "batches": batches},
            "expect": [_weakness_record(w) for w in result],
        })

    reduce_specs = [
        ("reduce_above_floor_not_mastered", dict(base_record, severity=3, review_count=1), [{"concept": "极限"}], ),
        ("reduce_to_one_masters_with_two_reviews", dict(base_record, severity=2, review_count=1), [{"concept": "极限"}]),
        ("reduce_at_floor_still_masters_with_two_reviews", dict(base_record, severity=1, review_count=1), [{"concept": "极限"}]),
        ("reduce_missing_concept_returns_false", base_record, [{"concept": "不存在"}]),
        ("reduce_with_amount_two", dict(base_record, severity=5, review_count=1), [{"concept": "极限", "amount": 2}]),
        ("second_reduce_after_mastered_stays_mastered", dict(base_record, severity=2, review_count=1), [{"concept": "极限"}, {"concept": "极限"}]),
    ]
    reduce_cases = []
    for name, record, ops in reduce_specs:
        storage = Storage(tmp / f"red-{name}")
        seed(storage, [record])
        returns = []
        for op in ops:
            returns.append(memory.reduce_weakness_severity(
                storage, COURSE, op["concept"], chapter=1, amount=op.get("amount", 1),
            ))
        final = [memory.Weakness.model_validate(item) for item in storage.read_json(storage.weaknesses_path())]
        reduce_cases.append({
            "name": name,
            "input": {"initial_weaknesses": [record], "ops": ops},
            "expect": {"returns": returns, "weaknesses": [_weakness_record(w) for w in final]},
        })

    resolve_specs = [
        ("resolve_existing", base_record, "极限", True),
        ("resolve_missing_returns_false", base_record, "不存在", False),
    ]
    resolve_cases = []
    for name, record, concept, _ret in resolve_specs:
        storage = Storage(tmp / f"res-{name}")
        seed(storage, [record])
        ret = memory.mark_weakness_resolved(storage, COURSE, concept, chapter=1)
        final = [memory.Weakness.model_validate(item) for item in storage.read_json(storage.weaknesses_path())]
        resolve_cases.append({
            "name": name,
            "input": {"initial_weaknesses": [record], "concept": concept, "chapter": 1},
            "expect": {"return": ret, "weaknesses": [_weakness_record(w) for w in final]},
        })

    return {
        "schema_version": 1,
        "domain": "weakness",
        "source_module": "legacy/python/src/memory.py",
        "notes": [
            "first_seen/last_seen 为墙钟时间，不参与 golden 比较；expect 中省略这两个字段。",
            "severity=0 因 `int(x or 3)` 被当作缺省 3（legacy 现状，golden 如实冻结）。",
            "update 命中已存在概念时 mastered 恒重置为 False（legacy 现状）。",
        ],
        "id": {
            "function": "weakness_id(course, chapter, concept)",
            "cases": id_cases,
        },
        "update": {
            "function": "update_weaknesses(storage, course, weak_concepts, chapter)",
            "batch_defaults": {"course": COURSE, "chapter": CHAPTER},
            "cases": update_cases,
        },
        "reduce": {
            "function": "reduce_weakness_severity(storage, course, concept, chapter, amount)",
            "defaults": {"course": COURSE, "chapter": CHAPTER, "amount": 1},
            "cases": reduce_cases,
        },
        "resolve": {
            "function": "mark_weakness_resolved(storage, course, concept, chapter)",
            "defaults": {"course": COURSE, "chapter": CHAPTER},
            "cases": resolve_cases,
        },
    }


# ---------------------------------------------------------------------------
# mastery.golden.json
# ---------------------------------------------------------------------------

def gen_mastery() -> dict:
    tmp = Path(tempfile.mkdtemp(prefix="golden-mastery-"))

    def w(concept, severity, mastered, chapter=1, course=COURSE):
        return {
            "id": memory.weakness_id(course, chapter, concept),
            "course": course,
            "chapter": chapter,
            "concept": concept,
            "weakness_description": f"{concept} 的薄弱说明",
            "evidence": "",
            "severity": severity,
            "first_seen": "2026-01-01T09:00:00+08:00",
            "last_seen": "2026-01-01T09:00:00+08:00",
            "review_count": 1,
            "mastered": mastered,
        }

    scores_two = [
        {"subject": COURSE, "lesson": 1, "score": 6, "total": 10, "timestamp": "2026-01-01 10:00:00"},
        {"subject": COURSE, "lesson": 1, "score": 8, "total": 10, "timestamp": "2026-01-02 10:00:00"},
    ]

    chapter_specs = [
        ("no_data_returns_zero", [], []),
        ("scores_only", scores_two, []),
        ("scores_and_weaknesses_combined", scores_two, [
            w("极限", 3, False), w("连续性", 1, True), w("导数", 5, False), w("级数", 2, True),
        ]),
        ("weaknesses_filtered_by_course_and_chapter", scores_two, [
            w("极限", 5, False),
            w("其他课程概念", 5, False, course="demo-linear-algebra"),
            w("其他章节概念", 5, False, chapter=2),
            w("跨章节概念计入", 1, True, chapter=None),
        ]),
        ("weaknesses_only_no_scores", [], [w("极限", 3, False), w("连续性", 1, True)]),
        ("zero_total_score_records_skipped", [
            {"subject": COURSE, "lesson": 1, "score": 5, "total": 0, "timestamp": "2026-01-01 10:00:00"},
            {"subject": COURSE, "lesson": 1, "score": 9, "total": 10, "timestamp": "2026-01-02 10:00:00"},
        ], []),
        ("scores_of_other_lessons_ignored", scores_two + [
            {"subject": COURSE, "lesson": 2, "score": 0, "total": 10, "timestamp": "2026-01-03 10:00:00"},
        ], []),
    ]
    chapter_cases = []
    for name, scores, weaknesses in chapter_specs:
        storage = Storage(tmp / f"mastery-{name}")
        if scores:
            storage.save_json(storage.scores_path(), scores)
        if weaknesses:
            storage.save_json(storage.weaknesses_path(), weaknesses)
        chapter_cases.append({
            "name": name,
            "input": {"scores": scores, "weaknesses": weaknesses, "course": COURSE, "chapter": CHAPTER},
            "expect": memory.compute_chapter_mastery(storage, COURSE, CHAPTER),
        })

    def attempt(name, flags):
        return {"name": name, "data": {"answers": [{"is_correct": f} for f in flags]}}

    consecutive_specs = [
        ("no_attempts", []),
        ("all_correct", [attempt("20260601_100000_answers.json", [True, True, True])]),
        ("trailing_wrong_streak", [attempt("20260601_100000_answers.json", [True, False, False])]),
        ("trailing_correct_streak", [attempt("20260601_100000_answers.json", [False, True, True])]),
        ("alternating_ends_wrong", [attempt("20260601_100000_answers.json", [True, False, True, False])]),
        ("null_is_correct_counts_as_wrong", [attempt("20260601_100000_answers.json", [True, None, None])]),
        ("latest_attempt_file_wins", [
            attempt("20260601_100000_answers.json", [False, False, False]),
            attempt("20260603_100000_answers.json", [True, True, False]),
        ]),
        ("attempt_without_answers_key_ignored", [
            {"name": "20260601_100000_answers.json", "data": {"quiz_id": "q1"}},
            attempt("20260602_100000_answers.json", [True, False]),
        ]),
    ]
    consecutive_cases = []
    for name, files in consecutive_specs:
        storage = Storage(tmp / f"consec-{name}")
        exam_dir = storage.exam_dir(COURSE, CHAPTER)
        for item in files:
            storage.save_json(exam_dir / item["name"], item["data"])
        consecutive_cases.append({
            "name": name,
            "input": {
                "attempt_files": [{"name": item["name"], "data": item["data"]} for item in files],
                "course": COURSE,
                "chapter": CHAPTER,
            },
            "expect": memory.get_consecutive_stats(storage, COURSE, CHAPTER),
        })

    return {
        "schema_version": 1,
        "domain": "mastery",
        "source_module": "legacy/python/src/memory.py",
        "notes": [
            "compute_chapter_mastery = 0.50*成绩加权 + 0.30*薄弱点解决率 + 0.20*严重度因子，无薄弱点时只用成绩。",
            "float_compare: JSON 保存完整双精度；跨语言比较建议使用 1e-12 绝对容差。",
            "get_consecutive_stats 只读取文件名排序后的最后一个 *_answers.json。",
        ],
        "chapter_mastery": {
            "function": "compute_chapter_mastery(storage, course, chapter)",
            "cases": chapter_cases,
        },
        "consecutive_stats": {
            "function": "get_consecutive_stats(storage, course, chapter)",
            "cases": consecutive_cases,
        },
    }


def main() -> None:
    dump("question-policy.golden.json", gen_question_policy())
    dump("objective-grading.golden.json", gen_objective_grading())
    dump("weakness.golden.json", gen_weakness())
    dump("mastery.golden.json", gen_mastery())


if __name__ == "__main__":
    main()
