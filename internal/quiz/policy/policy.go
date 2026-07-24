// Package policy 是从 legacy/python/src/question_policy.py 迁移的确定性出题
// 策略：题量、认知层次分布、难度分布、提前停止与加权掌握度。全部为纯函数，
// 行为由 testdata/legacy-fixtures/question-policy.golden.json 表驱动冻结。
//
// B4 决策记录（docs/web-rebuild/12-domain-decisions.md）：
//   - 分布表与题量规则保持 legacy 冻结行为（golden 验证）；
//   - legacy 提示词中“至少 20% 迁移题”的硬约束与低掌握度分布（transfer=0）
//     冲突，按 03 §10.1 v2 决策：迁移题为建议而非硬约束，分布表不变。
package policy

import (
	"math"
	"sort"
	"strings"
	"unicode/utf8"
)

// 题量边界（legacy question_policy.py 常量）。
const (
	MinQuestions     = 5
	MaxQuestions     = 15
	DefaultQuestions = 8
)

// CognitiveLevels 是认知层次声明顺序；分布结果恒包含全部键（缺省为 0），
// 最大余数法的同分并列按该顺序分配（对应 Python dict 插入序的稳定性）。
var CognitiveLevels = []string{"recall", "explain", "apply", "transfer", "reflect"}

// Difficulties 是难度声明顺序。
var Difficulties = []string{"easy", "medium", "hard", "challenge"}

// 认知层次比例按掌握度分档（legacy RATIOS_LOW/MID/HIGH；HIGH 档 explain 缺省
// 即 0，声明顺序对应 Python dict 插入序）。
var (
	ratiosLow  = []ratio{{"recall", 0.50}, {"explain", 0.30}, {"apply", 0.20}, {"transfer", 0.00}, {"reflect", 0.00}}
	ratiosMid  = []ratio{{"recall", 0.20}, {"explain", 0.25}, {"apply", 0.40}, {"transfer", 0.15}, {"reflect", 0.00}}
	ratiosHigh = []ratio{{"recall", 0.10}, {"apply", 0.40}, {"transfer", 0.40}, {"reflect", 0.10}}
)

// 难度比例（legacy DIFFICULTY_RATIOS）。
var difficultyRatios = []ratio{{"easy", 0.20}, {"medium", 0.50}, {"hard", 0.20}, {"challenge", 0.10}}

// 内容复杂度对应的基础题量（legacy COMPLEXITY_COUNTS）。
var complexityCounts = map[string]int{"simple": 5, "normal": 8, "complex": 12}

type ratio struct {
	key   string
	share float64
}

// DetermineQuestionCount 返回一次生成的题量；返回 0 表示策略判定无需继续
// 练习（调用方映射 practice_not_needed，不创建生成任务）。
//
// 规则（legacy determine_question_count）：
//   - user_override 非空时直接使用并夹取到 [5,15]，不再评估提前停止；
//   - accuracy >= 0.85 且 consecutive_correct >= 3 时提前停止（返回 0）；
//   - 基础题量按内容复杂度（未知复杂度回退默认 8）；连续错误 >= 2 加 3 题；
//     最终夹取到 [5,15]。
func DetermineQuestionCount(contentComplexity string, consecutiveErrors, consecutiveCorrect int,
	accuracy float64, userOverride *int32) int32 {
	if userOverride != nil {
		return clampQuestions(*userOverride)
	}
	if ShouldStopEarly(accuracy, consecutiveCorrect) {
		return 0
	}
	base, ok := complexityCounts[contentComplexity]
	if !ok {
		base = DefaultQuestions
	}
	if consecutiveErrors >= 2 {
		base += 3
	}
	return clampQuestions(int32(base))
}

// ShouldStopEarly 判定学生是否已表现足够掌握度（legacy should_stop_early）。
func ShouldStopEarly(accuracy float64, consecutiveCorrect int) bool {
	return accuracy >= 0.85 && consecutiveCorrect >= 3
}

func clampQuestions(v int32) int32 {
	if v < MinQuestions {
		return MinQuestions
	}
	if v > MaxQuestions {
		return MaxQuestions
	}
	return v
}

// DetermineCognitiveDistribution 按掌握度分档比例把 count 题分配到各认知
// 层次（最大余数法，结果总和恒等于 count；恒返回全部 5 个层次键）。
func DetermineCognitiveDistribution(mastery float64, questionCount int32) map[string]int32 {
	ratios := ratiosLow
	switch {
	case mastery < 0.60:
		ratios = ratiosLow
	case mastery <= 0.85:
		ratios = ratiosMid
	default:
		ratios = ratiosHigh
	}
	return distributeCounts(ratios, CognitiveLevels, questionCount)
}

// DetermineDifficultyDistribution 按固定比例分配难度题数（最大余数法）。
func DetermineDifficultyDistribution(questionCount int32) map[string]int32 {
	return distributeCounts(difficultyRatios, Difficulties, questionCount)
}

// distributeCounts 是 legacy _distribute_counts 的等价实现：先向下取整，
// 剩余名额按余数降序分配；余数并列时保持 ratios 声明顺序（Python sort 稳定）。
// keys 为结果全集（含比例为 0 或缺省的键）。
func distributeCounts(ratios []ratio, keys []string, total int32) map[string]int32 {
	out := make(map[string]int32, len(keys))
	for _, k := range keys {
		out[k] = 0
	}
	if total <= 0 {
		return out
	}
	type remainder struct {
		frac float64
		key  string
	}
	remainders := make([]remainder, 0, len(ratios))
	var allocated int32
	for _, r := range ratios {
		// 显式 float64 转换阻断 Go 编译器的跨语句浮点融合（Go spec 允许融合
		// 导致中间结果超出双精度），保证与 CPython IEEE754 双精度逐步舍入
		// 完全一致——最大余数分配对 1 ulp 差异敏感（如 15*0.1 的余数并列）。
		exact := float64(float64(total) * r.share)
		floor := int32(math.Floor(exact))
		out[r.key] = floor
		allocated += floor
		remainders = append(remainders, remainder{frac: exact - float64(floor), key: r.key})
	}
	sort.SliceStable(remainders, func(i, j int) bool {
		return remainders[i].frac > remainders[j].frac
	})
	for i := int32(0); i < total-allocated && int(i) < len(remainders); i++ {
		out[remainders[i].key]++
	}
	return out
}

// ScoreRecord 是一条历史成绩记录（Timestamp 为可字典序比较的时间字符串；
// 空字符串排最前，对应 legacy 缺失 timestamp 的行为）。
type ScoreRecord struct {
	Score     float64
	Total     float64
	Timestamp string
}

// ComputeWeightedMastery 计算成绩加权掌握度（legacy compute_mastery）：
// 按时间升序后权重线性递增（weight = 1 + i*0.5，i 为排序后下标，被跳过的
// total<=0 记录也占用下标）；全部记录 total<=0 时返回 0。
func ComputeWeightedMastery(scores []ScoreRecord) float64 {
	if len(scores) == 0 {
		return 0
	}
	sorted := make([]ScoreRecord, len(scores))
	copy(sorted, scores)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Timestamp < sorted[j].Timestamp
	})
	var totalWeight, weightedSum float64
	for i, s := range sorted {
		if s.Total <= 0 {
			continue
		}
		weight := 1.0 + float64(i)*0.5
		weightedSum += (s.Score / s.Total) * weight
		totalWeight += weight
	}
	if totalWeight == 0 {
		return 0
	}
	return weightedSum / totalWeight
}

// DetectComplexity 是 legacy exam_logic._detect_complexity 的复杂度启发式：
// 数学指标（每个 2 分）+ 步骤指标（每个 1 分）+ “概念/定义”出现次数；
// 长度与分数阈值判定 simple/normal/complex。
func DetectComplexity(rawText, lessonText string) string {
	combined := rawText + "\n" + lessonText
	length := utf8.RuneCountInString(combined)

	var mathIndicators int
	for _, kw := range []string{"推导", "证明", "公式", "定理", "计算", "微分", "积分"} {
		if strings.Contains(combined, kw) {
			mathIndicators++
		}
	}
	var stepIndicators int
	for _, kw := range []string{"步骤", "流程", "方法", "步骤一", "步骤二", "首先", "然后", "接着"} {
		if strings.Contains(combined, kw) {
			stepIndicators++
		}
	}
	conceptCount := strings.Count(combined, "概念") + strings.Count(combined, "定义")

	score := mathIndicators*2 + stepIndicators + conceptCount
	switch {
	case length < 3000 && score < 5:
		return "simple"
	case score >= 8 || length > 15000:
		return "complex"
	default:
		return "normal"
	}
}
