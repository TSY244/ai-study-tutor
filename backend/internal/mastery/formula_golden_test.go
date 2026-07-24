// Package mastery_test 是掌握度公式与薄弱点生命周期的 golden 测试：
// 以 legacy mastery.golden.json / weakness.golden.json 为行为基线。
// evidence 存储格式（JSONB 数组 vs 旧换行字符串）等有意差异不参与比较，
// 见 docs/web-rebuild/12-domain-decisions.md。
package mastery_test

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/TSY244/ai-study-tutor/backend/internal/mastery"
	"github.com/TSY244/ai-study-tutor/backend/internal/quiz/policy"
)

// ---- mastery.golden.json ----

type masteryGolden struct {
	ChapterMastery struct {
		Cases []struct {
			Name  string `json:"name"`
			Input struct {
				Scores []struct {
					Subject   string  `json:"subject"`
					Lesson    int     `json:"lesson"`
					Score     float64 `json:"score"`
					Total     float64 `json:"total"`
					Timestamp string  `json:"timestamp"`
				} `json:"scores"`
				Weaknesses []struct {
					Course   string `json:"course"`
					Chapter  *int   `json:"chapter"`
					Severity int    `json:"severity"`
					Mastered bool   `json:"mastered"`
				} `json:"weaknesses"`
				Course  string `json:"course"`
				Chapter int    `json:"chapter"`
			} `json:"input"`
			Expect float64 `json:"expect"`
		} `json:"cases"`
	} `json:"chapter_mastery"`
	ConsecutiveStats struct {
		Cases []struct {
			Name  string `json:"name"`
			Input struct {
				AttemptFiles []struct {
					Name string `json:"name"`
					Data struct {
						Answers []struct {
							IsCorrect *bool `json:"is_correct"`
						} `json:"answers"`
					} `json:"data"`
				} `json:"attempt_files"`
			} `json:"input"`
			Expect struct {
				ConsecutiveCorrect int `json:"consecutive_correct"`
				ConsecutiveWrong   int `json:"consecutive_wrong"`
				TotalAnswered      int `json:"total_answered"`
			} `json:"expect"`
		} `json:"cases"`
	} `json:"consecutive_stats"`
}

// ---- weakness.golden.json ----

type weaknessGolden struct {
	ID struct {
		Cases []struct {
			Name  string `json:"name"`
			Input struct {
				Course  string `json:"course"`
				Chapter any    `json:"chapter"`
				Concept string `json:"concept"`
			} `json:"input"`
			Expect string `json:"expect"`
		} `json:"cases"`
	} `json:"id"`
	Update struct {
		Cases []struct {
			Name  string `json:"name"`
			Input struct {
				InitialWeaknesses []goldenWeakness `json:"initial_weaknesses"`
				Batches           []struct {
					Concepts []struct {
						Concept  string `json:"concept"`
						Severity any    `json:"severity"`
					} `json:"concepts"`
				} `json:"batches"`
			} `json:"input"`
			Expect []goldenWeakness `json:"expect"`
		} `json:"cases"`
	} `json:"update"`
	Reduce struct {
		Defaults struct {
			Amount int `json:"amount"`
		} `json:"defaults"`
		Cases []struct {
			Name  string `json:"name"`
			Input struct {
				InitialWeaknesses []goldenWeakness `json:"initial_weaknesses"`
				Ops               []struct {
					Concept string `json:"concept"`
					Amount  *int   `json:"amount"`
				} `json:"ops"`
			} `json:"input"`
			Expect struct {
				Returns    []bool           `json:"returns"`
				Weaknesses []goldenWeakness `json:"weaknesses"`
			} `json:"expect"`
		} `json:"cases"`
	} `json:"reduce"`
	Resolve struct {
		Cases []struct {
			Name  string `json:"name"`
			Input struct {
				InitialWeaknesses []goldenWeakness `json:"initial_weaknesses"`
				Concept           string           `json:"concept"`
			} `json:"input"`
			Expect struct {
				Return     bool             `json:"return"`
				Weaknesses []goldenWeakness `json:"weaknesses"`
			} `json:"expect"`
		} `json:"cases"`
	} `json:"resolve"`
}

// goldenWeakness 只取参与比较的标量字段（evidence/description 格式差异不比较）。
type goldenWeakness struct {
	Concept     string `json:"concept"`
	Severity    int    `json:"severity"`
	ReviewCount int    `json:"review_count"`
	Mastered    bool   `json:"mastered"`
}

func readFixture(t *testing.T, name string, dst any) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "legacy-fixtures", name))
	if err != nil {
		t.Fatalf("读取 golden 夹具 %s 失败: %v", name, err)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		t.Fatalf("解析 golden 夹具 %s 失败: %v", name, err)
	}
}

func TestChapterMasteryGolden(t *testing.T) {
	var g masteryGolden
	readFixture(t, "mastery.golden.json", &g)
	const tolerance = 1e-12
	for _, tc := range g.ChapterMastery.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			// legacy 过滤语义：成绩按 course+chapter；薄弱点含跨章节（chapter null）。
			var scores []policy.ScoreRecord
			for _, s := range tc.Input.Scores {
				if s.Subject == tc.Input.Course && s.Lesson == tc.Input.Chapter {
					scores = append(scores, policy.ScoreRecord{
						Score: s.Score, Total: s.Total, Timestamp: s.Timestamp})
				}
			}
			var weaknesses []mastery.WeaknessInput
			for _, w := range tc.Input.Weaknesses {
				if w.Course == tc.Input.Course && (w.Chapter == nil || *w.Chapter == tc.Input.Chapter) {
					weaknesses = append(weaknesses, mastery.WeaknessInput{Severity: w.Severity, Mastered: w.Mastered})
				}
			}
			got := mastery.ComputeMastery(scores, weaknesses)
			if math.Abs(got-tc.Expect) > tolerance {
				t.Fatalf("ComputeMastery = %v, golden 期望 %v", got, tc.Expect)
			}
		})
	}
}

func TestConsecutiveStatsGolden(t *testing.T) {
	var g masteryGolden
	readFixture(t, "mastery.golden.json", &g)
	for _, tc := range g.ConsecutiveStats.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			// legacy 只读取文件名排序后的最后一个 *_answers.json。
			var flags []*bool
			if len(tc.Input.AttemptFiles) > 0 {
				latest := 0
				for i := range tc.Input.AttemptFiles {
					if tc.Input.AttemptFiles[i].Name >= tc.Input.AttemptFiles[latest].Name {
						latest = i
					}
				}
				for _, a := range tc.Input.AttemptFiles[latest].Data.Answers {
					flags = append(flags, a.IsCorrect)
				}
			}
			correct, wrong, answered := mastery.ConsecutiveStats(flags)
			if correct != tc.Expect.ConsecutiveCorrect || wrong != tc.Expect.ConsecutiveWrong ||
				answered != tc.Expect.TotalAnswered {
				t.Fatalf("ConsecutiveStats = (%d,%d,%d), golden 期望 (%d,%d,%d)",
					correct, wrong, answered,
					tc.Expect.ConsecutiveCorrect, tc.Expect.ConsecutiveWrong, tc.Expect.TotalAnswered)
			}
		})
	}
}

// TestConceptKeyNormalizationGolden 验证去重键规范化：legacy id 由
// course|chapter|concept 哈希生成；新系统以 (course_id, lesson_id, concept_key)
// 数据库唯一约束表达同一去重语义，concept_key 即规范化概念。因此在相同
// (course, chapter) 组内：legacy id 相同 ⟺ 规范化概念键相同。
func TestConceptKeyNormalizationGolden(t *testing.T) {
	var g weaknessGolden
	readFixture(t, "weakness.golden.json", &g)
	type groupKey struct {
		Course  string
		Chapter string
	}
	groups := map[groupKey]map[string]string{} // group → golden id → normalized concept
	for _, tc := range g.ID.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			got := mastery.NormalizeConceptKey(tc.Input.Concept)
			gk := groupKey{Course: tc.Input.Course, Chapter: fmt.Sprint(tc.Input.Chapter)}
			ids, ok := groups[gk]
			if !ok {
				ids = map[string]string{}
				groups[gk] = ids
			}
			if prev, dup := ids[tc.Expect]; dup {
				if prev != got {
					t.Fatalf("同组内 golden id %s 相同但规范化键不同：%q vs %q", tc.Expect, prev, got)
				}
				return
			}
			for otherID, otherKey := range ids {
				if otherKey == got {
					t.Fatalf("同组内不同 golden id（%s/%s）的规范化键冲突：%q", tc.Expect, otherID, got)
				}
			}
			ids[tc.Expect] = got
		})
	}
}

// severityOf 复刻 legacy 的宽松输入：`int(x or 3)`（字符串可解析、0/缺失走缺省）。
func severityOf(v any) int {
	switch s := v.(type) {
	case nil:
		return 3
	case float64:
		return mastery.NormalizeSeverity(int(s))
	case string:
		var n int
		if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
			return 3
		}
		return mastery.NormalizeSeverity(n)
	default:
		return 3
	}
}

func TestWeaknessMergeGolden(t *testing.T) {
	var g weaknessGolden
	readFixture(t, "weakness.golden.json", &g)
	for _, tc := range g.Update.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			states := map[string]mastery.WeaknessState{}
			for _, w := range tc.Input.InitialWeaknesses {
				states[mastery.NormalizeConceptKey(w.Concept)] = mastery.WeaknessState{
					Severity: w.Severity, ReviewCount: w.ReviewCount, Mastered: w.Mastered}
			}
			for _, batch := range tc.Input.Batches {
				for _, c := range batch.Concepts {
					key := mastery.NormalizeConceptKey(c.Concept)
					if key == "" {
						continue // legacy 跳过空概念
					}
					cur, ok := states[key]
					var curPtr *mastery.WeaknessState
					if ok {
						curPtr = &cur
					}
					states[key] = mastery.MergeReport(curPtr, severityOf(c.Severity))
				}
			}
			assertWeaknessStates(t, states, tc.Expect)
		})
	}
}

func TestWeaknessReduceGolden(t *testing.T) {
	var g weaknessGolden
	readFixture(t, "weakness.golden.json", &g)
	for _, tc := range g.Reduce.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			states := map[string]mastery.WeaknessState{}
			for _, w := range tc.Input.InitialWeaknesses {
				states[mastery.NormalizeConceptKey(w.Concept)] = mastery.WeaknessState{
					Severity: w.Severity, ReviewCount: w.ReviewCount, Mastered: w.Mastered}
			}
			returns := make([]bool, 0, len(tc.Input.Ops))
			for _, op := range tc.Input.Ops {
				key := mastery.NormalizeConceptKey(op.Concept)
				cur, ok := states[key]
				if !ok {
					returns = append(returns, false)
					continue
				}
				amount := g.Reduce.Defaults.Amount
				if op.Amount != nil {
					amount = *op.Amount
				}
				states[key] = mastery.ApplyCorrectReview(cur, amount)
				returns = append(returns, states[key].Mastered)
			}
			if len(returns) != len(tc.Expect.Returns) {
				t.Fatalf("返回序列长度 %d, golden 期望 %d", len(returns), len(tc.Expect.Returns))
			}
			for i := range returns {
				if returns[i] != tc.Expect.Returns[i] {
					t.Fatalf("第 %d 次 reduce 返回 %v, golden 期望 %v", i, returns[i], tc.Expect.Returns[i])
				}
			}
			assertWeaknessStates(t, states, tc.Expect.Weaknesses)
		})
	}
}

func TestWeaknessResolveGolden(t *testing.T) {
	var g weaknessGolden
	readFixture(t, "weakness.golden.json", &g)
	for _, tc := range g.Resolve.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			states := map[string]mastery.WeaknessState{}
			for _, w := range tc.Input.InitialWeaknesses {
				states[mastery.NormalizeConceptKey(w.Concept)] = mastery.WeaknessState{
					Severity: w.Severity, ReviewCount: w.ReviewCount, Mastered: w.Mastered}
			}
			key := mastery.NormalizeConceptKey(tc.Input.Concept)
			cur, ok := states[key]
			if ok != tc.Expect.Return {
				t.Fatalf("resolve 返回 %v, golden 期望 %v", ok, tc.Expect.Return)
			}
			if ok {
				states[key] = mastery.ResolveState(cur)
			}
			assertWeaknessStates(t, states, tc.Expect.Weaknesses)
		})
	}
}

// assertWeaknessStates 按概念键比较最终标量状态（severity/review_count/mastered）。
func assertWeaknessStates(t *testing.T, got map[string]mastery.WeaknessState, expect []goldenWeakness) {
	t.Helper()
	if len(got) != len(expect) {
		t.Fatalf("薄弱点数量 %d, golden 期望 %d（got=%v）", len(got), len(expect), got)
	}
	for _, e := range expect {
		key := mastery.NormalizeConceptKey(e.Concept)
		s, ok := got[key]
		if !ok {
			t.Fatalf("缺少概念 %q 的状态", e.Concept)
		}
		if s.Severity != e.Severity || s.ReviewCount != e.ReviewCount || s.Mastered != e.Mastered {
			t.Fatalf("概念 %q 状态 = %+v, golden 期望 {severity:%d review_count:%d mastered:%v}",
				e.Concept, s, e.Severity, e.ReviewCount, e.Mastered)
		}
	}
}
