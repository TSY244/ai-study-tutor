package claudegateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/backend/internal/apitest"
	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
	"github.com/TSY244/ai-study-tutor/backend/internal/idempotency"
	"github.com/TSY244/ai-study-tutor/backend/internal/job"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/databasetest"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/httpserver"
)

func TestMain(m *testing.M) { databasetest.Main(m) }

type gatewayHandlerEnv struct {
	pool   *pgxpool.Pool
	engine http.Handler
	store  *Store
}

func newGatewayHandlerEnv(t *testing.T, writeEnabled bool) gatewayHandlerEnv {
	return newGatewayHandlerEnvWithClaudeCodeSettings(t, writeEnabled, "")
}

func newGatewayHandlerEnvWithClaudeCodeSettings(t *testing.T, writeEnabled bool, settingsFile string) gatewayHandlerEnv {
	t.Helper()
	pool := databasetest.New(t)
	engine := apitest.NewEngine(t)
	path := filepath.Join(t.TempDir(), "keys.json")
	store := NewStore(path)
	h := &Handler{
		Pool: pool, Store: store, State: NewStateTracker(StatePathFor(path), nil),
		Jobs: job.NewService(pool), WriteEnabled: writeEnabled, ClaudeCodeSettingsFile: settingsFile,
	}
	h.RegisterRoutes(engine.Group("/api/v1"),
		httpserver.BodyLimit(1<<20, http.StatusBadRequest, "bad_request"),
		&httpx.IdemExecutor{Svc: idempotency.NewService(pool)})
	return gatewayHandlerEnv{pool: pool, engine: engine, store: store}
}

func TestClaudeCodeSettingsRouteOverwritesSelectedKeyWithoutResponseSecret(t *testing.T) {
	settingsFile := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(settingsFile, []byte(`{"previous":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(settingsFile)
	if err != nil {
		t.Fatal(err)
	}
	env := newGatewayHandlerEnvWithClaudeCodeSettings(t, true, settingsFile)
	secret := strings.Join([]string{"runtime", "claude-code", uuid.NewString()}, "-")
	key, err := env.store.Create(Key{Name: "Export me", APIKey: secret, Model: "claude-test", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	rec := apitest.Do(t, env.engine, http.MethodPost,
		"/api/v1/system/claude-keys/"+key.ID+"/claude-code-settings", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("export status=%d body=%s", rec.Code, rec.Body.String())
	}
	assertSafeGatewayResponse(t, rec.Body.Bytes(), secret)
	var body struct {
		Status string `json:"status"`
		Target string `json:"target"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "saved" || body.Target != "~/.claude/settings.json" {
		t.Fatalf("unexpected safe response: %+v", body)
	}
	// 导出确认也必须持续符合 OpenAPI，且不包含任何密钥字段。
	apiTest := apitest.Load(t)
	apiTest.ValidateJSONResponse(t, http.MethodPost,
		"/api/v1/system/claude-keys/{key_id}/claude-code-settings", http.StatusOK, rec.Body.Bytes())

	after, err := os.Stat(settingsFile)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(before, after) {
		t.Fatal("Claude Code settings must be atomically replaced")
	}
	if got := after.Mode().Perm(); got != 0o600 {
		t.Fatalf("settings.json permissions=%o want 600", got)
	}
	raw, err := os.ReadFile(settingsFile)
	if err != nil {
		t.Fatal(err)
	}
	var saved struct {
		Schema string            `json:"$schema"`
		Env    map[string]string `json:"env"`
	}
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Schema != "https://json.schemastore.org/claude-code-settings.json" ||
		saved.Env["ANTHROPIC_BASE_URL"] != "https://app.haoshuai.cc.cd" ||
		saved.Env["ANTHROPIC_AUTH_TOKEN"] != secret ||
		saved.Env["CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC"] != "1" ||
		saved.Env["CLAUDE_CODE_ATTRIBUTION_HEADER"] != "0" {
		t.Fatalf("saved Claude Code settings are not the requested document: %+v", saved)
	}
}

func TestClaudeCodeSettingsRouteRejectsUnavailableTargetAndWriteDisabled(t *testing.T) {
	blockedParent := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blockedParent, []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	secret := strings.Join([]string{"runtime", "claude-code-rejected", uuid.NewString()}, "-")

	env := newGatewayHandlerEnvWithClaudeCodeSettings(t, true, filepath.Join(blockedParent, "settings.json"))
	key, err := env.store.Create(Key{Name: "Unavailable", APIKey: secret, Model: "claude-test", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	failed := apitest.Do(t, env.engine, http.MethodPost,
		"/api/v1/system/claude-keys/"+key.ID+"/claude-code-settings", nil, "")
	if failed.Code != http.StatusServiceUnavailable {
		t.Fatalf("unavailable target status=%d body=%s", failed.Code, failed.Body.String())
	}
	assertSafeGatewayResponse(t, failed.Body.Bytes(), secret)

	missing := apitest.Do(t, env.engine, http.MethodPost,
		"/api/v1/system/claude-keys/ck_missing/claude-code-settings", nil, "")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing key status=%d body=%s", missing.Code, missing.Body.String())
	}
	assertSafeGatewayResponse(t, missing.Body.Bytes(), secret)

	readOnly := newGatewayHandlerEnvWithClaudeCodeSettings(t, false, filepath.Join(t.TempDir(), "settings.json"))
	readOnlyKey, err := readOnly.store.Create(Key{Name: "Read-only", APIKey: secret, Model: "claude-test", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	blocked := apitest.Do(t, readOnly.engine, http.MethodPost,
		"/api/v1/system/claude-keys/"+readOnlyKey.ID+"/claude-code-settings", nil, "")
	if blocked.Code != http.StatusForbidden {
		t.Fatalf("write-disabled status=%d body=%s", blocked.Code, blocked.Body.String())
	}
	assertSafeGatewayResponse(t, blocked.Body.Bytes(), secret)
}

func TestClaudeKeyManagementLifecycleNeverReturnsSecret(t *testing.T) {
	env := newGatewayHandlerEnv(t, true)
	secret := strings.Join([]string{"runtime", "anthropic", "credential", uuid.NewString()}, "-")
	requestBody, err := json.Marshal(map[string]any{
		"api_key": secret, "name": "Primary", "model": "claude-test", "enabled": true,
		"priority": 5, "weight": 25, "timeout_seconds": 45, "cooldown_seconds": 90,
		"max_consecutive_failures": 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	create := apitest.Do(t, env.engine, http.MethodPost, "/api/v1/system/claude-keys",
		map[string]string{"Idempotency-Key": uuid.NewString()}, string(requestBody))
	if create.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", create.Code, create.Body.String())
	}
	assertSafeGatewayResponse(t, create.Body.Bytes(), secret)
	apitest.Load(t).ValidateJSONResponse(t, http.MethodPost, "/api/v1/system/claude-keys", http.StatusCreated, create.Body.Bytes())
	var created SafeKey
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.MaskedKey == "" || created.Fingerprint == "" || created.Status != StatusUnknown {
		t.Fatalf("创建响应安全视图不完整: %+v", created)
	}
	if created.TimeoutSeconds != 45 || created.CooldownSeconds != 90 || created.MaxConsecutiveFailures != 3 {
		t.Fatalf("创建响应缺少可编辑路由参数: %+v", created)
	}

	list := apitest.Do(t, env.engine, http.MethodGet, "/api/v1/system/claude-keys", nil, "")
	if list.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}
	assertSafeGatewayResponse(t, list.Body.Bytes(), secret)
	apitest.Load(t).ValidateJSONResponse(t, http.MethodGet, "/api/v1/system/claude-keys", http.StatusOK, list.Body.Bytes())

	patch := apitest.Do(t, env.engine, http.MethodPatch, "/api/v1/system/claude-keys/"+created.ID,
		nil, `{"name":"Renamed","weight":40}`)
	if patch.Code != http.StatusOK {
		t.Fatalf("patch status=%d body=%s", patch.Code, patch.Body.String())
	}
	assertSafeGatewayResponse(t, patch.Body.Bytes(), secret)
	apitest.Load(t).ValidateJSONResponse(t, http.MethodPatch, "/api/v1/system/claude-keys/{key_id}", http.StatusOK, patch.Body.Bytes())
	persisted, err := env.store.Get(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.APIKey != secret || persisted.Name != "Renamed" || persisted.Weight != 40 {
		t.Fatal("省略 api_key 的 PATCH 必须保留原密钥并更新非敏感字段")
	}

	testJob := apitest.Do(t, env.engine, http.MethodPost, "/api/v1/system/claude-keys/"+created.ID+"/test",
		map[string]string{"Idempotency-Key": uuid.NewString()}, "")
	if testJob.Code != http.StatusAccepted {
		t.Fatalf("test status=%d body=%s", testJob.Code, testJob.Body.String())
	}
	assertSafeGatewayResponse(t, testJob.Body.Bytes(), secret)
	apitest.Load(t).ValidateJSONResponse(t, http.MethodPost, "/api/v1/system/claude-keys/{key_id}/test", http.StatusAccepted, testJob.Body.Bytes())
	var accepted struct {
		Job struct {
			ID string `json:"id"`
		} `json:"job"`
	}
	if err := json.Unmarshal(testJob.Body.Bytes(), &accepted); err != nil {
		t.Fatal(err)
	}
	var payload []byte
	if err := env.pool.QueryRow(context.Background(), `SELECT payload FROM jobs WHERE id=$1`, accepted.Job.ID).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	var target map[string]string
	if err := json.Unmarshal(payload, &target); err != nil || target["key_id"] != created.ID || len(target) != 1 {
		t.Fatalf("单 Key 测试 payload 不正确: %s err=%v", payload, err)
	}

	deleted := apitest.Do(t, env.engine, http.MethodDelete, "/api/v1/system/claude-keys/"+created.ID, nil, "")
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete status=%d body=%s", deleted.Code, deleted.Body.String())
	}
	if _, err := env.store.Get(created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("删除后仍能读取 Key: %v", err)
	}
}

func TestClaudeKeyBaseURLCreateUpdateAndList(t *testing.T) {
	env := newGatewayHandlerEnv(t, true)
	secret := strings.Join([]string{"runtime", "base-url", uuid.NewString()}, "-")

	// 创建：合法 base_url 去尾 / 后持久化并出现在安全视图。
	body, err := json.Marshal(map[string]any{
		"api_key": secret, "name": "Gateway A", "model": "claude-test",
		"base_url": "https://gw1.example/",
	})
	if err != nil {
		t.Fatal(err)
	}
	create := apitest.Do(t, env.engine, http.MethodPost, "/api/v1/system/claude-keys",
		map[string]string{"Idempotency-Key": uuid.NewString()}, string(body))
	if create.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", create.Code, create.Body.String())
	}
	assertSafeGatewayResponse(t, create.Body.Bytes(), secret)
	apitest.Load(t).ValidateJSONResponse(t, http.MethodPost, "/api/v1/system/claude-keys", http.StatusCreated, create.Body.Bytes())
	var created SafeKey
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.BaseURL != "https://gw1.example" {
		t.Fatalf("创建响应 base_url 应去尾 /: %q", created.BaseURL)
	}

	// 创建：非 https 或含 userinfo 的 base_url 一律 400。
	for _, raw := range []string{"http://insecure.example", "https://user:pw@gw.example"} {
		badBody, err := json.Marshal(map[string]any{
			"api_key": secret, "name": "Bad", "model": "claude-test", "base_url": raw,
		})
		if err != nil {
			t.Fatal(err)
		}
		bad := apitest.Do(t, env.engine, http.MethodPost, "/api/v1/system/claude-keys",
			map[string]string{"Idempotency-Key": uuid.NewString()}, string(badBody))
		if bad.Code != http.StatusBadRequest {
			t.Fatalf("base_url=%q status=%d body=%s", raw, bad.Code, bad.Body.String())
		}
		assertSafeGatewayResponse(t, bad.Body.Bytes(), secret)
	}

	// 更新：非空设置生效；省略保持不变；空串清除。
	patch := apitest.Do(t, env.engine, http.MethodPatch, "/api/v1/system/claude-keys/"+created.ID,
		nil, `{"base_url":"https://gw2.example"}`)
	if patch.Code != http.StatusOK {
		t.Fatalf("patch set status=%d body=%s", patch.Code, patch.Body.String())
	}
	assertSafeGatewayResponse(t, patch.Body.Bytes(), secret)
	apitest.Load(t).ValidateJSONResponse(t, http.MethodPatch, "/api/v1/system/claude-keys/{key_id}", http.StatusOK, patch.Body.Bytes())
	var patched SafeKey
	if err := json.Unmarshal(patch.Body.Bytes(), &patched); err != nil {
		t.Fatal(err)
	}
	if patched.BaseURL != "https://gw2.example" {
		t.Fatalf("更新响应 base_url 未生效: %q", patched.BaseURL)
	}

	patch = apitest.Do(t, env.engine, http.MethodPatch, "/api/v1/system/claude-keys/"+created.ID,
		nil, `{"name":"Renamed"}`)
	if patch.Code != http.StatusOK {
		t.Fatalf("patch keep status=%d body=%s", patch.Code, patch.Body.String())
	}
	if err := json.Unmarshal(patch.Body.Bytes(), &patched); err != nil {
		t.Fatal(err)
	}
	if patched.BaseURL != "https://gw2.example" {
		t.Fatalf("省略 base_url 的 PATCH 不得改动该字段: %q", patched.BaseURL)
	}

	patch = apitest.Do(t, env.engine, http.MethodPatch, "/api/v1/system/claude-keys/"+created.ID,
		nil, `{"base_url":""}`)
	if patch.Code != http.StatusOK {
		t.Fatalf("patch clear status=%d body=%s", patch.Code, patch.Body.String())
	}
	if err := json.Unmarshal(patch.Body.Bytes(), &patched); err != nil {
		t.Fatal(err)
	}
	if patched.BaseURL != "" {
		t.Fatalf("空串 PATCH 应清除 base_url: %q", patched.BaseURL)
	}
	persisted, err := env.store.Get(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.BaseURL != "" {
		t.Fatalf("清除后持久化的 base_url 应为空: %q", persisted.BaseURL)
	}

	badPatch := apitest.Do(t, env.engine, http.MethodPatch, "/api/v1/system/claude-keys/"+created.ID,
		nil, `{"base_url":"http://insecure.example"}`)
	if badPatch.Code != http.StatusBadRequest {
		t.Fatalf("非法 base_url 的 PATCH status=%d body=%s", badPatch.Code, badPatch.Body.String())
	}
	assertSafeGatewayResponse(t, badPatch.Body.Bytes(), secret)

	// 列表响应同样携带 base_url。
	list := apitest.Do(t, env.engine, http.MethodGet, "/api/v1/system/claude-keys", nil, "")
	if list.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}
	assertSafeGatewayResponse(t, list.Body.Bytes(), secret)
	apitest.Load(t).ValidateJSONResponse(t, http.MethodGet, "/api/v1/system/claude-keys", http.StatusOK, list.Body.Bytes())
	var listed struct {
		Items []SafeKey `json:"items"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range listed.Items {
		if item.ID == created.ID {
			found = true
			if item.BaseURL != "" {
				t.Fatalf("列表响应 base_url 应为空（已清除）: %q", item.BaseURL)
			}
		}
	}
	if !found {
		t.Fatal("列表响应缺少刚创建的 Key")
	}
}

func TestWriteClaudeCodeSettingsBaseURLOverride(t *testing.T) {
	readEnv := func(t *testing.T, path string) map[string]string {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var saved struct {
			Env map[string]string `json:"env"`
		}
		if err := json.Unmarshal(raw, &saved); err != nil {
			t.Fatal(err)
		}
		return saved.Env
	}

	// Key 有 base_url 时写入该值。
	custom := filepath.Join(t.TempDir(), "settings.json")
	if err := writeClaudeCodeSettings(custom, Key{APIKey: "sk-ant-unit", BaseURL: "https://gw1.example"}); err != nil {
		t.Fatal(err)
	}
	if got := readEnv(t, custom)["ANTHROPIC_BASE_URL"]; got != "https://gw1.example" {
		t.Fatalf("Key 有 base_url 时应覆盖导出: %q", got)
	}

	// Key 无 base_url 时回退既有硬编码默认值。
	fallback := filepath.Join(t.TempDir(), "settings.json")
	if err := writeClaudeCodeSettings(fallback, Key{APIKey: "sk-ant-unit"}); err != nil {
		t.Fatal(err)
	}
	if got := readEnv(t, fallback)["ANTHROPIC_BASE_URL"]; got != "https://app.haoshuai.cc.cd" {
		t.Fatalf("Key 无 base_url 时应保留默认值: %q", got)
	}
}

func TestClaudeKeyWritesCanBeDisabledAndErrorsNeverEchoSecret(t *testing.T) {
	env := newGatewayHandlerEnv(t, false)
	secret := strings.Join([]string{"runtime", "rejected", uuid.NewString()}, "-")
	body, err := json.Marshal(map[string]any{"api_key": secret, "name": "Blocked", "model": "claude-test"})
	if err != nil {
		t.Fatal(err)
	}
	blocked := apitest.Do(t, env.engine, http.MethodPost, "/api/v1/system/claude-keys",
		map[string]string{"Idempotency-Key": uuid.NewString()}, string(body))
	if blocked.Code != http.StatusForbidden {
		t.Fatalf("write-disabled status=%d body=%s", blocked.Code, blocked.Body.String())
	}
	assertSafeGatewayResponse(t, blocked.Body.Bytes(), secret)

	env = newGatewayHandlerEnv(t, true)
	invalidBody, err := json.Marshal(map[string]any{
		"api_key": secret, "name": "Invalid", "model": "claude-test", "weight": 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	invalid := apitest.Do(t, env.engine, http.MethodPost, "/api/v1/system/claude-keys",
		map[string]string{"Idempotency-Key": uuid.NewString()}, string(invalidBody))
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid status=%d body=%s", invalid.Code, invalid.Body.String())
	}
	assertSafeGatewayResponse(t, invalid.Body.Bytes(), secret)
}

func assertSafeGatewayResponse(t *testing.T, body []byte, secret string) {
	t.Helper()
	if strings.Contains(string(body), secret) {
		t.Fatal("Claude 网关响应泄露了完整密钥")
	}
	var decoded any
	if err := json.Unmarshal(body, &decoded); err == nil && containsJSONField(decoded, "api_key") {
		t.Fatal("Claude 网关响应包含 api_key 字段")
	}
}

func containsJSONField(value any, field string) bool {
	switch value := value.(type) {
	case map[string]any:
		for key, child := range value {
			if key == field || containsJSONField(child, field) {
				return true
			}
		}
	case []any:
		for _, child := range value {
			if containsJSONField(child, field) {
				return true
			}
		}
	}
	return false
}
