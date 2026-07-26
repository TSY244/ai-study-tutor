package claudegateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
	"github.com/TSY244/ai-study-tutor/backend/internal/idempotency"
	"github.com/TSY244/ai-study-tutor/backend/internal/job"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/httpserver"
)

// Handler 暴露 /api/v1/system/claude-keys 管理端点。
// 所有响应只含安全视图（masked_key/fingerprint），绝不含完整 Key。
type Handler struct {
	Pool  *pgxpool.Pool
	Store *Store
	State *StateTracker
	Jobs  *job.Service

	// WriteEnabled 为 false 时写端点返回 403 claude_gateway_write_disabled；
	// GET 列表永远可用。
	WriteEnabled bool

	// LegacyFallback 报告 keys.json 无启用 Key 时旧单 Key 路径是否仍在
	// 工作。API 进程看不到 Anthropic secret，该信号由 Worker 心跳推导；
	// nil 时按 false 处理。
	LegacyFallback func(ctx context.Context) bool

	// ClaudeCodeSettingsFile 是 API 容器内映射到宿主机
	// ~/.claude/settings.json 的唯一允许写入目标。为空时导出端点安全失败。
	ClaudeCodeSettingsFile string

	idem *httpx.IdemExecutor
}

// RegisterRoutes 注册 Claude 网关密钥管理路由（启动期调用一次）。
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, jsonLimit gin.HandlerFunc, idem *httpx.IdemExecutor) {
	h.idem = idem
	rg.GET("/system/claude-keys", h.listKeys)
	rg.POST("/system/claude-keys", jsonLimit, h.createKey)
	rg.PATCH("/system/claude-keys/:key_id", jsonLimit, h.updateKey)
	rg.DELETE("/system/claude-keys/:key_id", h.deleteKey)
	rg.POST("/system/claude-keys/:key_id/test", jsonLimit, h.createKeyTest)
	rg.POST("/system/claude-keys/:key_id/claude-code-settings", jsonLimit, h.saveClaudeCodeSettings)
}

// requireWrite 在写端点前校验开关；未开启时写出 403 并返回 false。
func (h *Handler) requireWrite(c *gin.Context) bool {
	if h.WriteEnabled {
		return true
	}
	httpserver.AbortWithProblem(c, http.StatusForbidden, "claude_gateway_write_disabled",
		"Claude 网关为只读模式", "当前部署未开启 Claude 网关写端点（CLAUDE_GATEWAY_WRITE_ENABLED）。")
	return false
}

type keyListResponse struct {
	Items          []SafeKey `json:"items"`
	Routing        string    `json:"routing"`
	LegacyFallback bool      `json:"legacy_fallback"`
	WriteEnabled   bool      `json:"write_enabled"`
}

func (h *Handler) listKeys(c *gin.Context) {
	keys, err := h.Store.Load()
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	resp := keyListResponse{
		Items:        make([]SafeKey, 0, len(keys)),
		Routing:      "priority_failover",
		WriteEnabled: h.WriteEnabled,
	}
	enabled := 0
	for _, k := range keys {
		if k.Enabled {
			enabled++
		}
		resp.Items = append(resp.Items, safeView(k, h.State.Snapshot(k)))
	}
	if enabled == 0 && h.LegacyFallback != nil && h.LegacyFallback(c.Request.Context()) {
		resp.LegacyFallback = true
	}
	c.JSON(http.StatusOK, resp)
}

type createKeyRequest struct {
	APIKey                 string `json:"api_key"`
	Name                   string `json:"name"`
	Model                  string `json:"model"`
	Enabled                *bool  `json:"enabled"`
	Priority               *int   `json:"priority"`
	Weight                 *int   `json:"weight"`
	TimeoutSeconds         *int   `json:"timeout_seconds"`
	CooldownSeconds        *int   `json:"cooldown_seconds"`
	MaxConsecutiveFailures *int   `json:"max_consecutive_failures"`
}

func (h *Handler) createKey(c *gin.Context) {
	if !h.requireWrite(c) {
		return
	}
	var req createKeyRequest
	if !httpx.BindJSON(c, &req) {
		badRequest(c, "请求体不是有效的 JSON。")
		return
	}
	key := Key{
		APIKey:                 strings.TrimSpace(req.APIKey),
		Name:                   strings.TrimSpace(req.Name),
		Model:                  strings.TrimSpace(req.Model),
		Enabled:                true,
		Priority:               DefaultPriority,
		Weight:                 DefaultWeight,
		TimeoutSeconds:         DefaultTimeoutSeconds,
		CooldownSeconds:        DefaultCooldownSeconds,
		MaxConsecutiveFailures: DefaultMaxConsecutiveFailures,
	}
	if req.Enabled != nil {
		key.Enabled = *req.Enabled
	}
	if req.Priority != nil {
		key.Priority = *req.Priority
	}
	if req.Weight != nil {
		key.Weight = *req.Weight
	}
	if req.TimeoutSeconds != nil {
		key.TimeoutSeconds = *req.TimeoutSeconds
	}
	if req.CooldownSeconds != nil {
		key.CooldownSeconds = *req.CooldownSeconds
	}
	if req.MaxConsecutiveFailures != nil {
		key.MaxConsecutiveFailures = *req.MaxConsecutiveFailures
	}
	if !validateKeyFields(c, key.APIKey, key.Name, key.Model) || !validateKeyLimits(c, key) {
		return
	}
	hash := httpx.HashRequest(c.Request.Method, c.FullPath(), httpx.CanonicalJSON(req), "")
	h.idem.Run(c, "createClaudeKey", hash,
		func(ctx context.Context, rec *idempotency.Record) (int, any, error) {
			created, err := h.Store.Create(key)
			if err != nil {
				if errors.Is(err, ErrInvalidKey) {
					return 0, nil, httpx.NewProblemError(http.StatusBadRequest, "bad_request",
						"请求无效", "Claude Key 创建失败：api_key/name/model 不能为空。")
				}
				return 0, nil, err
			}
			body := safeView(created, h.State.Snapshot(created))
			tx, err := h.Pool.Begin(ctx)
			if err != nil {
				return 0, nil, err
			}
			defer func() { _ = tx.Rollback(ctx) }()
			headers := map[string]string{"Location": "/api/v1/system/claude-keys/" + created.ID}
			if err := httpx.CompleteJSON(ctx, tx, h.idem.Svc, rec.ID,
				http.StatusCreated, body, headers, nil, nil, nil); err != nil {
				return 0, nil, err
			}
			if err := tx.Commit(ctx); err != nil {
				return 0, nil, err
			}
			c.Header("Location", headers["Location"])
			return http.StatusCreated, body, nil
		})
}

type updateKeyRequest struct {
	APIKey                 *string `json:"api_key"`
	Name                   *string `json:"name"`
	Model                  *string `json:"model"`
	Enabled                *bool   `json:"enabled"`
	Priority               *int    `json:"priority"`
	Weight                 *int    `json:"weight"`
	TimeoutSeconds         *int    `json:"timeout_seconds"`
	CooldownSeconds        *int    `json:"cooldown_seconds"`
	MaxConsecutiveFailures *int    `json:"max_consecutive_failures"`
}

func (h *Handler) updateKey(c *gin.Context) {
	if !h.requireWrite(c) {
		return
	}
	id := c.Param("key_id")
	var req updateKeyRequest
	if !httpx.BindJSON(c, &req) {
		badRequest(c, "请求体不是有效的 JSON。")
		return
	}
	patch := KeyPatch{
		APIKey:                 req.APIKey,
		Name:                   req.Name,
		Model:                  req.Model,
		Enabled:                req.Enabled,
		Priority:               req.Priority,
		Weight:                 req.Weight,
		TimeoutSeconds:         req.TimeoutSeconds,
		CooldownSeconds:        req.CooldownSeconds,
		MaxConsecutiveFailures: req.MaxConsecutiveFailures,
	}
	// 提供的字符串先 TrimSpace；空/全空白 → 400，错误响应不含 Key 值。
	if patch.APIKey != nil {
		trimmed := strings.TrimSpace(*patch.APIKey)
		if trimmed == "" {
			badRequest(c, "api_key 不能为空或全为空白。")
			return
		}
		patch.APIKey = &trimmed
	}
	if patch.Name != nil {
		trimmed := strings.TrimSpace(*patch.Name)
		if trimmed == "" || len(trimmed) > 100 {
			badRequest(c, "name 不能为空且长度不能超过 100。")
			return
		}
		patch.Name = &trimmed
	}
	if patch.Model != nil {
		trimmed := strings.TrimSpace(*patch.Model)
		if trimmed == "" || len(trimmed) > 100 {
			badRequest(c, "model 不能为空且长度不能超过 100。")
			return
		}
		patch.Model = &trimmed
	}
	if !validatePatchLimits(c, patch) {
		return
	}
	updated, err := h.Store.Update(id, patch)
	if err != nil {
		h.writeStoreError(c, err)
		return
	}
	// 换 Key 或改 enabled 时复位运行态：旧的 auth_failed/cooling_down
	// 不再适用于新配置（unknown/disabled 由配置侧推导）。
	if patch.APIKey != nil || patch.Enabled != nil {
		h.State.Reset(updated.ID)
	}
	c.JSON(http.StatusOK, safeView(updated, h.State.Snapshot(updated)))
}

func (h *Handler) deleteKey(c *gin.Context) {
	if !h.requireWrite(c) {
		return
	}
	id := c.Param("key_id")
	if err := h.Store.Delete(id); err != nil {
		h.writeStoreError(c, err)
		return
	}
	h.State.Reset(id)
	c.Status(http.StatusNoContent)
}

func (h *Handler) createKeyTest(c *gin.Context) {
	if !h.requireWrite(c) {
		return
	}
	id := c.Param("key_id")
	if _, err := h.Store.Get(id); err != nil {
		h.writeStoreError(c, err)
		return
	}
	hash := httpx.HashRequest(c.Request.Method, c.FullPath(),
		httpx.CanonicalJSON(map[string]string{"key_id": id}), "")
	h.idem.Run(c, "createClaudeKeyTest", hash,
		func(ctx context.Context, rec *idempotency.Record) (int, any, error) {
			status, body, err := h.Jobs.CreateKeyConnectivity(ctx, rec, id)
			if err == nil {
				c.Header("Location", "/api/v1/jobs/"+body.Job.ID.String())
			}
			return status, body, err
		})
}

type claudeCodeSettingsResponse struct {
	Status string `json:"status"`
	Target string `json:"target"`
}

const claudeCodeSettingsTarget = "~/.claude/settings.json"

// saveClaudeCodeSettings 将指定已保存 Key 写入宿主机 Claude Code 设置。
// 浏览器仅发送 Key ID；完整 Key 始终留在 API 进程和目标文件中。
func (h *Handler) saveClaudeCodeSettings(c *gin.Context) {
	if !h.requireWrite(c) {
		return
	}
	key, err := h.Store.Get(c.Param("key_id"))
	if err != nil {
		h.writeStoreError(c, err)
		return
	}
	if err := writeClaudeCodeSettings(h.ClaudeCodeSettingsFile, key); err != nil {
		httpserver.AbortWithProblem(c, http.StatusServiceUnavailable, "claude_code_settings_unavailable",
			"Claude Code 配置不可用", "无法写入 Claude Code 设置；请检查本机 ~/.claude 目录挂载和权限。")
		return
	}
	c.JSON(http.StatusOK, claudeCodeSettingsResponse{Status: "saved", Target: claudeCodeSettingsTarget})
}

type claudeCodeSettingsDocument struct {
	Schema string `json:"$schema"`
	Env    struct {
		AnthropicBaseURL           string `json:"ANTHROPIC_BASE_URL"`
		AnthropicAuthToken         string `json:"ANTHROPIC_AUTH_TOKEN"`
		DisableNonessentialTraffic string `json:"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC"`
		AttributionHeader          string `json:"CLAUDE_CODE_ATTRIBUTION_HEADER"`
	} `json:"env"`
}

// writeClaudeCodeSettings 原子覆盖指定文件。调用方不得返回该错误原文，
// 因为它可能包含宿主机路径；完整 Key 只会被序列化到预期目标文件。
func writeClaudeCodeSettings(path string, key Key) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("Claude Code settings file is not configured")
	}
	doc := claudeCodeSettingsDocument{Schema: "https://json.schemastore.org/claude-code-settings.json"}
	doc.Env.AnthropicBaseURL = "https://app.haoshuai.cc.cd"
	doc.Env.AnthropicAuthToken = key.APIKey
	doc.Env.DisableNonessentialTraffic = "1"
	doc.Env.AttributionHeader = "0"
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, raw)
}

func (h *Handler) writeStoreError(c *gin.Context, err error) {
	if errors.Is(err, ErrNotFound) {
		httpserver.AbortWithProblem(c, http.StatusNotFound, "not_found",
			"资源不存在", "请求的 Claude Key 不存在或已删除。")
		return
	}
	if errors.Is(err, ErrInvalidKey) {
		badRequest(c, "api_key/name/model 不能为空。")
		return
	}
	httpx.WriteError(c, err)
}

func badRequest(c *gin.Context, detail string) {
	httpserver.AbortWithProblem(c, http.StatusBadRequest, "bad_request", "请求无效", detail)
}

// validateKeyFields 校验必填字符串；错误响应绝不回显 Key 值。
func validateKeyFields(c *gin.Context, apiKey, name, model string) bool {
	switch {
	case apiKey == "":
		badRequest(c, "api_key 不能为空或全为空白。")
	case name == "":
		badRequest(c, "name 不能为空。")
	case len(name) > 100:
		badRequest(c, "name 长度不能超过 100。")
	case model == "":
		badRequest(c, "model 不能为空。")
	case len(model) > 100:
		badRequest(c, "model 长度不能超过 100。")
	default:
		return true
	}
	return false
}

// validateKeyLimits 校验数值范围（与契约 ClaudeKeyCreate 对齐）。
func validateKeyLimits(c *gin.Context, k Key) bool {
	switch {
	case k.Priority < 0:
		badRequest(c, "priority 必须 >= 0。")
	case k.Weight < 1 || k.Weight > 1000:
		badRequest(c, "weight 必须在 1-1000 之间。")
	case k.TimeoutSeconds < 1 || k.TimeoutSeconds > 3600:
		badRequest(c, "timeout_seconds 必须在 1-3600 之间。")
	case k.CooldownSeconds < 1 || k.CooldownSeconds > 86400:
		badRequest(c, "cooldown_seconds 必须在 1-86400 之间。")
	case k.MaxConsecutiveFailures < 1 || k.MaxConsecutiveFailures > 100:
		badRequest(c, "max_consecutive_failures 必须在 1-100 之间。")
	default:
		return true
	}
	return false
}

func validatePatchLimits(c *gin.Context, p KeyPatch) bool {
	switch {
	case p.Priority != nil && *p.Priority < 0:
		badRequest(c, "priority 必须 >= 0。")
	case p.Weight != nil && (*p.Weight < 1 || *p.Weight > 1000):
		badRequest(c, "weight 必须在 1-1000 之间。")
	case p.TimeoutSeconds != nil && (*p.TimeoutSeconds < 1 || *p.TimeoutSeconds > 3600):
		badRequest(c, "timeout_seconds 必须在 1-3600 之间。")
	case p.CooldownSeconds != nil && (*p.CooldownSeconds < 1 || *p.CooldownSeconds > 86400):
		badRequest(c, "cooldown_seconds 必须在 1-86400 之间。")
	case p.MaxConsecutiveFailures != nil && (*p.MaxConsecutiveFailures < 1 || *p.MaxConsecutiveFailures > 100):
		badRequest(c, "max_consecutive_failures 必须在 1-100 之间。")
	default:
		return true
	}
	return false
}
