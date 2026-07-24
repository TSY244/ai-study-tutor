package lessonimport

import (
	"strings"
	"testing"
)

func TestChunkTextEmpty(t *testing.T) {
	if got := ChunkText("  \n\n "); got != nil {
		t.Fatalf("空文本应返回 nil，得到 %v", got)
	}
}

func TestChunkTextSingleParagraph(t *testing.T) {
	chunks := ChunkText("极限的定义。")
	if len(chunks) != 1 || chunks[0].ID != "c0001" {
		t.Fatalf("单段应产生一个块: %+v", chunks)
	}
	if chunks[0].Runes != 6 {
		t.Fatalf("rune 数错误: %d", chunks[0].Runes)
	}
	if len(chunks[0].SHA256) != 64 {
		t.Fatalf("sha256 长度错误: %q", chunks[0].SHA256)
	}
}

func TestChunkTextParagraphPacking(t *testing.T) {
	// 两段各 800 rune：第一段装下后第二段超限，应分两块。
	p1 := strings.Repeat("甲", 800)
	p2 := strings.Repeat("乙", 800)
	chunks := ChunkText(p1 + "\n\n" + p2)
	if len(chunks) != 2 {
		t.Fatalf("应分两块: %d", len(chunks))
	}
	contents := ChunkContents(p1 + "\n\n" + p2)
	if contents[0] != p1 || contents[1] != p2 {
		t.Fatalf("块正文不忠实于原段落")
	}
}

func TestChunkTextHardSplit(t *testing.T) {
	// 单段超过上限按 rune 边界硬切。
	p := strings.Repeat("数", ChunkTargetRunes+100)
	contents := ChunkContents(p)
	if len(contents) != 2 {
		t.Fatalf("应硬切为两块: %d", len(contents))
	}
	if len([]rune(contents[0])) != ChunkTargetRunes || len([]rune(contents[1])) != 100 {
		t.Fatalf("硬切长度错误: %d/%d", len([]rune(contents[0])), len([]rune(contents[1])))
	}
}

func TestChunkTextDeterministic(t *testing.T) {
	text := "第一章 极限\n\n" + strings.Repeat("内容。", 500) + "\n\n第二章 连续"
	first := ChunkText(text)
	second := ChunkText(text)
	if len(first) != len(second) {
		t.Fatalf("两次分块数量不同")
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("块 %d 不一致: %+v != %+v", i, first[i], second[i])
		}
	}
	// 清单与正文一一对应。
	if got := ChunksFromContents(ChunkContents(text)); len(got) != len(first) {
		t.Fatalf("Contents/Manifest 数量不一致")
	} else {
		for i := range got {
			if got[i] != first[i] {
				t.Fatalf("块 %d 清单与正文推导不一致", i)
			}
		}
	}
}
