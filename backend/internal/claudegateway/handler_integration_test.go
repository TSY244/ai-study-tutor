package claudegateway

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	aipkg "github.com/TSY244/ai-study-tutor/backend/internal/ai"
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
	state  *StateTracker
}

func newGatewayHandlerEnv(t *testing.T, writeEnabled bool) gatewayHandlerEnv {
	return newGatewayHandlerEnvWithClaudeCodeSettings(t, writeEnabled, "")
}

func newGatewayHandlerEnvWithClaudeCodeSettings(t *testing.T, writeEnabled bool, settingsFile string) gatewayHandlerEnv {
	return newGatewayHandlerEnvWithClaudeCodeSettingsAndDefaultBaseURL(t, writeEnabled, settingsFile, "")
}

func newGatewayHandlerEnvWithClaudeCodeSettingsAndDefaultBaseURL(
	t *testing.T,
	writeEnabled bool,
	settingsFile string,
	defaultBaseURL string,
) gatewayHandlerEnv {
	t.Helper()
	pool := databasetest.New(t)
	engine := apitest.NewEngine(t)
	path := filepath.Join(t.TempDir(), "keys.json")
	store := NewStore(path)
	state := NewStateTracker(StatePathFor(path), nil)
	h := &Handler{
		Pool: pool, Store: store, State: state,
		Jobs: job.NewService(pool), WriteEnabled: writeEnabled, ClaudeCodeSettingsFile: settingsFile,
		DefaultBaseURL: defaultBaseURL,
	}
	h.RegisterRoutes(engine.Group("/api/v1"),
		httpserver.BodyLimit(1<<20, http.StatusBadRequest, "bad_request"),
		&httpx.IdemExecutor{Svc: idempotency.NewService(pool)})
	return gatewayHandlerEnv{pool: pool, engine: engine, store: store, state: state}
}

func TestClaudeCodeSettingsRouteOverwritesSelectedKeyWithoutResponseSecret(t *testing.T) {
	settingsDir := filepath.Join(t.TempDir(), ".claude")
	settingsFile := filepath.Join(settingsDir, "settings.json")
	env := newGatewayHandlerEnvWithClaudeCodeSettings(t, true, settingsFile)
	secret := strings.Join([]string{"runtime", "claude-code", uuid.NewString()}, "-")
	key, err := env.store.Create(Key{
		Name:    "DeepSeek runtime",
		APIKey:  secret,
		Model:   "deepseek-chat",
		BaseURL: "https://api.deepseek.com/anthropic",
		Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	created := apitest.Do(t, env.engine, http.MethodPost,
		"/api/v1/system/claude-keys/"+key.ID+"/claude-code-settings", nil, "")
	assertSafeGatewayResponse(t, created.Body.Bytes(), secret)
	if created.Code != http.StatusOK {
		t.Fatalf("initial export status=%d body=%s", created.Code, created.Body.String())
	}
	if info, err := os.Stat(settingsDir); err != nil {
		t.Fatal(err)
	} else if got := info.Mode().Perm(); got != 0o700 {
		t.Fatalf("new .claude directory permissions=%o want 700", got)
	}
	if err := os.WriteFile(settingsFile, []byte(`{"previous":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(settingsFile)
	if err != nil {
		t.Fatal(err)
	}

	rec := apitest.Do(t, env.engine, http.MethodPost,
		"/api/v1/system/claude-keys/"+key.ID+"/claude-code-settings", nil, "")
	assertSafeGatewayResponse(t, rec.Body.Bytes(), secret)
	if rec.Code != http.StatusOK {
		t.Fatalf("export status=%d body=%s", rec.Code, rec.Body.String())
	}
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
	if saved.Schema != "https://json.schemastore.org/claude-code-settings.json" {
		t.Fatalf("unexpected Claude Code settings schema: %q", saved.Schema)
	}
	if saved.Env["ANTHROPIC_AUTH_TOKEN"] != secret {
		t.Fatal("saved Claude Code settings did not contain the selected credential")
	}
	wantEnv := map[string]string{
		"ANTHROPIC_BASE_URL":                       "https://api.deepseek.com/anthropic",
		"ANTHROPIC_MODEL":                          key.Model,
		"ANTHROPIC_DEFAULT_OPUS_MODEL":             key.Model,
		"ANTHROPIC_DEFAULT_SONNET_MODEL":           key.Model,
		"ANTHROPIC_DEFAULT_HAIKU_MODEL":            key.Model,
		"CLAUDE_CODE_SUBAGENT_MODEL":               key.Model,
		"AI_STUDY_TUTOR_CLAUDE_KEY_ID":             key.ID,
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
		"CLAUDE_CODE_ATTRIBUTION_HEADER":           "0",
	}
	for name, want := range wantEnv {
		if got := saved.Env[name]; got != want {
			t.Fatalf("saved Claude Code settings %s=%q want %q", name, got, want)
		}
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
	assertSafeGatewayResponse(t, failed.Body.Bytes(), secret)
	if failed.Code != http.StatusServiceUnavailable {
		t.Fatalf("unavailable target status=%d body=%s", failed.Code, failed.Body.String())
	}
	detail := failed.Body.String()
	if !strings.Contains(detail, "运行时 Claude HOME") || !strings.Contains(detail, "settings.json") {
		t.Fatalf("unavailable target detail must describe runtime Claude HOME/settings permissions: %s", detail)
	}
	if strings.Contains(detail, "宿主机") || strings.Contains(detail, "挂载") || strings.Contains(detail, blockedParent) {
		t.Fatalf("unavailable target detail exposed host-mount information: %s", detail)
	}

	missing := apitest.Do(t, env.engine, http.MethodPost,
		"/api/v1/system/claude-keys/ck_missing/claude-code-settings", nil, "")
	assertSafeGatewayResponse(t, missing.Body.Bytes(), secret)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing key status=%d body=%s", missing.Code, missing.Body.String())
	}

	readOnly := newGatewayHandlerEnvWithClaudeCodeSettings(t, false, filepath.Join(t.TempDir(), "settings.json"))
	readOnlyKey, err := readOnly.store.Create(Key{Name: "Read-only", APIKey: secret, Model: "claude-test", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	blocked := apitest.Do(t, readOnly.engine, http.MethodPost,
		"/api/v1/system/claude-keys/"+readOnlyKey.ID+"/claude-code-settings", nil, "")
	assertSafeGatewayResponse(t, blocked.Body.Bytes(), secret)
	if blocked.Code != http.StatusForbidden {
		t.Fatalf("write-disabled status=%d body=%s", blocked.Code, blocked.Body.String())
	}
}

func TestClaudeCodeSettingsRouteRejectsDisabledKeyWithoutResponseSecret(t *testing.T) {
	settingsFile := filepath.Join(t.TempDir(), ".claude", "settings.json")
	env := newGatewayHandlerEnvWithClaudeCodeSettings(t, true, settingsFile)
	secret := strings.Join([]string{"runtime", "disabled", uuid.NewString()}, "-")
	key, err := env.store.Create(Key{
		Name: "Disabled", APIKey: secret, Model: "deepseek-chat", Enabled: false,
	})
	if err != nil {
		t.Fatal(err)
	}

	rec := apitest.Do(t, env.engine, http.MethodPost,
		"/api/v1/system/claude-keys/"+key.ID+"/claude-code-settings", nil, "")
	assertSafeGatewayResponse(t, rec.Body.Bytes(), secret)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("disabled-key export status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "已禁用") {
		t.Fatalf("disabled-key response must explain rejection: %s", rec.Body.String())
	}
	if _, err := os.Stat(settingsFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("disabled key must not create settings file: %v", err)
	}
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
	assertSafeGatewayResponse(t, create.Body.Bytes(), secret)
	if create.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", create.Code, create.Body.String())
	}
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
	assertSafeGatewayResponse(t, list.Body.Bytes(), secret)
	if list.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}
	apitest.Load(t).ValidateJSONResponse(t, http.MethodGet, "/api/v1/system/claude-keys", http.StatusOK, list.Body.Bytes())

	patch := apitest.Do(t, env.engine, http.MethodPatch, "/api/v1/system/claude-keys/"+created.ID,
		nil, `{"name":"Renamed","weight":40}`)
	assertSafeGatewayResponse(t, patch.Body.Bytes(), secret)
	if patch.Code != http.StatusOK {
		t.Fatalf("patch status=%d body=%s", patch.Code, patch.Body.String())
	}
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
	assertSafeGatewayResponse(t, testJob.Body.Bytes(), secret)
	if testJob.Code != http.StatusAccepted {
		t.Fatalf("test status=%d body=%s", testJob.Code, testJob.Body.String())
	}
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
	assertSafeGatewayResponse(t, deleted.Body.Bytes(), secret)
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
	assertSafeGatewayResponse(t, create.Body.Bytes(), secret)
	if create.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", create.Code, create.Body.String())
	}
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
		assertSafeGatewayResponse(t, bad.Body.Bytes(), secret)
		if bad.Code != http.StatusBadRequest {
			t.Fatalf("base_url=%q status=%d body=%s", raw, bad.Code, bad.Body.String())
		}
	}

	// 更新：非空设置生效；省略保持不变；空串清除。
	patch := apitest.Do(t, env.engine, http.MethodPatch, "/api/v1/system/claude-keys/"+created.ID,
		nil, `{"base_url":"https://gw2.example"}`)
	assertSafeGatewayResponse(t, patch.Body.Bytes(), secret)
	if patch.Code != http.StatusOK {
		t.Fatalf("patch set status=%d body=%s", patch.Code, patch.Body.String())
	}
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
	assertSafeGatewayResponse(t, patch.Body.Bytes(), secret)
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
	assertSafeGatewayResponse(t, patch.Body.Bytes(), secret)
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
	assertSafeGatewayResponse(t, badPatch.Body.Bytes(), secret)
	if badPatch.Code != http.StatusBadRequest {
		t.Fatalf("非法 base_url 的 PATCH status=%d body=%s", badPatch.Code, badPatch.Body.String())
	}

	// 列表响应同样携带 base_url。
	list := apitest.Do(t, env.engine, http.MethodGet, "/api/v1/system/claude-keys", nil, "")
	assertSafeGatewayResponse(t, list.Body.Bytes(), secret)
	if list.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}
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

func TestClaudeKeyRuntimeConfigPatchExplicitlyResetsState(t *testing.T) {
	env := newGatewayHandlerEnv(t, true)
	secret := strings.Join([]string{"runtime", "reset", uuid.NewString()}, "-")
	key, err := env.store.Create(Key{
		Name:    "Runtime reset",
		APIKey:  secret,
		Model:   "deepseek-chat",
		BaseURL: "https://gateway.example/anthropic",
		Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	patches := []struct {
		name string
		body string
	}{
		{name: "model", body: `{"model":"deepseek-chat"}`},
		{name: "base URL", body: `{"base_url":"https://gateway.example/anthropic"}`},
	}
	for _, tc := range patches {
		t.Run(tc.name, func(t *testing.T) {
			current, err := env.store.Get(key.ID)
			if err != nil {
				t.Fatal(err)
			}
			env.state.MarkAuthFailedForKey(current, "claude_auth_failed", 2*time.Millisecond)
			if got := env.state.Snapshot(current).Status; got != StatusAuthFailed {
				t.Fatalf("precondition status=%q want %q", got, StatusAuthFailed)
			}

			rec := apitest.Do(t, env.engine, http.MethodPatch,
				"/api/v1/system/claude-keys/"+key.ID, nil, tc.body)
			assertSafeGatewayResponse(t, rec.Body.Bytes(), secret)
			if rec.Code != http.StatusOK {
				t.Fatalf("patch status=%d body=%s", rec.Code, rec.Body.String())
			}
			var updated SafeKey
			if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
				t.Fatal(err)
			}
			if updated.Status != StatusUnknown || updated.LastErrorCode != nil || updated.LastFailureAt != nil {
				t.Fatalf("runtime config patch retained stale state: %+v", updated)
			}
		})
	}
}

func TestClaudeKeyRuntimeConfigPatchBlocksOldOrdinaryOutcomeWithoutSnapshot(t *testing.T) {
	outcomes := []struct {
		name       string
		patchField string
		patchValue string
		statusCode int
		body       string
		wantCode   string
		wantStatus string
	}{
		{
			name:       "model change then current success",
			patchField: "model",
			patchValue: "deepseek-reasoner",
			statusCode: http.StatusOK,
			body:       `{"model":"deepseek-reasoner","stop_reason":"tool_use","content":[{"type":"tool_use","name":"structured_result","input":{"ok":true}}]}`,
			wantStatus: StatusHealthy,
		},
		{
			name:       "base URL change then current authentication failure",
			patchField: "base_url",
			patchValue: "https://new-gateway.example/anthropic",
			statusCode: http.StatusUnauthorized,
			body:       `{"error":{"type":"authentication_error","message":"bad key"}}`,
			wantCode:   aipkg.ErrAuthFailed,
			wantStatus: StatusAuthFailed,
		},
		{
			name:       "token change then current transient failure",
			patchField: "api_key",
			statusCode: http.StatusServiceUnavailable,
			body:       `{"error":{"type":"overloaded_error","message":"busy"}}`,
			wantCode:   aipkg.ErrOverloaded,
			wantStatus: StatusCoolingDown,
		},
	}

	for _, tc := range outcomes {
		t.Run(tc.name, func(t *testing.T) {
			env := newGatewayHandlerEnv(t, true)
			oldSecret := strings.Join([]string{"runtime", "old", uuid.NewString()}, "-")
			newSecret := strings.Join([]string{"runtime", "new", uuid.NewString()}, "-")
			key, err := env.store.Create(Key{
				Name: "Runtime generation", APIKey: oldSecret, Model: "deepseek-chat",
				BaseURL: "https://old-gateway.example/anthropic", Enabled: true,
				MaxConsecutiveFailures: 1,
			})
			if err != nil {
				t.Fatal(err)
			}

			started := make(chan struct{})
			releaseOld := make(chan struct{})
			calls := 0
			router := &Router{
				Store: env.store, State: env.state, Endpoint: "https://anthropic.test/v1/messages",
				DefaultModel: "claude-test", MaxOutputTokens: 64,
			}
			router.HTTPClient = &http.Client{Transport: gatewayRoundTripFunc(func(request *http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					close(started)
					<-releaseOld
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body: io.NopCloser(strings.NewReader(
							`{"model":"deepseek-chat","stop_reason":"tool_use","content":[{"type":"tool_use","name":"structured_result","input":{"ok":true}}]}`)),
						Request: request,
					}, nil
				}
				return &http.Response{
					StatusCode: tc.statusCode,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(tc.body)),
					Request:    request,
				}, nil
			})}

			oldDone := make(chan error, 1)
			go func() {
				_, callErr := router.Generate(context.Background(), routerTestRequest())
				oldDone <- callErr
			}()
			<-started

			patchValue := tc.patchValue
			if tc.patchField == "api_key" {
				patchValue = newSecret
			}
			patchBody, err := json.Marshal(map[string]string{tc.patchField: patchValue})
			if err != nil {
				t.Fatal(err)
			}
			rec := apitest.Do(t, env.engine, http.MethodPatch,
				"/api/v1/system/claude-keys/"+key.ID, nil, string(patchBody))
			assertSafeGatewayResponse(t, rec.Body.Bytes(), oldSecret)
			assertSafeGatewayResponse(t, rec.Body.Bytes(), newSecret)
			if rec.Code != http.StatusOK {
				t.Fatalf("patch status=%d body=%s", rec.Code, rec.Body.String())
			}
			updated, err := env.store.Get(key.ID)
			if err != nil {
				t.Fatal(err)
			}

			close(releaseOld)
			if err := <-oldDone; err != nil {
				t.Fatalf("old request returned unexpected error: %v", err)
			}
			_, currentErr := router.Generate(context.Background(), routerTestRequest())
			if tc.wantCode == "" {
				if currentErr != nil {
					t.Fatalf("current request returned unexpected error: %v", currentErr)
				}
			} else if got := aipkg.AsProviderError(currentErr).Code; got != tc.wantCode {
				t.Fatalf("current request code=%q want %q (err=%v)", got, tc.wantCode, currentErr)
			}

			fresh := NewStateTracker(env.state.path, nil)
			fresh.mu.Lock()
			fresh.reloadLocked()
			got := fresh.states[key.ID]
			fresh.mu.Unlock()
			if got.Fingerprint != keyStateFingerprint(updated) || got.Status != tc.wantStatus {
				t.Fatalf("current request did not own updated generation state: %+v", got)
			}
			if tc.wantCode == "" {
				if got.LastSuccessAt == nil || got.LastErrorCode != nil {
					t.Fatalf("current success diagnostics not recorded: %+v", got)
				}
			} else if got.LastErrorCode == nil || *got.LastErrorCode != tc.wantCode {
				t.Fatalf("current failure diagnostics not recorded: %+v", got)
			}
		})
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
	if err := writeClaudeCodeSettings(custom, Key{APIKey: "sk-ant-unit", BaseURL: "https://gw1.example"}, "https://configured.example"); err != nil {
		t.Fatal(err)
	}
	if got := readEnv(t, custom)["ANTHROPIC_BASE_URL"]; got != "https://gw1.example" {
		t.Fatalf("Key 有 base_url 时应覆盖导出: %q", got)
	}

	// Key 无 base_url 时回退部署配置，而不是历史硬编码网关。
	fallback := filepath.Join(t.TempDir(), "settings.json")
	if err := writeClaudeCodeSettings(fallback, Key{APIKey: "sk-ant-unit"}, " https://configured.example/ "); err != nil {
		t.Fatal(err)
	}
	if got := readEnv(t, fallback)["ANTHROPIC_BASE_URL"]; got != "https://configured.example" {
		t.Fatalf("Key 无 base_url 时应使用部署默认值: %q", got)
	}
	if strings.Contains(string(mustReadFile(t, fallback)), "https://app.haoshuai.cc.cd") {
		t.Fatal("saved settings must never use the obsolete hardcoded gateway")
	}
}

func TestClaudeCodeSettingsEffectiveBaseURLPrecedence(t *testing.T) {
	tests := []struct {
		name       string
		perKey     string
		configured string
		want       string
	}{
		{
			name:       "per-key override",
			perKey:     " https://per-key.example/anthropic/ ",
			configured: "https://configured.example/anthropic",
			want:       "https://per-key.example/anthropic",
		},
		{
			name:       "per-key full messages endpoint",
			perKey:     " https://api.deepseek.com/anthropic/v1/messages/ ",
			configured: "https://configured.example/anthropic",
			want:       "https://api.deepseek.com/anthropic",
		},
		{
			name:       "configured deployment",
			configured: " https://configured.example/anthropic/ ",
			want:       "https://configured.example/anthropic",
		},
		{
			name:       "configured full messages endpoint",
			configured: " https://api.deepseek.com/anthropic/v1/messages/ ",
			want:       "https://api.deepseek.com/anthropic",
		},
		{
			name: "Anthropic default",
			want: "https://api.anthropic.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EffectiveBaseURL(tt.perKey, tt.configured); got != tt.want {
				t.Fatalf("EffectiveBaseURL(%q, %q)=%q want %q", tt.perKey, tt.configured, got, tt.want)
			}
		})
	}
}

func TestClaudeCodeSettingsRouteUsesHandlerDefaultBaseURL(t *testing.T) {
	settingsFile := filepath.Join(t.TempDir(), ".claude", "settings.json")
	env := newGatewayHandlerEnvWithClaudeCodeSettingsAndDefaultBaseURL(
		t, true, settingsFile, " https://configured.deepseek.example/anthropic/ ",
	)
	secret := strings.Join([]string{"runtime", "configured-default", uuid.NewString()}, "-")
	key, err := env.store.Create(Key{Name: "Configured default", APIKey: secret, Model: "deepseek-chat", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	rec := apitest.Do(t, env.engine, http.MethodPost,
		"/api/v1/system/claude-keys/"+key.ID+"/claude-code-settings", nil, "")
	assertSafeGatewayResponse(t, rec.Body.Bytes(), secret)
	if rec.Code != http.StatusOK {
		t.Fatalf("export status=%d body=%s", rec.Code, rec.Body.String())
	}
	raw := mustReadFile(t, settingsFile)
	if strings.Contains(string(raw), "https://app.haoshuai.cc.cd") {
		t.Fatal("saved settings must never use the obsolete hardcoded gateway")
	}
	var saved struct {
		Env map[string]string `json:"env"`
	}
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	if got, want := saved.Env["ANTHROPIC_BASE_URL"], "https://configured.deepseek.example/anthropic"; got != want {
		t.Fatalf("ANTHROPIC_BASE_URL=%q want configured handler default %q", got, want)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
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
	assertSafeGatewayResponse(t, blocked.Body.Bytes(), secret)
	if blocked.Code != http.StatusForbidden {
		t.Fatalf("write-disabled status=%d body=%s", blocked.Code, blocked.Body.String())
	}

	env = newGatewayHandlerEnv(t, true)
	invalidBody, err := json.Marshal(map[string]any{
		"api_key": secret, "name": "Invalid", "model": "claude-test", "weight": 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	invalid := apitest.Do(t, env.engine, http.MethodPost, "/api/v1/system/claude-keys",
		map[string]string{"Idempotency-Key": uuid.NewString()}, string(invalidBody))
	assertSafeGatewayResponse(t, invalid.Body.Bytes(), secret)
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid status=%d body=%s", invalid.Code, invalid.Body.String())
	}
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
