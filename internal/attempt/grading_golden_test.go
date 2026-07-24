// Package attempt_test 是客观判分的 golden 测试：以 legacy
// objective-grading.golden.json 为基线；多选集合判分的有意偏差按用例名
// legacy_quirk 显式断言（B4 决策，见 docs/web-rebuild/12-domain-decisions.md）。
package attempt_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TSY244/ai-study-tutor/internal/attempt"
)

type gradingGolden struct {
	Normalize struct {
		Cases []struct {
			Name   string `json:"name"`
			Input  string `json:"input"`
			Expect string `json:"expect"`
		} `json:"cases"`
	} `json:"normalize"`
	IsObjectiveCorrect struct {
		Cases []struct {
			Name  string `json:"name"`
			Input struct {
				Question struct {
					Type          string          `json:"type"`
					CorrectAnswer json.RawMessage `json:"correct_answer"`
				} `json:"question"`
				UserAnswer string `json:"user_answer"`
			} `json:"input"`
			Expect bool `json:"expect"`
		} `json:"cases"`
	} `json:"is_objective_correct"`
}

func loadGradingGolden(t *testing.T) gradingGolden {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "legacy-fixtures", "objective-grading.golden.json"))
	if err != nil {
		t.Fatalf("读取 golden 夹具失败: %v", err)
	}
	var g gradingGolden
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatalf("解析 golden 夹具失败: %v", err)
	}
	return g
}

func TestNormalizeGolden(t *testing.T) {
	g := loadGradingGolden(t)
	for _, tc := range g.Normalize.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			if got := attempt.NormalizeObjectiveAnswer(tc.Input); got != tc.Expect {
				t.Fatalf("NormalizeObjectiveAnswer(%q) = %q, golden 期望 %q", tc.Input, got, tc.Expect)
			}
		})
	}
}

// quirkTarget 是 legacy_quirk 用例在 Go 目标行为（多选集合比较）下的期望值。
var quirkTarget = map[string]bool{
	"multiple_choice_reordered_is_wrong_legacy_quirk": true,
	"multiple_choice_duplicate_is_wrong_legacy_quirk": true,
}

func TestIsObjectiveCorrectGolden(t *testing.T) {
	g := loadGradingGolden(t)
	for _, tc := range g.IsObjectiveCorrect.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			got := attempt.IsObjectiveCorrect(tc.Input.Question.Type,
				tc.Input.Question.CorrectAnswer, tc.Input.UserAnswer)
			if strings.HasSuffix(tc.Name, "legacy_quirk") {
				// legacy 逐项比较 quirk 不迁移：Go 目标行为为集合比较，
				// 期望值显式登记在 quirkTarget，且必须与 legacy 冻结值不同。
				want, known := quirkTarget[tc.Name]
				if !known {
					t.Fatalf("未登记的 legacy_quirk 用例 %s：先决策目标行为再更新测试", tc.Name)
				}
				if want == tc.Expect {
					t.Fatalf("quirk 用例 %s 的 Go 期望值应区别于 legacy 冻结值", tc.Name)
				}
				if got != want {
					t.Fatalf("quirk 用例 %s：IsObjectiveCorrect = %v, Go 目标行为期望 %v", tc.Name, got, want)
				}
				return
			}
			if got != tc.Expect {
				t.Fatalf("IsObjectiveCorrect = %v, golden 期望 %v", got, tc.Expect)
			}
		})
	}
}

// TestGradeObjectiveSemantics 覆盖 golden 之外的目标系统语义：
// 未答题与错误不同、多选集合判分、得分计算。
func TestGradeObjectiveSemantics(t *testing.T) {
	correct := json.RawMessage(`["A","C"]`)

	// 未作答：isCorrect=nil、0 分（与答错语义不同）。
	isCorrect, awarded := attempt.GradeObjective(attempt.TypeMultipleChoice, correct,
		json.RawMessage(`null`), 2)
	if isCorrect != nil || awarded != 0 {
		t.Fatalf("未作答应返回 (nil, 0)，实际 (%v, %v)", isCorrect, awarded)
	}

	// 答错：isCorrect=false、0 分。
	isCorrect, awarded = attempt.GradeObjective(attempt.TypeMultipleChoice, correct,
		json.RawMessage(`{"value":"A"}`), 2)
	if isCorrect == nil || *isCorrect || awarded != 0 {
		t.Fatalf("答错应返回 (false, 0)，实际 (%v, %v)", isCorrect, awarded)
	}

	// 集合判分：乱序且大小写混合仍正确、得满分。
	isCorrect, awarded = attempt.GradeObjective(attempt.TypeMultipleChoice, correct,
		json.RawMessage(`{"value":"c,A"}`), 2)
	if isCorrect == nil || !*isCorrect || awarded != 2 {
		t.Fatalf("集合判分应返回 (true, 2)，实际 (%v, %v)", isCorrect, awarded)
	}

	// 判断题布尔值答案：value 为 JSON true 时映射到 TRUE。
	isCorrect, _ = attempt.GradeObjective(attempt.TypeTrueFalse,
		json.RawMessage(`"对"`), json.RawMessage(`{"value":true}`), 1)
	if isCorrect == nil || !*isCorrect {
		t.Fatalf("布尔 true 应判对，实际 %v", isCorrect)
	}

	// 主观题不在本地判分。
	isCorrect, awarded = attempt.GradeObjective(attempt.TypeShortAnswer,
		json.RawMessage(`"极限是趋势"`), json.RawMessage(`{"value":"极限是趋势"}`), 5)
	if isCorrect == nil || *isCorrect || awarded != 0 {
		t.Fatalf("主观题本地不判分，实际 (%v, %v)", isCorrect, awarded)
	}
}
