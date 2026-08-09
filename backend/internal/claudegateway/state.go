package claudegateway

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sys/unix"
)

// Key 运行态词汇（契约 ClaudeKey.status 枚举）。
const (
	StatusUnknown     = "unknown"
	StatusHealthy     = "healthy"
	StatusCoolingDown = "cooling_down"
	StatusAuthFailed  = "auth_failed"
	StatusDisabled    = "disabled"
	StatusTesting     = "testing"
)

// KeyState 是单个 Key 的非敏感运行态（持久化在 state.json）。
// Fingerprint 是内部配置代际指纹，与保持 token-only 的
// SafeKey.Fingerprint 不同；该结构绝不包含密钥材料。
type KeyState struct {
	Status              string     `json:"status"`
	Fingerprint         string     `json:"fingerprint,omitempty"`
	ConsecutiveFailures int        `json:"consecutive_failures"`
	LastSuccessAt       *time.Time `json:"last_success_at,omitempty"`
	LastFailureAt       *time.Time `json:"last_failure_at,omitempty"`
	LastErrorCode       *string    `json:"last_error_code,omitempty"`
	CooldownUntil       *time.Time `json:"cooldown_until,omitempty"`
	LastLatencyMs       *int64     `json:"last_latency_ms,omitempty"`
	TestingAttemptID    string     `json:"testing_attempt_id,omitempty"`
}

// stateFile 是 state.json 的磁盘形状。
type stateFile struct {
	Keys map[string]KeyState `json:"keys"`
}

// StateTracker 维护 per-key 运行态：内存副本 + state.json 原子持久化
// （0600）。读侧在文件 mtime/size 变化时重载，因此 API 与 Worker 两个
// 进程都能看到对方的最新写入。持久化失败只记日志，不阻断路由。
type StateTracker struct {
	path          string
	now           func() time.Time
	logger        *slog.Logger
	beforePersist func()
	writeFile     func(string, []byte) error

	mu       sync.Mutex
	states   map[string]KeyState
	modTime  time.Time
	size     int64
	fileInfo os.FileInfo
	loaded   bool
}

// NewStateTracker 创建运行态跟踪器；state.json 缺失/损坏时从空状态开始。
func NewStateTracker(path string, logger *slog.Logger) *StateTracker {
	return &StateTracker{path: path, now: time.Now, logger: logger}
}

// Now 返回当前时间（测试可注入）。
func (t *StateTracker) Now() time.Time { return t.now() }

// reloadLocked 在文件 mtime/size 变化时从磁盘重载。
func (t *StateTracker) reloadLocked() {
	info, err := os.Stat(t.path)
	if errors.Is(err, os.ErrNotExist) {
		t.states = map[string]KeyState{}
		t.modTime = time.Time{}
		t.size = 0
		t.fileInfo = nil
		t.loaded = true
		return
	}
	if err != nil {
		t.warn("claude_gateway_state_load_failed", err)
		if t.states == nil {
			t.states = map[string]KeyState{}
		}
		t.loaded = true
		return
	}
	if t.loaded && t.fileInfo != nil && os.SameFile(info, t.fileInfo) &&
		info.ModTime().Equal(t.modTime) && info.Size() == t.size {
		return
	}
	raw, err := os.ReadFile(t.path)
	if err != nil {
		t.warn("claude_gateway_state_load_failed", err)
		if t.states == nil {
			t.states = map[string]KeyState{}
		}
		t.loaded = true
		return
	}
	var f stateFile
	if err := json.Unmarshal(raw, &f); err != nil || f.Keys == nil {
		if err != nil {
			t.warn("claude_gateway_state_parse_failed", err)
		}
		f.Keys = map[string]KeyState{}
	}
	t.states = f.Keys
	t.modTime = info.ModTime()
	t.size = info.Size()
	t.fileInfo = info
	t.loaded = true
}

// persistMutationLocked 跨进程序列化一次单 Key 状态变更：持有文件锁后
// 强制重载最新 state.json，合并本次变更，再原子写回。
func (t *StateTracker) persistMutationLocked(id string, mutate func(KeyState, bool) (KeyState, bool)) {
	_ = t.persistMutation(id, mutate, false)
}

// persistRequiredMutationLocked is the fail-closed variant used for targeted
// test ownership. A failed write rolls back the in-memory mutation so disk
// remains the single authority for attempt matching across processes.
func (t *StateTracker) persistRequiredMutationLocked(id string, mutate func(KeyState, bool) (KeyState, bool)) error {
	return t.persistMutation(id, mutate, true)
}

func (t *StateTracker) persistMutation(
	id string,
	mutate func(KeyState, bool) (KeyState, bool),
	rollbackOnFailure bool,
) error {
	if t.beforePersist != nil {
		t.beforePersist()
	}
	unlock, err := t.lockFile()
	if err != nil {
		t.warn("claude_gateway_state_lock_failed", err)
		return err
	}
	defer unlock()

	// 锁内必须绕过 mtime/size 缓存，合并其他进程刚提交的状态。
	t.states = nil
	t.modTime = time.Time{}
	t.size = 0
	t.loaded = false
	t.reloadLocked()
	current, exists := t.states[id]
	next, keep := mutate(current, exists)
	if exists == keep && (!exists || reflect.DeepEqual(current, next)) {
		return nil
	}
	if !keep {
		delete(t.states, id)
	} else {
		t.states[id] = next
	}
	if err := t.persistLocked(); err != nil {
		if rollbackOnFailure {
			if exists {
				t.states[id] = current
			} else {
				delete(t.states, id)
			}
		}
		return err
	}
	return nil
}

// persistLocked 原子写入当前 state.json（调用方已持有进程间文件锁）。
func (t *StateTracker) persistLocked() error {
	raw, err := json.MarshalIndent(stateFile{Keys: t.states}, "", "  ")
	if err != nil {
		t.warn("claude_gateway_state_marshal_failed", err)
		return err
	}
	writeFile := writeFileAtomic
	if t.writeFile != nil {
		writeFile = t.writeFile
	}
	if err := writeFile(t.path, raw); err != nil {
		t.warn("claude_gateway_state_persist_failed", err)
		return err
	}
	if info, err := os.Stat(t.path); err == nil {
		t.modTime = info.ModTime()
		t.size = info.Size()
		t.fileInfo = info
	}
	return nil
}

func (t *StateTracker) lockFile() (func(), error) {
	if err := os.MkdirAll(filepath.Dir(t.path), 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(t.path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, err
	}
	return func() {
		_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
		_ = f.Close()
	}, nil
}

func (t *StateTracker) warn(msg string, err error) {
	if t.logger != nil {
		t.logger.Warn(msg, slog.String("error", err.Error()))
	}
}

// effectiveLocked 计算合并配置后的展示状态：禁用来自配置；
// 过期的 cooling_down 视为 unknown（等待下一次真实调用判定）。
func (t *StateTracker) effectiveLocked(k Key, now time.Time) KeyState {
	st, ok := t.states[k.ID]
	if !ok {
		st = KeyState{Status: StatusUnknown}
	}
	if fingerprint := keyStateFingerprint(k); fingerprint != "" && st.Fingerprint != fingerprint {
		st = KeyState{Status: StatusUnknown}
	}
	if st.Status == "" {
		st.Status = StatusUnknown
	}
	if !k.Enabled {
		st.Status = StatusDisabled
		return st
	}
	if st.Status == StatusCoolingDown && st.CooldownUntil != nil && !now.Before(*st.CooldownUntil) {
		st.Status = StatusUnknown
		st.CooldownUntil = nil
	}
	return st
}

func cooldownExpired(st KeyState, now time.Time) bool {
	return st.Status == StatusCoolingDown && st.CooldownUntil != nil && !now.Before(*st.CooldownUntil)
}

// Snapshot 返回合并配置后的运行态视图（API 列表用）。
func (t *StateTracker) Snapshot(k Key) KeyState {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.reloadLocked()
	now := t.now()
	fingerprint := keyStateFingerprint(k)
	if st, ok := t.states[k.ID]; ok && fingerprint != "" && st.Fingerprint != fingerprint {
		t.persistMutationLocked(k.ID, func(current KeyState, exists bool) (KeyState, bool) {
			if exists && current.Fingerprint != fingerprint {
				return KeyState{}, false
			}
			return current, exists
		})
	} else if ok && cooldownExpired(st, now) {
		t.persistMutationLocked(k.ID, func(current KeyState, exists bool) (KeyState, bool) {
			if exists && cooldownExpired(current, now) {
				current.Status = StatusUnknown
				current.CooldownUntil = nil
			}
			return current, exists
		})
	}
	return t.effectiveLocked(k, now)
}

// Eligible 报告 Key 当前是否可参与路由：启用 + 非 auth_failed +
// 非冷却中。unknown/healthy/testing 均可参与。
func (t *StateTracker) Eligible(k Key, now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.reloadLocked()
	if !k.Enabled {
		return false
	}
	st := t.effectiveLocked(k, now)
	switch st.Status {
	case StatusAuthFailed, StatusCoolingDown, StatusDisabled:
		return false
	default:
		return true
	}
}

// MarkSuccess 记录一次成功调用：healthy，清零连续失败，清除冷却。
func (t *StateTracker) MarkSuccess(id string, latency time.Duration) {
	t.markSuccess(Key{ID: id}, latency)
}

func (t *StateTracker) MarkSuccessForKey(k Key, latency time.Duration) {
	t.markSuccess(k, latency)
}

func (t *StateTracker) markSuccess(k Key, latency time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now().UTC()
	latencyMs := latency.Milliseconds()
	t.persistMutationLocked(k.ID, func(st KeyState, exists bool) (KeyState, bool) {
		if hasDifferentFingerprint(st, exists, k) {
			return st, true
		}
		return successState(k, now, latencyMs), true
	})
}

// MarkAuthFailed 记录 401/403：进入 auth_failed，不自动恢复
// （等待用户更新 Key 或手动测试复位）。
func (t *StateTracker) MarkAuthFailed(id, code string, latency time.Duration) {
	t.markAuthFailed(Key{ID: id}, code, latency)
}

func (t *StateTracker) MarkAuthFailedForKey(k Key, code string, latency time.Duration) {
	t.markAuthFailed(k, code, latency)
}

func (t *StateTracker) markAuthFailed(k Key, code string, latency time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now().UTC()
	latencyMs := latency.Milliseconds()
	t.persistMutationLocked(k.ID, func(st KeyState, exists bool) (KeyState, bool) {
		if hasDifferentFingerprint(st, exists, k) {
			return st, true
		}
		return authFailedState(st, k, code, now, latencyMs), true
	})
}

// MarkTransientFailure 记录可故障转移的瞬时失败（429/5xx/超时/网络错误）：
// 连续失败达到 maxConsecutiveFailures 后进入 cooling_down，冷却
// cooldownSeconds 秒。
func (t *StateTracker) MarkTransientFailure(k Key, code string, latency time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now().UTC()
	latencyMs := latency.Milliseconds()
	t.persistMutationLocked(k.ID, func(st KeyState, exists bool) (KeyState, bool) {
		if hasDifferentFingerprint(st, exists, k) {
			return st, true
		}
		return transientFailureState(st, k, code, now, latencyMs), true
	})
}

// MarkNonFailoverFailure 记录不触发故障转移的失败（refusal/max_tokens/
// invalid_json 等）：只更新诊断字段，不改变路由状态。
func (t *StateTracker) MarkNonFailoverFailure(id, code string, latency time.Duration) {
	t.markNonFailoverFailure(Key{ID: id}, code, latency)
}

func (t *StateTracker) MarkNonFailoverFailureForKey(k Key, code string, latency time.Duration) {
	t.markNonFailoverFailure(k, code, latency)
}

func (t *StateTracker) markNonFailoverFailure(k Key, code string, latency time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now().UTC()
	latencyMs := latency.Milliseconds()
	t.persistMutationLocked(k.ID, func(st KeyState, exists bool) (KeyState, bool) {
		if hasDifferentFingerprint(st, exists, k) {
			return st, true
		}
		st = stateForKey(st, k)
		if st.TestingAttemptID != "" {
			st.TestingAttemptID = ""
			if st.Status == StatusTesting {
				st.Status = StatusUnknown
			}
		}
		if st.Status == "" {
			st.Status = StatusUnknown
		}
		st.LastFailureAt = &now
		st.LastErrorCode = &code
		st.LastLatencyMs = &latencyMs
		return st, true
	})
}

type testOutcome int

const (
	testSucceeded testOutcome = iota
	testAuthFailed
	testTransientFailure
	testNonFailoverFailure
	testCancelled
)

// beginTestForKey persists a unique attempt token so only this targeted test's
// terminal result may replace its testing state.
func (t *StateTracker) beginTestForKey(k Key) (string, error) {
	attemptID := uuid.NewString()
	t.mu.Lock()
	defer t.mu.Unlock()
	err := t.persistRequiredMutationLocked(k.ID, func(st KeyState, _ bool) (KeyState, bool) {
		st = stateForKey(st, k)
		st.Status = StatusTesting
		st.ConsecutiveFailures = 0
		st.TestingAttemptID = attemptID
		return st, true
	})
	if err != nil {
		return "", err
	}
	return attemptID, nil
}

// finishTestForKey atomically applies a targeted terminal outcome only while
// both the key fingerprint and persisted attempt token still match. A normal
// route outcome or newer targeted test makes an older completion a no-op.
func (t *StateTracker) finishTestForKey(
	k Key,
	attemptID string,
	outcome testOutcome,
	code string,
	latency time.Duration,
) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now().UTC()
	latencyMs := latency.Milliseconds()
	fingerprint := keyStateFingerprint(k)
	return t.persistRequiredMutationLocked(k.ID, func(st KeyState, exists bool) (KeyState, bool) {
		if !exists || attemptID == "" || st.Fingerprint != fingerprint || st.TestingAttemptID != attemptID {
			return st, exists
		}
		switch outcome {
		case testSucceeded:
			st = successState(k, now, latencyMs)
		case testAuthFailed:
			st = authFailedState(st, k, code, now, latencyMs)
		case testTransientFailure:
			st = transientFailureState(st, k, code, now, latencyMs)
		case testNonFailoverFailure:
			st.Status = StatusUnknown
			st.ConsecutiveFailures = 0
			st.CooldownUntil = nil
			st.LastFailureAt = &now
			st.LastErrorCode = &code
			st.LastLatencyMs = &latencyMs
			st.TestingAttemptID = ""
		case testCancelled:
			st.Status = StatusUnknown
			st.ConsecutiveFailures = 0
			st.CooldownUntil = nil
			st.TestingAttemptID = ""
		}
		return st, true
	})
}

func successState(k Key, now time.Time, latencyMs int64) KeyState {
	return KeyState{
		Status:        StatusHealthy,
		Fingerprint:   keyStateFingerprint(k),
		LastSuccessAt: &now,
		LastLatencyMs: &latencyMs,
	}
}

func authFailedState(st KeyState, k Key, code string, now time.Time, latencyMs int64) KeyState {
	st = stateForKey(st, k)
	st.Status = StatusAuthFailed
	st.LastFailureAt = &now
	st.LastErrorCode = &code
	st.LastLatencyMs = &latencyMs
	st.TestingAttemptID = ""
	return st
}

func transientFailureState(st KeyState, k Key, code string, now time.Time, latencyMs int64) KeyState {
	st = stateForKey(st, k)
	st.ConsecutiveFailures++
	st.LastFailureAt = &now
	st.LastErrorCode = &code
	st.LastLatencyMs = &latencyMs
	st.TestingAttemptID = ""
	threshold := k.MaxConsecutiveFailures
	if threshold <= 0 {
		threshold = DefaultMaxConsecutiveFailures
	}
	if st.ConsecutiveFailures >= threshold {
		cooldown := k.CooldownSeconds
		if cooldown <= 0 {
			cooldown = DefaultCooldownSeconds
		}
		until := now.Add(time.Duration(cooldown) * time.Second)
		st.Status = StatusCoolingDown
		st.CooldownUntil = &until
	} else if st.Status == "" || st.Status == StatusHealthy || st.Status == StatusTesting {
		st.Status = StatusUnknown
	}
	return st
}

// Reset 清除运行态（回到 unknown；禁用展示由配置侧推导）。
// API 侧换 Key 或改 enabled 后调用，使旧的 auth_failed/cooling_down 失效。
func (t *StateTracker) Reset(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.persistMutationLocked(id, func(KeyState, bool) (KeyState, bool) {
		return KeyState{}, false
	})
}

func keyStateFingerprint(k Key) string {
	apiKey := strings.TrimSpace(k.APIKey)
	if apiKey == "" {
		return ""
	}
	baseURL := strings.TrimSpace(k.BaseURL)
	if baseURL != "" {
		// Hash the same complete Messages endpoint representation used by
		// Router.attempt, so harmless trailing slash or base-vs-endpoint
		// spelling changes do not create a new state generation.
		baseURL = messagesEndpoint(baseURL)
	}
	material, _ := json.Marshal(struct {
		APIKey  string `json:"api_key"`
		Model   string `json:"model"`
		BaseURL string `json:"base_url"`
	}{
		APIKey:  apiKey,
		Model:   strings.TrimSpace(k.Model),
		BaseURL: baseURL,
	})
	sum := sha256.Sum256(material)
	return "config-sha256:" + hex.EncodeToString(sum[:])
}

func hasDifferentFingerprint(st KeyState, exists bool, k Key) bool {
	fingerprint := keyStateFingerprint(k)
	return exists && fingerprint != "" && st.Fingerprint != fingerprint
}

func stateForKey(st KeyState, k Key) KeyState {
	fingerprint := keyStateFingerprint(k)
	if fingerprint != "" && st.Fingerprint != fingerprint {
		st = KeyState{}
	}
	if fingerprint != "" {
		st.Fingerprint = fingerprint
	}
	return st
}
