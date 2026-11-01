package lessonimport

import (
	"archive/zip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TSY244/ai-study-tutor/backend/internal/worker"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker/proc"
)

func TestPDFPreservesPageBoundariesAndRecognizesMixedScannedPages(t *testing.T) {
	fake := proc.NewFake()
	native := "原生文字页面包含足够的课程知识点内容，不需要重复进行图片文字识别。"
	fake.OnRun = func(ctx context.Context, cmd proc.Cmd) (*proc.Result, error) {
		switch cmd.Name {
		case "pdftotext":
			return &proc.Result{Stdout: []byte(native + "\f2\f\f")}, nil
		case "pdftoppm":
			page := cmd.Args[1]
			if page != "2" && page != "3" {
				t.Fatalf("rendered native text page: %v", cmd.Args)
			}
			return &proc.Result{}, nil
		case "tesseract":
			text := ""
			if strings.Contains(cmd.Args[0], "page-2") {
				text = "扫描页知识点：二分查找要求有序输入。"
			}
			return &proc.Result{Stdout: []byte(text)}, nil
		default:
			t.Fatalf("unexpected command %s", cmd.Name)
			return nil, nil
		}
	}
	h := Handler{Proc: fake, PdftotextBin: "pdftotext"}
	text, version, err := h.extractPDF(context.Background(), "mixed.pdf", t.TempDir())
	if err != nil || version != pdfVersion || !strings.Contains(text, native) || !strings.Contains(text, "## 第 2 页\n\n扫描页") || strings.Contains(text, "第 3 页") {
		t.Fatalf("text=%q version=%s err=%v", text, version, err)
	}
}
func TestOCRFailuresAreActionableAndCancellationSurvives(t *testing.T) {
	for _, cause := range []error{context.Canceled, &proc.MissingToolError{Tool: "tesseract"}, &proc.ExitError{Code: 1}} {
		fake := proc.NewFake()
		fake.OnRun = func(context.Context, proc.Cmd) (*proc.Result, error) { return nil, cause }
		h := Handler{Proc: fake}
		_, err := h.extractImage(context.Background(), "scan.png", t.TempDir())
		var coded *worker.Error
		if cause == context.Canceled {
			if !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
		} else if !errors.As(err, &coded) || coded.Retryable {
			t.Fatalf("must fail actionably: %v", err)
		}
	}
}
func TestVisualSignaturesBeatMisleadingExtensions(t *testing.T) {
	for _, head := range [][]byte{[]byte("\x89PNG\r\n\x1a\n"), {0xff, 0xd8, 0xff}, []byte("RIFFxxxxWEBP"), []byte("II\x2a\x00"), []byte("MM\x00\x2a"), []byte("GIF89a"), []byte("BMxxxxx")} {
		if got := DetectType(head, ".mp3"); got != TypeImage {
			t.Fatalf("image signature %x classified %s", head, got)
		}
	}
	if got := DetectType([]byte("PK\x03\x04"), ".pptx"); got != TypePPTX {
		t.Fatal(got)
	}
}
func writePPTXFixture(t *testing.T, members map[string]string) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "deck.pptx")
	f, e := os.Create(file)
	if e != nil {
		t.Fatal(e)
	}
	z := zip.NewWriter(f)
	for name, data := range members {
		w, e := z.Create(name)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = w.Write([]byte(data)); e != nil {
			t.Fatal(e)
		}
	}
	if e = z.Close(); e != nil {
		t.Fatal(e)
	}
	if e = f.Close(); e != nil {
		t.Fatal(e)
	}
	return file
}
func TestPPTXUsesPresentationOrderAndIncludesNotesChartsAndImageOCR(t *testing.T) {
	fixture := writePPTXFixture(t, map[string]string{
		"ppt/presentation.xml":              `<p:presentation xmlns:p="p" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="first"/><p:sldId id="257" r:id="second"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels":   `<Relationships><Relationship Id="first" Type="x/slide" Target="slides/slide10.xml"/><Relationship Id="second" Type="x/slide" Target="slides/slide2.xml"/></Relationships>`,
		"ppt/slides/slide10.xml":            `<s><p><t>第一张：有序输入</t></p><p><t>表格第一行</t></p></s>`,
		"ppt/slides/slide2.xml":             `<s><p><t>第二张：缩小搜索范围</t></p></s>`,
		"ppt/slides/_rels/slide10.xml.rels": `<Relationships><Relationship Id="n" Type="x/notesSlide" Target="../notesSlides/notesSlide1.xml"/><Relationship Id="c" Type="x/chart" Target="../charts/chart1.xml"/><Relationship Id="i" Type="x/image" Target="../media/image1.png"/><Relationship Id="external" Type="x/image" Target="https://example.invalid/secret.png" TargetMode="External"/></Relationships>`,
		"ppt/notesSlides/notesSlide1.xml":   `<s><p><t>演讲者补充：每次排除一半。</t></p></s>`,
		"ppt/charts/chart1.xml":             `<chart><t>复杂度</t><v>输入数量</v><v>100</v><v>7</v><f>Sheet!A1</f></chart>`,
		"ppt/media/image1.png":              "PNG fixture",
	})
	fake := proc.NewFake()
	calls := 0
	fake.OnRun = func(ctx context.Context, cmd proc.Cmd) (*proc.Result, error) {
		calls++
		if cmd.Name != "tesseract" {
			t.Fatal(cmd.Name)
		}
		return &proc.Result{Stdout: []byte("图片说明：对数时间复杂度")}, nil
	}
	h := Handler{Proc: fake}
	text, err := h.extractPPTX(context.Background(), fixture, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"第 1 张幻灯片", "表格第一行", "演讲者补充", "输入数量", "100", "图片说明：对数时间复杂度", "第 2 张幻灯片"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %q", want, text)
		}
	}
	if strings.Index(text, "第一张") > strings.Index(text, "第二张") || calls != 1 || strings.Contains(text, "Sheet!A1") {
		t.Fatalf("incorrect order, external fetch or formula extraction: %s calls=%d", text, calls)
	}
}
func TestPPTXRejectsBrokenOrEmptyArchives(t *testing.T) {
	h := Handler{}
	for _, members := range []map[string]string{{"word/document.xml": "<p/>"}, {"ppt/presentation.xml": "<broken"}} {
		_, err := h.extractPPTX(context.Background(), writePPTXFixture(t, members), t.TempDir())
		var coded *worker.Error
		if !errors.As(err, &coded) || coded.Code != worker.ErrCodeParseFailed {
			t.Fatalf("expected parse failure: %v", err)
		}
	}
}
