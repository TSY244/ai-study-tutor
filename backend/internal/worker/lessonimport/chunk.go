package lessonimport

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// ChunkVersion 是分块算法版本。B6 生成任务以同一版本从 active raw_text
// 文档重新分块，算法不变则结果逐字节可复现；变更必须引入新版本号。
const ChunkVersion = "chunk/v1"

// ChunkTargetRunes 是单个 chunk 的目标上限（按 rune 计）。
const ChunkTargetRunes = 1200

// Chunk 是一个确定性文本块的清单项（只含定位与校验信息，不含正文）。
type Chunk struct {
	ID     string `json:"id"`
	SHA256 string `json:"sha256"`
	Runes  int    `json:"runes"`
}

// ChunkContents 按段落（空行分隔）贪心装箱切分文本，返回正文切片：
//
//   - 段落按出现顺序依次装入当前块；装入会超过 ChunkTargetRunes 且当前块
//     非空时封块，开启新块；
//   - 单段超过上限时按 rune 边界硬切；
//   - 空文本返回空切片。
//
// 同一输入永远产生同一块序列（无随机性、无时间依赖）。
func ChunkContents(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	paragraphs := strings.Split(text, "\n\n")
	var out []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() == 0 {
			return
		}
		out = append(out, cur.String())
		cur.Reset()
	}
	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		for len([]rune(p)) > ChunkTargetRunes {
			// 当前块先封块，再硬切超长段落。
			flush()
			head, tail := splitRunes(p, ChunkTargetRunes)
			out = append(out, head)
			p = strings.TrimSpace(tail)
		}
		if p == "" {
			continue
		}
		if cur.Len() > 0 && len([]rune(cur.String()))+len([]rune(p))+2 > ChunkTargetRunes {
			flush()
		}
		if cur.Len() > 0 {
			cur.WriteString("\n\n")
		}
		cur.WriteString(p)
	}
	flush()
	return out
}

// ChunkText 返回与 ChunkContents 对应的清单（ID/sha256/runes，不含正文）。
func ChunkText(text string) []Chunk {
	return ChunksFromContents(ChunkContents(text))
}

// ChunksFromContents 由正文切片生成清单（B6 复现路径使用）。
func ChunksFromContents(contents []string) []Chunk {
	var chunks []Chunk
	for _, c := range contents {
		sum := sha256.Sum256([]byte(c))
		chunks = append(chunks, Chunk{
			ID:     fmt.Sprintf("c%04d", len(chunks)+1),
			SHA256: hex.EncodeToString(sum[:]),
			Runes:  len([]rune(c)),
		})
	}
	return chunks
}

// splitRunes 把 s 按 rune 数切为前 n 与剩余部分。
func splitRunes(s string, n int) (head, tail string) {
	r := []rune(s)
	if len(r) <= n {
		return s, ""
	}
	return string(r[:n]), string(r[n:])
}
