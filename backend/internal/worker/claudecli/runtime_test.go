package claudecli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	aipkg "github.com/TSY244/ai-study-tutor/backend/internal/ai"
	"github.com/TSY244/ai-study-tutor/backend/internal/claudegateway"
)

func TestRuntimeResolveUsesExactSelectedEnabledKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	keysFile := filepath.Join(dir, "keys.json")
	settingsFile := filepath.Join(dir, ".claude", "settings.json")
	keys := []claudegateway.Key{
		{ID: "key-high-priority", APIKey: "token-high-priority", BaseURL: "https://high.example/anthropic", Model: "model-high-priority", Enabled: true, Priority: 1, Weight: 100},
		{ID: "key-explicit", APIKey: "token-explicit", BaseURL: "https://selected.example/anthropic/v1/messages/", Model: "model-explicit", Enabled: true, Priority: 99, Weight: 1},
	}
	mustWriteKeys(t, keysFile, keys)
	mustWriteSettings(t, settingsFile, settingsEnv(keys[1], "https://unused.example"))

	runtime := Runtime{
		KeysFile:       keysFile,
		SettingsFile:   settingsFile,
		DefaultBaseURL: "https://unused.example",
		Home:           dir,
	}
	selection, err := runtime.Resolve()
	if err != nil {
		t.Fatal("Resolve returned an unexpected error")
	}
	if selection.Key.ID != keys[1].ID || selection.Key.APIKey != keys[1].APIKey || selection.Key.Model != keys[1].Model {
		t.Fatal("Resolve did not return the explicitly selected Key")
	}
	if selection.BaseURL != claudegateway.EffectiveBaseURL(keys[1].BaseURL, runtime.DefaultBaseURL) {
		t.Fatal("Resolve did not return the selected Key's normalized effective Base URL")
	}
}

func TestRuntimeResolveUsesIDWhenTokensAreDuplicated(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	keysFile := filepath.Join(dir, "keys.json")
	settingsFile := filepath.Join(dir, "settings.json")
	keys := []claudegateway.Key{
		{ID: "key-shared-first", APIKey: "token-shared", BaseURL: "https://first.example/anthropic", Model: "model-first", Enabled: true, Priority: 1, Weight: 100},
		{ID: "key-shared-selected", APIKey: "token-shared", BaseURL: "https://second.example/anthropic", Model: "model-second", Enabled: true, Priority: 20, Weight: 1},
	}
	mustWriteKeys(t, keysFile, keys)
	mustWriteSettings(t, settingsFile, settingsEnv(keys[1], "https://unused.example"))

	selection, err := (Runtime{
		KeysFile:       keysFile,
		SettingsFile:   settingsFile,
		DefaultBaseURL: "https://unused.example",
		Home:           dir,
	}).Resolve()
	if err != nil {
		t.Fatal("Resolve returned an unexpected error")
	}
	if selection.Key.ID != keys[1].ID || selection.Key.Model != keys[1].Model {
		t.Fatal("Resolve matched a duplicate token instead of the exact selector ID")
	}
	if selection.BaseURL != claudegateway.EffectiveBaseURL(keys[1].BaseURL, "https://unused.example") {
		t.Fatal("Resolve returned the duplicate token record's wrong Base URL")
	}
}

func TestRuntimeResolveUsesSharedEffectiveBaseURL(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	keysFile := filepath.Join(dir, "keys.json")
	settingsFile := filepath.Join(dir, "settings.json")
	key := claudegateway.Key{
		ID: "key-default-base", APIKey: "token-default-base", Model: "model-default-base",
		Enabled: true, Priority: 10, Weight: 100,
	}
	defaultBaseURL := " https://configured.example/anthropic/v1/messages/ "
	mustWriteKeys(t, keysFile, []claudegateway.Key{key})
	mustWriteSettings(t, settingsFile, settingsEnv(key, defaultBaseURL))

	selection, err := (Runtime{
		KeysFile: keysFile, SettingsFile: settingsFile,
		DefaultBaseURL: defaultBaseURL, Home: dir,
	}).Resolve()
	if err != nil {
		t.Fatal("Resolve returned an unexpected error")
	}
	want := claudegateway.EffectiveBaseURL(key.BaseURL, defaultBaseURL)
	if selection.BaseURL != want {
		t.Fatal("Resolve did not use claudegateway.EffectiveBaseURL")
	}
}

func TestRuntimeResolveFallsBackDeterministicallyWithoutSelector(t *testing.T) {
	t.Parallel()
	keys := []claudegateway.Key{
		{ID: "disabled-best", APIKey: "token-disabled", Model: "model-disabled", Enabled: false, Priority: -100, Weight: 1000},
		{ID: "worse-priority", APIKey: "token-worse", Model: "model-worse", Enabled: true, Priority: 20, Weight: 999},
		{ID: "lower-weight", APIKey: "token-lower-weight", Model: "model-lower-weight", Enabled: true, Priority: 10, Weight: 50},
		{ID: "file-order-winner", APIKey: "token-file-order", Model: "model-file-order", Enabled: true, Priority: 10, Weight: 100},
		{ID: "file-order-loser", APIKey: "token-later", Model: "model-later", Enabled: true, Priority: 10, Weight: 100},
	}

	tests := []struct {
		name          string
		settingsSetup func(*testing.T, string)
		settingsFile  func(string) string
	}{
		{
			name: "missing settings file",
			settingsFile: func(dir string) string {
				return filepath.Join(dir, "missing-settings.json")
			},
		},
		{
			name: "blank settings path",
			settingsFile: func(string) string {
				return ""
			},
		},
		{
			name: "valid pre-existing settings without application selector",
			settingsFile: func(dir string) string {
				return filepath.Join(dir, "settings.json")
			},
			settingsSetup: func(t *testing.T, path string) {
				t.Helper()
				mustWriteSettings(t, path, map[string]string{
					"ANTHROPIC_AUTH_TOKEN": "pre-existing-token-sentinel",
					"ANTHROPIC_MODEL":      "pre-existing-model-sentinel",
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			keysFile := filepath.Join(dir, "keys.json")
			mustWriteKeys(t, keysFile, keys)
			settingsFile := tt.settingsFile(dir)
			if tt.settingsSetup != nil {
				tt.settingsSetup(t, settingsFile)
			}

			selection, err := (Runtime{
				KeysFile: keysFile, SettingsFile: settingsFile,
				DefaultBaseURL: "https://fallback.example", Home: dir,
			}).Resolve()
			if err != nil {
				t.Fatal("fallback Resolve returned an unexpected error")
			}
			if selection.Key.ID != "file-order-winner" {
				t.Fatal("fallback did not use enabled, priority, weight, and file-order precedence")
			}
		})
	}
}

func TestRuntimeResolveRejectsNoEnabledFallback(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	keysFile := filepath.Join(dir, "keys.json")
	secret := "token-disabled-sentinel"
	mustWriteKeys(t, keysFile, []claudegateway.Key{{
		ID: "disabled", APIKey: secret, BaseURL: "https://disabled-sentinel.example", Model: "model-disabled-sentinel",
		Enabled: false, Priority: 1, Weight: 100,
	}})

	_, err := (Runtime{KeysFile: keysFile, SettingsFile: filepath.Join(dir, "missing.json"), Home: dir}).Resolve()
	assertSanitizedProviderError(t, err, aipkg.ErrNotConfigured, secret, "https://disabled-sentinel.example", "model-disabled-sentinel")
}

func TestRuntimeResolveRejectsUnsafeOrStaleSettings(t *testing.T) {
	t.Parallel()
	const (
		selectedToken = "token-selected-sentinel"
		selectedBase  = "https://selected-base-sentinel.example/anthropic"
		selectedModel = "model-selected-sentinel"
		disabledToken = "token-disabled-sentinel"
		disabledBase  = "https://disabled-base-sentinel.example/anthropic"
		disabledModel = "model-disabled-sentinel"
	)
	selected := claudegateway.Key{
		ID: "selected-key", APIKey: selectedToken, BaseURL: selectedBase, Model: selectedModel,
		Enabled: true, Priority: 1, Weight: 100,
	}
	disabled := claudegateway.Key{
		ID: "disabled-key", APIKey: disabledToken, BaseURL: disabledBase, Model: disabledModel,
		Enabled: false, Priority: 2, Weight: 100,
	}
	defaultBaseURL := "https://default-base-sentinel.example/anthropic"

	tests := []struct {
		name     string
		wantCode string
		prepare  func(*testing.T, *Runtime, string, map[string]string)
	}{
		{
			name:     "malformed JSON",
			wantCode: aipkg.ErrNotConfigured,
			prepare: func(t *testing.T, _ *Runtime, path string, _ map[string]string) {
				t.Helper()
				mustWriteRaw(t, path, []byte(`{"env":{"AI_STUDY_TUTOR_CLAUDE_KEY_ID":"selected-key","ANTHROPIC_AUTH_TOKEN":"token-malformed-sentinel"`))
			},
		},
		{
			name:     "settings read error",
			wantCode: aipkg.ErrUnavailable,
			prepare: func(_ *testing.T, runtime *Runtime, _ string, _ map[string]string) {
				runtime.readFile = func(string) ([]byte, error) {
					return nil, errors.New("token-read-error-sentinel https://read-error-sentinel.example model-read-error-sentinel")
				}
			},
		},
		{
			name:     "unknown selector ID",
			wantCode: aipkg.ErrNotConfigured,
			prepare: func(t *testing.T, _ *Runtime, path string, env map[string]string) {
				t.Helper()
				env["AI_STUDY_TUTOR_CLAUDE_KEY_ID"] = "unknown-key"
				mustWriteSettings(t, path, env)
			},
		},
		{
			name:     "disabled selector ID",
			wantCode: aipkg.ErrNotConfigured,
			prepare: func(t *testing.T, _ *Runtime, path string, _ map[string]string) {
				t.Helper()
				mustWriteSettings(t, path, settingsEnv(disabled, defaultBaseURL))
			},
		},
		{
			name:     "token mismatch",
			wantCode: aipkg.ErrNotConfigured,
			prepare: func(t *testing.T, _ *Runtime, path string, env map[string]string) {
				t.Helper()
				env["ANTHROPIC_AUTH_TOKEN"] = "token-settings-mismatch-sentinel"
				mustWriteSettings(t, path, env)
			},
		},
		{
			name:     "Base URL mismatch",
			wantCode: aipkg.ErrNotConfigured,
			prepare: func(t *testing.T, _ *Runtime, path string, env map[string]string) {
				t.Helper()
				env["ANTHROPIC_BASE_URL"] = "https://settings-base-mismatch-sentinel.example/anthropic"
				mustWriteSettings(t, path, env)
			},
		},
		{
			name:     "model mismatch",
			wantCode: aipkg.ErrNotConfigured,
			prepare: func(t *testing.T, _ *Runtime, path string, env map[string]string) {
				t.Helper()
				env["ANTHROPIC_MODEL"] = "model-settings-mismatch-sentinel"
				mustWriteSettings(t, path, env)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			keysFile := filepath.Join(dir, "keys.json")
			settingsFile := filepath.Join(dir, "settings.json")
			mustWriteKeys(t, keysFile, []claudegateway.Key{selected, disabled})
			runtime := Runtime{
				KeysFile: keysFile, SettingsFile: settingsFile,
				DefaultBaseURL: defaultBaseURL, Home: dir,
			}
			env := settingsEnv(selected, defaultBaseURL)
			tt.prepare(t, &runtime, settingsFile, env)

			_, err := runtime.Resolve()
			assertSanitizedProviderError(t, err, tt.wantCode,
				selectedToken, selectedBase, selectedModel,
				disabledToken, disabledBase, disabledModel,
				defaultBaseURL,
				"token-malformed-sentinel",
				"token-read-error-sentinel", "https://read-error-sentinel.example", "model-read-error-sentinel",
				"token-settings-mismatch-sentinel",
				"https://settings-base-mismatch-sentinel.example/anthropic",
				"model-settings-mismatch-sentinel",
			)
			if tt.wantCode == aipkg.ErrNotConfigured {
				perr := aipkg.AsProviderError(err)
				if !strings.Contains(perr.Detail, "重新应用") {
					t.Fatal("configuration error did not give safe re-apply guidance")
				}
			}
		})
	}
}

func TestRuntimeEnvIsFreshMinimalAndMatchesSelection(t *testing.T) {
	t.Setenv("PATH", "/runtime-test/bin")
	t.Setenv("PARENT_SECRET_SENTINEL", "must-not-be-inherited")
	runtime := Runtime{Home: "/runtime-home"}
	selection := Selection{
		Key: claudegateway.Key{
			ID: "key-env", APIKey: "token-env", Model: "model-env",
		},
		BaseURL: "https://env.example/anthropic",
	}

	env := runtime.Env(selection)
	got := envMap(t, env)
	wantNames := map[string]bool{
		"HOME": true, "PATH": true, "SHELL": true, "CI": true, "NO_COLOR": true,
		"ANTHROPIC_AUTH_TOKEN": true, "ANTHROPIC_BASE_URL": true, "ANTHROPIC_MODEL": true,
		"ANTHROPIC_DEFAULT_OPUS_MODEL": true, "ANTHROPIC_DEFAULT_SONNET_MODEL": true,
		"ANTHROPIC_DEFAULT_HAIKU_MODEL": true, "CLAUDE_CODE_SUBAGENT_MODEL": true,
		"AI_STUDY_TUTOR_CLAUDE_KEY_ID":             true,
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": true,
		"CLAUDE_CODE_ATTRIBUTION_HEADER":           true,
	}
	if len(got) != len(wantNames) {
		t.Fatal("Env returned the wrong number of allowlisted variables")
	}
	for name := range got {
		if !wantNames[name] {
			t.Fatalf("Env returned non-allowlisted variable name %q", name)
		}
	}
	if got["HOME"] != runtime.Home || got["PATH"] != os.Getenv("PATH") || got["SHELL"] != "/bin/bash" || got["CI"] != "1" || got["NO_COLOR"] != "1" {
		t.Fatal("Env did not provide the expected process runtime values")
	}
	if got["ANTHROPIC_AUTH_TOKEN"] != selection.Key.APIKey || got["ANTHROPIC_BASE_URL"] != selection.BaseURL || got["AI_STUDY_TUTOR_CLAUDE_KEY_ID"] != selection.Key.ID {
		t.Fatal("Env did not match the selected Key")
	}
	for _, name := range []string{
		"ANTHROPIC_MODEL", "ANTHROPIC_DEFAULT_OPUS_MODEL", "ANTHROPIC_DEFAULT_SONNET_MODEL",
		"ANTHROPIC_DEFAULT_HAIKU_MODEL", "CLAUDE_CODE_SUBAGENT_MODEL",
	} {
		if got[name] != selection.Key.Model {
			t.Fatalf("Env model variable %q did not match the Selection", name)
		}
	}
	if got["CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC"] != "1" || got["CLAUDE_CODE_ATTRIBUTION_HEADER"] != "0" {
		t.Fatal("Env did not provide the expected Claude Code traffic controls")
	}
	if _, exists := got["PARENT_SECRET_SENTINEL"]; exists {
		t.Fatal("Env inherited a non-allowlisted parent variable")
	}

	env[0] = "HOME=/mutated"
	fresh := envMap(t, runtime.Env(selection))
	if fresh["HOME"] != runtime.Home {
		t.Fatal("Env did not return a fresh environment slice")
	}

	withoutBase := selection
	withoutBase.BaseURL = ""
	if _, exists := envMap(t, runtime.Env(withoutBase))["ANTHROPIC_BASE_URL"]; exists {
		t.Fatal("Env included ANTHROPIC_BASE_URL for an empty effective value")
	}
}

func mustWriteKeys(t *testing.T, path string, keys []claudegateway.Key) {
	t.Helper()
	raw, err := json.Marshal(struct {
		Keys []claudegateway.Key `json:"keys"`
	}{Keys: keys})
	if err != nil {
		t.Fatal("could not encode test keys")
	}
	mustWriteRaw(t, path, raw)
}

func mustWriteSettings(t *testing.T, path string, env map[string]string) {
	t.Helper()
	raw, err := json.Marshal(struct {
		Schema string            `json:"$schema"`
		Env    map[string]string `json:"env"`
	}{
		Schema: "https://json.schemastore.org/claude-code-settings.json",
		Env:    env,
	})
	if err != nil {
		t.Fatal("could not encode test settings")
	}
	mustWriteRaw(t, path, raw)
}

func mustWriteRaw(t *testing.T, path string, raw []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal("could not create test configuration directory")
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal("could not write test configuration")
	}
}

func settingsEnv(key claudegateway.Key, defaultBaseURL string) map[string]string {
	return map[string]string{
		"ANTHROPIC_AUTH_TOKEN":                     key.APIKey,
		"ANTHROPIC_BASE_URL":                       claudegateway.EffectiveBaseURL(key.BaseURL, defaultBaseURL),
		"ANTHROPIC_MODEL":                          key.Model,
		"AI_STUDY_TUTOR_CLAUDE_KEY_ID":             key.ID,
		"CLAUDE_CODE_SUBAGENT_MODEL":               key.Model,
		"ANTHROPIC_DEFAULT_OPUS_MODEL":             key.Model,
		"ANTHROPIC_DEFAULT_SONNET_MODEL":           key.Model,
		"ANTHROPIC_DEFAULT_HAIKU_MODEL":            key.Model,
		"CLAUDE_CODE_ATTRIBUTION_HEADER":           "0",
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
	}
}

func assertSanitizedProviderError(t *testing.T, err error, wantCode string, sensitive ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("Resolve unexpectedly succeeded")
	}
	var perr *aipkg.ProviderError
	if !errors.As(err, &perr) {
		t.Fatal("Resolve did not return a ProviderError")
	}
	if perr.Code != wantCode {
		t.Fatalf("ProviderError code=%q want %q", perr.Code, wantCode)
	}
	for current := err; current != nil; current = errors.Unwrap(current) {
		message := current.Error()
		for _, value := range sensitive {
			if value != "" && strings.Contains(message, value) {
				t.Fatal("configuration error retained a sensitive value")
			}
		}
	}
	if errors.Unwrap(perr) != nil {
		t.Fatal("configuration error retained an underlying parser or data error")
	}
}

func envMap(t *testing.T, env []string) map[string]string {
	t.Helper()
	result := make(map[string]string, len(env))
	for _, entry := range env {
		name, value, ok := strings.Cut(entry, "=")
		if !ok || name == "" {
			t.Fatal("Env returned a malformed environment entry")
		}
		if _, duplicate := result[name]; duplicate {
			t.Fatalf("Env returned duplicate variable name %q", name)
		}
		result[name] = value
	}
	return result
}
