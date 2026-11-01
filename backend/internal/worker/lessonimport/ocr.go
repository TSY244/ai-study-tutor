package lessonimport

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/TSY244/ai-study-tutor/backend/internal/worker"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker/proc"
)

// Local OCR keeps the extracted source text available to every downstream generator.
func (h *Handler) extractImage(ctx context.Context, source, workDir string) (string, error) {
	// Tesseract/Leptonica cannot consistently decode WebP across distributions.
	head := make([]byte, 12)
	if f, err := os.Open(source); err == nil {
		_, _ = f.Read(head)
		_ = f.Close()
	}
	if strings.EqualFold(filepath.Ext(source), ".webp") || (string(head[:4]) == "RIFF" && string(head[8:12]) == "WEBP") {
		converted := filepath.Join(workDir, "ocr-converted.png")
		defer os.Remove(converted)
		if _, err := h.Proc.Run(ctx, proc.Cmd{Name: h.FFmpegBin, Args: []string{"-nostdin", "-i", source, "-frames:v", "1", "-y", converted}, Dir: workDir}); err != nil {
			return "", classifyProcErr(h.FFmpegBin, err)
		}
		source = converted
	}
	result, err := h.Proc.Run(ctx, proc.Cmd{Name: "tesseract", Args: []string{source, "stdout", "-l", "chi_sim+chi_tra+eng"}, Dir: workDir})
	if err != nil {
		var exit *proc.ExitError
		if errors.As(err, &exit) {
			return "", worker.NewError(worker.ErrCodeParseFailed, "图片文字识别失败，请上传清晰的 PNG/JPEG 图片或 PDF。", false)
		}
		return "", classifyProcErr("tesseract", err)
	}
	if result.StdoutTruncated {
		return "", worker.NewError(worker.ErrCodeParseFailed, "识别文字过多，请拆分文件后导入。", false)
	}
	return Normalize(string(result.Stdout)), nil
}

func (h *Handler) extractPDF(ctx context.Context, source, workDir string) (string, string, error) {
	result, err := h.Proc.Run(ctx, proc.Cmd{Name: h.PdftotextBin, Args: []string{"-layout", "-enc", "UTF-8", source, "-"}, Dir: workDir})
	if err != nil {
		var exit *proc.ExitError
		if errors.As(err, &exit) {
			return "", "", worker.NewError(worker.ErrCodeParseFailed, "PDF 文件解析失败，文件可能损坏或加密。", false)
		}
		return "", "", classifyProcErr(h.PdftotextBin, err)
	}
	if result.StdoutTruncated {
		return "", "", worker.NewError(worker.ErrCodeParseFailed, "PDF 文字过多，请拆分文件后导入。", false)
	}
	// Poppler emits one form-feed per page, including empty scanned pages. Keep
	// this boundary until OCR is complete, so mixed PDFs never lose scanned pages.
	pages := strings.Split(strings.TrimSuffix(string(result.Stdout), "\f"), "\f")
	if len(pages) > 300 {
		return "", "", worker.NewError(worker.ErrCodeParseFailed, "PDF 超过 300 页，请拆分后导入。", false)
	}
	var out strings.Builder
	for i, native := range pages {
		if err := ctx.Err(); err != nil {
			return "", "", err
		}
		text := Normalize(native)
		letters := 0
		for _, r := range text {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				letters++
			}
		}
		// A page containing only a page number/watermark may still be a scanned page.
		if letters < 20 {
			prefix := filepath.Join(workDir, fmt.Sprintf("pdf-page-%d", i+1))
			page := fmt.Sprint(i + 1)
			_, err := h.Proc.Run(ctx, proc.Cmd{Name: "pdftoppm", Args: []string{"-f", page, "-l", page, "-singlefile", "-r", "180", "-scale-to", "3200", "-png", source, prefix}, Dir: workDir})
			if err != nil {
				return "", "", classifyProcErr("pdftoppm", err)
			}
			recognized, err := h.extractImage(ctx, prefix+".png", workDir)
			_ = os.Remove(prefix + ".png")
			if err != nil {
				return "", "", err
			}
			if recognized != "" {
				text = recognized
			}
		}
		if text != "" {
			fmt.Fprintf(&out, "## 第 %d 页\n\n%s\n\n", i+1, text)
		}
		if out.Len() > 16<<20 {
			return "", "", worker.NewError(worker.ErrCodeParseFailed, "PDF 识别文字过多，请拆分后导入。", false)
		}
	}
	return out.String(), pdfVersion, nil
}
