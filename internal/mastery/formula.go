// formula.go 是掌握度公式与薄弱点生命周期的确定性实现（纯函数）。
// 行为基线：testdata/legacy-fixtures/mastery.golden.json 与
// weakness.golden.json 冻结的 legacy memory.py；与旧文件格式的差异
// （evidence 改为 JSONB 数组、章节改为 lesson UUID）见
// docs/web-rebuild/12-domain-decisions.md。
package mastery

import (
	"strings"

	"github.com/TSY244/ai-study-tutor/internal/quiz/policy"
)

// FormulaV1 是当前掌握度公式版本：迁移自 legacy compute_chapter_mastery
// （0.50*成绩加权 + 0.30*薄弱点解决率 + 0.20*严重度因子，无薄弱点时只用成绩）。
// 公式变更必须引入新版本号并保留历史快照（03 §10.3）。
const FormulaV1 = "mastery/v1"

// WeaknessInput 是公式所需的最小薄弱点视图。
type WeaknessInput struct {
	Severity int
	Mastered bool
}

// ComputeMastery 计算范围掌握度（0..1），完全复刻 legacy
// compute_chapter_mastery：
//   - 无成绩且无薄弱点 → 0；
//   - 无薄弱点 → 只返回成绩加权掌握度；
//   - 否则 0.5*成绩 + 0.3*解决率 + 0.2*严重度因子，夹取到 [0,1]。
//
// 调用方负责按范围过滤 scores 与 weaknesses（lesson 范围含 lesson_id 为空的
// 课程级薄弱点，与 legacy “chapter None 计入每章”一致）。
func ComputeMastery(scores []policy.ScoreRecord, weaknesses []WeaknessInput) float64 {
	scoreMastery := policy.ComputeWeightedMastery(scores)
	if len(weaknesses) == 0 {
		return scoreMastery
	}
	total := len(weaknesses)
	var mastered, severitySum int
	for _, w := range weaknesses {
		if w.Mastered {
			mastered++
		}
		severitySum += w.Severity
	}
	resolution := float64(mastered) / float64(total)
	avgSeverity := float64(severitySum) / float64(total)
	severityFactor := 1.0 - (avgSeverity-1)/4.0 // severity [1,5] → [1.0,0.0]
	combined := 0.50*scoreMastery + 0.30*resolution + 0.20*severityFactor
	return min(1.0, max(0.0, combined))
}

// ConsecutiveStats 从最新一次作答的逐题正误（按题目顺序）计算连续统计，
// 复刻 legacy get_consecutive_stats：从队尾数连续正确/连续错误（遇相反即停），
// 连续错误取队尾与队首两个方向的较大者；nil（未答/未判）按错误计。
func ConsecutiveStats(isCorrect []*bool) (correct, wrong, answered int) {
	answered = len(isCorrect)
	for i := len(isCorrect) - 1; i >= 0; i-- {
		if isCorrect[i] != nil && *isCorrect[i] {
			correct++
			if wrong > 0 {
				break
			}
		} else {
			wrong++
			if correct > 0 {
				break
			}
		}
	}
	wrongFromStart := 0
	for _, c := range isCorrect {
		if c != nil && *c {
			break
		}
		wrongFromStart++
	}
	if wrongFromStart > wrong {
		wrong = wrongFromStart
	}
	return correct, wrong, answered
}

// NormalizeConceptKey 规范化概念去重键：去首尾空白并转小写
// （legacy weakness_id 的 concept.strip().lower() 等价物）。
func NormalizeConceptKey(concept string) string {
	return strings.ToLower(strings.TrimSpace(concept))
}

// NormalizeSeverity 规范化严重度到 [1,5]；<=0 视为缺省 3
// （legacy `int(x or 3)` 冻结行为，见 weakness.golden.json notes）。
func NormalizeSeverity(v int) int {
	if v <= 0 {
		return 3
	}
	if v > 5 {
		return 5
	}
	return v
}

// WeaknessState 是薄弱点生命周期所需的最小状态。
type WeaknessState struct {
	Severity    int
	ReviewCount int
	Mastered    bool
}

// MergeReport 把一次新的错误证据合并进薄弱点状态（legacy update_weaknesses
// 的既有概念分支）：严重度取新旧较大者、review_count+1、mastered 复位。
// 新建概念（cur 为 nil）使用 NormalizeSeverity 后的严重度、review_count=1。
func MergeReport(cur *WeaknessState, severity int) WeaknessState {
	severity = NormalizeSeverity(severity)
	if cur == nil {
		return WeaknessState{Severity: severity, ReviewCount: 1, Mastered: false}
	}
	return WeaknessState{
		Severity:    max(cur.Severity, severity),
		ReviewCount: cur.ReviewCount + 1,
		Mastered:    false,
	}
}

// ApplyCorrectReview 是答对复习的状态迁移（legacy reduce_weakness_severity）：
// 严重度降 amount（下限 1）、review_count+1；严重度降到 1 且复习次数 >= 2 时
// 标记已掌握。
func ApplyCorrectReview(cur WeaknessState, amount int) WeaknessState {
	cur.Severity = max(1, cur.Severity-max(1, amount))
	cur.ReviewCount++
	if cur.Severity <= 1 && cur.ReviewCount >= 2 {
		cur.Mastered = true
	}
	return cur
}

// ResolveState 是手动标记已掌握的状态迁移（legacy mark_weakness_resolved）：
// mastered=true 且严重度归 1。
func ResolveState(cur WeaknessState) WeaknessState {
	cur.Mastered = true
	cur.Severity = 1
	return cur
}
