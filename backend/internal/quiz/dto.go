package quiz

import (
	"encoding/json"

	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
)

// QuestionDTO 是学生可见的题目视图（契约 Question）：
// 不含 correct_answer/explanation——它们只在 Attempt 反馈中按状态出现。
type QuestionDTO struct {
	ID             string            `json:"id"`
	Type           string            `json:"type"`
	Stem           string            `json:"stem"`
	Options        map[string]string `json:"options"`
	TestedConcepts []string          `json:"tested_concepts"`
	Difficulty     string            `json:"difficulty"`
	CognitiveLevel string            `json:"cognitive_level"`
	Score          float64           `json:"score"`
}

// ToQuestionDTO 把学生可见字段映射为 DTO；score 取 quiz_items 的当次分值。
func ToQuestionDTO(qq QuizQuestion) QuestionDTO {
	return QuestionDTO{
		ID:             qq.Question.ID.String(),
		Type:           qq.Question.Type,
		Stem:           qq.Question.Stem,
		Options:        optionsMap(qq.Question.Options),
		TestedConcepts: conceptsSlice(qq.Question.TestedConcepts),
		Difficulty:     qq.Question.Difficulty,
		CognitiveLevel: qq.Question.CognitiveLevel,
		Score:          qq.Score,
	}
}

// optionsMap 解析 options JSONB（选择题选项对象；其他题型为 null）。
func optionsMap(raw []byte) map[string]string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	return m
}

// conceptsSlice 解析 tested_concepts JSONB（缺省为空数组）。
func conceptsSlice(raw []byte) []string {
	out := []string{}
	if len(raw) == 0 {
		return out
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return []string{}
	}
	return out
}

// QuizDTO 是试卷详情（契约 Quiz）。
type QuizDTO struct {
	ID        string        `json:"id"`
	LessonID  string        `json:"lesson_id"`
	Title     string        `json:"title"`
	Source    string        `json:"source"`
	Status    string        `json:"status"`
	Questions []QuestionDTO `json:"questions"`
	CreatedAt string        `json:"created_at"`
}

// ToQuizDTO 映射试卷详情（学生视图）。
func ToQuizDTO(z *Quiz, questions []QuizQuestion) QuizDTO {
	qs := make([]QuestionDTO, 0, len(questions))
	for _, qq := range questions {
		qs = append(qs, ToQuestionDTO(qq))
	}
	return QuizDTO{
		ID:        z.ID.String(),
		LessonID:  z.LessonID.String(),
		Title:     z.Title,
		Source:    z.Source,
		Status:    z.Status,
		Questions: qs,
		CreatedAt: httpx.Time(z.CreatedAt),
	}
}

// QuizSummaryDTO 是试卷列表项（契约 QuizSummary）。
type QuizSummaryDTO struct {
	ID            string `json:"id"`
	LessonID      string `json:"lesson_id"`
	Title         string `json:"title"`
	Source        string `json:"source"`
	QuestionCount int32  `json:"question_count"`
	CreatedAt     string `json:"created_at"`
}

// ToQuizSummaryDTO 映射列表行。
func ToQuizSummaryDTO(r SummaryRow) QuizSummaryDTO {
	return QuizSummaryDTO{
		ID:            r.ID.String(),
		LessonID:      r.LessonID.String(),
		Title:         r.Title,
		Source:        r.Source,
		QuestionCount: r.QuestionCount,
		CreatedAt:     httpx.Time(r.CreatedAt),
	}
}
