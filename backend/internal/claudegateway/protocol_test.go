package claudegateway

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	aipkg "github.com/TSY244/ai-study-tutor/backend/internal/ai"
	"github.com/TSY244/ai-study-tutor/backend/internal/apitest"
	"github.com/google/uuid"
)

func TestProtocolDefaultsAndConfigurationGeneration(t *testing.T) {
	k := routerTestKey("protocol", 1, 100)
	if EffectiveKeyBaseURL(k, "https://deployment.example") != "https://deployment.example" {
		t.Fatal("legacy default changed")
	}
	legacy := keyStateFingerprint(k)
	k.Protocol = ProtocolAnthropic
	if keyStateFingerprint(k) != legacy {
		t.Fatal("explicit Anthropic must preserve existing runtime state")
	}
	k.Protocol = ProtocolOpenAI
	if EffectiveKeyBaseURL(k, "https://anthropic.example") != "https://api.openai.com/v1" {
		t.Fatal("OpenAI must not inherit Anthropic default")
	}
	if keyStateFingerprint(k) == legacy {
		t.Fatal("switching protocol must fence old outcomes")
	}
}

func TestGatewayMixedProtocolFailoverAndTargetedTest(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "keys.json"))
	first := routerTestKey("anthropic", 1, 100)
	first.BaseURL = "https://anthropic.example"
	second := routerTestKey("openai", 2, 100)
	second.Protocol = ProtocolOpenAI
	second.BaseURL = "https://openai.example/proxy/v1/chat/completions"
	writeRouterKeys(t, store, []Key{first, second})
	var calls []string
	client := &http.Client{Transport: gatewayRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls = append(calls, req.URL.Host)
		if req.URL.Host == "anthropic.example" {
			if req.Header.Get("x-api-key") != first.APIKey || req.URL.Path != "/v1/messages" {
				t.Fatal("Anthropic protocol changed")
			}
			return &http.Response{StatusCode: 503, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{}`))}, nil
		}
		if req.Header.Get("Authorization") != "Bearer "+second.APIKey || req.Header.Get("x-api-key") != "" || req.URL.Path != "/proxy/v1/chat/completions" {
			t.Fatal("incorrect OpenAI protocol")
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"choices":[{"finish_reason":"stop","message":{"content":"{\"ok\":true}"}}]}`))}, nil
	})}
	router := &Router{Store: store, State: NewStateTracker(StatePathFor(store.Path()), nil), HTTPClient: client}
	resp, err := router.Generate(context.Background(), routerTestRequest())
	if err != nil || string(resp.JSON) != `{"ok":true}` || len(calls) != 2 {
		t.Fatalf("mixed protocol failover: %+v %v %v", resp, err, calls)
	}
	calls = nil
	if _, err = router.GenerateForKey(context.Background(), second.ID, routerTestRequest()); err != nil || len(calls) != 1 || calls[0] != "openai.example" {
		t.Fatalf("targeted test: %v %v", err, calls)
	}
}

func TestProtocolAPIUpdateAndPiSettings(t *testing.T) {
	settings := filepath.Join(t.TempDir(), "settings.json")
	e := newGatewayHandlerEnvWithClaudeCodeSettings(t, true, settings)
	secret := "protocol-secret-keep-private"
	body, _ := json.Marshal(map[string]any{"api_key": secret, "name": "protocol", "model": "model"})
	created := apitest.Do(t, e.engine, http.MethodPost, "/api/v1/system/claude-keys", map[string]string{"Idempotency-Key": uuid.NewString()}, string(body))
	if created.Code != 201 {
		t.Fatalf("create: %s", created.Body.String())
	}
	var safe SafeKey
	if err := json.Unmarshal(created.Body.Bytes(), &safe); err != nil || safe.Protocol != ProtocolAnthropic {
		t.Fatal("old clients must default to Anthropic")
	}
	old, _ := e.store.Get(safe.ID)
	e.state.MarkAuthFailedForKey(old, aipkg.ErrAuthFailed, time.Second)
	patched := apitest.Do(t, e.engine, http.MethodPatch, "/api/v1/system/claude-keys/"+safe.ID, nil, `{"protocol":"openai","base_url":"https://proxy.example/v1/chat/completions"}`)
	assertSafeGatewayResponse(t, patched.Body.Bytes(), secret)
	if patched.Code != 200 {
		t.Fatalf("patch: %s", patched.Body.String())
	}
	if err := json.Unmarshal(patched.Body.Bytes(), &safe); err != nil || safe.Protocol != ProtocolOpenAI || safe.Status != StatusUnknown {
		t.Fatalf("protocol/state: %+v %v", safe, err)
	}
	persisted, _ := e.store.Get(safe.ID)
	if persisted.APIKey != secret || persisted.Protocol != ProtocolOpenAI {
		t.Fatal("protocol patch lost key or protocol")
	}
	e.state.MarkAuthFailedForKey(old, aipkg.ErrAuthFailed, time.Second)
	if e.state.Snapshot(persisted).Status != StatusUnknown {
		t.Fatal("old protocol outcome contaminated new state")
	}
	applied := apitest.Do(t, e.engine, http.MethodPost, "/api/v1/system/claude-keys/"+safe.ID+"/claude-code-settings", nil, "")
	if applied.Code != 200 {
		t.Fatal(applied.Body.String())
	}
	var doc struct {
		Env map[string]string `json:"env"`
	}
	raw, err := os.ReadFile(settings)
	if err != nil || json.Unmarshal(raw, &doc) != nil || doc.Env["AI_STUDY_TUTOR_AI_PROTOCOL"] != ProtocolOpenAI || doc.Env["ANTHROPIC_BASE_URL"] != "https://proxy.example/v1" || doc.Env["ANTHROPIC_AUTH_TOKEN"] != secret {
		t.Fatal("Pi did not receive exact protocol/base/key")
	}
	for _, protocol := range []string{"", "responses", "athropic"} {
		body, _ := json.Marshal(map[string]string{"protocol": protocol})
		bad := apitest.Do(t, e.engine, http.MethodPatch, "/api/v1/system/claude-keys/"+safe.ID, nil, string(body))
		if bad.Code != 400 {
			t.Fatalf("invalid protocol accepted: %q", protocol)
		}
	}
}
