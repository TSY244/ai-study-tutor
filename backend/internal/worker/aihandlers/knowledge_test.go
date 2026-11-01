package aihandlers

import (
	"encoding/json"
	"testing"

	"github.com/TSY244/ai-study-tutor/backend/internal/course"
)

func TestKnowledgeRequiresOriginalEvidence(t *testing.T) {
	valid := course.KnowledgeDraft{Label: "有序性", Explanation: "二分查找需要有序数据。", LearningObjective: "判断何时可以使用二分查找。", SourceExcerpt: "数据必须有序", Importance: "core"}
	for _, tc := range []struct {
		name string
		edit func(*course.KnowledgeDraft)
	}{
		{"invented evidence", func(p *course.KnowledgeDraft) { p.SourceExcerpt = "数据无需有序" }},
		{"missing objective", func(p *course.KnowledgeDraft) { p.LearningObjective = "" }},
		{"missing explanation", func(p *course.KnowledgeDraft) { p.Explanation = "" }},
		{"invalid importance", func(p *course.KnowledgeDraft) { p.Importance = "other" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := valid
			tc.edit(&p)
			if validateKnowledge([]course.KnowledgeDraft{p}, "二分查找的数据必须有序。", 5) == nil {
				t.Fatal("invalid analysis accepted")
			}
		})
	}
	if err := validateKnowledge([]course.KnowledgeDraft{valid}, "二分查找的数据必须有序。", 5); err != nil {
		t.Fatal(err)
	}
	if validateKnowledge([]course.KnowledgeDraft{valid, valid}, "数据必须有序", 5) == nil {
		t.Fatal("duplicate knowledge accepted")
	}
}

func TestQuestionsMustCoverKnownPoints(t *testing.T) {
	points := []course.KnowledgeDraft{{Label: "有序性", Importance: "core"}, {Label: "边界条件", Importance: "core"}}
	valid := quizResult{}
	if err := json.Unmarshal([]byte(`{"questions":[{"stem":"无序数组是否适合二分查找？","tested_concepts":["有序性"],"explanation":"利用有序性排除一半区间。"}]}`), &valid); err != nil {
		t.Fatal(err)
	}
	q := valid.Questions[0]
	if validateKnowledgeQuestions(valid, points, 1, "") == nil {
		t.Fatal("missing core coverage accepted")
	}
	if err := validateKnowledgeQuestions(valid, points, 1, "只练习有序性"); err != nil {
		t.Fatal(err)
	}
	valid.Questions[0].TestedConcepts = []string{"视频标题"}
	if validateKnowledgeQuestions(valid, points, 1, "只练习有序性") == nil {
		t.Fatal("unknown point accepted")
	}
	valid.Questions[0] = q
	valid.Questions = append(valid.Questions, q)
	if validateKnowledgeQuestions(valid, points, 2, "只练习有序性") == nil {
		t.Fatal("duplicate questions accepted")
	}
}
