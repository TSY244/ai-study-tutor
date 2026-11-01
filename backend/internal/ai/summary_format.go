package ai

import (
	"strings"
	"unicode/utf8"
)

// StudySummaryInstructions is shared by document generation and video extraction.
const StudySummaryInstructions = `学习总结统一格式 study-summary/v1：使用中文 Markdown，严格按以下标题和顺序输出，每节都要有内容：
# <简洁的学习主题名称，最多80字符，不使用“教师教纲”“学生笔记”等泛称>
## 综合总结
### 内容概览
用一小段说明主题、适用场景和学习目标。
### 核心知识点
分条解释关键概念、公式、方法及其适用条件。
### 知识结构
按逻辑或学习顺序组织知识，说明各部分的联系。
### 示例与应用
整理资料中的例子、步骤和应用；没有示例则注明“资料未提供示例”。
### 结论与复习
列出关键结论和可用于自测的复习问题。
### 来源与局限
注明材料来源、可用的时间戳或章节，以及缺失、采样和不确定信息。
只依据资料，不编造示例、引用、时间戳或事实；缺失内容明确注明。教纲侧重教学安排和讲解顺序，笔记侧重概念理解和复习。视频可在综合总结前单独保留“## 画面内容”，原文和字幕保持完整。`

var summaryHeadings = []string{
	"## 综合总结", "### 内容概览", "### 核心知识点", "### 知识结构",
	"### 示例与应用", "### 结论与复习", "### 来源与局限",
}

// StudySummaryTitle validates the shared format before publication and returns
// the AI-generated topic title, which can name a course awaiting its first summary.
func StudySummaryTitle(markdown string) (string, error) {
	lines := strings.Split(strings.ReplaceAll(strings.TrimSpace(markdown), "\r\n", "\n"), "\n")
	invalid := func() (string, error) {
		return "", ErrorForCode(ErrSchemaInvalid, "AI 总结未遵循统一格式 study-summary/v1，将自动重试。")
	}
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "# ") {
		return invalid()
	}
	title := strings.TrimSpace(strings.TrimPrefix(lines[0], "# "))
	if title == "" || utf8.RuneCountInString(title) > 80 {
		return invalid()
	}
	next, content := 0, false
	fence := ""
	for _, raw := range lines[1:] {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
			marker := line[:3]
			if fence == "" {
				fence = marker
			} else if fence == marker {
				fence = ""
			}
			continue
		}
		if fence != "" {
			if next > 1 && line != "" {
				content = true
			}
			continue
		}
		if strings.HasPrefix(line, "# ") {
			return invalid()
		}
		matched := false
		for i, heading := range summaryHeadings {
			if line != heading {
				continue
			}
			if i != next || (next > 1 && !content) {
				return invalid()
			}
			next++
			content = false
			matched = true
			break
		}
		if !matched && line != "" && !strings.HasPrefix(line, "#") && next > 1 {
			content = true
		}
	}
	if next != len(summaryHeadings) || !content || fence != "" {
		return invalid()
	}
	return title, nil
}
