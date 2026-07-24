// Package policy_test 以 testdata/legacy-fixtures/question-policy.golden.json
// 冻结的 legacy question_policy.py 行为为基线的表驱动 golden 测试。
package policy_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/TSY244/ai-study-tutor/backend/internal/quiz/policy"
)

// goldenFile 是 question-policy golden 夹具的镜像结构。
type goldenFile struct {
	QuestionCount struct {
		Cases []struct {
			Name  string `json:"name"`
			Input struct {
				ContentComplexity  string  `json:"content_complexity"`
				ConsecutiveErrors  int     `json:"consecutive_errors"`
				ConsecutiveCorrect int     `json:"consecutive_correct"`
				Accuracy           float64 `json:"accuracy"`
				UserOverride       *int32  `json:"user_override"`
			} `json:"input"`
			Expect int32 `json:"expect"`
		} `json:"cases"`
	} `json:"question_count"`
	CognitiveDistribution struct {
		Levels []string `json:"levels"`
		Cases  []struct {
			Name  string `json:"name"`
			Input struct {
				Mastery       float64 `json:"mastery"`
				QuestionCount int32   `json:"question_count"`
			} `json:"input"`
			Expect map[string]int32 `json:"expect"`
		} `json:"cases"`
	} `json:"cognitive_distribution"`
	DifficultyDistribution struct {
		Difficulties []string `json:"difficulties"`
		Cases        []struct {
			Name  string `json:"name"`
			Input struct {
				QuestionCount int32 `json:"question_count"`
			} `json:"input"`
			Expect map[string]int32 `json:"expect"`
		} `json:"cases"`
	} `json:"difficulty_distribution"`
	WeightedMastery struct {
		Cases []struct {
			Name  string `json:"name"`
			Input struct {
				Scores []struct {
					Score     float64 `json:"score"`
					Total     float64 `json:"total"`
					Timestamp string  `json:"timestamp"`
				} `json:"scores"`
			} `json:"input"`
			Expect float64 `json:"expect"`
		} `json:"cases"`
	} `json:"weighted_mastery"`
	DetectComplexity struct {
		Cases []struct {
			Name  string `json:"name"`
			Input struct {
				RawText    string `json:"raw_text"`
				LessonText string `json:"lesson_text"`
			} `json:"input"`
			Expect string `json:"expect"`
		} `json:"cases"`
	} `json:"detect_complexity"`
}

func loadGolden(t *testing.T) goldenFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "legacy-fixtures", "question-policy.golden.json"))
	if err != nil {
		t.Fatalf("读取 golden 夹具失败: %v", err)
	}
	var g goldenFile
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatalf("解析 golden 夹具失败: %v", err)
	}
	return g
}

func TestQuestionCountGolden(t *testing.T) {
	g := loadGolden(t)
	for _, tc := range g.QuestionCount.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			got := policy.DetermineQuestionCount(tc.Input.ContentComplexity,
				tc.Input.ConsecutiveErrors, tc.Input.ConsecutiveCorrect,
				tc.Input.Accuracy, tc.Input.UserOverride)
			if got != tc.Expect {
				t.Fatalf("DetermineQuestionCount = %d, golden 期望 %d", got, tc.Expect)
			}
		})
	}
}

func TestCognitiveDistributionGolden(t *testing.T) {
	g := loadGolden(t)
	for _, tc := range g.CognitiveDistribution.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			got := policy.DetermineCognitiveDistribution(tc.Input.Mastery, tc.Input.QuestionCount)
			assertDistribution(t, got, tc.Expect, g.CognitiveDistribution.Levels)
		})
	}
}

func TestDifficultyDistributionGolden(t *testing.T) {
	g := loadGolden(t)
	for _, tc := range g.DifficultyDistribution.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			got := policy.DetermineDifficultyDistribution(tc.Input.QuestionCount)
			assertDistribution(t, got, tc.Expect, g.DifficultyDistribution.Difficulties)
		})
	}
}

// assertDistribution 校验各键分配与 golden 一致，且总和等于题量（最大余数法不变量）。
func assertDistribution(t *testing.T, got, expect map[string]int32, keys []string) {
	t.Helper()
	var totalGot, totalExpect int32
	for _, k := range keys {
		totalGot += got[k]
		totalExpect += expect[k]
		if got[k] != expect[k] {
			t.Fatalf("分布[%s] = %d, golden 期望 %d（完整分布 got=%v expect=%v）", k, got[k], expect[k], got, expect)
		}
	}
	if totalGot != totalExpect {
		t.Fatalf("分布总和 = %d, golden 期望 %d", totalGot, totalExpect)
	}
}

func TestWeightedMasteryGolden(t *testing.T) {
	g := loadGolden(t)
	const tolerance = 1e-12
	for _, tc := range g.WeightedMastery.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			scores := make([]policy.ScoreRecord, 0, len(tc.Input.Scores))
			for _, s := range tc.Input.Scores {
				scores = append(scores, policy.ScoreRecord{Score: s.Score, Total: s.Total, Timestamp: s.Timestamp})
			}
			got := policy.ComputeWeightedMastery(scores)
			if math.Abs(got-tc.Expect) > tolerance {
				t.Fatalf("ComputeWeightedMastery = %v, golden 期望 %v", got, tc.Expect)
			}
		})
	}
}

func TestDetectComplexityGolden(t *testing.T) {
	g := loadGolden(t)
	for _, tc := range g.DetectComplexity.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			got := policy.DetectComplexity(tc.Input.RawText, tc.Input.LessonText)
			if got != tc.Expect {
				t.Fatalf("DetectComplexity = %q, golden 期望 %q", got, tc.Expect)
			}
		})
	}
}
