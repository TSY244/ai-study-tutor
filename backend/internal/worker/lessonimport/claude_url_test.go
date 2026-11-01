package lessonimport

import (
	"errors"
	"strings"
	"testing"

	aipkg "github.com/TSY244/ai-study-tutor/backend/internal/ai"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker"
)

// TestCanonicalClaudeURL 覆盖 URL 再校验/规范化。纯函数，不解析 DNS、不发 HTTP。
func TestCanonicalClaudeURL(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		cases := []struct {
			name        string
			raw         string
			allowlist   []string
			wantURL     string
			wantDomains []string
		}{
			{
				name:      "youtube_strips_tracking_keeps_semantic",
				raw:       "https://www.youtube.com/watch?v=abc&utm_source=x&list=p1",
				allowlist: []string{"youtube.com", "bilibili.com"},
				wantURL:   "https://www.youtube.com/watch?list=p1&v=abc",
				// [hostname, 最长匹配 root]，去重。
				wantDomains: []string{"www.youtube.com", "youtube.com"},
			},
			{
				name:        "bilibili_keeps_p_strips_spm_trackid_vdsource_share",
				raw:         "https://www.bilibili.com/video/BV1xx?p=2&spm_id_from=333&trackid=x&vd_source=y&share_source=copy",
				allowlist:   []string{"bilibili.com"},
				wantURL:     "https://www.bilibili.com/video/BV1xx?p=2",
				wantDomains: []string{"www.bilibili.com", "bilibili.com"},
			},
			{
				name:        "generic_query_sorted",
				raw:         "https://youtube.com/watch?b=2&a=1",
				allowlist:   []string{"youtube.com"},
				wantURL:     "https://youtube.com/watch?a=1&b=2",
				wantDomains: []string{"youtube.com"},
			},
			{
				name:        "duplicate_and_blank_allowlist_entries",
				raw:         "https://www.youtube.com/watch?v=abc",
				allowlist:   []string{" youtube.com ", "", "youtube.com"},
				wantURL:     "https://www.youtube.com/watch?v=abc",
				wantDomains: []string{"www.youtube.com", "youtube.com"},
			},
			{
				name:        "longest_matching_root_dedupes_hostname",
				raw:         "https://www.youtube.com/watch?v=abc",
				allowlist:   []string{"youtube.com", "www.youtube.com"},
				wantURL:     "https://www.youtube.com/watch?v=abc",
				wantDomains: []string{"www.youtube.com"},
			},
			{
				name:        "subdomain_selects_longest_root",
				raw:         "https://sub.a.example.com/x",
				allowlist:   []string{"example.com", "a.example.com"},
				wantURL:     "https://sub.a.example.com/x",
				wantDomains: []string{"sub.a.example.com", "a.example.com"},
			},
			{
				name:        "uppercase_host_and_scheme_normalized",
				raw:         "HTTPS://WWW.YouTube.COM/Watch?v=abc",
				allowlist:   []string{"youtube.com"},
				wantURL:     "https://www.youtube.com/Watch?v=abc",
				wantDomains: []string{"www.youtube.com", "youtube.com"},
			},
			{
				name:        "fragment_dropped",
				raw:         "https://www.youtube.com/watch?v=abc#t=30",
				allowlist:   []string{"youtube.com"},
				wantURL:     "https://www.youtube.com/watch?v=abc",
				wantDomains: []string{"www.youtube.com", "youtube.com"},
			},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				gotURL, gotDomains, err := canonicalClaudeURL(c.raw, c.allowlist)
				if err != nil {
					t.Fatalf("期望通过，得到 %v", err)
				}
				if gotURL != c.wantURL {
					t.Fatalf("canonical URL 期望 %q，得到 %q", c.wantURL, gotURL)
				}
				if len(gotDomains) != len(c.wantDomains) {
					t.Fatalf("allowedDomains 期望 %v，得到 %v", c.wantDomains, gotDomains)
				}
				for i := range c.wantDomains {
					if gotDomains[i] != c.wantDomains[i] {
						t.Fatalf("allowedDomains 期望 %v，得到 %v", c.wantDomains, gotDomains)
					}
				}
			})
		}
	})

	t.Run("rejected", func(t *testing.T) {
		cases := []struct {
			name      string
			raw       string
			allowlist []string
		}{
			{"http_rejected", "http://www.youtube.com/watch?v=1", []string{"youtube.com"}},
			{"userinfo_rejected", "https://user@www.youtube.com/watch?v=1", []string{"youtube.com"}},
			{"port_rejected", "https://www.youtube.com:8443/watch?v=1", []string{"youtube.com"}},
			{"not_allowlisted", "https://evil.example.com/x", []string{"youtube.com"}},
			{"empty_allowlist", "https://www.youtube.com/watch?v=1", nil},
			{"malformed_url", "https://", []string{"youtube.com"}},
			{"missing_host", "https:///watch?v=1", []string{"youtube.com"}},
			{
				"over_250_chars_after_reencode",
				"https://www.youtube.com/watch?v=" + strings.Repeat("a", 300),
				[]string{"youtube.com"},
			},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				_, _, err := canonicalClaudeURL(c.raw, c.allowlist)
				if err == nil {
					t.Fatalf("期望拒绝 %q", c.raw)
				}
				var werr *worker.Error
				if !errors.As(err, &werr) || werr.Code != worker.ErrCodeURLBlocked {
					t.Fatalf("期望错误码 %s，得到 %v", worker.ErrCodeURLBlocked, err)
				}
				if werr.Retryable {
					t.Fatalf("URL 校验失败不应可重试: %v", werr)
				}
			})
		}
	})
}

// TestParseClaudeURLResult 覆盖 Claude URL 提取结果的 JSON/schema 校验与脱敏。
func TestParseClaudeURLResult(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		res, err := parseClaudeURLResult([]byte(
			`{"title":"课程视频","raw_text_markdown":"# 正文\n内容","extraction_note":"含字幕原文"}`))
		if err != nil {
			t.Fatalf("期望通过，得到 %v", err)
		}
		if res.Title != "课程视频" || res.RawTextMarkdown != "# 正文\n内容" || res.ExtractionNote != "含字幕原文" {
			t.Fatalf("解析结果不符: %+v", res)
		}
	})

	t.Run("malformed_json_is_claude_error", func(t *testing.T) {
		sentinel := "SENTINEL-RAW-PAYLOAD"
		_, err := parseClaudeURLResult([]byte(`{"title": "` + sentinel))
		perr := aipkg.AsProviderError(err)
		if perr.Code != aipkg.ErrInvalidJSON {
			t.Fatalf("期望错误码 %s，得到 %v", aipkg.ErrInvalidJSON, err)
		}
		if !perr.Retryable {
			t.Fatalf("claude_invalid_json 应可重试: %v", perr)
		}
		if strings.Contains(perr.Detail, sentinel) {
			t.Fatalf("错误详情泄露原始内容: %q", perr.Detail)
		}
	})

	t.Run("missing_raw_text_is_schema_invalid", func(t *testing.T) {
		sentinel := "SENTINEL-TITLE"
		_, err := parseClaudeURLResult([]byte(
			`{"title":"` + sentinel + `","raw_text_markdown":"  ","extraction_note":""}`))
		perr := aipkg.AsProviderError(err)
		if perr.Code != aipkg.ErrSchemaInvalid {
			t.Fatalf("期望错误码 %s，得到 %v", aipkg.ErrSchemaInvalid, err)
		}
		if strings.Contains(perr.Detail, sentinel) {
			t.Fatalf("错误详情泄露原始内容: %q", perr.Detail)
		}
	})

	t.Run("missing_title_is_schema_invalid", func(t *testing.T) {
		_, err := parseClaudeURLResult([]byte(
			`{"raw_text_markdown":"正文","extraction_note":""}`))
		perr := aipkg.AsProviderError(err)
		if perr.Code != aipkg.ErrSchemaInvalid {
			t.Fatalf("期望错误码 %s，得到 %v", aipkg.ErrSchemaInvalid, err)
		}
	})
}

// TestClaudeURLSourceInfo 覆盖 Claude 路径来源摘要的固定标识与非敏感字段。
func TestClaudeURLSourceInfo(t *testing.T) {
	res := claudeURLResult{
		Title:           " 课程视频 ",
		RawTextMarkdown: "正文",
		ExtractionNote:  " 含字幕原文 ",
	}
	info := claudeURLSourceInfo(res, "https://www.youtube.com/watch?v=abc")
	if info.Extractor != "pi_sdk_fetch" {
		t.Fatalf("extractor 期望 pi_sdk_fetch，得到 %q", info.Extractor)
	}
	if info.Title != "课程视频" || info.ExtractionNote != "含字幕原文" {
		t.Fatalf("来源摘要字段未清洗: %+v", info)
	}
	if info.WebpageURL != "https://www.youtube.com/watch?v=abc" {
		t.Fatalf("webpage_url 期望 canonical URL，得到 %q", info.WebpageURL)
	}
	if info.SubtitleOrigin != "" || info.YtDlpVersion != "" {
		t.Fatalf("Claude 路径不应携带本地下载元数据: %+v", info)
	}
	if claudeURLParserVersion != "pi-sdk-fetch/v1" {
		t.Fatalf("parser version 期望 pi-sdk-fetch/v1，得到 %q", claudeURLParserVersion)
	}
}

// TestClaudeURLWorkerErrorMapping 覆盖 Claude URL 执行分支的错误映射：
// material_fetch_failed 保留 retryability，空/超长正文映射为稳定的不可重试码。
func TestClaudeURLWorkerErrorMapping(t *testing.T) {
	t.Run("material_fetch_failed preserves retryability", func(t *testing.T) {
		for _, retryable := range []bool{true, false} {
			err := claudeWorkerError(aipkg.MaterialFetchError(retryable))
			var werr *worker.Error
			if !errors.As(err, &werr) {
				t.Fatalf("应映射为 *worker.Error: %T", err)
			}
			if werr.Code != worker.ErrCodeMaterialFetchFailed {
				t.Fatalf("错误码期望 %s，得到 %s", worker.ErrCodeMaterialFetchFailed, werr.Code)
			}
			if werr.Retryable != retryable {
				t.Fatalf("retryability 期望 %v，得到 %v", retryable, werr.Retryable)
			}
		}
	})

	t.Run("empty and oversize text map to stable codes", func(t *testing.T) {
		err := validateClaudeURLText("", 100)
		var werr *worker.Error
		if !errors.As(err, &werr) || werr.Code != worker.ErrCodeEmptyContent || werr.Retryable {
			t.Fatalf("空正文应映射为不可重试 %s: %v", worker.ErrCodeEmptyContent, err)
		}
		err = validateClaudeURLText(strings.Repeat("x", 101), 100)
		if !errors.As(err, &werr) || werr.Code != worker.ErrCodeTooLarge || werr.Retryable {
			t.Fatalf("超长正文应映射为不可重试 %s: %v", worker.ErrCodeTooLarge, err)
		}
		if err := validateClaudeURLText(strings.Repeat("x", 100), 100); err != nil {
			t.Fatalf("上限内的正文应通过: %v", err)
		}
	})
}
