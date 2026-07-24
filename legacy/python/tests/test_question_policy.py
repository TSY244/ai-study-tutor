"""Tests for the question generation policy engine."""

import pytest
import sys
from pathlib import Path

# Make src importable
sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from src.question_policy import (
    DEFAULT_QUESTIONS,
    MAX_QUESTIONS,
    MIN_QUESTIONS,
    determine_question_count,
    determine_cognitive_distribution,
    determine_difficulty_distribution,
    compute_mastery,
    should_stop_early,
    build_policy_context,
)
from src.models import CognitiveLevel


# ---------------------------------------------------------------------------
# determine_question_count
# ---------------------------------------------------------------------------

class TestDetermineQuestionCount:
    def test_default_normal_content(self):
        assert determine_question_count("normal") == 8

    def test_simple_content(self):
        assert determine_question_count("simple") == 5

    def test_complex_content(self):
        assert determine_question_count("complex") == 12

    def test_consecutive_errors_add_3(self):
        count = determine_question_count("normal", consecutive_errors=2)
        assert count == 11  # 8 + 3

        count = determine_question_count("simple", consecutive_errors=3)
        assert count == 8  # 5 + 3

    def test_early_stop_when_mastered(self):
        count = determine_question_count(
            "normal", accuracy=0.90, consecutive_correct=3
        )
        assert count == 0  # Stop early

    def test_no_early_stop_when_not_met(self):
        # accuracy high but consecutive_correct too low
        count = determine_question_count(
            "normal", accuracy=0.90, consecutive_correct=2
        )
        assert count == 8

        # consecutive high but accuracy too low
        count = determine_question_count(
            "normal", accuracy=0.80, consecutive_correct=3
        )
        assert count == 8

    def test_clamp_to_max(self):
        count = determine_question_count("complex", consecutive_errors=5)
        assert count == 15  # 12 + 3 clamped to 15

    def test_clamp_to_min(self):
        count = determine_question_count("simple", consecutive_errors=0)
        assert count == 5  # minimum

    def test_user_override(self):
        count = determine_question_count("normal", user_override=20)
        assert count == 15  # clamped to max

    def test_user_override_small(self):
        count = determine_question_count("normal", user_override=3)
        assert count == 5  # clamped to min


# ---------------------------------------------------------------------------
# should_stop_early
# ---------------------------------------------------------------------------

class TestShouldStopEarly:
    def test_both_met(self):
        assert should_stop_early(0.85, 3) is True
        assert should_stop_early(0.90, 5) is True

    def test_accuracy_below(self):
        assert should_stop_early(0.84, 3) is False

    def test_consecutive_below(self):
        assert should_stop_early(0.85, 2) is False

    def test_neither_met(self):
        assert should_stop_early(0.50, 1) is False
        assert should_stop_early(0.0, 0) is False


# ---------------------------------------------------------------------------
# determine_cognitive_distribution
# ---------------------------------------------------------------------------

class TestDetermineCognitiveDistribution:
    def test_low_mastery_below_60(self):
        dist = determine_cognitive_distribution(0.40, 10)
        # recall 50%, explain 30%, apply 20%
        assert dist[CognitiveLevel.recall] == 5
        assert dist[CognitiveLevel.explain] == 3
        assert dist[CognitiveLevel.apply] == 2
        assert dist.get(CognitiveLevel.transfer, 0) == 0
        assert dist.get(CognitiveLevel.reflect, 0) == 0

    def test_mid_mastery(self):
        dist = determine_cognitive_distribution(0.70, 10)
        # recall 20%, explain 25%, apply 40%, transfer 15%
        assert dist[CognitiveLevel.recall] == 2
        assert dist[CognitiveLevel.explain] in (2, 3)  # rounding
        assert dist[CognitiveLevel.apply] == 4
        assert dist.get(CognitiveLevel.transfer, 0) in (1, 2)
        assert dist.get(CognitiveLevel.reflect, 0) == 0

    def test_high_mastery(self):
        dist = determine_cognitive_distribution(0.90, 10)
        # recall 10%, apply 40%, transfer 40%, reflect 10%
        assert dist[CognitiveLevel.recall] == 1
        assert dist[CognitiveLevel.apply] == 4
        assert dist[CognitiveLevel.transfer] == 4
        assert dist[CognitiveLevel.reflect] == 1
        assert dist.get(CognitiveLevel.explain, 0) == 0

    def test_sums_to_question_count(self):
        for mastery in [0.0, 0.40, 0.60, 0.70, 0.85, 0.90, 1.0]:
            for count in [5, 8, 10, 12, 15]:
                dist = determine_cognitive_distribution(mastery, count)
                total = sum(dist.values())
                assert total == count, f"mastery={mastery}, count={count}, sum={total}"

    def test_boundary_60_pct(self):
        """60% exactly → mid tier."""
        dist = determine_cognitive_distribution(0.60, 10)
        # Should be mid tier (recall 20%)
        assert dist[CognitiveLevel.recall] == 2
        assert dist[CognitiveLevel.apply] == 4

    def test_boundary_85_pct(self):
        """85% exactly → mid tier (≤ 85%)."""
        dist = determine_cognitive_distribution(0.85, 10)
        # mid tier: recall 20%
        assert dist[CognitiveLevel.recall] == 2


# ---------------------------------------------------------------------------
# determine_difficulty_distribution
# ---------------------------------------------------------------------------

class TestDetermineDifficultyDistribution:
    def test_default_10_questions(self):
        dist = determine_difficulty_distribution(10)
        assert dist["easy"] == 2    # 20%
        assert dist["medium"] == 5  # 50%
        assert dist["hard"] == 2    # 20%
        assert dist["challenge"] == 1  # 10%

    def test_sums_to_total(self):
        for count in [5, 6, 7, 8, 9, 10, 11, 12, 15]:
            dist = determine_difficulty_distribution(count)
            total = sum(dist.values())
            assert total == count, f"count={count}, sum={total}"

    def test_minimum_count(self):
        dist = determine_difficulty_distribution(5)
        assert sum(dist.values()) == 5
        # Easy should get at least 1
        assert dist["easy"] >= 1


# ---------------------------------------------------------------------------
# compute_mastery
# ---------------------------------------------------------------------------

class TestComputeMastery:
    def test_no_scores(self):
        assert compute_mastery([]) == 0.0

    def test_single_perfect_score(self):
        scores = [{"score": 10, "total": 10, "timestamp": "2026-01-01"}]
        assert compute_mastery(scores) == 1.0

    def test_single_zero_score(self):
        scores = [{"score": 0, "total": 10, "timestamp": "2026-01-01"}]
        assert compute_mastery(scores) == 0.0

    def test_weighted_recent_heavier(self):
        scores = [
            {"score": 0, "total": 10, "timestamp": "2026-01-01"},
            {"score": 10, "total": 10, "timestamp": "2026-01-02"},
            {"score": 10, "total": 10, "timestamp": "2026-01-03"},
        ]
        mastery = compute_mastery(scores)
        # Recent scores are 10/10, should be > 0.67 (avg) due to weighting
        assert mastery > 0.70

    def test_skips_zero_total(self):
        scores = [
            {"score": 5, "total": 0, "timestamp": "2026-01-01"},
            {"score": 10, "total": 10, "timestamp": "2026-01-02"},
        ]
        mastery = compute_mastery(scores)
        assert mastery == 1.0


# ---------------------------------------------------------------------------
# build_policy_context
# ---------------------------------------------------------------------------

class TestBuildPolicyContext:
    def test_includes_question_count(self):
        ctx = build_policy_context(
            cognitive_dist={CognitiveLevel.recall: 3, CognitiveLevel.explain: 2,
                            CognitiveLevel.apply: 2, CognitiveLevel.transfer: 1},
            difficulty_dist={"easy": 2, "medium": 4, "hard": 1, "challenge": 1},
            question_count=8,
            mastery=0.0,
        )
        assert "**8** 题" in ctx

    def test_includes_mastery_tier(self):
        ctx = build_policy_context(
            cognitive_dist={CognitiveLevel.recall: 5, CognitiveLevel.explain: 3,
                            CognitiveLevel.apply: 2},
            difficulty_dist={"easy": 2, "medium": 5, "hard": 2, "challenge": 1},
            question_count=10,
            mastery=0.55,
        )
        assert "55%" in ctx
        assert "低（<60%）" in ctx

    def test_includes_constraints(self):
        ctx = build_policy_context(
            cognitive_dist={CognitiveLevel.recall: 3, CognitiveLevel.explain: 2,
                            CognitiveLevel.apply: 2, CognitiveLevel.transfer: 1},
            difficulty_dist={"easy": 2, "medium": 4, "hard": 1, "challenge": 1},
            question_count=8,
            mastery=0.75,
        )
        assert "不直接复制" in ctx
        assert "50%" in ctx
        assert "20%" in ctx
        assert "单题只测一个核心知识点" in ctx

    def test_includes_weaknesses(self):
        ctx = build_policy_context(
            cognitive_dist={CognitiveLevel.recall: 3, CognitiveLevel.explain: 2,
                            CognitiveLevel.apply: 2, CognitiveLevel.transfer: 1},
            difficulty_dist={"easy": 2, "medium": 4, "hard": 1, "challenge": 1},
            question_count=8,
            mastery=0.50,
            weaknesses=[
                {"concept": "K线形态", "weakness_description": "无法区分锤子线和倒锤子线",
                 "severity": 4, "mastered": False},
            ],
        )
        assert "K线形态" in ctx
        assert "严重度 4" in ctx

    def test_excludes_mastered_weaknesses(self):
        ctx = build_policy_context(
            cognitive_dist={CognitiveLevel.recall: 3, CognitiveLevel.explain: 2,
                            CognitiveLevel.apply: 2, CognitiveLevel.transfer: 1},
            difficulty_dist={"easy": 2, "medium": 4, "hard": 1, "challenge": 1},
            question_count=8,
            mastery=0.80,
            weaknesses=[
                {"concept": "已掌握", "weakness_description": "test",
                 "severity": 1, "mastered": True},
            ],
        )
        # Mastered weaknesses should be filtered out
        assert "已掌握" not in ctx
