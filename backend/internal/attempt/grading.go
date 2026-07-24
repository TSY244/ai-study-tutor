// grading.go 是客观题的确定性判分（纯函数，无数据库/模型依赖）。
// 行为基线：testdata/legacy-fixtures/objective-grading.golden.json 冻结的
// legacy exam_logic.is_objective_correct；唯一的有意偏差是多选题按
// 03-backend-architecture.md §10.2 与 06 阶段 5 关键不变量改为规范化集合
// 比较（顺序/重复不敏感），见 docs/web-rebuild/12-domain-decisions.md。
package attempt

import (
	"encoding/json"
	"strings"
)

// 客观题型集合；short_answer/case_analysis 需要 AI 批阅，本地恒不判分。
const (
	TypeSingleChoice   = "single_choice"
	TypeMultipleChoice = "multiple_choice"
	TypeTrueFalse      = "true_false"
	TypeShortAnswer    = "short_answer"
	TypeCaseAnalysis   = "case_analysis"
)

// ObjectiveType 返回题型是否可本地判分。
func ObjectiveType(questionType string) bool {
	switch questionType {
	case TypeSingleChoice, TypeMultipleChoice, TypeTrueFalse:
		return true
	default:
		return false
	}
}

// NormalizeObjectiveAnswer 是 legacy normalize_objective_answer 的等价实现：
// 中文逗号归一、按逗号切分、逐段去空白并转大写、丢弃空段、以逗号连接。
func NormalizeObjectiveAnswer(value string) string {
	parts := strings.Split(strings.ReplaceAll(value, "，", ","), ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, strings.ToUpper(s))
		}
	}
	return strings.Join(out, ",")
}

// 布尔表达映射（legacy is_objective_correct）。userBoolMap 比 correctBoolMap
// 多 YES/NO 两个键——legacy 对学生答案和正确答案使用不同映射表，golden 冻结。
var (
	userBoolMap = map[string]string{
		"Y": "TRUE", "N": "FALSE",
		"YES": "TRUE", "NO": "FALSE",
		"对": "TRUE", "错": "FALSE",
	}
	correctBoolMap = map[string]string{
		"Y": "TRUE", "N": "FALSE",
		"对": "TRUE", "错": "FALSE",
	}
)

// IsObjectiveCorrect 判定客观题作答是否正确。
//
//   - questionType：Question.type；主观题恒返回 false（需要 AI 批阅）；
//   - correctAnswer：Question.correct_answer（JSON 字符串或字符串数组）；
//   - userAnswer：学生答案的标量文本（AnswerValue.value 的字符串化结果）。
//
// 单选/判断按规范化字符串相等；多选按规范化集合比较（B4 决策：去重、无序；
// legacy 的逐项比较 quirk 不迁移，golden 中对应用例以 legacy_quirk 标记）。
func IsObjectiveCorrect(questionType string, correctAnswer json.RawMessage, userAnswer string) bool {
	if !ObjectiveType(questionType) {
		return false
	}
	normalizedUser := NormalizeObjectiveAnswer(userAnswer)
	normalizedCorrect := NormalizeObjectiveAnswer(correctAnswerString(correctAnswer))
	if questionType == TypeTrueFalse {
		if mapped, ok := userBoolMap[normalizedUser]; ok {
			normalizedUser = mapped
		}
		if mapped, ok := correctBoolMap[normalizedCorrect]; ok {
			normalizedCorrect = mapped
		}
		return normalizedUser == normalizedCorrect
	}
	if questionType == TypeMultipleChoice {
		return optionSet(normalizedUser) == optionSet(normalizedCorrect)
	}
	return normalizedUser == normalizedCorrect
}

// correctAnswerString 把 correct_answer JSON（字符串或字符串数组）还原为
// legacy 的比较输入：数组以逗号连接（legacy ",".join(map(str, correct))）。
func correctAnswerString(raw json.RawMessage) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return ""
	}
	if trimmed[0] == '[' {
		var items []any
		if err := json.Unmarshal(raw, &items); err != nil {
			return ""
		}
		parts := make([]string, 0, len(items))
		for _, it := range items {
			parts = append(parts, scalarToString(it))
		}
		return strings.Join(parts, ",")
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return trimmed
}

// optionSet 把规范化答案（逗号分隔）转为去重集合的规范形式。
func optionSet(normalized string) string {
	if normalized == "" {
		return ""
	}
	seen := make(map[string]struct{})
	keys := make([]string, 0, 4)
	for _, p := range strings.Split(normalized, ",") {
		if _, ok := seen[p]; !ok {
			seen[p] = struct{}{}
			keys = append(keys, p)
		}
	}
	// 排序后连接，集合相等 ⇔ 规范形式相等。
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return strings.Join(keys, ",")
}

// AnswerValueString 把 AnswerValue.value（任意 JSON）转为判分用标量文本：
// 字符串原样；布尔转 true/false；其余（数字、null、对象）取其 JSON 文本。
func AnswerValueString(value json.RawMessage) string {
	trimmed := strings.TrimSpace(string(value))
	if trimmed == "" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw(trimmed), &s); err == nil {
		return s
	}
	var b bool
	if err := json.Unmarshal(raw(trimmed), &b); err == nil {
		if b {
			return "true"
		}
		return "false"
	}
	return trimmed
}

func raw(s string) json.RawMessage { return json.RawMessage(s) }

// scalarToString 是 legacy str(item) 的等价：字符串原样，其余取 JSON 文本。
func scalarToString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

// GradeObjective 对一道客观题判分。
//
// 返回值 (isCorrect, awarded)：
//   - answer 为 null/缺失（未作答）：isCorrect=nil、awarded=0——未答题与
//     错误答案语义不同（06 阶段 5 关键不变量）；
//   - 已作答：isCorrect 指向判定结果，正确得满分、错误得 0。
func GradeObjective(questionType string, correctAnswer, answer json.RawMessage, maxScore float64) (isCorrect *bool, awarded float64) {
	userText, answered := answerText(answer)
	if !answered {
		return nil, 0
	}
	ok := IsObjectiveCorrect(questionType, correctAnswer, userText)
	if ok {
		return &ok, maxScore
	}
	return &ok, 0
}

// answerText 从 answer JSONB（AnswerValue 对象或 null）提取学生答案文本；
// answered=false 表示未作答（answer 为 null 或 value 为 null/缺失）。
func answerText(answer json.RawMessage) (text string, answered bool) {
	trimmed := strings.TrimSpace(string(answer))
	if trimmed == "" || trimmed == "null" {
		return "", false
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(answer, &obj); err != nil {
		return "", false
	}
	value, ok := obj["value"]
	if !ok || strings.TrimSpace(string(value)) == "null" {
		return "", false
	}
	return AnswerValueString(value), true
}
