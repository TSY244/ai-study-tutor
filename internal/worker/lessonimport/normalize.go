// Package lessonimport 实现 lesson_import Job 的素材处理管线（03 §8）：
//
//	acquiring -> detecting -> extracting -> normalizing -> chunking ->
//	validating -> publishing
//
// generating_teacher_outline / generating_student_notes 需要 Claude，
// 在 B6 接入（稳定 stage code 已预留，本阶段“按需跳过”）。
//
// 成功路径在 Job Complete 同事务发布 raw_text 文档新版本、推进素材/课时
// 状态；失败/取消不改变任何 active 文档。
package lessonimport

import (
	"regexp"
	"strings"
)

// NormalizeVersion 是文本规范化算法版本（结果随 material metadata 持久化，
// 版本变更必须引入新版本号以保证可复现）。
const NormalizeVersion = "norm/v1"

var (
	wsRun   = regexp.MustCompile(`[ \t]+`)
	blankLn = regexp.MustCompile(`\n{3,}`)
)

// Normalize 统一 UTF-8 文本的换行与空白（与 legacy chunking.clean_text
// 行为一致：\r\n/\r -> \n，连续空格/制表符合并为单个空格，
// 三行及以上空行压缩为一个空行，整体去首尾空白）。
func Normalize(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = wsRun.ReplaceAllString(text, " ")
	text = blankLn.ReplaceAllString(text, "\n\n")
	return strings.TrimSpace(text)
}
