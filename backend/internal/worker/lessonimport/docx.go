package lessonimport

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// DocxVersion 是 DOCX 提取器版本。
const DocxVersion = "docx/v1"

// ExtractDocx 用标准库解压 DOCX 并拼接 word/document.xml 段落文本
// （w:p 为段落、w:t 为文本、w:tab 展开为空格、w:br 换行）。
// 行为对应 legacy read_docx（逐段拼接非空段落）。
func ExtractDocx(path string) (string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("docx 打开失败: %w", err)
	}
	defer func() { _ = r.Close() }()
	var docFile *zip.File
	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			docFile = f
			break
		}
	}
	if docFile == nil {
		return "", fmt.Errorf("docx 缺少 word/document.xml")
	}
	rc, err := docFile.Open()
	if err != nil {
		return "", fmt.Errorf("docx 读取失败: %w", err)
	}
	defer func() { _ = rc.Close() }()
	// 限制 XML 输入体积，防御恶意压缩包。
	data, err := io.ReadAll(io.LimitReader(rc, 64<<20))
	if err != nil {
		return "", fmt.Errorf("docx 读取失败: %w", err)
	}
	paragraphs, err := parseDocxParagraphs(data)
	if err != nil {
		return "", err
	}
	var out []string
	for _, p := range paragraphs {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return Normalize(strings.Join(out, "\n")), nil
}

// parseDocxParagraphs 流式解析 document.xml，返回段落文本序列。
func parseDocxParagraphs(data []byte) ([]string, error) {
	dec := xml.NewDecoder(strings.NewReader(string(data)))
	var paragraphs []string
	var cur strings.Builder
	inP := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("docx XML 解析失败: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "p":
				inP = true
				cur.Reset()
			case "tab":
				if inP {
					cur.WriteByte(' ')
				}
			case "br", "cr":
				if inP {
					cur.WriteByte('\n')
				}
			}
		case xml.EndElement:
			if t.Name.Local == "p" && inP {
				paragraphs = append(paragraphs, cur.String())
				inP = false
			}
		case xml.CharData:
			if inP {
				cur.Write(t)
			}
		}
	}
	return paragraphs, nil
}
