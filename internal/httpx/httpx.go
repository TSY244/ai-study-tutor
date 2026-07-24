// Package httpx 提供 B3+ 业务 Handler 共用的 HTTP 辅助：时间格式、
// 游标分页、请求绑定、稳定错误码映射与幂等执行器。
// Gin 类型只允许出现在 handler 层（本包与模块 handler）。
package httpx

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/TSY244/ai-study-tutor/internal/filestore"
	"github.com/TSY244/ai-study-tutor/internal/platform/gate"
	"github.com/TSY244/ai-study-tutor/internal/platform/httpserver"
)

// Time 输出契约要求的 UTC RFC 3339 时间字符串。
func Time(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// TimePtr 输出可空时间。
func TimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := Time(*t)
	return &s
}

// PageInfo 是游标分页元数据（04 §2.4）。
type PageInfo struct {
	NextCursor *string `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
}

// Collection 是分页响应信封。
type Collection[T any] struct {
	Items    []T      `json:"items"`
	PageInfo PageInfo `json:"page_info"`
}

// EncodeCursor 生成不透明游标（base64url JSON；前端不得解析）。
func EncodeCursor(v map[string]any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// DecodeCursor 解析游标；非法游标返回 false（映射 400 bad_request）。
func DecodeCursor(s string, dst *map[string]any) bool {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return false
	}
	return json.Unmarshal(b, dst) == nil
}

// ParseLimit 解析 limit 查询参数：默认 20，最大 100；非法值返回 false。
func ParseLimit(c *gin.Context) (int, bool) {
	raw := c.DefaultQuery("limit", "")
	if raw == "" {
		return 20, true
	}
	var n int
	if _, err := fmt.Sscanf(raw, "%d", &n); err != nil || n < 1 || n > 100 {
		return 0, false
	}
	return n, true
}

// BindJSON 解析 JSON 请求体：未知字段（additionalProperties:false 对齐）、
// 空体、畸形 JSON 均返回 false，由调用方映射 400 bad_request。
func BindJSON(c *gin.Context, dst any) bool {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil || len(bytes.TrimSpace(body)) == 0 {
		return false
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return false
	}
	return true
}

// WriteValidation 输出 422 validation_error（含字段级错误列表）。
func WriteValidation(c *gin.Context, errs []httpserver.FieldError) {
	p := httpserver.NewProblem(http.StatusUnprocessableEntity, "validation_error",
		"请求参数无效", "请修正标记的字段后重试。")
	p.Errors = errs
	httpserver.WriteProblem(c, p)
}

// FieldErr 构造字段错误。
func FieldErr(field, code, message string) httpserver.FieldError {
	return httpserver.FieldError{Field: field, Code: code, Message: message}
}

// WriteError 把领域哨兵错误映射为稳定 Problem（04 §3 错误码表）。
// 未识别错误映射 500 internal_error（细节只进服务端日志）。
func WriteError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	var pe *ProblemError
	if errors.As(err, &pe) {
		httpserver.WriteProblem(c, pe.Problem)
		return
	}
	switch {
	// 注意：course/lesson/document/quiz/attempt 的哨兵错误不在此映射——这些
	// 模块依赖本包（IdemExecutor、ProblemError 等），反向引用会造成包循环；
	// 它们的 404/409 由各自 service 转为 ProblemError（响应形状一致）。
	case errors.Is(err, filestore.ErrNotFound):
		httpserver.AbortWithProblem(c, 404, "not_found", "资源不存在", "请求的资源不存在或已删除。")
	case errors.Is(err, gate.ErrMaintenanceMode):
		httpserver.AbortWithProblem(c, 503, "maintenance_mode", "系统维护中", "系统正在执行一致性备份等维护操作，暂时拒绝写入。")
	case errors.Is(err, filestore.ErrPathEscape):
		httpserver.AbortWithProblem(c, 400, "bad_request", "请求无效", "文件路径非法。")
	default:
		httpserver.AbortWithProblem(c, 500, "internal_error", "服务器内部错误",
			"处理请求时发生未预期错误，请携带 request_id 查看服务日志。")
	}
}

// ProblemError 允许 service 携带完整 Problem（含扩展字段）向上传递。
type ProblemError struct {
	Problem httpserver.Problem
}

func (e *ProblemError) Error() string { return e.Problem.Code }

// NewProblemError 包装一个 Problem 为 error（service 层使用）。
func NewProblemError(status int, code, title, detail string) *ProblemError {
	return &ProblemError{Problem: httpserver.NewProblem(status, code, title, detail)}
}
