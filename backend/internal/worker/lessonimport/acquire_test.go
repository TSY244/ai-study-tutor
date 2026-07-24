package lessonimport

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TSY244/ai-study-tutor/backend/internal/worker"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker/proc"
)

func publicResolver(context.Context, string) ([]net.IP, error) {
	return []net.IP{net.ParseIP("93.184.216.34")}, nil
}

func privateResolver(context.Context, string) ([]net.IP, error) {
	return []net.IP{net.ParseIP("192.168.1.10")}, nil
}

func TestCheckURLHost(t *testing.T) {
	cases := []struct {
		name      string
		url       string
		allowlist []string
		resolver  func(context.Context, string) ([]net.IP, error)
		wantCode  string
	}{
		{"http_rejected", "http://www.youtube.com/watch?v=1", []string{"youtube.com"}, publicResolver, worker.ErrCodeURLBlocked},
		{"not_allowlisted", "https://evil.example.com/x", []string{"youtube.com"}, publicResolver, worker.ErrCodeURLBlocked},
		{"private_ip", "https://www.youtube.com/watch?v=1", []string{"youtube.com"}, privateResolver, worker.ErrCodeURLBlocked},
		{"unresolvable", "https://www.youtube.com/watch?v=1", []string{"youtube.com"},
			func(context.Context, string) ([]net.IP, error) { return nil, errors.New("no such host") },
			worker.ErrCodeMaterialFetchFailed},
		{"allowed", "https://www.youtube.com/watch?v=1", []string{"youtube.com"}, publicResolver, ""},
		{"subdomain_allowed", "https://m.youtube.com/watch?v=1", []string{"youtube.com"}, publicResolver, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := &Handler{URLAllowlist: c.allowlist, Resolver: c.resolver}
			err := h.checkURLHost(context.Background(), c.url)
			if c.wantCode == "" {
				if err != nil {
					t.Fatalf("期望通过，得到 %v", err)
				}
				return
			}
			var werr *worker.Error
			if !errors.As(err, &werr) || werr.Code != c.wantCode {
				t.Fatalf("期望错误码 %s，得到 %v", c.wantCode, err)
			}
		})
	}
}

// roundTripFunc 适配函数到 http.RoundTripper。
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func redirectResponse(status int, location string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Location": []string{location}},
		Body:       io.NopCloser(strings.NewReader("")),
	}
}

func okResponse() *http.Response {
	return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("ok"))}
}

func TestValidateRedirectChain(t *testing.T) {
	cases := []struct {
		name     string
		hops     map[string]*http.Response
		wantCode string
	}{
		{
			name: "direct_ok",
			hops: map[string]*http.Response{
				"https://www.youtube.com/watch": okResponse(),
			},
		},
		{
			name: "redirect_to_allowlisted",
			hops: map[string]*http.Response{
				"https://youtu.be/abc":                redirectResponse(302, "https://www.youtube.com/watch?v=abc"),
				"https://www.youtube.com/watch?v=abc": okResponse(),
			},
		},
		{
			name: "redirect_to_private_scheme",
			hops: map[string]*http.Response{
				"https://youtu.be/abc": redirectResponse(302, "http://www.youtube.com/watch"),
			},
			wantCode: worker.ErrCodeURLBlocked,
		},
		{
			name: "redirect_to_not_allowlisted",
			hops: map[string]*http.Response{
				"https://youtu.be/abc": redirectResponse(302, "https://evil.example.com/x"),
			},
			wantCode: worker.ErrCodeURLBlocked,
		},
		{
			name: "too_many_redirects",
			hops: func() map[string]*http.Response {
				m := map[string]*http.Response{}
				m["https://youtu.be/abc"] = redirectResponse(302, "https://www.youtube.com/1")
				m["https://www.youtube.com/1"] = redirectResponse(302, "https://www.youtube.com/2")
				m["https://www.youtube.com/2"] = redirectResponse(302, "https://www.youtube.com/3")
				m["https://www.youtube.com/3"] = redirectResponse(302, "https://www.youtube.com/4")
				m["https://www.youtube.com/4"] = redirectResponse(302, "https://www.youtube.com/5")
				m["https://www.youtube.com/5"] = redirectResponse(302, "https://www.youtube.com/6")
				m["https://www.youtube.com/6"] = okResponse()
				return m
			}(),
			wantCode: worker.ErrCodeURLBlocked,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if resp, ok := c.hops[r.URL.String()]; ok {
					return resp, nil
				}
				return nil, errors.New("unexpected URL: " + r.URL.String())
			})
			h := &Handler{
				URLAllowlist: []string{"youtube.com", "youtu.be"},
				Resolver:     publicResolver,
				Transport:    transport,
			}
			start := "https://youtu.be/abc"
			if c.name == "direct_ok" {
				start = "https://www.youtube.com/watch"
			}
			err := h.validateRedirectChain(context.Background(), start)
			if c.wantCode == "" {
				if err != nil {
					t.Fatalf("期望通过，得到 %v", err)
				}
				return
			}
			var werr *worker.Error
			if !errors.As(err, &werr) || werr.Code != c.wantCode {
				t.Fatalf("期望错误码 %s，得到 %v", c.wantCode, err)
			}
		})
	}
}

func TestClassifyProcErr(t *testing.T) {
	cases := []struct {
		name          string
		err           error
		wantCode      string
		wantRetryable bool
	}{
		{"missing_tool", &proc.MissingToolError{Tool: "yt-dlp"}, worker.ErrCodeToolMissing, false},
		{"exit_429_retryable", &proc.ExitError{Code: 1, StderrTail: "HTTP Error 429: Too Many Requests"}, worker.ErrCodeMaterialFetchFailed, true},
		{"exit_412_actionable", &proc.ExitError{Code: 1, StderrTail: "HTTP Error 412: Precondition Failed"}, worker.ErrCodeMaterialFetchFailed, false},
		{"exit_generic", &proc.ExitError{Code: 2, StderrTail: "some unknown failure"}, worker.ErrCodeMaterialFetchFailed, false},
		{"other_internal", errors.New("spawn failed"), worker.ErrCodeInternal, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			werr := classifyProcErr("yt-dlp", c.err)
			if werr.Code != c.wantCode || werr.Retryable != c.wantRetryable {
				t.Fatalf("得到 (%s, %v)，期望 (%s, %v)", werr.Code, werr.Retryable, c.wantCode, c.wantRetryable)
			}
		})
	}
}

func TestPickSubtitleFile(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("src.en.vtt", "WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nhello world\n")
	write("src.zh-Hans.vtt", "WEBVTT\n\n00:00:01.000 --> 00:00:02.000\n你好世界\n")
	write("src.info.json", `{"title":"演示视频","duration":42,"webpage_url":"https://youtu.be/abc","extractor":"youtube"}`)

	path, lang, err := pickSubtitleFile(dir, "zh-Hans,zh,en")
	if err != nil {
		t.Fatal(err)
	}
	if lang != "zh-Hans" {
		t.Fatalf("语言优先级错误: %s", lang)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "你好世界" {
		t.Fatalf("清洗结果错误: %q", raw)
	}

	// info.json 只取安全字段。
	info := &sourceInfo{}
	h := &Handler{}
	h.loadInfoJSON(dir, info)
	if info.Title != "演示视频" || info.Duration != 42 || info.Extractor != "youtube" {
		t.Fatalf("info.json 解析错误: %+v", info)
	}
}

func TestPickSubtitleFileNone(t *testing.T) {
	dir := t.TempDir()
	path, _, err := pickSubtitleFile(dir, "zh,en")
	if err != nil {
		t.Fatal(err)
	}
	if path != "" {
		t.Fatalf("无字幕应返回空路径，得到 %s", path)
	}
}

func TestSubtitleLangsPreferredFirst(t *testing.T) {
	got := subtitleLangs("en")
	if !strings.HasPrefix(got, "en,") {
		t.Fatalf("偏好语言应置前: %s", got)
	}
	if strings.Count(got, ",en") != 0 && !strings.Contains(got, ",en,") {
		t.Fatalf("偏好语言不应重复: %s", got)
	}
}
