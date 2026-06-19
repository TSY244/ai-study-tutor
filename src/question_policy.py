"""Question generation policy engine.

Encodes the question-generation policy (question_count, cognitive-level ratios,
difficulty distribution, stop conditions) as pure functions so both the
direct-API path (prompts.py → exam_logic.py) and the agent-task path
(agent_runner.py) share the same rules.

Policy source: question_generation_policy_final.txt
"""

from __future__ import annotations

import math
from typing import Any

from .models import CognitiveLevel

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

MIN_QUESTIONS = 5
MAX_QUESTIONS = 15
DEFAULT_QUESTIONS = 8

# Default cognitive-level ratios (mastery 60%–85%, the "normal" tier)
DEFAULT_RATIOS: dict[CognitiveLevel, float] = {
    CognitiveLevel.recall: 0.30,
    CognitiveLevel.explain: 0.20,
    CognitiveLevel.apply: 0.30,
    CognitiveLevel.transfer: 0.15,
    CognitiveLevel.reflect: 0.05,
}

# Mastery-driven ratios by tier
RATIOS_LOW: dict[CognitiveLevel, float] = {       # mastery < 60%
    CognitiveLevel.recall: 0.50,
    CognitiveLevel.explain: 0.30,
    CognitiveLevel.apply: 0.20,
    CognitiveLevel.transfer: 0.00,
    CognitiveLevel.reflect: 0.00,
}
RATIOS_MID: dict[CognitiveLevel, float] = {       # 60% ≤ mastery ≤ 85%
    CognitiveLevel.recall: 0.20,
    CognitiveLevel.explain: 0.25,
    CognitiveLevel.apply: 0.40,
    CognitiveLevel.transfer: 0.15,
    CognitiveLevel.reflect: 0.00,
}
RATIOS_HIGH: dict[CognitiveLevel, float] = {      # mastery > 85%
    CognitiveLevel.recall: 0.10,
    CognitiveLevel.apply: 0.40,
    CognitiveLevel.transfer: 0.40,
    CognitiveLevel.reflect: 0.10,
}

# Difficulty distribution
DIFFICULTY_RATIOS: dict[str, float] = {
    "easy": 0.20,
    "medium": 0.50,
    "hard": 0.20,
    "challenge": 0.10,
}

# Question count by content complexity
COMPLEXITY_COUNTS = {
    "simple": 5,
    "normal": 8,
    "complex": 12,
}


# ---------------------------------------------------------------------------
# Public helpers
# ---------------------------------------------------------------------------

def determine_question_count(
    content_complexity: str = "normal",
    consecutive_errors: int = 0,
    consecutive_correct: int = 0,
    accuracy: float = 0.0,
    user_override: int | None = None,
) -> int:
    """Return the number of questions to generate for one knowledge point.

    Parameters
    ----------
    content_complexity : "simple" | "normal" | "complex"
    consecutive_errors : from the most recent quiz
    consecutive_correct : from the most recent quiz
    accuracy : overall accuracy ratio (0–1) for this chapter
    user_override : if set, use this value (still clamped)

    Returns
    -------
    int in [5, 15]
    """
    if user_override is not None:
        return max(MIN_QUESTIONS, min(MAX_QUESTIONS, user_override))

    # Early stop: accuracy >= 85% AND consecutive_correct >= 3
    if should_stop_early(accuracy, consecutive_correct):
        return 0  # caller should interpret as "no more questions needed"

    base = COMPLEXITY_COUNTS.get(content_complexity, DEFAULT_QUESTIONS)

    # Consecutive errors ≥ 2 → add 3 more questions
    if consecutive_errors >= 2:
        base += 3

    return max(MIN_QUESTIONS, min(MAX_QUESTIONS, base))


def should_stop_early(accuracy: float, consecutive_correct: int) -> bool:
    """Return True if the student has demonstrated sufficient mastery."""
    return accuracy >= 0.85 and consecutive_correct >= 3


def determine_cognitive_distribution(
    mastery: float,
    question_count: int,
) -> dict[CognitiveLevel, int]:
    """Return the exact count of questions at each cognitive level.

    Uses the largest-remainder method to distribute integer counts
    proportional to the mastery-tier ratios.
    """
    if mastery < 0.60:
        ratios = RATIOS_LOW
    elif mastery <= 0.85:
        ratios = RATIOS_MID
    else:
        ratios = RATIOS_HIGH

    return _distribute_counts(ratios, question_count)


def determine_difficulty_distribution(question_count: int) -> dict[str, int]:
    """Return exact question counts per difficulty tier."""
    return _distribute_counts(DIFFICULTY_RATIOS, question_count)


def compute_mastery(scores: list[dict[str, Any]]) -> float:
    """Compute weighted mastery from historical score records.

    Each record: {'score': float, 'total': float, 'timestamp': str}
    Recent scores are weighted more heavily (exponential decay).
    """
    if not scores:
        return 0.0

    # Sort by timestamp ascending
    sorted_scores = sorted(scores, key=lambda s: s.get("timestamp", ""))

    total_weight = 0.0
    weighted_sum = 0.0

    for i, s in enumerate(sorted_scores):
        score = float(s.get("score", 0))
        total = float(s.get("total", 1))
        if total <= 0:
            continue
        ratio = score / total
        # More recent = higher index = higher weight (linear ramp)
        weight = 1.0 + i * 0.5
        weighted_sum += ratio * weight
        total_weight += weight

    if total_weight == 0:
        return 0.0

    return weighted_sum / total_weight


def build_policy_context(
    cognitive_dist: dict[CognitiveLevel, int],
    difficulty_dist: dict[str, int],
    question_count: int,
    mastery: float,
    weaknesses: list[dict[str, Any]] | None = None,
) -> str:
    """Build a natural-language policy section for AI prompts.

    Returns a Chinese string that can be inlined into a user/system prompt.
    """
    parts: list[str] = []

    # ── Header ──
    parts.append("## 题目生成策略（必须严格遵守）")
    parts.append("")
    parts.append(f"- 共生成 **{question_count}** 题")

    # ── Mastery context ──
    if mastery > 0:
        tier = "高（>85%）" if mastery > 0.85 else "中（60%-85%）" if mastery >= 0.60 else "低（<60%）"
        parts.append(f"- 当前掌握度：{mastery:.0%}（{tier}）")
    else:
        parts.append("- 当前掌握度：无历史数据（首次练习）")

    # ── Cognitive distribution ──
    parts.append("")
    parts.append("### 认知层次分布（题型映射）")
    parts.append("")
    parts.append("| 认知层次 | 题数 | 推荐题型 |")
    parts.append("|----------|------|----------|")
    for level in [CognitiveLevel.recall, CognitiveLevel.explain, CognitiveLevel.apply,
                   CognitiveLevel.transfer, CognitiveLevel.reflect]:
        count = cognitive_dist.get(level, 0)
        if count <= 0:
            continue
        types = _level_to_types(level)
        parts.append(f"| {_level_label(level)} | {count} | {types} |")
    parts.append("")

    # ── Difficulty distribution ──
    parts.append("### 难度分布")
    parts.append("")
    parts.append(f"- Easy（简单）：{difficulty_dist.get('easy', 0)} 题")
    parts.append(f"- Medium（中等）：{difficulty_dist.get('medium', 0)} 题")
    parts.append(f"- Hard（困难）：{difficulty_dist.get('hard', 0)} 题")
    parts.append(f"- Challenge（挑战）：{difficulty_dist.get('challenge', 0)} 题")
    parts.append("")
    parts.append("**难度必须递增排列**：Easy → Medium → Hard → Challenge")

    # ── Constraints ──
    parts.append("")
    parts.append("### 生成约束")
    parts.append("")
    parts.append("1. **不直接复制**学习资料原文，必须用自己的话重新表述")
    parts.append("2. **至少 50%** 的题目要求主动推理（Apply + Transfer + Reflect）")
    parts.append("3. **至少 20%** 的题目要求迁移应用（Transfer）")
    parts.append("4. **单题只测一个核心知识点**，不要一题多概念")
    parts.append("5. **难度递增**，从简单回忆逐步过渡到挑战迁移")
    parts.append("6. 每题必须标注 `tested_concepts`、`difficulty`、`cognitive_level`")

    # ── Weakness context ──
    if weaknesses:
        active_weak = [w for w in weaknesses if not w.get("mastered", False)]
        if active_weak:
            parts.append("")
            parts.append("### ⚠️ 历史薄弱知识点（优先出题）")
            parts.append("")
            for w in active_weak[:8]:  # cap at 8 to avoid prompt bloat
                concept = w.get("concept", "未知")
                desc = w.get("weakness_description", "")
                sev = w.get("severity", 3)
                parts.append(f"- **{concept}**（严重度 {sev}/5）：{desc}")
            parts.append("")
            parts.append("请确保至少 30% 的题目覆盖上述薄弱点。")

    return "\n".join(parts)


# ---------------------------------------------------------------------------
# Internal helpers
# ---------------------------------------------------------------------------

def _distribute_counts(ratios: dict[Any, float], total: int) -> dict[Any, int]:
    """Distribute `total` items across keys proportionally to `ratios`.

    Uses the largest-remainder method so the result sums to `total` exactly.
    """
    if total <= 0:
        return {k: 0 for k in ratios}

    # Step 1: floor allocation
    result: dict[Any, int] = {}
    remainders: list[tuple[float, Any]] = []
    allocated = 0

    for key, ratio in ratios.items():
        exact = total * ratio
        floor = int(math.floor(exact))
        result[key] = floor
        allocated += floor
        remainders.append((exact - floor, key))

    # Step 2: distribute remaining slots by largest remainder
    remaining = total - allocated
    remainders.sort(key=lambda x: -x[0])

    for i in range(remaining):
        if i < len(remainders):
            key = remainders[i][1]
            result[key] += 1

    return result


def _level_label(level: CognitiveLevel) -> str:
    labels = {
        CognitiveLevel.recall: "回忆 (Recall)",
        CognitiveLevel.explain: "理解 (Explain)",
        CognitiveLevel.apply: "应用 (Apply)",
        CognitiveLevel.transfer: "迁移 (Transfer)",
        CognitiveLevel.reflect: "反思 (Reflect)",
    }
    return labels.get(level, str(level.value))


def _level_to_types(level: CognitiveLevel) -> str:
    type_map = {
        CognitiveLevel.recall: "single_choice / true_false",
        CognitiveLevel.explain: "short_answer",
        CognitiveLevel.apply: "short_answer / case_analysis",
        CognitiveLevel.transfer: "case_analysis",
        CognitiveLevel.reflect: "short_answer / case_analysis",
    }
    return type_map.get(level, "任意题型")
