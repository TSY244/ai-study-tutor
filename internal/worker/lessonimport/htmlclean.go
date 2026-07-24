package lessonimport

import (
	"html"
	"regexp"
	"strings"
)

// HTMLVersion 是 HTML 清洗器版本（03 §8.3：移除脚本、样式和不可见文本）。
const HTMLVersion = "html/v1"

var (
	reScript   = regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script\s*>`)
	reStyle    = regexp.MustCompile(`(?is)<style\b[^>]*>.*?</style\s*>`)
	reHidden   = regexp.MustCompile(`(?is)<!--.*?-->`)
	reTag      = regexp.MustCompile(`(?s)<[^>]*>`)
	reBlockEnd = regexp.MustCompile(`(?is)</(p|div|section|article|h[1-6]|li|tr|table|ul|ol|blockquote|pre|br)\s*>`)
	// 不可见容器（display:none / visibility:hidden / hidden 属性）。
	// RE2 不支持反向引用，对常见容器逐个匹配；嵌套同标签的极端
	// 文档会残留在边界，属已记录的近似限制。
	reInvisibleDiv     = regexp.MustCompile(`(?is)<div\b[^>]*?(?:display\s*:\s*none|visibility\s*:\s*hidden|\shidden[\s>])[^>]*>.*?</div\s*>`)
	reInvisibleSpan    = regexp.MustCompile(`(?is)<span\b[^>]*?(?:display\s*:\s*none|visibility\s*:\s*hidden|\shidden[\s>])[^>]*>.*?</span\s*>`)
	reInvisibleSection = regexp.MustCompile(`(?is)<section\b[^>]*?(?:display\s*:\s*none|visibility\s*:\s*hidden|\shidden[\s>])[^>]*>.*?</section\s*>`)
)

// StripHTML 移除脚本、样式、注释、不可见容器与全部标签，返回纯文本：
// 块级结束标签先替换为换行以保留段落结构，标签剥离后做 HTML 实体反转义。
// 输入必须已是合法 UTF-8（detecting 阶段保证）。
func StripHTML(text string) string {
	text = reScript.ReplaceAllString(text, " ")
	text = reStyle.ReplaceAllString(text, " ")
	text = reHidden.ReplaceAllString(text, " ")
	text = reInvisibleDiv.ReplaceAllString(text, " ")
	text = reInvisibleSpan.ReplaceAllString(text, " ")
	text = reInvisibleSection.ReplaceAllString(text, " ")
	text = reBlockEnd.ReplaceAllString(text, "\n")
	text = reTag.ReplaceAllString(text, " ")
	text = html.UnescapeString(text)
	// HTML 清洗产生的行首尾空白无语义（标签剥离残留），逐行去除。
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(l)
	}
	return Normalize(strings.Join(lines, "\n"))
}
