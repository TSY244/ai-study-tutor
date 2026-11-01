package ai

import (
	"strings"
	"testing"
)

func TestStudySummaryFormat(t *testing.T) {
	valid := "# 极限入门\n\n" + strings.Join(summaryHeadings, "\n内容。\n") + "\n依据资料，无法确认未提供的信息。"
	if title, err := StudySummaryTitle(valid); err != nil || title != "极限入门" {
		t.Fatalf("valid summary: title=%q err=%v", title, err)
	}
	for name, text := range map[string]string{
		"missing title":    strings.TrimPrefix(valid, "# 极限入门\n\n"),
		"long title":       strings.Replace(valid, "极限入门", strings.Repeat("长", 81), 1),
		"missing section":  strings.Replace(valid, "### 来源与局限", "### 其他", 1),
		"empty section":    strings.Replace(valid, "### 核心知识点\n内容。", "### 核心知识点", 1),
		"wrong order":      strings.Replace(valid, "### 核心知识点", "### 知识结构", 1),
		"headings in code": "# 极限\n```markdown\n" + strings.Join(summaryHeadings, "\n内容\n") + "\n```",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := StudySummaryTitle(text)
			if err == nil || AsProviderError(err).Code != ErrSchemaInvalid || !AsProviderError(err).Retryable {
				t.Fatalf("malformed summary accepted: %v", err)
			}
		})
	}
	for _, kind := range []string{"teacher_outline", "student_notes"} {
		req, err := BuildLessonDocumentRequest(LessonDocumentPromptInput{Kind: kind, RawText: "source"})
		if err != nil || !strings.Contains(req.System, StudySummaryInstructions) {
			t.Fatalf("shared format not used by %s", kind)
		}
	}
}
