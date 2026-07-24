package lessonimport

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

// buildDocx 在临时目录构造最小 DOCX（zip + word/document.xml）。
func buildDocx(t *testing.T, documentXML string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sample.docx")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	w := zip.NewWriter(f)
	entry, err := w.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte(documentXML)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

const sampleDocxXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>第一章 极限</w:t></w:r></w:p>
    <w:p><w:r><w:t>极限的</w:t></w:r><w:r><w:t>定义</w:t></w:r></w:p>
    <w:p><w:r><w:t></w:t></w:r></w:p>
    <w:p><w:r><w:t>第二章 连续</w:t></w:r></w:p>
  </w:body>
</w:document>`

func TestExtractDocx(t *testing.T) {
	path := buildDocx(t, sampleDocxXML)
	text, err := ExtractDocx(path)
	if err != nil {
		t.Fatalf("ExtractDocx 失败: %v", err)
	}
	want := "第一章 极限\n极限的定义\n第二章 连续"
	if text != want {
		t.Fatalf("提取结果:\n期望 %q\n实际 %q", want, text)
	}
}

func TestExtractDocxNotAZip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.docx")
	if err := os.WriteFile(path, []byte("not a zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ExtractDocx(path); err == nil {
		t.Fatal("损坏文件应返回错误")
	}
}

func TestStripHTML(t *testing.T) {
	input := `<!DOCTYPE html><html><head><style>body{color:red}</style></head>` +
		`<body><h1>第一章</h1><script>alert(1)</script><!-- 注释 -->` +
		`<p>极限 &amp; 连续</p><div style="display:none">隐藏文本</div><p>第二节</p></body></html>`
	got := StripHTML(input)
	want := "第一章\n极限 & 连续\n第二节"
	if got != want {
		t.Fatalf("StripHTML:\n期望 %q\n实际 %q", want, got)
	}
}
