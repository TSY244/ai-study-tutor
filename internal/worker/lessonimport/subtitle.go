package lessonimport

import (
	"regexp"
	"strings"
)

// 字幕清洗与 legacy material_parse 行为对齐（golden fixture 冻结）：
// clean_subtitle_text / _clean_vtt 去除序号、时间轴行与内联标签。

var (
	seqLine   = regexp.MustCompile(`^\d+$`)
	tagInline = regexp.MustCompile(`<[^>]+>`)
	cTag      = regexp.MustCompile(`</?c[^>]*>`)
	lrcTime   = regexp.MustCompile(`\[\d{2}:\d{2}[.\]]\d*\]`)
)

// CleanSubtitle 清洗 SRT 字幕文本：去空行、纯数字序号行、时间轴行（-->）、
// 内联标签，剩余行拼接后整体 Normalize。
func CleanSubtitle(text string) string {
	return cleanTimedText(text, false)
}

// CleanVTT 清洗 WebVTT 文本：额外跳过 WEBVTT 头与 NOTE 注释行，
// 并清除 <c> 类标签。
func CleanVTT(text string) string {
	return cleanTimedText(text, true)
}

func cleanTimedText(text string, vtt bool) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	var lines []string
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		upper := strings.ToUpper(line)
		if upper == "WEBVTT" {
			continue
		}
		if vtt && strings.HasPrefix(upper, "NOTE") {
			continue
		}
		if seqLine.MatchString(line) {
			continue
		}
		if strings.Contains(line, "-->") {
			continue
		}
		line = tagInline.ReplaceAllString(line, "")
		if vtt {
			line = cTag.ReplaceAllString(line, "")
		}
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return Normalize(strings.Join(lines, "\n"))
}

// CleanLRC 清洗 LRC 歌词/字幕格式（去 [mm:ss.xx] 时间戳）。
// 与 legacy _clean_lrc 一致：不再做整体空白规范化（管线的
// normalizing 阶段统一兜底）。
func CleanLRC(text string) string {
	var lines []string
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(lrcTime.ReplaceAllString(raw, ""))
		if line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}
