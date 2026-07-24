package httpserver

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
)

// Problem Details（RFC 9457 风格），Schema 与 api/openapi.yaml 的 Problem 一致。
// 稳定 code 见 docs/web-rebuild/04-api-contract.md §3；B1 中间件层补充：
// forbidden、method_not_allowed、service_unavailable、internal_error。

const problemBaseURL = "https://ai-study-tutor.local/problems/"

// Problem 是统一错误响应体。request_id 始终由中间件回填。
// Extensions 是 Problem Details 扩展成员（如 resource_busy 的
// active_job_id/generation_family），序列化时平铺到顶层。
type Problem struct {
	Type       string         `json:"type"`
	Title      string         `json:"title"`
	Status     int            `json:"status"`
	Detail     string         `json:"detail"`
	Instance   string         `json:"instance,omitempty"`
	Code       string         `json:"code"`
	RequestID  string         `json:"request_id"`
	Errors     []FieldError   `json:"errors,omitempty"`
	Extensions map[string]any `json:"-"`
}

// MarshalJSON 平铺扩展成员（Problem Schema additionalProperties: true）。
func (p Problem) MarshalJSON() ([]byte, error) {
	type alias Problem
	b, err := json.Marshal(alias(p))
	if err != nil {
		return nil, err
	}
	if len(p.Extensions) == 0 {
		return b, nil
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	for k, v := range p.Extensions {
		m[k] = v
	}
	return json.Marshal(m)
}

// FieldError 描述字段级校验失败（422 validation_error）。
type FieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// NewProblem 构造 Problem；code 自动转为 kebab-case type URI。
func NewProblem(status int, code, title, detail string) Problem {
	return Problem{
		Type:   problemBaseURL + strings.ReplaceAll(code, "_", "-"),
		Title:  title,
		Status: status,
		Detail: detail,
		Code:   code,
	}
}

func (p Problem) withInstance(instance string) Problem {
	p.Instance = instance
	return p
}

// WriteProblem 以 application/problem+json 输出错误；request_id 取自上下文。
func WriteProblem(c *gin.Context, p Problem) {
	p.RequestID = requestID(c)
	if p.Instance == "" {
		p.Instance = c.Request.URL.Path
	}
	c.Header("Content-Type", "application/problem+json")
	c.JSON(p.Status, p)
}

// AbortWithProblem 写出错误并终止后续 handler。
func AbortWithProblem(c *gin.Context, status int, code, title, detail string) {
	p := NewProblem(status, code, title, detail).withInstance(c.Request.URL.Path)
	WriteProblem(c, p)
	c.Abort()
}

// 常用 Problem 构造器（中间件层）。

func badRequestProblem(detail string) Problem {
	return NewProblem(400, "bad_request", "请求无效", detail)
}

func forbiddenProblem(detail string) Problem {
	return NewProblem(403, "forbidden", "请求被拒绝", detail)
}

func tooLargeProblem(detail string) Problem {
	return NewProblem(413, "upload_too_large", "上传超过限制", detail)
}

func internalErrorProblem() Problem {
	return NewProblem(500, "internal_error", "服务器内部错误",
		fmt.Sprintf("处理请求时发生未预期错误，请携带 request_id 查看服务日志。"))
}

func serviceUnavailableProblem(detail string) Problem {
	return NewProblem(503, "service_unavailable", "依赖不可用", detail)
}
