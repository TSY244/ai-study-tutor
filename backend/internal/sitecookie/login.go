package sitecookie

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// LoginSessions is the private file handoff between Worker and API. QR keys and
// credentials never enter jobs.payload/result, logs, or browser responses.
type LoginSessions struct {
	Root string
	mu   sync.Mutex
}

type LoginStatus struct {
	State          string  `json:"state"`
	Message        string  `json:"message"`
	ExpiresAt      *string `json:"expires_at,omitempty"`
	QRImageDataURL string  `json:"qr_image_data_url,omitempty"`
	Cookie         *Status `json:"cookie,omitempty"`
}

func NewLoginSessions(dataDir string) *LoginSessions {
	return &LoginSessions{Root: filepath.Join(dataDir, "bilibili-login")}
}

func (s *LoginSessions) Dir(id uuid.UUID) string {
	return filepath.Join(s.Root, id.String())
}

func (s *LoginSessions) Profile(id uuid.UUID) string {
	return filepath.Join(s.Dir(id), "profile")
}

func (s *LoginSessions) Write(id uuid.UUID, status LoginStatus) error {
	if err := os.MkdirAll(s.Dir(id), 0o700); err != nil {
		return errors.New("无法创建扫码登录目录")
	}
	raw, err := json.Marshal(status)
	if err != nil {
		return errors.New("无法编码扫码登录状态")
	}
	tmp := filepath.Join(s.Dir(id), "state.tmp")
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return errors.New("无法保存扫码登录状态")
	}
	return os.Rename(tmp, filepath.Join(s.Dir(id), "state.json"))
}

func (s *LoginSessions) Read(id uuid.UUID) (LoginStatus, error) {
	raw, err := os.ReadFile(filepath.Join(s.Dir(id), "state.json"))
	if err != nil {
		return LoginStatus{}, err
	}
	var st LoginStatus
	if json.Unmarshal(raw, &st) != nil {
		return st, errors.New("扫码登录状态不可读取")
	}
	if st.State == "waiting_scan" || st.State == "scanned" {
		if st.ExpiresAt != nil {
			expires, err := time.Parse(time.RFC3339, *st.ExpiresAt)
			if err != nil || time.Now().After(expires) {
				return LoginStatus{State: "expired", Message: "二维码已过期，请重新生成。"}, nil
			}
		}
		image, err := os.ReadFile(filepath.Join(s.Profile(id), "login_qrcode.png"))
		if err == nil && len(image) <= 256<<10 && bytes.HasPrefix(image, []byte("\x89PNG\r\n\x1a\n")) {
			st.QRImageDataURL = "data:image/png;base64," + base64.StdEncoding.EncodeToString(image)
		}
	}
	return st, nil
}

// Complete imports the CLI credential only after the Worker job has succeeded.
// The API owns the Cookie mount; the Worker keeps its existing read-only mount.
func (s *LoginSessions) Complete(id uuid.UUID, store *Store) (LoginStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.Read(id)
	if err != nil {
		return st, errors.New("扫码登录结果已清理，请重新扫码。")
	}
	if st.State != "logged_in" {
		if st.State != "confirmed" {
			return st, errors.New("扫码登录尚未确认")
		}
		raw, err := os.ReadFile(filepath.Join(s.Profile(id), "credential.json"))
		if err != nil || len(raw) > MaxContentBytes {
			return st, errors.New("扫码登录凭证不可读取，请重新扫码。")
		}
		var cred map[string]string
		if json.Unmarshal(raw, &cred) != nil || cred["sessdata"] == "" || cred["bili_jct"] == "" {
			return st, errors.New("扫码登录凭证不完整，请重新扫码。")
		}
		fields := []struct{ from, to string }{{"sessdata", "SESSDATA"}, {"bili_jct", "bili_jct"},
			{"dedeuserid", "DedeUserID"}, {"buvid3", "buvid3"}, {"buvid4", "buvid4"}, {"ac_time_value", "ac_time_value"}}
		var pairs []string
		for _, f := range fields {
			if value := cred[f.from]; value != "" {
				pairs = append(pairs, f.to+"="+value)
			}
		}
		if _, err := store.Save(strings.Join(pairs, "; ")); err != nil {
			return st, errors.New("保存登录状态失败，请重试。")
		}
		st = LoginStatus{State: "logged_in", Message: "登录成功，已自动保存，导入视频时立即生效。"}
		if err := s.Write(id, st); err != nil {
			return st, err
		}
		if err := os.RemoveAll(s.Profile(id)); err != nil {
			return st, errors.New("登录成功，但清理临时凭证失败。")
		}
	}
	status, err := store.Status()
	st.Cookie = &status
	return st, err
}

func (s *LoginSessions) Cleanup(id uuid.UUID) error {
	return os.RemoveAll(s.Dir(id))
}

// Prune removes abandoned private sessions, including credentials left when a
// browser closes after confirmation. Active QR sessions last at most 3 minutes.
func (s *LoginSessions) Prune() error {
	entries, err := os.ReadDir(s.Root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		id, err := uuid.Parse(entry.Name())
		if err != nil || !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if time.Since(info.ModTime()) > 10*time.Minute {
			if err := s.Cleanup(id); err != nil {
				return err
			}
		}
	}
	return nil
}
