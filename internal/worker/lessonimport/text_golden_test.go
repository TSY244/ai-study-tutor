// text_golden_test.go 以 testdata/legacy-fixtures/text-processing.golden.json
// 冻结 legacy clean_text / clean_subtitle_text / _clean_vtt / _clean_lrc 行为。
// fixture 期望值已用 legacy Python 运行时实际执行核对（见交付报告）。
package lessonimport

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type textFixture struct {
	Groups map[string]struct {
		Cases []struct {
			Name   string `json:"name"`
			Input  string `json:"input"`
			Expect string `json:"expect"`
		} `json:"cases"`
	} `json:"-"`
}

func loadTextFixture(t *testing.T) map[string][]struct {
	Name   string `json:"name"`
	Input  string `json:"input"`
	Expect string `json:"expect"`
} {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "legacy-fixtures", "text-processing.golden.json"))
	if err != nil {
		t.Fatalf("读取 fixture 失败: %v", err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("解析 fixture 失败: %v", err)
	}
	type group struct {
		Cases []struct {
			Name   string `json:"name"`
			Input  string `json:"input"`
			Expect string `json:"expect"`
		} `json:"cases"`
	}
	out := map[string][]struct {
		Name   string `json:"name"`
		Input  string `json:"input"`
		Expect string `json:"expect"`
	}{}
	for _, key := range []string{"normalize", "srt", "vtt", "lrc"} {
		var g group
		if err := json.Unmarshal(doc[key], &g); err != nil {
			t.Fatalf("解析 fixture 组 %s 失败: %v", key, err)
		}
		out[key] = g.Cases
	}
	return out
}

func TestNormalizeGolden(t *testing.T) {
	for _, c := range loadTextFixture(t)["normalize"] {
		t.Run(c.Name, func(t *testing.T) {
			if got := Normalize(c.Input); got != c.Expect {
				t.Fatalf("Normalize 不一致:\n输入: %q\n期望: %q\n实际: %q", c.Input, c.Expect, got)
			}
		})
	}
}

func TestCleanSubtitleGolden(t *testing.T) {
	for _, c := range loadTextFixture(t)["srt"] {
		t.Run(c.Name, func(t *testing.T) {
			if got := CleanSubtitle(c.Input); got != c.Expect {
				t.Fatalf("CleanSubtitle 不一致:\n输入: %q\n期望: %q\n实际: %q", c.Input, c.Expect, got)
			}
		})
	}
}

func TestCleanVTTGolden(t *testing.T) {
	for _, c := range loadTextFixture(t)["vtt"] {
		t.Run(c.Name, func(t *testing.T) {
			if got := CleanVTT(c.Input); got != c.Expect {
				t.Fatalf("CleanVTT 不一致:\n输入: %q\n期望: %q\n实际: %q", c.Input, c.Expect, got)
			}
		})
	}
}

func TestCleanLRCGolden(t *testing.T) {
	for _, c := range loadTextFixture(t)["lrc"] {
		t.Run(c.Name, func(t *testing.T) {
			if got := CleanLRC(c.Input); got != c.Expect {
				t.Fatalf("CleanLRC 不一致:\n输入: %q\n期望: %q\n实际: %q", c.Input, c.Expect, got)
			}
		})
	}
}
