package lessonimport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/TSY244/ai-study-tutor/backend/internal/material"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker/proc"
)

// URL 获取（acquiring 阶段）安全约束（03 §8.2）：
//
//   - 仅 https；host 必须在白名单内；DNS 解析结果必须全部公网
//     （Worker 侧二次校验，API 创建时的校验不能覆盖 DNS 重绑定）；
//   - 预检请求跟随重定向时逐跳重复上述校验，最多 maxRedirects 跳。
//
// 注意：URL 素材导入已改走 Claude Web Fetch（见 claude_url.go 与
// Handler.executeURL），不再从这里进入。本文件保留的本地辅助仅供既有
// 单元测试覆盖（checkURLHost/validateRedirectChain/pickSubtitleFile/
// classifyProcErr），生产路径不再调用。

const maxRedirects = 5

// errRedirectBlocked 是重定向链校验失败的哨兵（经 url.Error 包装后 errors.Is 可判）。
var errRedirectBlocked = errors.New("重定向目标未通过 SSRF 校验")

// sourceInfo 是写入素材 metadata 的来源摘要（不含正文）。
type sourceInfo struct {
	Title          string `json:"title,omitempty"`
	Duration       int    `json:"duration,omitempty"`
	WebpageURL     string `json:"webpage_url,omitempty"`
	Extractor      string `json:"extractor,omitempty"`
	SubtitleOrigin string `json:"subtitle_origin,omitempty"` // manual/auto/whisper_local
	SubtitleLang   string `json:"subtitle_lang,omitempty"`
	YtDlpVersion   string `json:"ytdlp_version,omitempty"`
	// ExtractionNote 是 Claude Web Fetch 路径的非敏感提取说明（如内容
	// 受限原因）；本地下载路径不写入该字段。
	ExtractionNote string `json:"extraction_note,omitempty"`
}

// checkURLHost 校验单个 URL：https + 白名单 + DNS 全公网。
func (h *Handler) checkURLHost(ctx context.Context, raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || !strings.EqualFold(u.Scheme, "https") {
		return worker.NewError(worker.ErrCodeURLBlocked, "素材 URL 不是合法的 https 地址。", false)
	}
	host := strings.ToLower(u.Hostname())
	if !material.HostAllowed(host, h.URLAllowlist) {
		return worker.NewError(worker.ErrCodeURLBlocked, "素材 URL 主机不在白名单内。", false)
	}
	resolve := h.Resolver
	if resolve == nil {
		resolve = func(ctx context.Context, host string) ([]net.IP, error) {
			return net.DefaultResolver.LookupIP(ctx, "ip", host)
		}
	}
	ips, err := resolve(ctx, host)
	if err != nil || len(ips) == 0 {
		return worker.NewError(worker.ErrCodeMaterialFetchFailed, "素材 URL 主机名暂时无法解析，将自动重试。", true)
	}
	for _, ip := range ips {
		if !material.IsPublicIP(ip) {
			return worker.NewError(worker.ErrCodeURLBlocked, "素材 URL 指向非公网地址，已拒绝。", false)
		}
	}
	return nil
}

// validateRedirectChain 预检重定向链：逐跳重复 SSRF 校验，只读取响应头
// 与极小前缀，不下载正文。HTTP 错误状态不阻断（yt-dlp 自带请求头与重试）。
func (h *Handler) validateRedirectChain(ctx context.Context, raw string) error {
	client := &http.Client{
		Transport: h.Transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("%w: 重定向超过 %d 跳", errRedirectBlocked, maxRedirects)
			}
			if err := h.checkURLHost(req.Context(), req.URL.String()); err != nil {
				return fmt.Errorf("%w", errRedirectBlocked)
			}
			return nil
		},
	}
	if client.Transport == nil {
		client.Transport = http.DefaultTransport
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return worker.NewError(worker.ErrCodeURLBlocked, "素材 URL 不是合法的 https 地址。", false)
	}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errors.Is(err, errRedirectBlocked) {
			return worker.NewError(worker.ErrCodeURLBlocked,
				"素材 URL 的重定向链包含非公网或非白名单主机，已拒绝。", false)
		}
		return worker.WrapError(worker.ErrCodeMaterialFetchFailed, "素材 URL 预检请求失败，将自动重试。", true, err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	return nil
}

// subtitleLangs 计算字幕语言优先级：preferred 置前，其余与 legacy 一致。
func subtitleLangs(preferred string) string {
	base := []string{"zh-Hans", "zh", "en", "zh-CN", "zh-TW", "zh-Hant", "ai-zh"}
	if preferred = strings.TrimSpace(preferred); preferred != "" {
		out := []string{preferred}
		for _, l := range base {
			if !strings.EqualFold(l, preferred) {
				out = append(out, l)
			}
		}
		return strings.Join(out, ",")
	}
	return strings.Join(base, ",")
}

// pickSubtitleFile 在工作目录扫描字幕文件，清洗后写入 acquired.txt。
// 偏好：语言列表靠前 > vtt > srt > lrc > txt；文件名排序保证确定性。
func pickSubtitleFile(workDir, langs string) (string, string, error) {
	entries, err := os.ReadDir(workDir)
	if err != nil {
		return "", "", worker.WrapError(worker.ErrCodeInternal, "读取任务目录失败，将自动重试。", true, err)
	}
	type candidate struct {
		name string
		ext  string
		lang string
		rank int
	}
	extRank := map[string]int{".vtt": 0, ".srt": 1, ".lrc": 2, ".txt": 3}
	langRank := map[string]int{}
	for i, l := range strings.Split(langs, ",") {
		langRank[strings.ToLower(l)] = i
	}
	var cands []candidate
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		rank, ok := extRank[ext]
		if !ok || !strings.HasPrefix(e.Name(), "src.") {
			continue
		}
		// src.<lang>.<ext> 命名；提取语言段。
		mid := strings.TrimSuffix(e.Name(), ext)
		lang := strings.TrimPrefix(mid, "src.")
		lr := len(langRank)
		if r, ok := langRank[strings.ToLower(lang)]; ok {
			lr = r
		}
		cands = append(cands, candidate{e.Name(), ext, lang, lr*10 + rank})
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].rank != cands[j].rank {
			return cands[i].rank < cands[j].rank
		}
		return cands[i].name < cands[j].name
	})
	for _, c := range cands {
		raw, err := os.ReadFile(filepath.Join(workDir, c.name))
		if err != nil {
			continue
		}
		var cleaned string
		switch c.ext {
		case ".vtt":
			cleaned = CleanVTT(string(raw))
		case ".srt":
			cleaned = CleanSubtitle(string(raw))
		case ".lrc":
			cleaned = CleanLRC(string(raw))
		default:
			cleaned = Normalize(string(raw))
		}
		if cleaned == "" {
			continue
		}
		out := filepath.Join(workDir, "acquired.txt")
		if err := os.WriteFile(out, []byte(cleaned), 0o644); err != nil {
			return "", "", worker.WrapError(worker.ErrCodeInternal, "写入临时文件失败，将自动重试。", true, err)
		}
		return out, c.lang, nil
	}
	return "", "", nil
}

// runWhisper 调用本地 Whisper CLI 转写 wav 文件，返回清洗文本。
// 保留给上传音视频的 whisper_local 转写（extract.go），URL 路径不再使用。
func (h *Handler) runWhisper(ctx context.Context, wavPath, workDir, language string) (string, error) {
	if strings.TrimSpace(h.WhisperModelPath) == "" {
		return "", worker.NewError(worker.ErrCodeToolMissing,
			"TRANSCRIPTION_MODE=whisper_local 已启用但 WHISPER_MODEL_PATH 未配置。", false)
	}
	language = strings.ToLower(strings.TrimSpace(language))
	if language == "" {
		language = "auto"
	}
	// Whisper accepts base language codes, e.g. zh rather than zh-CN.
	language = strings.SplitN(language, "-", 2)[0]
	outPrefix := filepath.Join(workDir, "transcript")
	args := []string{
		"-m", h.WhisperModelPath,
		"-f", wavPath,
		"-l", language,
		"-t", "4",
		"-otxt",
		"-of", outPrefix,
	}
	if _, err := h.Proc.Run(ctx, proc.Cmd{Name: h.WhisperBin, Args: args, Dir: workDir}); err != nil {
		return "", classifyProcErr(h.WhisperBin, err)
	}
	raw, err := os.ReadFile(outPrefix + ".txt")
	if err != nil {
		return "", worker.NewError(worker.ErrCodeMaterialFetchFailed, "语音转写未产出文本。", false)
	}
	text := Normalize(string(raw))
	if text == "" {
		return "", worker.NewError(worker.ErrCodeEmptyContent, "语音转写结果为空。", false)
	}
	return text, nil
}

// classifyProcErr 把子进程错误映射为稳定错误码（03 §7.3 重试分类）。
func classifyProcErr(tool string, err error) *worker.Error {
	var missing *proc.MissingToolError
	if errors.As(err, &missing) {
		return worker.NewError(worker.ErrCodeToolMissing,
			fmt.Sprintf("外部工具 %s 不可用，请在 Worker 环境安装后重试。", tool), false)
	}
	var exitErr *proc.ExitError
	if errors.As(err, &exitErr) {
		tail := strings.ToLower(exitErr.StderrTail)
		switch {
		case containsAny(tail, "429", "too many requests", "timed out", "timeout",
			"temporary failure", "connection reset", "connection refused",
			"http error 5", "503", "502"):
			return worker.NewError(worker.ErrCodeMaterialFetchFailed,
				"下载暂时失败（限流或网络错误），将自动重试。", true)
		case containsAny(tail, "412", "precondition"):
			return worker.NewError(worker.ErrCodeMaterialFetchFailed,
				"站点拒绝访问（反爬拦截）。请改用手动下载字幕后上传，或直接粘贴文本。", false)
		case containsAny(tail, "unsupported url"):
			return worker.NewError(worker.ErrCodeMaterialFetchFailed,
				"该 URL 不受下载工具支持。请改用文件上传或粘贴文本。", false)
		case containsAny(tail, "sign in", "login", "private video", "video unavailable"):
			return worker.NewError(worker.ErrCodeMaterialFetchFailed,
				"视频需要登录或不可用。请改用文件上传或粘贴文本。", false)
		default:
			return worker.NewError(worker.ErrCodeMaterialFetchFailed,
				fmt.Sprintf("外部工具 %s 执行失败（退出码 %d）。", tool, exitErr.Code), false)
		}
	}
	return worker.WrapError(worker.ErrCodeInternal, "外部工具调用失败，将自动重试。", true, err)
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// loadInfoJSON 读取 yt-dlp info.json 中的安全字段（标题/时长/来源）。
// 保留给既有单元测试；生产 URL 路径已改走 Claude Web Fetch，不再调用。
func (h *Handler) loadInfoJSON(workDir string, info *sourceInfo) {
	entries, err := os.ReadDir(workDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".info.json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(workDir, e.Name()))
		if err != nil {
			return
		}
		parseInfoJSON(raw, info)
		return
	}
}
