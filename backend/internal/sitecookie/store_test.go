package sitecookie

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var testNow = time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)

const (
	testSessdata    = "dd7d21e6%2C1800601560%2C0078f%2A71CjAfvsF8BaG8mOeT0kc1jIS6QpmiVZnyq9ejA-35sq9cMKxEQLYKb8xE9QaC7TEyiqgSVnBBREtkdXlNTmNHWWRyYzRyQnFRcFlIZFFkY1RicEVsODFabWhocjEzQkNLdmU5ZXNLN3lqNjhSek84dUZmQ2hkWnlON1o5bXhwNGltWmE5UEdxU1VnIIEC"
	testBiliJct     = "bili-jct-value-0123456789"
	futureExpires   = "1800601560"
	pastExpires     = "1700000000"
	netscapeContent = `# Netscape HTTP Cookie File
.bilibili.com	TRUE	/	TRUE	` + futureExpires + `	SESSDATA	` + testSessdata + `
.bilibili.com	TRUE	/	FALSE	` + futureExpires + `	bili_jct	` + testBiliJct + `
#HttpOnly_.bilibili.com	TRUE	/	TRUE	` + futureExpires + `	OTHER_FLAG	v1
.google.com	TRUE	/	TRUE	` + futureExpires + `	SID	should-be-dropped
`
)

func TestParseBareSessdata(t *testing.T) {
	f, err := Parse(testSessdata+"\n", testNow)
	if err != nil {
		t.Fatalf("bare SESSDATA 应可解析: %v", err)
	}
	if len(f.Entries) != 1 {
		t.Fatalf("应合成单条 SESSDATA 条目, got %d", len(f.Entries))
	}
	e := f.Entries[0]
	if e.Name != "SESSDATA" || e.Value != testSessdata || e.Domain != ".bilibili.com" || !e.Secure {
		t.Fatalf("合成条目不符: %+v", e)
	}
	if e.Expires <= testNow.Unix() {
		t.Fatal("合成条目应带未来有效期")
	}
}

func TestParseCookieHeaderString(t *testing.T) {
	f, err := Parse("SESSDATA="+testSessdata+"; bili_jct="+testBiliJct, testNow)
	if err != nil {
		t.Fatalf("Cookie 头串应可解析: %v", err)
	}
	if len(f.Entries) != 2 {
		t.Fatalf("应解析出两条, got %d", len(f.Entries))
	}
	if got := f.CookieHeader(); got != "SESSDATA="+testSessdata+"; bili_jct="+testBiliJct {
		t.Fatalf("CookieHeader 不符: %s", got)
	}
}

func TestParseNetscapeFiltersNonBilibiliAndComments(t *testing.T) {
	f, err := Parse(netscapeContent, testNow)
	if err != nil {
		t.Fatalf("Netscape 内容应可解析: %v", err)
	}
	if len(f.Entries) != 3 {
		t.Fatalf("非 bilibili 条目应被丢弃, got %d", len(f.Entries))
	}
	httpOnlySeen := false
	for _, e := range f.Entries {
		if e.Domain == ".google.com" || e.Value == "should-be-dropped" {
			t.Fatal("非 bilibili 条目泄漏进结果")
		}
		if e.HTTPOnly {
			httpOnlySeen = true
		}
	}
	if !httpOnlySeen {
		t.Fatal("#HttpOnly_ 前缀应被识别")
	}
	if _, ok := f.Sessdata(); !ok {
		t.Fatal("应包含 SESSDATA")
	}
}

func TestParseRejectsInvalidContent(t *testing.T) {
	cases := map[string]string{
		"empty":            "   \n ",
		"too_large":        strings.Repeat("a", MaxContentBytes+1),
		"missing_sessdata": ".bilibili.com\tTRUE\t/\tTRUE\t" + futureExpires + "\tbili_jct\t" + testBiliJct + "\n",
		"bad_columns":      "not-a-netscape-line\tbut\ttabs\n",
		"bad_expires":      ".bilibili.com\tTRUE\t/\tTRUE\tnotanumber\tSESSDATA\t" + testSessdata + "\n",
		"header_no_sess":   "bili_jct=" + testBiliJct + "; other=1",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Parse(content, testNow)
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("应返回 ErrInvalid, got %v", err)
			}
			if strings.Contains(err.Error(), testSessdata) || strings.Contains(err.Error(), testBiliJct) {
				t.Fatal("错误信息不得包含 Cookie 值")
			}
		})
	}
}

func TestStoreSaveLoadStatusDeleteRoundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cookies", "bilibili.txt")
	s := NewStore(path)

	// 未保存时：Status 非错误、Configured=false；Load/Delete 报 ErrNotFound。
	st, err := s.Status()
	if err != nil || st.Configured {
		t.Fatalf("未保存状态应为 configured=false: %+v %v", st, err)
	}
	if _, err := s.Load(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Load 应报 ErrNotFound, got %v", err)
	}
	if err := s.Delete(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete 应报 ErrNotFound, got %v", err)
	}

	// 保存 Netscape 内容 → 0600 → 可加载；Status 含脱敏与过期时间。
	st, err = s.Save(netscapeContent)
	if err != nil {
		t.Fatalf("Save 失败: %v", err)
	}
	if !st.Configured || st.CookieCount != 3 || st.MaskedSessdata == "" || st.SessdataExpiresAt == nil || st.Expired {
		t.Fatalf("保存后状态不符: %+v", st)
	}
	if strings.Contains(st.MaskedSessdata, testSessdata) {
		t.Fatal("脱敏值不得包含完整 SESSDATA")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("文件权限应为 0600, got %o", info.Mode().Perm())
	}
	f, err := s.Load()
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if len(f.Entries) != 3 {
		t.Fatalf("Load 应返回 3 条, got %d", len(f.Entries))
	}

	// 覆盖保存 bare SESSDATA。
	st, err = s.Save(testSessdata)
	if err != nil {
		t.Fatalf("覆盖保存失败: %v", err)
	}
	if !st.Configured || st.CookieCount != 1 {
		t.Fatalf("覆盖保存后状态不符: %+v", st)
	}

	// 删除后回到未配置。
	if err := s.Delete(); err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}
	st, err = s.Status()
	if err != nil || st.Configured {
		t.Fatalf("删除后应为 configured=false: %+v %v", st, err)
	}
}

func TestStoreStatusReportsExpired(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bilibili.txt")
	s := NewStore(path)
	content := ".bilibili.com\tTRUE\t/\tTRUE\t" + pastExpires + "\tSESSDATA\t" + testSessdata + "\n"
	st, err := s.Save(content)
	if err != nil {
		t.Fatalf("Save 失败: %v", err)
	}
	if !st.Expired || st.SessdataExpiresAt == nil {
		t.Fatalf("应报告已过期: %+v", st)
	}
}

func TestStoreEmptyPathIsNotConfigured(t *testing.T) {
	s := NewStore("")
	if _, err := s.Save(testSessdata); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("空路径 Save 应报 ErrNotConfigured, got %v", err)
	}
	if _, err := s.Load(); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("空路径 Load 应报 ErrNotConfigured, got %v", err)
	}
	if err := s.Delete(); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("空路径 Delete 应报 ErrNotConfigured, got %v", err)
	}
	st, err := s.Status()
	if err != nil || st.Configured {
		t.Fatalf("空路径 Status 应为 configured=false: %+v %v", st, err)
	}
}
