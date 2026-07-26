package sitecookie

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testFile(t *testing.T) File {
	t.Helper()
	f, err := Parse(netscapeContent, testNow)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestCheckBilibiliLoggedIn(t *testing.T) {
	var gotCookie, gotUA, gotReferer string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCookie = r.Header.Get("Cookie")
		gotUA = r.Header.Get("User-Agent")
		gotReferer = r.Header.Get("Referer")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"0","data":{"isLogin":true,"mid":654321,"uname":"测试用户"}}`))
	}))
	defer srv.Close()

	checker := NewChecker(srv.Client())
	checker.NavURL = srv.URL
	res, err := checker.CheckBilibili(context.Background(), testFile(t))
	if err != nil {
		t.Fatalf("已登录应成功: %v", err)
	}
	if res.Mid != 654321 || res.Uname != "测试用户" {
		t.Fatalf("账号信息不符: %+v", res)
	}
	if !strings.Contains(gotCookie, "SESSDATA=") || !strings.Contains(gotCookie, "bili_jct=") {
		t.Fatalf("应携带 Cookie 头, got %q", gotCookie)
	}
	if gotUA == "" || gotReferer != "https://www.bilibili.com" {
		t.Fatalf("应携带浏览器 UA 与 Referer, got UA=%q Referer=%q", gotUA, gotReferer)
	}
}

func TestCheckBilibiliNotLoggedIn(t *testing.T) {
	cases := map[string]string{
		"code_-101":    `{"code":-101,"message":"账号未登录","data":{"isLogin":false}}`,
		"isloginfalse": `{"code":0,"message":"0","data":{"isLogin":false}}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(body))
			}))
			defer srv.Close()
			checker := NewChecker(srv.Client())
			checker.NavURL = srv.URL
			_, err := checker.CheckBilibili(context.Background(), testFile(t))
			if !errors.Is(err, ErrCookieInvalid) {
				t.Fatalf("未登录应映射为 ErrCookieInvalid, got %v", err)
			}
		})
	}
}

func TestCheckBilibiliTransientFailures(t *testing.T) {
	t.Run("http_412", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusPreconditionFailed)
		}))
		defer srv.Close()
		checker := NewChecker(srv.Client())
		checker.NavURL = srv.URL
		_, err := checker.CheckBilibili(context.Background(), testFile(t))
		var checkErr *CheckError
		if !errors.As(err, &checkErr) {
			t.Fatalf("412 应映射为 CheckError, got %v", err)
		}
		if strings.Contains(err.Error(), "SESSDATA") {
			t.Fatal("错误信息不得包含 Cookie")
		}
	})
	t.Run("malformed_json", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("<html>blocked</html>"))
		}))
		defer srv.Close()
		checker := NewChecker(srv.Client())
		checker.NavURL = srv.URL
		_, err := checker.CheckBilibili(context.Background(), testFile(t))
		var checkErr *CheckError
		if !errors.As(err, &checkErr) {
			t.Fatalf("非法 JSON 应映射为 CheckError, got %v", err)
		}
	})
	t.Run("network_error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		url := srv.URL
		srv.Close() // 立即关闭 → 连接失败
		checker := NewChecker(nil)
		checker.NavURL = url
		_, err := checker.CheckBilibili(context.Background(), testFile(t))
		var checkErr *CheckError
		if !errors.As(err, &checkErr) {
			t.Fatalf("网络错误应映射为 CheckError, got %v", err)
		}
	})
}
