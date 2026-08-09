// Package claudecli resolves the gateway Key and minimal environment used by
// Worker-owned Claude Code subprocesses.
package claudecli

import (
	"encoding/json"
	"errors"
	"os"
	"strings"

	aipkg "github.com/TSY244/ai-study-tutor/backend/internal/ai"
	"github.com/TSY244/ai-study-tutor/backend/internal/claudegateway"
)

// Runtime describes the shared configuration boundary for a Claude Code
// subprocess. SettingsFile may be empty or absent for legacy priority fallback.
type Runtime struct {
	KeysFile       string
	SettingsFile   string
	DefaultBaseURL string
	Home           string

	readFile func(string) ([]byte, error)
}

// Selection is the exact enabled gateway Key and its normalized effective
// Base URL. Key.APIKey must only be passed to a child process environment.
type Selection struct {
	Key     claudegateway.Key
	BaseURL string
}

type settingsDocument struct {
	Env map[string]string `json:"env"`
}

// Resolve loads gateway Keys once and selects the Key applied to the runtime
// settings file. An explicit application selector is authoritative and never
// silently falls back to another Key.
func (r Runtime) Resolve() (Selection, error) {
	keys, err := claudegateway.NewStore(r.KeysFile).Load()
	if err != nil {
		return Selection{}, aipkg.ErrorForCode(aipkg.ErrUnavailable,
			"无法读取 Claude 网关配置；请检查配置文件后重试。")
	}

	doc, hasSettings, err := r.loadSettings()
	if err != nil {
		return Selection{}, err
	}
	if !hasSettings {
		return r.fallback(keys)
	}

	selector, selected := doc.Env["AI_STUDY_TUTOR_CLAUDE_KEY_ID"]
	if !selected {
		return r.fallback(keys)
	}
	for _, key := range keys {
		if key.ID != selector {
			continue
		}
		if !key.Enabled || !r.settingsMatch(doc.Env, key) {
			return Selection{}, staleSettingsError()
		}
		return Selection{
			Key:     key,
			BaseURL: claudegateway.EffectiveBaseURL(key.BaseURL, r.DefaultBaseURL),
		}, nil
	}
	return Selection{}, staleSettingsError()
}

func (r Runtime) loadSettings() (*settingsDocument, bool, error) {
	if strings.TrimSpace(r.SettingsFile) == "" {
		return nil, false, nil
	}
	readFile := r.readFile
	if readFile == nil {
		readFile = os.ReadFile
	}
	raw, err := readFile(r.SettingsFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, aipkg.ErrorForCode(aipkg.ErrUnavailable,
			"无法读取 Claude Code 运行时设置；请检查文件权限后重试。")
	}
	var doc *settingsDocument
	if json.Unmarshal(raw, &doc) != nil || doc == nil {
		return nil, false, staleSettingsError()
	}
	return doc, true, nil
}

func (r Runtime) settingsMatch(env map[string]string, key claudegateway.Key) bool {
	token, hasToken := env["ANTHROPIC_AUTH_TOKEN"]
	baseURL, hasBaseURL := env["ANTHROPIC_BASE_URL"]
	model, hasModel := env["ANTHROPIC_MODEL"]
	if !hasToken || !hasBaseURL || !hasModel || strings.TrimSpace(baseURL) == "" {
		return false
	}
	return token == key.APIKey &&
		claudegateway.EffectiveBaseURL(baseURL, "") == claudegateway.EffectiveBaseURL(key.BaseURL, r.DefaultBaseURL) &&
		model == key.Model
}

func (r Runtime) fallback(keys []claudegateway.Key) (Selection, error) {
	best := -1
	for i, key := range keys {
		if !key.Enabled {
			continue
		}
		if best < 0 || key.Priority < keys[best].Priority ||
			(key.Priority == keys[best].Priority && key.Weight > keys[best].Weight) {
			best = i
		}
	}
	if best < 0 {
		return Selection{}, aipkg.ErrorForCode(aipkg.ErrNotConfigured,
			"Claude 网关没有启用的 Key；请在网关配置中启用 Key 后重试。")
	}
	key := keys[best]
	return Selection{
		Key:     key,
		BaseURL: claudegateway.EffectiveBaseURL(key.BaseURL, r.DefaultBaseURL),
	}, nil
}

func staleSettingsError() error {
	return aipkg.ErrorForCode(aipkg.ErrNotConfigured,
		"Claude Code 运行时设置已失效；请重新应用一个启用的 Key 后重试。")
}

// Env returns a fresh allowlisted subprocess environment for selection. It
// never inherits arbitrary parent variables; PATH is the sole inherited value.
func (r Runtime) Env(selection Selection) []string {
	env := []string{
		"HOME=" + r.Home,
		"PATH=" + os.Getenv("PATH"),
		"SHELL=/bin/bash",
		"CI=1",
		"NO_COLOR=1",
		"ANTHROPIC_AUTH_TOKEN=" + selection.Key.APIKey,
	}
	if strings.TrimSpace(selection.BaseURL) != "" {
		env = append(env, "ANTHROPIC_BASE_URL="+selection.BaseURL)
	}
	env = append(env,
		"ANTHROPIC_MODEL="+selection.Key.Model,
		"ANTHROPIC_DEFAULT_OPUS_MODEL="+selection.Key.Model,
		"ANTHROPIC_DEFAULT_SONNET_MODEL="+selection.Key.Model,
		"ANTHROPIC_DEFAULT_HAIKU_MODEL="+selection.Key.Model,
		"CLAUDE_CODE_SUBAGENT_MODEL="+selection.Key.Model,
		"AI_STUDY_TUTOR_CLAUDE_KEY_ID="+selection.Key.ID,
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1",
		"CLAUDE_CODE_ATTRIBUTION_HEADER=0",
	)
	return env
}
