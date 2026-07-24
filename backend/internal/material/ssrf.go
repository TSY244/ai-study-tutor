package material

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/httpserver"
)

// SSRF 校验（API 侧，不下载 URL 内容）：
//
//  1. 仅允许 https scheme；
//  2. host 必须在注入的 allowlist 内（精确匹配或后缀匹配，如
//     "youtube.com" 覆盖 "www.youtube.com"；allowlist 为空则全部拒绝）；
//  3. 解析 host，所有 IP 必须是公网地址（拒绝 loopback / RFC1918 私网 /
//     link-local / unspecified / multicast）。
//
// 校验失败返回 422 validation_error（field=url，
// code=url_not_allowed / url_private / url_unresolvable）。

// validateSourceURL 校验并规范化素材 URL：scheme/host 小写化、去掉 fragment。
// 返回的字符串直接落库为 materials.source_url。
func (s *Service) validateSourceURL(ctx context.Context, raw string) (string, *httpx.ProblemError) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.Scheme == "" {
		return "", urlFieldErr("invalid_url", "url 不是合法的绝对 URL。")
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return "", urlFieldErr("url_not_allowed", "仅允许 https 协议的 URL。")
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return "", urlFieldErr("invalid_url", "url 缺少主机名。")
	}
	if !hostAllowed(host, s.urlAllowlist) {
		return "", urlFieldErr("url_not_allowed", "该站点不在允许的 URL 白名单内。")
	}
	resolve := s.HostResolver
	if resolve == nil {
		resolve = defaultHostResolver
	}
	ips, err := resolve(ctx, host)
	if err != nil || len(ips) == 0 {
		return "", urlFieldErr("url_unresolvable", "URL 主机名无法解析。")
	}
	for _, ip := range ips {
		if !isPublicIP(ip) {
			return "", urlFieldErr("url_private", "URL 指向非公网地址，已拒绝。")
		}
	}
	// 规范化：scheme/host 小写化，去掉 fragment，其余部分保持不变。
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""
	u.RawFragment = ""
	return u.String(), nil
}

// hostAllowed 判定 host 是否在白名单内：精确匹配或 "."+entry 后缀匹配。
func hostAllowed(host string, allowlist []string) bool {
	return HostAllowed(host, allowlist)
}

// HostAllowed 判定 host 是否在白名单内：精确匹配或 "."+entry 后缀匹配
// （Worker 二次 SSRF 校验复用同一实现）。
func HostAllowed(host string, allowlist []string) bool {
	for _, entry := range allowlist {
		entry = strings.ToLower(strings.TrimSpace(entry))
		if entry == "" {
			continue
		}
		if host == entry || strings.HasSuffix(host, "."+entry) {
			return true
		}
	}
	return false
}

// isPublicIP 判定 IP 是否为可路由的公网地址。
func isPublicIP(ip net.IP) bool {
	return IsPublicIP(ip)
}

// IsPublicIP 判定 IP 是否为可路由的公网地址（Worker 二次 SSRF 校验复用）。
func IsPublicIP(ip net.IP) bool {
	if ip == nil || !ip.IsGlobalUnicast() {
		return false
	}
	return !ip.IsPrivate() &&
		!ip.IsLoopback() &&
		!ip.IsLinkLocalUnicast() &&
		!ip.IsLinkLocalMulticast() &&
		!ip.IsMulticast() &&
		!ip.IsUnspecified()
}

// defaultHostResolver 使用系统 DNS 解析 host（支持 context 取消）。
func defaultHostResolver(ctx context.Context, host string) ([]net.IP, error) {
	return net.DefaultResolver.LookupIP(ctx, "ip", host)
}

// urlFieldErr 构造 field=url 的 422 validation_error Problem。
func urlFieldErr(code, message string) *httpx.ProblemError {
	p := httpserver.NewProblem(http.StatusUnprocessableEntity, "validation_error",
		"请求参数无效", "请修正标记的字段后重试。")
	p.Errors = []httpserver.FieldError{httpx.FieldErr("url", code, message)}
	return &httpx.ProblemError{Problem: p}
}
