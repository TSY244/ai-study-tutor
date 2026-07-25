package lessonimport

import (
	"encoding/json"
	"net/url"
	"strings"

	aipkg "github.com/TSY244/ai-study-tutor/backend/internal/ai"
	"github.com/TSY244/ai-study-tutor/backend/internal/material"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker"
)

// Claude Web Fetch URL 提取路径的固定标识与非敏感边界（03 §8.2）：
//
//   - canonicalClaudeURL 是纯函数：重新解析持久化 URL、复用白名单判定、
//     剥离追踪参数并重编码；不解析 DNS、不发 HTTP、不调用任何本地工具；
//   - URL 提取不经 Resolver/Transport/proc.Runner/yt-dlp/FFmpeg/Whisper，
//     抓取发生在 Claude 服务端（web_fetch server tool）；
//   - 结果解析失败一律归为 Claude 错误（claude_invalid_json /
//     claude_schema_invalid），错误详情不回带上游原文。

const (
	// claudeURLExtractor 与 claudeURLParserVersion 写入素材 metadata，
	// 标记原文由 Claude Web Fetch 提取而非本地下载管线。
	claudeURLExtractor       = "claude_web_fetch"
	claudeURLParserVersion   = "claude-web-fetch/v1"
	maxCanonicalClaudeURLLen = 250
)

// canonicalClaudeURL 重新解析持久化的素材 URL 并返回规范化形式与本任务
// 允许的域名集 [hostname, 最长匹配 root]（去重）。拒绝非 HTTPS、userinfo、
// 显式端口、非白名单主机，以及重编码后超过长度上限的 URL。
func canonicalClaudeURL(rawURL string, allowlist []string) (string, []string, error) {
	blocked := func(msg string) (string, []string, error) {
		return "", nil, worker.NewError(worker.ErrCodeURLBlocked, msg, false)
	}
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Host == "" || !strings.EqualFold(u.Scheme, "https") {
		return blocked("素材 URL 不是合法的 https 地址。")
	}
	if u.User != nil || u.Port() != "" {
		return blocked("素材 URL 不允许携带用户信息或端口。")
	}
	host := strings.ToLower(u.Hostname())
	if !material.HostAllowed(host, allowlist) {
		return blocked("素材 URL 主机不在白名单内。")
	}
	root := longestAllowlistRoot(host, allowlist)

	u.Scheme = "https"
	u.Host = host
	u.Fragment = ""
	u.RawFragment = ""
	u.RawQuery = stripTrackingQuery(u.Query()).Encode()
	canonical := u.String()
	if len(canonical) > maxCanonicalClaudeURLLen {
		return blocked("素材 URL 过长，已拒绝。")
	}
	return canonical, allowedDomainPair(host, root), nil
}

// longestAllowlistRoot 返回白名单中最长（最具体）的匹配 root；调用前需
// 已通过 material.HostAllowed 判定，保证必然存在匹配。
func longestAllowlistRoot(host string, allowlist []string) string {
	best := ""
	seen := map[string]bool{}
	for _, entry := range allowlist {
		entry = strings.ToLower(strings.TrimSpace(entry))
		if entry == "" || seen[entry] {
			continue
		}
		seen[entry] = true
		if (host == entry || strings.HasSuffix(host, "."+entry)) && len(entry) > len(best) {
			best = entry
		}
	}
	return best
}

// allowedDomainPair 生成 [hostname, root] 去重序列，作为 Web Fetch
// allowed_domains 与本任务的域名授权边界。
func allowedDomainPair(host, root string) []string {
	if host == root || root == "" {
		return []string{host}
	}
	return []string{host, root}
}

// stripTrackingQuery 移除来源追踪参数（YouTube utm_* 等、Bilibili
// spm*/trackid/vd_source/share_*），保留语义参数（YouTube v/list、
// Bilibili p 及其余未列名参数）。
func stripTrackingQuery(q url.Values) url.Values {
	out := url.Values{}
	for key, vals := range q {
		if isTrackingParam(strings.ToLower(key)) {
			continue
		}
		out[key] = vals
	}
	return out
}

func isTrackingParam(key string) bool {
	if strings.HasPrefix(key, "utm_") || strings.HasPrefix(key, "spm") ||
		strings.HasPrefix(key, "share_") {
		return true
	}
	switch key {
	case "trackid", "vd_source", "fbclid", "gclid", "dclid", "mc_cid", "mc_eid":
		return true
	}
	return false
}

// claudeURLResult 是 lesson_url_extract 结构化结果的私有解码类型。
type claudeURLResult struct {
	Title           string `json:"title"`
	RawTextMarkdown string `json:"raw_text_markdown"`
	ExtractionNote  string `json:"extraction_note"`
}

// parseClaudeURLResult 校验 Claude 返回的 URL 提取结果。畸形 JSON 与
// schema 缺字段一律视为 Claude 错误（可重试），错误详情不回带原文；
// 正文的规范化与上限校验由 handler 在发布前统一执行。
func parseClaudeURLResult(raw json.RawMessage) (claudeURLResult, error) {
	var res claudeURLResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return claudeURLResult{}, aipkg.WrapError(aipkg.ErrInvalidJSON,
			"Claude 返回的 URL 提取结果无法解析，将自动重试。", err)
	}
	if strings.TrimSpace(res.RawTextMarkdown) == "" {
		return claudeURLResult{}, aipkg.ErrorForCode(aipkg.ErrSchemaInvalid,
			"Claude 返回的 URL 提取结果缺少正文，将自动重试。")
	}
	if strings.TrimSpace(res.Title) == "" {
		return claudeURLResult{}, aipkg.ErrorForCode(aipkg.ErrSchemaInvalid,
			"Claude 返回的 URL 提取结果缺少标题，将自动重试。")
	}
	return res, nil
}

// claudeURLSourceInfo 生成 Claude 提取路径的来源摘要：仅标题、canonical
// URL 与非敏感的提取说明，不含正文，也不携带本地下载工具元数据。
func claudeURLSourceInfo(res claudeURLResult, canonicalURL string) *sourceInfo {
	return &sourceInfo{
		Title:          strings.TrimSpace(res.Title),
		WebpageURL:     canonicalURL,
		Extractor:      claudeURLExtractor,
		ExtractionNote: strings.TrimSpace(res.ExtractionNote),
	}
}
