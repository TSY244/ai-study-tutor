// Package claudegateway 实现本地 Claude 多 Key 网关：
//   - keys.json 文件存储（含密钥，0600，原子写入，mtime 变化自动重载）；
//   - 绝不外泄完整 Key 的安全视图（masked_key + fingerprint）；
//   - state.json 非敏感运行态（0600，原子写入）；
//   - 实现 ai.Provider 的优先级故障转移路由；
//   - /api/v1/system/claude-keys 管理端点。
//
// 安全边界：完整 API Key 只允许存在于 keys.json 与进程内存，绝不进入
// API 响应、日志、Job 错误信息。
package claudegateway

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Key 是 keys.json 中持久化的单条密钥记录。APIKey 绝不离开进程：
// API 响应只携带 MaskedKey/Fingerprint。
type Key struct {
	ID                     string    `json:"id"`
	Name                   string    `json:"name"`
	APIKey                 string    `json:"api_key"`
	Model                  string    `json:"model"`
	Enabled                bool      `json:"enabled"`
	Priority               int       `json:"priority"`
	Weight                 int       `json:"weight"`
	TimeoutSeconds         int       `json:"timeout_seconds"`
	CooldownSeconds        int       `json:"cooldown_seconds"`
	MaxConsecutiveFailures int       `json:"max_consecutive_failures"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

// 默认值（契约 ClaudeKeyCreate 的 default 对齐）。
const (
	DefaultPriority               = 10
	DefaultWeight                 = 100
	DefaultTimeoutSeconds         = 300
	DefaultCooldownSeconds        = 300
	DefaultMaxConsecutiveFailures = 2
)

// ErrNotFound 表示指定的 Key 不存在。
var ErrNotFound = errors.New("claude gateway key not found")

// ErrInvalidKey 表示必填字段为空（调用方应先做 TrimSpace 校验）。
var ErrInvalidKey = errors.New("claude gateway key invalid")

// configFile 是 keys.json 的磁盘形状。
type configFile struct {
	Keys []Key `json:"keys"`
}

// Store 加载/保存 keys.json。读侧在文件 mtime/size 变化时自动重载，
// 因此 API 侧改动后 Worker 无需重启。
type Store struct {
	path      string
	now       func() time.Time
	writeFile func(string, []byte) error

	mu       sync.Mutex
	cached   *configFile
	modTime  time.Time
	size     int64
	fileInfo os.FileInfo
}

// NewStore 创建以 path 为后端的 Store；文件缺失视为空配置（非错误）。
func NewStore(path string) *Store {
	return &Store{path: path, now: time.Now}
}

// Path 返回 keys.json 路径。
func (s *Store) Path() string { return s.path }

// StatePathFor 返回与 keys.json 同目录的 state.json 路径。
func StatePathFor(keysPath string) string {
	return filepath.Join(filepath.Dir(keysPath), "state.json")
}

// Load 返回当前全部 Key（含禁用）；文件缺失返回空列表。
func (s *Store) Load() ([]Key, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *Store) loadLocked() ([]Key, error) {
	info, err := os.Stat(s.path)
	if errors.Is(err, os.ErrNotExist) {
		s.cached = nil
		s.modTime = time.Time{}
		s.size = 0
		s.fileInfo = nil
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if s.cached != nil && s.fileInfo != nil && os.SameFile(info, s.fileInfo) &&
		info.ModTime().Equal(s.modTime) && info.Size() == s.size {
		return cloneKeys(s.cached.Keys), nil
	}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return nil, err
	}
	var cfg configFile
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("解析 Claude 网关配置失败")
	}
	s.cached = &cfg
	s.modTime = info.ModTime()
	s.size = info.Size()
	s.fileInfo = info
	return cloneKeys(cfg.Keys), nil
}

// Get 按 ID 返回单个 Key。
func (s *Store) Get(id string) (Key, error) {
	keys, err := s.Load()
	if err != nil {
		return Key{}, err
	}
	for _, k := range keys {
		if k.ID == id {
			return k, nil
		}
	}
	return Key{}, ErrNotFound
}

// Create 追加一个 Key：分配 ID 与时间戳，应用默认值后原子写盘。
// k.Name/k.Model/k.APIKey 必须先经调用方 TrimSpace 且非空。
func (s *Store) Create(k Key) (Key, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.reloadLocked(); err != nil {
		return Key{}, err
	}
	k.APIKey = strings.TrimSpace(k.APIKey)
	k.Name = strings.TrimSpace(k.Name)
	k.Model = strings.TrimSpace(k.Model)
	if k.APIKey == "" || k.Name == "" || k.Model == "" {
		return Key{}, fmt.Errorf("%w: api_key/name/model 不能为空", ErrInvalidKey)
	}
	id, err := newKeyID()
	if err != nil {
		return Key{}, err
	}
	k.ID = id
	if k.Weight <= 0 {
		k.Weight = DefaultWeight
	}
	if k.TimeoutSeconds <= 0 {
		k.TimeoutSeconds = DefaultTimeoutSeconds
	}
	if k.CooldownSeconds <= 0 {
		k.CooldownSeconds = DefaultCooldownSeconds
	}
	if k.MaxConsecutiveFailures <= 0 {
		k.MaxConsecutiveFailures = DefaultMaxConsecutiveFailures
	}
	now := s.now().UTC()
	k.CreatedAt = now
	k.UpdatedAt = now
	cfg := s.cloneCachedOrEmpty()
	cfg.Keys = append(cfg.Keys, k)
	if err := s.saveLocked(cfg); err != nil {
		return Key{}, err
	}
	return k, nil
}

// KeyPatch 描述一次部分更新；nil 指针表示保持不变。
// APIKey 为 nil 时保留原 Key（PATCH 省略 api_key 的语义）。
type KeyPatch struct {
	APIKey                 *string
	Name                   *string
	Model                  *string
	Enabled                *bool
	Priority               *int
	Weight                 *int
	TimeoutSeconds         *int
	CooldownSeconds        *int
	MaxConsecutiveFailures *int
}

// Update 应用补丁并原子写盘；Key 不存在返回 ErrNotFound。
// 补丁值先 TrimSpace 校验，空/全空白的 api_key/name/model 直接拒绝，
// 不会污染内存缓存。
func (s *Store) Update(id string, patch KeyPatch) (Key, error) {
	if patch.APIKey != nil && strings.TrimSpace(*patch.APIKey) == "" {
		return Key{}, fmt.Errorf("%w: api_key 不能为空", ErrInvalidKey)
	}
	if patch.Name != nil && strings.TrimSpace(*patch.Name) == "" {
		return Key{}, fmt.Errorf("%w: name 不能为空", ErrInvalidKey)
	}
	if patch.Model != nil && strings.TrimSpace(*patch.Model) == "" {
		return Key{}, fmt.Errorf("%w: model 不能为空", ErrInvalidKey)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.reloadLocked(); err != nil {
		return Key{}, err
	}
	cfg := s.cloneCachedOrEmpty()
	for i := range cfg.Keys {
		k := &cfg.Keys[i]
		if k.ID != id {
			continue
		}
		if patch.APIKey != nil {
			k.APIKey = strings.TrimSpace(*patch.APIKey)
		}
		if patch.Name != nil {
			k.Name = strings.TrimSpace(*patch.Name)
		}
		if patch.Model != nil {
			k.Model = strings.TrimSpace(*patch.Model)
		}
		if patch.Enabled != nil {
			k.Enabled = *patch.Enabled
		}
		if patch.Priority != nil {
			k.Priority = *patch.Priority
		}
		if patch.Weight != nil {
			k.Weight = *patch.Weight
		}
		if patch.TimeoutSeconds != nil {
			k.TimeoutSeconds = *patch.TimeoutSeconds
		}
		if patch.CooldownSeconds != nil {
			k.CooldownSeconds = *patch.CooldownSeconds
		}
		if patch.MaxConsecutiveFailures != nil {
			k.MaxConsecutiveFailures = *patch.MaxConsecutiveFailures
		}
		k.UpdatedAt = s.now().UTC()
		updated := *k
		if err := s.saveLocked(cfg); err != nil {
			return Key{}, err
		}
		return updated, nil
	}
	return Key{}, ErrNotFound
}

// Delete 移除 Key 并原子写盘；不存在返回 ErrNotFound。
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.reloadLocked(); err != nil {
		return err
	}
	cfg := s.cloneCachedOrEmpty()
	for i := range cfg.Keys {
		if cfg.Keys[i].ID == id {
			cfg.Keys = append(cfg.Keys[:i], cfg.Keys[i+1:]...)
			return s.saveLocked(cfg)
		}
	}
	return ErrNotFound
}

// HasEnabledKey 报告是否存在至少一个启用 Key（启动期能力判定用）。
func (s *Store) HasEnabledKey() bool {
	keys, err := s.Load()
	if err != nil {
		return false
	}
	for _, k := range keys {
		if k.Enabled {
			return true
		}
	}
	return false
}

// reloadLocked 绕过缓存强制从磁盘重载（写前读取最新状态，
// 避免覆盖另一进程刚写入的改动）。
func (s *Store) reloadLocked() ([]Key, error) {
	s.cached = nil
	s.modTime = time.Time{}
	s.size = 0
	s.fileInfo = nil
	return s.loadLocked()
}

func (s *Store) cloneCachedOrEmpty() *configFile {
	if s.cached == nil {
		return &configFile{}
	}
	return &configFile{Keys: cloneKeys(s.cached.Keys)}
}

// saveLocked 原子写入 keys.json（同目录临时文件 → 0600 → fsync → rename）。
func (s *Store) saveLocked(cfg *configFile) error {
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	writeFile := s.writeFile
	if writeFile == nil {
		writeFile = writeFileAtomic
	}
	if err := writeFile(s.path, raw); err != nil {
		return err
	}
	info, err := os.Stat(s.path)
	if err != nil {
		return err
	}
	s.cached = cfg
	s.modTime = info.ModTime()
	s.size = info.Size()
	s.fileInfo = info
	return nil
}

// writeFileAtomic 以 0600 权限原子写入文件：同目录临时文件 → chmod 0600
// → fsync → rename。目录缺失时以 0700 创建。
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".claude-gateway-*.tmp")
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

// newKeyID 生成 ck_ 前缀的随机 ID（不含任何密钥材料）。
func newKeyID() (string, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "ck_" + hex.EncodeToString(b[:]), nil
}

func cloneKeys(keys []Key) []Key {
	out := make([]Key, len(keys))
	copy(out, keys)
	return out
}

// MaskKey 返回脱敏展示形式（如 sk-ant-...WXYZ）。短 Key 不暴露任何片段。
func MaskKey(key string) string {
	key = strings.TrimSpace(key)
	switch {
	case key == "":
		return ""
	case len(key) <= 8:
		return "..."
	default:
		prefix := key
		if len(prefix) > 7 {
			prefix = prefix[:7]
		}
		return prefix + "..." + key[len(key)-4:]
	}
}

// Fingerprint 返回 Key 的短指纹（sha256: 前缀），用于区分密钥。
func Fingerprint(key string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(key)))
	return "sha256:" + hex.EncodeToString(sum[:])[:12]
}

// SafeKey 是 API 响应的安全视图，绝不包含 APIKey。
type SafeKey struct {
	ID                     string  `json:"id"`
	Name                   string  `json:"name"`
	MaskedKey              string  `json:"masked_key"`
	Fingerprint            string  `json:"fingerprint"`
	Model                  string  `json:"model"`
	Enabled                bool    `json:"enabled"`
	Priority               int     `json:"priority"`
	Weight                 int     `json:"weight"`
	TimeoutSeconds         int     `json:"timeout_seconds"`
	CooldownSeconds        int     `json:"cooldown_seconds"`
	MaxConsecutiveFailures int     `json:"max_consecutive_failures"`
	Status                 string  `json:"status"`
	ConsecutiveFailures    int     `json:"consecutive_failures"`
	LastErrorCode          *string `json:"last_error_code"`
	LastSuccessAt          *string `json:"last_success_at"`
	LastFailureAt          *string `json:"last_failure_at"`
	CooldownUntil          *string `json:"cooldown_until"`
	LastLatencyMs          *int64  `json:"last_latency_ms"`
	CreatedAt              string  `json:"created_at"`
	UpdatedAt              string  `json:"updated_at"`
}

// safeView 由持久化 Key 与运行态构造安全视图。
func safeView(k Key, st KeyState) SafeKey {
	return SafeKey{
		ID:                     k.ID,
		Name:                   k.Name,
		MaskedKey:              MaskKey(k.APIKey),
		Fingerprint:            Fingerprint(k.APIKey),
		Model:                  k.Model,
		Enabled:                k.Enabled,
		Priority:               k.Priority,
		Weight:                 k.Weight,
		TimeoutSeconds:         k.TimeoutSeconds,
		CooldownSeconds:        k.CooldownSeconds,
		MaxConsecutiveFailures: k.MaxConsecutiveFailures,
		Status:                 st.Status,
		ConsecutiveFailures:    st.ConsecutiveFailures,
		LastErrorCode:          st.LastErrorCode,
		LastSuccessAt:          formatTimePtr(st.LastSuccessAt),
		LastFailureAt:          formatTimePtr(st.LastFailureAt),
		CooldownUntil:          formatTimePtr(st.CooldownUntil),
		LastLatencyMs:          st.LastLatencyMs,
		CreatedAt:              k.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:              k.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func formatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}
