package summary

import (
	"encoding/json"

	"github.com/google/uuid"

	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
	"github.com/TSY244/ai-study-tutor/backend/internal/job"
)

// DTO 是总结评估的 API 表示（契约 SummaryEvaluation）。
// 表 status 到契约枚举的映射：evaluating→evaluating、completed→succeeded、
// failed→failed（表无 queued：创建即 evaluating，恒如此映射）。
// summary_text 是用户内容，不属于契约字段，不得出现在 DTO 中。
type DTO struct {
	ID             uuid.UUID     `json:"id"`
	LessonID       uuid.UUID     `json:"lesson_id"`
	Status         string        `json:"status"`
	CoverageScore  *float64      `json:"coverage_score"`
	CorrectPoints  []string      `json:"correct_points"`
	Misconceptions []string      `json:"misconceptions"`
	MissingPoints  []string      `json:"missing_points"`
	Suggestions    []string      `json:"suggestions"`
	Error          *job.ErrorDTO `json:"error"`
	CreatedAt      string        `json:"created_at"`
	CompletedAt    *string       `json:"completed_at"`
}

// resultPayload 是 result JSONB 的两种形状：completed 时为 Claude 评估
// 五字段；failed 时为 {"code": ...}（worker 失败路径写入）。
type resultPayload struct {
	CoverageScore  *float64 `json:"coverage_score"`
	CorrectPoints  []string `json:"correct_points"`
	Misconceptions []string `json:"misconceptions"`
	MissingPoints  []string `json:"missing_points"`
	Suggestions    []string `json:"suggestions"`
	Code           string   `json:"code"`
}

// ToDTO 映射领域 Evaluation 到契约 DTO。进行中（evaluating）四数组为 []、
// coverage_score 为 null；completed 时从 result 解出五字段；failed 时
// error 优先取 jobs 错误列（与 job.ToDTO 的错误映射一致），缺失时回退
// result.code 与错误码重试分类。
func ToDTO(e *Evaluation) DTO {
	dto := DTO{
		ID:             e.ID,
		LessonID:       e.LessonID,
		Status:         contractStatus(e.Status),
		CorrectPoints:  []string{},
		Misconceptions: []string{},
		MissingPoints:  []string{},
		Suggestions:    []string{},
		CreatedAt:      httpx.Time(e.CreatedAt),
		CompletedAt:    httpx.TimePtr(e.CompletedAt),
	}
	var res resultPayload
	if len(e.Result) > 0 {
		_ = json.Unmarshal(e.Result, &res)
	}
	switch e.Status {
	case StatusCompleted:
		dto.CoverageScore = res.CoverageScore
		dto.CorrectPoints = nonNil(res.CorrectPoints)
		dto.Misconceptions = nonNil(res.Misconceptions)
		dto.MissingPoints = nonNil(res.MissingPoints)
		dto.Suggestions = nonNil(res.Suggestions)
	case StatusFailed:
		code := res.Code
		if e.ErrorCode != nil && *e.ErrorCode != "" {
			code = *e.ErrorCode
		}
		detail := ""
		if e.ErrorDetail != nil {
			detail = *e.ErrorDetail
		}
		retryable := job.ClassifyRetryable(code)
		if e.ErrorRetryable != nil {
			retryable = *e.ErrorRetryable
		}
		dto.Error = &job.ErrorDTO{Code: code, Detail: detail, Retryable: retryable}
	}
	return dto
}

// contractStatus 映射表 status 到契约 SummaryEvaluationStatus。
func contractStatus(status string) string {
	switch status {
	case StatusCompleted:
		return "succeeded"
	case StatusFailed:
		return "failed"
	default:
		return "evaluating"
	}
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
