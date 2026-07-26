package claudegateway

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	aipkg "github.com/TSY244/ai-study-tutor/backend/internal/ai"
)

// Router 是实现 ai.Provider 的 Claude 多 Key 网关路由：
//
//  1. 加载启用 Key，排除 disabled/auth_failed/冷却中；
//  2. 按 priority 升序分组，组内加权轮询（平滑 WRR）排序；
//  3. 每个逻辑请求每个 Key 最多尝试一次；
//  4. 401/403 → auth_failed（不自动恢复）；429/5xx/超时/网络错误 →
//     cooling_down；refusal/max_tokens/invalid_json 不故障转移；
//  5. 无可用 Key 时回退 Legacy（旧 ANTHROPIC_API_KEY_FILE 单 Key 或
//     NotConfiguredProvider），保持既有错误码语义；
//  6. GatewayOnly() 提供无 Legacy 回退的 URL 导入专用 Provider：无启用
//     Key 时返回 claude_not_configured，绝不触碰 Legacy。
type Router struct {
	Store *Store
	State *StateTracker

	// Legacy 是 keys.json 无启用 Key 时的回退 Provider；
	// nil 时使用 ai.NotConfiguredProvider。
	Legacy aipkg.Provider

	// Endpoint 是进程级默认 Anthropic 端点；Key.base_url 非空时优先于它。
	Endpoint string
	// HTTPClient 覆盖每次尝试使用的 HTTP 客户端（测试注入）。
	HTTPClient *http.Client
	// DefaultModel 是 req.Model 与 key.Model 均为空时的兜底。
	DefaultModel string
	// MaxOutputTokens 来自进程配置（CLAUDE_MAX_OUTPUT_TOKENS）。
	MaxOutputTokens int
	// Logger 可选；只记录非敏感事件，绝不记录 Key。
	Logger *slog.Logger

	mu sync.Mutex
	rr map[string]int // 平滑加权轮询游标（key ID → current weight）
}

var _ aipkg.Provider = (*Router)(nil)

// Generate 执行一次带故障转移的结构化生成。
func (r *Router) Generate(ctx context.Context, req aipkg.Request) (aipkg.Response, error) {
	return r.route(ctx, req, true)
}

// GatewayOnly 返回只用网关启用 Key 的 ai.Provider 包装（URL 导入专用）：
// 无启用 Key 时返回 claude_not_configured，绝不回退 Router.Legacy，防止
// 运行期 Key 被移除后 URL 导入静默落到 legacy 直连 Key。
func (r *Router) GatewayOnly() aipkg.Provider {
	return gatewayOnlyProvider{router: r}
}

type gatewayOnlyProvider struct {
	router *Router
}

var _ aipkg.Provider = gatewayOnlyProvider{}

func (p gatewayOnlyProvider) Generate(ctx context.Context, req aipkg.Request) (aipkg.Response, error) {
	return p.router.route(ctx, req, false)
}

// route 是统一的路由实现；allowLegacyFallback=false 时无启用 Key 直接返回
// claude_not_configured，绝不调用 Legacy。
func (r *Router) route(ctx context.Context, req aipkg.Request, allowLegacyFallback bool) (aipkg.Response, error) {
	keys, err := r.Store.Load()
	if err != nil {
		r.warn("claude_gateway_config_load_failed", err)
		return aipkg.Response{}, aipkg.WrapError(aipkg.ErrUnavailable,
			"读取 Claude 网关配置失败，请稍后重试。", err)
	}
	now := r.State.Now()
	candidates := make([]Key, 0, len(keys))
	hasEnabled := false
	for _, k := range keys {
		if k.Enabled {
			hasEnabled = true
		}
		if r.State.Eligible(k, now) {
			candidates = append(candidates, k)
		}
	}
	if !hasEnabled {
		if allowLegacyFallback {
			return r.legacy().Generate(ctx, req)
		}
		return aipkg.Response{}, aipkg.ErrorForCode(aipkg.ErrNotConfigured,
			"Claude 网关没有启用的 Key；请在网关配置中启用 Key 后重试。")
	}
	if len(candidates) == 0 {
		return aipkg.Response{}, r.noEligibleError(keys)
	}
	groups := priorityGroups(candidates)
	r.pruneWeightedState(candidates)
	var lastResp aipkg.Response
	var lastErr error
	for _, group := range groups {
		for _, key := range r.attemptGroupOrder(group) {
			resp, callErr := r.attempt(ctx, key, req)
			// 进程取消/关闭不污染后续 Key 状态判定，直接返回。
			if ctx.Err() != nil {
				return resp, callErr
			}
			r.recordOutcome(key, resp, callErr)
			if callErr == nil {
				return resp, nil
			}
			perr := aipkg.AsProviderError(callErr)
			if !failoverEligible(perr.Code) {
				return resp, callErr
			}
			lastResp, lastErr = resp, callErr
		}
	}
	// 全部候选都失败：返回最后一次尝试的既有 ProviderError 语义。
	return lastResp, lastErr
}

func (r *Router) noEligibleError(keys []Key) error {
	allAuthFailed := true
	for _, key := range keys {
		if !key.Enabled {
			continue
		}
		if r.State.Snapshot(key).Status != StatusAuthFailed {
			allAuthFailed = false
			break
		}
	}
	if allAuthFailed {
		return aipkg.ErrorForCode(aipkg.ErrAuthFailed,
			"Claude 网关的已启用 Key 均未通过认证；请更新或手动测试 Key。")
	}
	return aipkg.ErrorForCode(aipkg.ErrUnavailable,
		"Claude 网关当前没有可用 Key；请等待冷却结束或检查配置。")
}

// GenerateForKey 用指定 Key 执行一次调用（Worker 单 Key 连通性测试）。
// 手动测试是 auth_failed 的复位路径：先标记 testing，再按结果落态。
func (r *Router) GenerateForKey(ctx context.Context, keyID string, req aipkg.Request) (aipkg.Response, error) {
	key, err := r.Store.Get(keyID)
	if errors.Is(err, ErrNotFound) {
		return aipkg.Response{}, aipkg.ErrorForCode(aipkg.ErrNotConfigured,
			"指定的 Claude Key 不存在或已删除。")
	}
	if err != nil {
		return aipkg.Response{}, aipkg.WrapError(aipkg.ErrUnavailable,
			"读取 Claude 网关配置失败，请稍后重试。", err)
	}
	r.State.MarkTestingForKey(key)
	resp, callErr := r.attempt(ctx, key, req)
	if ctx.Err() != nil {
		return resp, callErr
	}
	r.recordOutcome(key, resp, callErr)
	return resp, callErr
}

// EnabledKeyCount 返回当前启用 Key 数（启动期能力判定与心跳用）。
func (r *Router) EnabledKeyCount() int {
	keys, err := r.Store.Load()
	if err != nil {
		return 0
	}
	n := 0
	for _, k := range keys {
		if k.Enabled {
			n++
		}
	}
	return n
}

func (r *Router) legacy() aipkg.Provider {
	if r.Legacy != nil {
		return r.Legacy
	}
	return aipkg.NotConfiguredProvider{}
}

// attempt 用选中的 Key 现场构造 Anthropic Provider 并发起一次调用。
// Model 解析：req.Model > key.Model > DefaultModel。
// 端点解析：Key.base_url（补全 /v1/messages）> r.Endpoint。
func (r *Router) attempt(ctx context.Context, key Key, req aipkg.Request) (aipkg.Response, error) {
	model := req.Model
	if model == "" {
		model = key.Model
	}
	if model == "" {
		model = r.DefaultModel
	}
	timeout := key.TimeoutSeconds
	if timeout <= 0 {
		timeout = DefaultTimeoutSeconds
	}
	endpoint := r.Endpoint
	// Store.Load 不归一化：手改 keys.json 带入的纯空白 base_url 视为未设置。
	if base := strings.TrimSpace(key.BaseURL); base != "" {
		endpoint = messagesEndpoint(base)
	}
	provider := aipkg.NewAnthropicProvider(aipkg.AnthropicConfig{
		APIKey:          key.APIKey,
		Model:           model,
		Endpoint:        endpoint,
		MaxOutputTokens: r.MaxOutputTokens,
		Timeout:         time.Duration(timeout) * time.Second,
		HTTPClient:      r.HTTPClient,
	})
	return provider.Generate(ctx, req)
}

// messagesEndpoint 把基础 URL 归一为完整 Messages 端点；规则与
// config.ClaudeConfig.AnthropicMessagesEndpoint 一致。
func messagesEndpoint(base string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if strings.HasSuffix(base, "/v1/messages") {
		return base
	}
	return base + "/v1/messages"
}

// recordOutcome 按错误码语义更新 Key 运行态。
func (r *Router) recordOutcome(key Key, resp aipkg.Response, callErr error) {
	if callErr == nil {
		r.State.MarkSuccessForKey(key, resp.Latency)
		return
	}
	perr := aipkg.AsProviderError(callErr)
	switch {
	case perr.Code == aipkg.ErrAuthFailed:
		r.State.MarkAuthFailedForKey(key, perr.Code, resp.Latency)
	case failoverEligible(perr.Code):
		r.State.MarkTransientFailure(key, perr.Code, resp.Latency)
	default:
		r.State.MarkNonFailoverFailureForKey(key, perr.Code, resp.Latency)
	}
}

// failoverEligible 报告错误码是否允许故障转移到下一个 Key。
func failoverEligible(code string) bool {
	switch code {
	case aipkg.ErrAuthFailed, aipkg.ErrRateLimited, aipkg.ErrOverloaded,
		aipkg.ErrTimeout, aipkg.ErrUnavailable:
		return true
	default:
		return false
	}
}

// priorityGroups 按 priority 升序分组；各组只在 Generate 实际进入时
// 才推进自己的加权轮询状态。
func priorityGroups(candidates []Key) [][]Key {
	byPriority := map[int][]Key{}
	priorities := []int{}
	for _, k := range candidates {
		if _, ok := byPriority[k.Priority]; !ok {
			priorities = append(priorities, k.Priority)
		}
		byPriority[k.Priority] = append(byPriority[k.Priority], k)
	}
	sort.Ints(priorities)
	groups := make([][]Key, 0, len(priorities))
	for _, priority := range priorities {
		groups = append(groups, byPriority[priority])
	}
	return groups
}

func (r *Router) pruneWeightedState(candidates []Key) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.rr == nil {
		r.rr = map[string]int{}
	}
	// 清理已不在候选集中的游标，避免无界增长。
	alive := map[string]bool{}
	for _, k := range candidates {
		alive[k.ID] = true
	}
	for id := range r.rr {
		if !alive[id] {
			delete(r.rr, id)
		}
	}
}

func (r *Router) attemptGroupOrder(group []Key) []Key {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.rr == nil {
		r.rr = map[string]int{}
	}
	return r.weightedOrderLocked(group, true)
}

// weightedOrderLocked 对同优先级组做平滑加权轮询排序：
// persistFirst 只对整个逻辑请求的首选 Key 推进一次持久游标；其余位置
// 仅用局部副本生成故障转移顺序，不影响下一次请求的加权选择。
func (r *Router) weightedOrderLocked(group []Key, persistFirst bool) []Key {
	type item struct {
		key     Key
		current int
	}
	items := make([]item, len(group))
	for i, k := range group {
		items[i] = item{key: k, current: r.rr[k.ID]}
	}
	order := make([]Key, 0, len(group))
	for len(items) > 0 {
		total := 0
		best := 0
		for i := range items {
			w := items[i].key.Weight
			if w <= 0 {
				w = 1
			}
			items[i].current += w
			total += w
			if items[i].current > items[best].current {
				best = i
			}
		}
		items[best].current -= total
		if len(order) == 0 && persistFirst {
			for _, item := range items {
				r.rr[item.key.ID] = item.current
			}
		}
		order = append(order, items[best].key)
		items = append(items[:best], items[best+1:]...)
	}
	return order
}

func (r *Router) warn(msg string, err error) {
	if r.Logger != nil {
		r.Logger.Warn(msg, slog.String("error", err.Error()))
	}
}
