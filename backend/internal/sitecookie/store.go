// Package sitecookie 管理站点登录 Cookie（当前为 B 站 bilibili.com）：
//   - Netscape cookie 文件内容的安全解析、校验、归一化与原子写入（0600）；
//   - 绝不外泄完整 Cookie 的安全状态视图（masked SESSDATA + 过期时间）；
//   - B 站登录状态可用性检测（x/web-interface/nav）；
//   - /api/v1/system/bilibili-cookie 管理端点。
//
// 安全边界：完整 Cookie 只允许存在于 bilibili.txt 与进程内存，绝不进入
// API 响应、日志、Job 错误信息；保存时仅保留 bilibili.com 域名条目，
// 其余站点条目在归一化时丢弃（更小的凭证暴露面）。
package sitecookie

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/TSY244/ai-study-tutor/backend/internal/claudegateway"
)

// MaxContentBytes 是 PUT 内容与磁盘文件的上限（64 KiB）。
const MaxContentBytes = 64 << 10

// bareSessdataLifetime 是仅提供 SESSDATA 值时合成条目的有效期
// （与 B 站 SESSDATA 大约一年的有效期对齐）。
const bareSessdataLifetime = 365 * 24 * time.Hour

var (
	// ErrNotConfigured 表示未配置 Cookie 文件路径（部署未启用该能力）。
	ErrNotConfigured = errors.New("bilibili cookie file path not configured")
	// ErrNotFound 表示磁盘上没有已保存的 Cookie 文件。
	ErrNotFound = errors.New("bilibili cookie file not found")
	// ErrInvalid 表示提交的内容不是可接受的 Cookie（错误信息不含内容本身）。
	ErrInvalid = errors.New("bilibili cookie content invalid")
)

// Entry 是归一化后的单条 Cookie。
type Entry struct {
	Domain   string
	Path     string
	Name     string
	Value    string
	Expires  int64 // Unix 秒；0 表示会话级
	Secure   bool
	HTTPOnly bool
}

// File 是解析后的 Cookie 集合（只含 bilibili.com 域名条目）。
type File struct {
	Entries []Entry
}

// Sessdata 返回首个 SESSDATA 条目。
func (f File) Sessdata() (Entry, bool) {
	for _, e := range f.Entries {
		if e.Name == "SESSDATA" && e.Value != "" {
			return e, true
		}
	}
	return Entry{}, false
}

// CookieHeader 构造请求用 Cookie 头值（name=value; ...）。
func (f File) CookieHeader() string {
	parts := make([]string, 0, len(f.Entries))
	for _, e := range f.Entries {
		if e.Name == "" || e.Value == "" {
			continue
		}
		parts = append(parts, e.Name+"="+e.Value)
	}
	return strings.Join(parts, "; ")
}

// Parse 校验并归一化提交的 Cookie 内容：接受 Netscape 导出文件、
// Cookie 头串（SESSDATA=xxx; ...）或单个 SESSDATA 值；仅保留
// bilibili.com 域名条目；必须包含非空 SESSDATA。
// 返回的错误不含内容本身，可安全用于 API 错误响应（调用方仍应映射为
// 通用文案）。
func Parse(content string, now time.Time) (File, error) {
	if len(content) > MaxContentBytes {
		return File{}, fmt.Errorf("%w: 内容超过 64 KiB 上限", ErrInvalid)
	}
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return File{}, fmt.Errorf("%w: 内容为空", ErrInvalid)
	}
	// 单个 SESSDATA 值：无制表符/换行/分号/等号的可打印 ASCII 串。
	if !strings.ContainsAny(trimmed, "\t\r\n;= ") && isPrintableASCII(trimmed) {
		return File{Entries: []Entry{{
			Domain:  ".bilibili.com",
			Path:    "/",
			Name:    "SESSDATA",
			Value:   trimmed,
			Expires: now.Add(bareSessdataLifetime).Unix(),
			Secure:  true,
		}}}, nil
	}
	// Cookie 头串（SESSDATA=xxx; bili_jct=yyy）：无制表符/换行但含等号。
	if !strings.ContainsAny(trimmed, "\t\r\n") && strings.Contains(trimmed, "=") {
		var entries []Entry
		for _, pair := range strings.Split(trimmed, ";") {
			name, value, ok := strings.Cut(strings.TrimSpace(pair), "=")
			if !ok || name == "" || value == "" || strings.ContainsAny(name, "\x20") {
				return File{}, fmt.Errorf("%w: Cookie 对格式无效", ErrInvalid)
			}
			if !isPrintableASCII(value) {
				return File{}, fmt.Errorf("%w: Cookie 值含非法字符", ErrInvalid)
			}
			entries = append(entries, Entry{
				Domain:  ".bilibili.com",
				Path:    "/",
				Name:    name,
				Value:   value,
				Expires: now.Add(bareSessdataLifetime).Unix(),
				Secure:  true,
			})
		}
		f := File{Entries: entries}
		if _, ok := f.Sessdata(); !ok {
			return File{}, fmt.Errorf("%w: 未找到 SESSDATA", ErrInvalid)
		}
		return f, nil
	}

	var entries []Entry
	for _, line := range strings.Split(trimmed, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		httpOnly := false
		if strings.HasPrefix(line, "#HttpOnly_") {
			httpOnly = true
			line = strings.TrimPrefix(line, "#HttpOnly_")
		} else if strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 7 {
			return File{}, fmt.Errorf("%w: 存在无法解析的行（需要 Netscape 7 列格式）", ErrInvalid)
		}
		domain := strings.TrimSpace(fields[0])
		if !isBilibiliDomain(domain) {
			continue
		}
		expires, err := strconv.ParseInt(strings.TrimSpace(fields[4]), 10, 64)
		if err != nil || expires < 0 {
			return File{}, fmt.Errorf("%w: 过期时间不是合法的 Unix 秒", ErrInvalid)
		}
		path := strings.TrimSpace(fields[2])
		if path == "" {
			path = "/"
		}
		entries = append(entries, Entry{
			Domain:   domain,
			Path:     path,
			Name:     strings.TrimSpace(fields[5]),
			Value:    strings.TrimSpace(fields[6]),
			Expires:  expires,
			Secure:   strings.EqualFold(strings.TrimSpace(fields[3]), "TRUE"),
			HTTPOnly: httpOnly,
		})
	}
	f := File{Entries: entries}
	if _, ok := f.Sessdata(); !ok {
		return File{}, fmt.Errorf("%w: 未找到 bilibili.com 的 SESSDATA", ErrInvalid)
	}
	return f, nil
}

// serialize 输出归一化 Netscape 文本（写盘用）。
func (f File) serialize() string {
	var b strings.Builder
	b.WriteString("# Netscape HTTP Cookie File\n")
	for _, e := range f.Entries {
		flag := "FALSE"
		if strings.HasPrefix(e.Domain, ".") {
			flag = "TRUE"
		}
		secure := "FALSE"
		if e.Secure {
			secure = "TRUE"
		}
		domain := e.Domain
		if e.HTTPOnly {
			domain = "#HttpOnly_" + domain
		}
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\t%d\t%s\t%s\n",
			domain, flag, e.Path, secure, e.Expires, e.Name, e.Value)
	}
	return b.String()
}

func isBilibiliDomain(domain string) bool {
	d := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(domain)), ".")
	return d == "bilibili.com" || strings.HasSuffix(d, ".bilibili.com")
}

func isPrintableASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x21 || s[i] > 0x7e {
			return false
		}
	}
	return true
}

// Status 是 API 响应的安全视图，绝不包含完整 Cookie。
type Status struct {
	Configured        bool    `json:"configured"`
	UpdatedAt         *string `json:"updated_at"`
	SessdataExpiresAt *string `json:"sessdata_expires_at"`
	Expired           bool    `json:"expired"`
	MaskedSessdata    string  `json:"masked_sessdata"`
	CookieCount       int     `json:"cookie_count"`
}

// Store 以单个 Netscape 文件为后端；读侧不做缓存，Worker 每次 Job
// 实时读取，因此 API 保存后无需重启。
type Store struct {
	path string
	now  func() time.Time
}

// NewStore 创建以 path 为后端的 Store；path 为空表示能力未配置。
func NewStore(path string) *Store {
	return &Store{path: strings.TrimSpace(path), now: time.Now}
}

// Path 返回 Cookie 文件路径。
func (s *Store) Path() string { return s.path }

// Save 校验内容并原子覆盖写盘（0600），返回保存后的安全状态。
func (s *Store) Save(content string) (Status, error) {
	if s.path == "" {
		return Status{}, ErrNotConfigured
	}
	f, err := Parse(content, s.now())
	if err != nil {
		return Status{}, err
	}
	if err := writeFileAtomic(s.path, []byte(f.serialize())); err != nil {
		return Status{}, err
	}
	return s.Status()
}

// Load 读取并解析磁盘文件；文件缺失返回 ErrNotFound。
func (s *Store) Load() (File, error) {
	if s.path == "" {
		return File{}, ErrNotConfigured
	}
	raw, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return File{}, ErrNotFound
	}
	if err != nil {
		return File{}, err
	}
	return Parse(string(raw), s.now())
}

// Delete 移除磁盘文件；不存在返回 ErrNotFound。
func (s *Store) Delete() error {
	if s.path == "" {
		return ErrNotConfigured
	}
	if err := os.Remove(s.path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

// Status 返回安全视图；文件缺失时 Configured=false（非错误）。
func (s *Store) Status() (Status, error) {
	st := Status{}
	if s.path == "" {
		return st, nil
	}
	f, err := s.Load()
	if errors.Is(err, ErrNotFound) {
		return st, nil
	}
	if err != nil {
		return st, err
	}
	st.Configured = true
	st.CookieCount = len(f.Entries)
	if info, statErr := os.Stat(s.path); statErr == nil {
		v := info.ModTime().UTC().Format(time.RFC3339)
		st.UpdatedAt = &v
	}
	if sess, ok := f.Sessdata(); ok {
		st.MaskedSessdata = claudegateway.MaskKey(sess.Value)
		if sess.Expires > 0 {
			v := time.Unix(sess.Expires, 0).UTC().Format(time.RFC3339)
			st.SessdataExpiresAt = &v
			st.Expired = sess.Expires <= s.now().Unix()
		}
	}
	return st, nil
}

// writeFileAtomic 以 0600 权限原子写入文件：同目录临时文件 → chmod 0600
// → fsync → rename。目录缺失时以 0700 创建。
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".site-cookie-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
