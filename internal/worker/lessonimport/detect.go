package lessonimport

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// 检测类型（detecting 阶段输出；以文件 Magic 为准，不信任扩展名，03 §8.3）。
const (
	TypePDF     = "pdf"
	TypeDOCX    = "docx"
	TypeTXT     = "txt"
	TypeMD      = "md"
	TypeSRT     = "srt"
	TypeVTT     = "vtt"
	TypeHTML    = "html"
	TypeAudio   = "audio"
	TypeVideo   = "video"
	TypeUnknown = "unknown"
)

// 音视频容器 Magic -> 类型。audio/video 的区分结合扩展名提示
// （Magic 只能给出容器，如 ftyp 同时覆盖 mp4/m4a）。
var audioExts = map[string]bool{".mp3": true, ".wav": true, ".flac": true, ".m4a": true, ".ogg": true}
var videoExts = map[string]bool{".mp4": true, ".mkv": true, ".avi": true, ".mov": true}

// DetectType 以内容 Magic 为主、扩展名为辅判定素材类型。
// originalExt 为小写扩展名（含点），可为空。head 为文件前若干字节。
func DetectType(head []byte, originalExt string) Type {
	return detectType(head, originalExt)
}

// detectFile 读取文件头并判定类型。
func detectFile(path, originalName string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	head := make([]byte, 8192)
	n, _ := f.Read(head)
	head = head[:n]
	ext := strings.ToLower(filepath.Ext(originalName))
	return string(detectType(head, ext)), nil
}

type Type = string

func detectType(head []byte, ext string) Type {
	// 容器/二进制格式。
	if bytes.HasPrefix(head, []byte("%PDF-")) {
		return TypePDF
	}
	if bytes.HasPrefix(head, []byte("PK\x03\x04")) {
		if isDocxBytes(head) || ext == ".docx" || ext == ".doc" {
			return TypeDOCX
		}
		return TypeUnknown
	}
	if isMediaMagic(head) {
		switch {
		case audioExts[ext]:
			return TypeAudio
		case videoExts[ext]:
			return TypeVideo
		default:
			// Magic 能区分的一部分容器。
			if isAudioMagic(head) {
				return TypeAudio
			}
			return TypeVideo
		}
	}
	// 文本类：必须合法 UTF-8。
	if !utf8.Valid(head) {
		return TypeUnknown
	}
	trimmed := strings.TrimSpace(string(head))
	upper := strings.ToUpper(trimmed)
	switch {
	case strings.HasPrefix(upper, "WEBVTT"):
		return TypeVTT
	case looksLikeSRT(trimmed):
		return TypeSRT
	case looksLikeHTML(trimmed):
		return TypeHTML
	case ext == ".vtt":
		return TypeVTT
	case ext == ".srt":
		return TypeSRT
	case ext == ".md" || ext == ".markdown":
		return TypeMD
	default:
		return TypeTXT
	}
}

// isDocxBytes 通过 zip 中央目录特征粗判 DOCX（含 word/document.xml 记录）。
// head 不足以完整解析 zip 时回退到扩展名（调用方已处理）。
func isDocxBytes(head []byte) bool {
	return bytes.Contains(head, []byte("word/document.xml"))
}

func isMediaMagic(head []byte) bool {
	if len(head) >= 12 && string(head[4:8]) == "ftyp" {
		return true // mp4/m4a/mov 等 ISO BMFF
	}
	if bytes.HasPrefix(head, []byte{0x1A, 0x45, 0xDF, 0xA3}) {
		return true // EBML（mkv/webm）
	}
	return isAudioMagic(head)
}

func isAudioMagic(head []byte) bool {
	switch {
	case bytes.HasPrefix(head, []byte("ID3")):
		return true // mp3
	case len(head) >= 2 && head[0] == 0xFF && head[1]&0xE0 == 0xE0:
		return true // mp3 frame sync
	case bytes.HasPrefix(head, []byte("RIFF")) && len(head) >= 12 && string(head[8:12]) == "WAVE":
		return true
	case bytes.HasPrefix(head, []byte("fLaC")):
		return true
	case bytes.HasPrefix(head, []byte("OggS")):
		return true
	}
	return false
}

// looksLikeSRT：首行纯序号 + 随后出现时间轴行。
func looksLikeSRT(text string) bool {
	lines := strings.Split(text, "\n")
	if len(lines) < 2 {
		return false
	}
	first := strings.TrimSpace(lines[0])
	if !seqLine.MatchString(first) {
		return false
	}
	for _, l := range lines[1:] {
		if strings.TrimSpace(l) == "" {
			continue
		}
		return strings.Contains(l, "-->")
	}
	return false
}

// looksLikeHTML：头部包含 HTML 文档特征。
func looksLikeHTML(text string) bool {
	probe := text
	if len(probe) > 2048 {
		probe = probe[:2048]
	}
	lower := strings.ToLower(probe)
	return strings.Contains(lower, "<!doctype html") ||
		strings.Contains(lower, "<html") ||
		(strings.Contains(lower, "<head") && strings.Contains(lower, "<body"))
}
