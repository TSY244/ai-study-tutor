package lessonimport

import "testing"

func TestDetectType(t *testing.T) {
	cases := []struct {
		name string
		head []byte
		ext  string
		want string
	}{
		{"pdf_magic", []byte("%PDF-1.7\n..."), ".pdf", TypePDF},
		{"pdf_magic_wins_over_ext", []byte("%PDF-1.7\n..."), ".txt", TypePDF},
		{"docx_by_zip_content", []byte("PK\x03\x04\x14\x00\x00\x00word/document.xml..."), ".bin", TypeDOCX},
		{"docx_by_ext", []byte("PK\x03\x04\x14\x00"), ".docx", TypeDOCX},
		{"zip_unknown", []byte("PK\x03\x04\x14\x00"), ".zip", TypeUnknown},
		{"vtt_header", []byte("WEBVTT\n\n00:00:01.000 --> 00:00:02.000\n你好"), ".txt", TypeVTT},
		{"srt_pattern", []byte("1\n00:00:01,000 --> 00:00:02,000\n你好"), ".txt", TypeSRT},
		{"html_doctype", []byte("<!DOCTYPE html><html><body>x</body></html>"), ".txt", TypeHTML},
		{"html_tag", []byte("<html><body>正文</body></html>"), "", TypeHTML},
		{"markdown_by_ext", []byte("# 标题\n正文"), ".md", TypeMD},
		{"plain_text", []byte("极限的定义是……"), ".txt", TypeTXT},
		{"plain_text_no_ext", []byte("你好，世界"), "", TypeTXT},
		{"mp4_magic_video_ext", []byte("\x00\x00\x00\x20ftypisom...."), ".mp4", TypeVideo},
		{"ftyp_audio_ext", []byte("\x00\x00\x00\x20ftypM4A ...."), ".m4a", TypeAudio},
		{"mp3_id3", []byte("ID3\x04\x00\x00\x00"), ".mp3", TypeAudio},
		{"wav_magic", []byte("RIFF\x24\x00\x00\x00WAVEfmt "), ".wav", TypeAudio},
		{"mkv_magic_video", []byte{0x1A, 0x45, 0xDF, 0xA3, 0x93}, ".mkv", TypeVideo},
		{"invalid_utf8", []byte{0x89, 0x87, 0x81, 0x80}, ".txt", TypeUnknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := detectType(c.head, c.ext); got != c.want {
				t.Fatalf("detectType = %q, 期望 %q", got, c.want)
			}
		})
	}
}
