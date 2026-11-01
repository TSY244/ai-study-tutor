package lessonimport

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	zipPath "path"
	"path/filepath"
	"strings"

	"github.com/TSY244/ai-study-tutor/backend/internal/worker"
)

type pptRelation struct {
	ID     string `xml:"Id,attr"`
	Type   string `xml:"Type,attr"`
	Target string `xml:"Target,attr"`
	Mode   string `xml:"TargetMode,attr"`
}

// Resolve only archive members; external relationships are never fetched.
func pptTarget(base, target string) string {
	if strings.HasPrefix(target, "/") {
		return strings.TrimPrefix(zipPath.Clean(target), "/")
	}
	return zipPath.Clean(zipPath.Join(zipPath.Dir(base), target))
}

func (h *Handler) extractPPTX(ctx context.Context, source, workDir string) (text string, err error) {
	defer func() {
		if err != nil && ctx.Err() == nil {
			var coded *worker.Error
			if !errors.As(err, &coded) {
				err = worker.WrapError(worker.ErrCodeParseFailed, "PPTX 解析失败，请检查文件，或将课件导出为 PDF 后导入。", false, err)
			}
		}
	}()
	z, err := zip.OpenReader(source)
	if err != nil {
		return "", err
	}
	defer z.Close()
	members := make(map[string]*zip.File, len(z.File))
	var total uint64
	if len(z.File) > 10000 {
		return "", fmt.Errorf("too many archive entries")
	}
	for _, f := range z.File {
		total += f.UncompressedSize64
		if total > 512<<20 {
			return "", fmt.Errorf("expanded PPTX too large")
		}
		members[f.Name] = f
	}
	read := func(name string) ([]byte, error) {
		f := members[name]
		if f == nil {
			return nil, fmt.Errorf("missing member %s", name)
		}
		if f.UncompressedSize64 > 32<<20 {
			return nil, fmt.Errorf("archive member too large")
		}
		r, e := f.Open()
		if e != nil {
			return nil, e
		}
		defer r.Close()
		return io.ReadAll(io.LimitReader(r, (32<<20)+1))
	}
	relations := func(base string) ([]pptRelation, error) {
		name := zipPath.Join(zipPath.Dir(base), "_rels", zipPath.Base(base)+".rels")
		if members[name] == nil {
			return nil, nil
		}
		data, e := read(name)
		if e != nil {
			return nil, e
		}
		var rels struct {
			Items []pptRelation `xml:"Relationship"`
		}
		e = xml.Unmarshal(data, &rels)
		return rels.Items, e
	}
	presentation, err := read("ppt/presentation.xml")
	if err != nil {
		return "", err
	}
	var order struct {
		Slides []struct {
			ID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
		} `xml:"sldIdLst>sldId"`
	}
	if err = xml.Unmarshal(presentation, &order); err != nil {
		return "", err
	}
	if len(order.Slides) == 0 || len(order.Slides) > 300 {
		return "", fmt.Errorf("PPTX must contain 1–300 slides")
	}
	rels, err := relations("ppt/presentation.xml")
	if err != nil {
		return "", err
	}
	slidePaths := map[string]string{}
	for _, rel := range rels {
		if rel.Mode != "External" && strings.HasSuffix(rel.Type, "/slide") {
			slidePaths[rel.ID] = pptTarget("ppt/presentation.xml", rel.Target)
		}
	}
	var out strings.Builder
	content := false
	imageCount := 0
	for i, slide := range order.Slides {
		if err = ctx.Err(); err != nil {
			return "", err
		}
		name := slidePaths[slide.ID]
		data, e := read(name)
		if e != nil {
			return "", e
		}
		body, e := pptText(data, false)
		if e != nil {
			return "", e
		}
		parts := []string{}
		if body != "" {
			parts = append(parts, body)
		}
		rels, e := relations(name)
		if e != nil {
			return "", e
		}
		for _, rel := range rels {
			if rel.Mode == "External" {
				continue
			}
			target := pptTarget(name, rel.Target)
			switch {
			case strings.HasSuffix(rel.Type, "/notesSlide"), strings.HasSuffix(rel.Type, "/chart"):
				data, e := read(target)
				if e != nil {
					return "", e
				}
				isChart := strings.HasSuffix(rel.Type, "/chart")
				extra, e := pptText(data, isChart)
				if e != nil {
					return "", e
				}
				label := "演讲者备注"
				if isChart {
					label = "图表文字与数据"
				}
				if extra != "" {
					parts = append(parts, "### "+label+"\n\n"+extra)
				}
			case strings.HasSuffix(rel.Type, "/image"):
				ext := strings.ToLower(zipPath.Ext(target))
				switch ext {
				case ".png", ".jpg", ".jpeg", ".tif", ".tiff", ".bmp", ".gif", ".webp":
				default:
					continue
				}
				imageCount++
				if imageCount > 1000 {
					return "", fmt.Errorf("too many slide images; export PDF or split deck")
				}
				data, e := read(target)
				if e != nil {
					return "", e
				}
				img := filepath.Join(workDir, fmt.Sprintf("ppt-image-%d%s", imageCount, ext))
				if e = os.WriteFile(img, data, 0600); e != nil {
					return "", e
				}
				recognized, e := h.extractImage(ctx, img, workDir)
				_ = os.Remove(img)
				if e != nil {
					return "", e
				}
				if recognized != "" {
					parts = append(parts, "### 图片文字（OCR）\n\n"+recognized)
				}
			}
		}
		if len(parts) > 0 {
			content = true
			fmt.Fprintf(&out, "## 第 %d 张幻灯片\n\n%s\n\n", i+1, strings.Join(parts, "\n\n"))
		}
		if out.Len() > 16<<20 {
			return "", fmt.Errorf("PPTX extracted text too large")
		}
	}
	if !content {
		return "", worker.NewError(worker.ErrCodeEmptyContent, "课件没有可识别的文字。矢量图片或特殊图表请导出为 PDF 再导入。", false)
	}
	return out.String(), nil
}

// DrawingML uses a:t for paragraphs/tables; chart caches use c:v. Formula
// references and formatting metadata are deliberately excluded from the source.
func pptText(data []byte, chart bool) (string, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var out strings.Builder
	inText := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "t" || (chart && t.Name.Local == "v") {
				inText = true
			}
			if t.Name.Local == "br" {
				out.WriteByte('\n')
			}
		case xml.CharData:
			if inText {
				out.Write(t)
			}
		case xml.EndElement:
			if t.Name.Local == "t" || (chart && t.Name.Local == "v") {
				inText = false
				if chart {
					out.WriteByte('\n')
				}
			}
			if t.Name.Local == "p" {
				out.WriteByte('\n')
			}
		}
	}
	return Normalize(out.String()), nil
}
