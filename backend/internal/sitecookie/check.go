package sitecookie

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DefaultNavURL 是 B 站登录状态查询端点（无需 WBI 签名）。
const DefaultNavURL = "https://api.bilibili.com/x/web-interface/nav"

// navUserAgent 模拟常规浏览器，避免反爬 412。
const navUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36"

// ErrCookieInvalid 表示 B 站明确返回未登录（Cookie 失效/过期，不可重试）。
var ErrCookieInvalid = errors.New("bilibili cookie rejected: not logged in")

// CheckError 是暂时性检测失败（网络、限流、远端异常，可重试）。
// 消息只含静态文案与 HTTP 状态码，绝不含 Cookie 或响应正文。
type CheckError struct {
	msg string
}

func (e *CheckError) Error() string { return e.msg }

// CheckResult 是可用性检测成功结果（账号昵称/ID 为本人本地部署的自查信息）。
type CheckResult struct {
	Mid   int64
	Uname string
}

// Checker 通过 B 站 nav 端点检测 Cookie 登录状态。
type Checker struct {
	Client *http.Client
	// NavURL 可覆盖（测试注入 httptest）；空用 DefaultNavURL。
	NavURL string
}

// NewChecker 创建默认 Checker（15s 超时）。
func NewChecker(client *http.Client) *Checker {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &Checker{Client: client}
}

type navResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		IsLogin bool   `json:"isLogin"`
		Mid     int64  `json:"mid"`
		Uname   string `json:"uname"`
	} `json:"data"`
}

// CheckBilibili 用 Cookie 请求 nav 端点：已登录返回账号信息；
// 未登录/被拒绝返回 ErrCookieInvalid；网络与远端异常返回 *CheckError。
func (c *Checker) CheckBilibili(ctx context.Context, f File) (CheckResult, error) {
	navURL := c.NavURL
	if navURL == "" {
		navURL = DefaultNavURL
	}
	client := c.Client
	if client == nil {
		client = NewChecker(nil).Client
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, navURL, nil)
	if err != nil {
		return CheckResult{}, fmt.Errorf("构造 B 站检测请求失败: %w", err)
	}
	req.Header.Set("User-Agent", navUserAgent)
	req.Header.Set("Referer", "https://www.bilibili.com")
	req.Header.Set("Origin", "https://www.bilibili.com")
	if header := f.CookieHeader(); header != "" {
		req.Header.Set("Cookie", header)
	}
	resp, err := client.Do(req)
	if err != nil {
		return CheckResult{}, &CheckError{msg: "无法连接 B 站检测端点"}
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return CheckResult{}, &CheckError{msg: fmt.Sprintf("B 站检测端点返回 HTTP %d", resp.StatusCode)}
	}
	var body navResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return CheckResult{}, &CheckError{msg: "B 站检测端点返回了无法解析的内容"}
	}
	if body.Code != 0 || !body.Data.IsLogin {
		return CheckResult{}, ErrCookieInvalid
	}
	return CheckResult{Mid: body.Data.Mid, Uname: body.Data.Uname}, nil
}
