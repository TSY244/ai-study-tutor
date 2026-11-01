package lessonimport

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"unicode/utf8"

	"github.com/TSY244/ai-study-tutor/backend/internal/worker"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker/proc"
)

// 提取器版本（随素材 metadata 持久化；算法变更必须升级版本号）。
const (
	txtVersion   = "txt/v1"
	srtVersion   = "srt/v1"
	vttVersion   = "vtt/v1"
	pdfVersion   = "pdf-ocr/v2"
	mediaVersion = "whisper-local/v2"
)

// extract 按检测类型提取文本，返回（可能未归一化的）文本与提取器版本。
func (h *Handler) extract(ctx context.Context, path, dtype, effectiveMode, workDir, language string) (string, string, error) {
	switch dtype {
	case TypeTXT, TypeMD:
		text, err := readTextFile(path)
		return text, txtVersion, err
	case TypeSRT:
		raw, err := readTextFile(path)
		if err != nil {
			return "", "", err
		}
		return CleanSubtitle(raw), srtVersion, nil
	case TypeVTT:
		raw, err := readTextFile(path)
		if err != nil {
			return "", "", err
		}
		return CleanVTT(raw), vttVersion, nil
	case TypeHTML:
		raw, err := readTextFile(path)
		if err != nil {
			return "", "", err
		}
		return StripHTML(raw), HTMLVersion, nil
	case TypeDOCX:
		text, err := ExtractDocx(path)
		if err != nil {
			return "", "", worker.WrapError(worker.ErrCodeParseFailed,
				"DOCX 文件解析失败，文件可能损坏。", false, err)
		}
		return text, DocxVersion, nil
	case TypeImage:
		text, err := h.extractImage(ctx, path, workDir)
		return text, "image-ocr/v1", err
	case TypePPTX:
		text, err := h.extractPPTX(ctx, path, workDir)
		return text, "pptx/v1", err
	case TypePDF:
		return h.extractPDF(ctx, path, workDir)
	case TypeAudio, TypeVideo:
		return h.extractMedia(ctx, path, dtype, effectiveMode, workDir, language)
	default:
		return "", "", worker.NewError(worker.ErrCodeUnsupportedType,
			"无法识别的文件类型。支持：图片/PDF/PPTX/DOCX/TXT/Markdown/SRT/VTT/HTML 及常见音视频。", false)
	}
}

// readTextFile 读取文本素材并校验 UTF-8（体积上限在 normalizing 阶段强制）。
func readTextFile(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", worker.WrapError(worker.ErrCodeInternal, "读取素材文件失败，将自动重试。", true, err)
	}
	if !utf8.Valid(raw) {
		return "", worker.NewError(worker.ErrCodeParseFailed,
			"文件不是有效的 UTF-8 文本。请转换为 UTF-8 编码后重新上传。", false)
	}
	return string(raw), nil
}

// extractMedia 音视频管线：内嵌字幕优先（ffmpeg -map 0:s:0），否则按
// 转写模式 whisper_local 转写或返回可行动 transcript_required（03 §8.4）。
// 同目录字幕不适用：上传是单文件，任务目录按 job_id 隔离。
func (h *Handler) extractMedia(ctx context.Context, path, dtype, effectiveMode, workDir, language string) (string, string, error) {
	// 1) 内嵌字幕。
	subPath := filepath.Join(workDir, "embedded.srt")
	args := []string{"-nostdin", "-i", path, "-map", "0:s:0", "-f", "srt", subPath, "-y"}
	if _, err := h.Proc.Run(ctx, proc.Cmd{Name: h.FFmpegBin, Args: args, Dir: workDir}); err == nil {
		if raw, rerr := os.ReadFile(subPath); rerr == nil {
			if text := CleanSubtitle(string(raw)); text != "" {
				return text, srtVersion, nil
			}
		}
	} else {
		var missing *proc.MissingToolError
		if errors.As(err, &missing) {
			return "", "", classifyProcErr(h.FFmpegBin, err)
		}
		// 无内嵌字幕流时 ffmpeg 以非零退出：属预期分支，继续转写路径。
	}

	// 2) 无内嵌字幕。
	if effectiveMode != "whisper_local" {
		return "", "", worker.NewError(worker.ErrCodeTranscriptRequired,
			"音视频未包含可用字幕。请上传字幕文件（.srt/.vtt）、直接粘贴文本，或配置 TRANSCRIPTION_MODE=whisper_local 后重试。", false)
	}
	// 3) whisper_local：ffmpeg 提取 16kHz 单声道音轨（与 legacy 参数一致）。
	wavPath := filepath.Join(workDir, "audio.wav")
	args = []string{"-nostdin", "-i", path, "-vn", "-acodec", "pcm_s16le", "-ar", "16000", "-ac", "1", wavPath, "-y"}
	if _, err := h.Proc.Run(ctx, proc.Cmd{Name: h.FFmpegBin, Args: args, Dir: workDir}); err != nil {
		return "", "", classifyProcErr(h.FFmpegBin, err)
	}
	text, err := h.runWhisper(ctx, wavPath, workDir, language)
	if err != nil {
		return "", "", err
	}
	return text, mediaVersion, nil
}
