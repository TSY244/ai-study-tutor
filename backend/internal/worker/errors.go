// Package worker 实现 PostgreSQL 任务队列的 Worker Runner（03 §7、§8）：
//
//   - Handler registry：按 Job kind 分发；
//   - 阶段事件：Handler 经 Execution.Stage 上报稳定 stage code（03 §6.4）；
//   - 取消：轮询 cancel_requested，取消即杀子进程；
//   - 超时：每个 Job 有总时限（WORKER_JOB_TIMEOUT）；
//   - 按 job_id 隔离的临时目录（数据目录 tmp/{job_id}/）。
//
// Runner 不实现具体素材/AI 逻辑；lesson_import 管线见
// internal/worker/lessonimport。Claude 相关 Handler 在 B6 注册。
package worker

import (
	"errors"
)

// 稳定 Job 错误码（B5 素材管线子集；03 §7.3 重试分类）。
// 前端与重试策略只依赖错误码，不解析自然语言 detail。
const (
	// 获取阶段（acquiring）。
	ErrCodeURLBlocked          = "url_blocked"           // Worker 二次 SSRF 校验失败（不可重试）
	ErrCodeMaterialFetchFailed = "material_fetch_failed" // 本地下载/外部工具失败或 Claude 链接提取失败（按原因分类）
	ErrCodeTranscriptRequired  = "transcript_required"   // 无字幕且未配置转写（不可重试，可行动）
	ErrCodeToolMissing         = "tool_missing"          // 外部二进制缺失（不可重试）
	// 解析阶段（detecting/extracting/normalizing）。
	ErrCodeUnsupportedType = "material_unsupported_type" // 文件类型不支持（不可重试）
	ErrCodeParseFailed     = "material_parse_failed"     // 文件损坏/解析失败（不可重试）
	ErrCodeEmptyContent    = "material_empty_content"    // 提取后无有效文本（不可重试）
	ErrCodeTooLarge        = "material_too_large"        // 下载或文本超过上限（不可重试）
	// 发布与内部。
	ErrCodePublishFailed   = "publish_failed"       // 发布事务失败（可重试）
	ErrCodeJobTimeout      = "job_timeout"          // 任务总时限到期（可重试）
	ErrCodeInternal        = "internal_error"       // 未分类内部错误（可重试）
	ErrCodeUnsupportedKind = "unsupported_job_kind" // 该 Job kind 尚无 Handler（不可重试）
	// Claude/AI 阶段（B6）。这些值与 internal/ai.ProviderError.Code 保持一致。
	ErrCodeClaudeNotConfigured   = "claude_not_configured"
	ErrCodeClaudeAuthFailed      = "claude_auth_failed"
	ErrCodeClaudeRateLimited     = "claude_rate_limited"
	ErrCodeClaudeTimeout         = "claude_timeout"
	ErrCodeClaudeOverloaded      = "claude_overloaded"
	ErrCodeClaudeInvalidRequest  = "claude_invalid_request"
	ErrCodeClaudeRefused         = "claude_refused"
	ErrCodeClaudeOutputTruncated = "claude_output_truncated"
	ErrCodeClaudeInvalidJSON     = "claude_invalid_json"
	ErrCodeClaudeSchemaInvalid   = "claude_schema_invalid"
	ErrCodeClaudeDomainInvalid   = "claude_domain_invalid"
	ErrCodeClaudeUnavailable     = "claude_unavailable"
)

// Error 是 Handler 返回的分类错误：Runner 据此决定重试与终态。
// Detail 是脱敏摘要（可行动建议），不含材料正文、完整 URL 查询串或密钥。
type Error struct {
	Code      string
	Detail    string
	Retryable bool
	cause     error
}

// NewError 构造分类错误。
func NewError(code, detail string, retryable bool) *Error {
	return &Error{Code: code, Detail: detail, Retryable: retryable}
}

// WrapError 携带底层原因（仅用于日志，不进入 Job error_detail）。
func WrapError(code, detail string, retryable bool, cause error) *Error {
	return &Error{Code: code, Detail: detail, Retryable: retryable, cause: cause}
}

func (e *Error) Error() string {
	if e.cause != nil {
		return e.Code + ": " + e.Detail + "（原因: " + e.cause.Error() + "）"
	}
	return e.Code + ": " + e.Detail
}

// Unwrap 返回底层原因。
func (e *Error) Unwrap() error { return e.cause }

// Classify 把任意错误归一为非重试的内部错误；*Error 原样返回。
// job.ErrLeaseLost 由 Runner 单独处理（租约丢失时不允许再写任务状态）。
func Classify(err error) *Error {
	var we *Error
	if errors.As(err, &we) {
		return we
	}
	return &Error{Code: ErrCodeInternal, Detail: "任务执行发生内部错误，将自动重试。", Retryable: true, cause: err}
}
