package lessonimport

import "encoding/json"

// parseInfoJSON 提取 yt-dlp info.json 中的安全展示字段。
// 只取标题/时长/来源 URL/提取器名，不保存完整 JSON（含可能的隐私字段）。
func parseInfoJSON(raw []byte, info *sourceInfo) {
	var v struct {
		Title      string `json:"title"`
		Duration   int    `json:"duration"`
		WebpageURL string `json:"webpage_url"`
		Extractor  string `json:"extractor"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return
	}
	if v.Title != "" {
		info.Title = v.Title
	}
	if v.Duration > 0 {
		info.Duration = v.Duration
	}
	if v.WebpageURL != "" {
		info.WebpageURL = v.WebpageURL
	}
	if v.Extractor != "" {
		info.Extractor = v.Extractor
	}
}
