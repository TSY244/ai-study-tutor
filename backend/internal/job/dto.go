package job

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ErrorDTO 是 Job 的稳定错误对象（契约 JobError）。
type ErrorDTO struct {
	Code      string `json:"code"`
	Detail    string `json:"detail"`
	Retryable bool   `json:"retryable"`
}

// DTO 是 Job 的 API 表示（契约 Job Schema；status 枚举与数据库一致：
// queued/running/validating/succeeded/failed/cancel_requested/cancelled）。
// message 取自最近一条事件摘要，可为 null。
type DTO struct {
	ID               uuid.UUID  `json:"id"`
	Kind             string     `json:"kind"`
	AggregateType    *string    `json:"aggregate_type"`
	AggregateID      *uuid.UUID `json:"aggregate_id"`
	GenerationFamily *string    `json:"generation_family"`
	RetryOfJobID     *uuid.UUID `json:"retry_of_job_id"`
	Status           string     `json:"status"`
	Stage            string     `json:"stage"`
	Progress         int16      `json:"progress"`
	Message          *string    `json:"message"`
	AttemptCount     int32      `json:"attempt_count"`
	MaxAttempts      int32      `json:"max_attempts"`
	Retryable        bool       `json:"retryable"`
	CancelRequested  bool       `json:"cancel_requested"`
	Error            *ErrorDTO  `json:"error"`
	CreatedAt        string     `json:"created_at"`
	StartedAt        *string    `json:"started_at"`
	CompletedAt      *string    `json:"completed_at"`
	UpdatedAt        string     `json:"updated_at"`
}

// ToDTO 映射领域 Job 到 API 表示。message 为最近事件摘要（可空）。
// retryable 是动作可用性：仅终态 failed 的任务可手动重试（04 §13）。
// Error.Retryable 是错误分类：worker_lost/瞬态错误归类为可重试。
func ToDTO(j *Job, message *string) DTO {
	dto := DTO{
		ID:               j.ID,
		Kind:             j.Kind,
		AggregateType:    &j.AggregateType,
		AggregateID:      &j.AggregateID,
		GenerationFamily: j.GenerationFamily,
		RetryOfJobID:     j.RetryOfJobID,
		Status:           j.Status,
		Stage:            j.Stage,
		Progress:         j.Progress,
		Message:          message,
		AttemptCount:     j.AttemptCount,
		MaxAttempts:      j.MaxAttempts,
		Retryable:        j.Status == StatusFailed,
		CancelRequested:  j.CancelRequested,
		CreatedAt:        formatTime(j.CreatedAt),
		StartedAt:        formatTimePtr(j.StartedAt),
		CompletedAt:      formatTimePtr(j.CompletedAt),
		UpdatedAt:        formatTime(j.UpdatedAt),
	}
	if j.ErrorCode != nil {
		detail := ""
		if j.ErrorDetail != nil {
			detail = *j.ErrorDetail
		}
		retryable := classifyRetryable(*j.ErrorCode)
		if j.ErrorRetryable != nil {
			retryable = *j.ErrorRetryable
		}
		dto.Error = &ErrorDTO{
			Code:      *j.ErrorCode,
			Detail:    detail,
			Retryable: retryable,
		}
	}
	return dto
}

// EventDTO 是 REST /jobs/{job_id}/events 的审计事件 DTO。
// 它故意不同于 SSE JobStreamData：REST 保留 event_type/data 审计字段，
// SSE 只发布前端进度所需的脱敏快照。
type EventDTO struct {
	ID         int64          `json:"id"`
	JobID      uuid.UUID      `json:"job_id"`
	EventType  string         `json:"event_type"`
	Stage      string         `json:"stage"`
	Progress   int16          `json:"progress"`
	Message    *string        `json:"message"`
	Data       map[string]any `json:"data"`
	OccurredAt string         `json:"occurred_at"`
}

// ToEventDTO 映射持久化事件到 REST 审计 DTO。
func ToEventDTO(e Event) EventDTO {
	var data map[string]any
	if len(e.Data) > 0 {
		_ = json.Unmarshal(e.Data, &data)
	}
	if data == nil {
		data = map[string]any{}
	}
	return EventDTO{
		ID:         e.ID,
		JobID:      e.JobID,
		EventType:  e.EventType,
		Stage:      e.Stage,
		Progress:   e.Progress,
		Message:    nullableMessage(e.Message),
		Data:       data,
		OccurredAt: formatTime(e.CreatedAt),
	}
}

// JobStreamData 是 PersistedJobStreamEvent.data；不包含 payload/result、
// event.data、学习正文、答案、Prompt 或上游响应。
type JobStreamData struct {
	JobID      uuid.UUID `json:"job_id"`
	Status     string    `json:"status"`
	Stage      string    `json:"stage"`
	Progress   int16     `json:"progress"`
	Message    *string   `json:"message"`
	OccurredAt string    `json:"occurred_at"`
}

// StreamFrame 是 Handler 写出 SSE 帧的协议无关表示。
type StreamFrame struct {
	ID    *int64
	Event string
	Data  any
}

// JobResyncRequired 是 stream.resync_required 的 data。
type JobResyncRequired struct {
	Reason               string `json:"reason"`
	HighWatermarkEventID int64  `json:"high_watermark_event_id"`
	ActiveJobsURL        string `json:"active_jobs_url"`
}

// HeartbeatStreamData 是 heartbeat 的 data；SSE 帧不携带 id。
type HeartbeatStreamData struct {
	ServerTime string `json:"server_time"`
}

// ToJobStreamData 把持久化事件映射为前端可安全展示的进度快照。
func ToJobStreamData(e Event) JobStreamData {
	return JobStreamData{
		JobID:      e.JobID,
		Status:     statusForEvent(e.EventType, e.Stage),
		Stage:      e.Stage,
		Progress:   e.Progress,
		Message:    nullableMessage(e.Message),
		OccurredAt: formatTime(e.CreatedAt),
	}
}

func nullableMessage(message string) *string {
	if message == "" {
		return nil
	}
	return &message
}

func statusForEvent(eventType, stage string) string {
	switch eventType {
	case EventQueued:
		return StatusQueued
	case EventValidating:
		return StatusValidating
	case EventCancelRequested:
		return StatusCancelRequested
	case EventSucceeded:
		return StatusSucceeded
	case EventFailed:
		return StatusFailed
	case EventCancelled:
		return StatusCancelled
	case EventProgress:
		if stage == "publishing" {
			return StatusValidating
		}
		return StatusRunning
	default:
		return StatusRunning
	}
}

// classifyRetryable 是错误码的重试分类（03 §7.3；B5/B6 补充素材/Claude 码）。
func classifyRetryable(code string) bool {
	switch code {
	case ErrCodeWorkerLost,
		ErrCodeMaintenanceLeaseExpired,
		"material_fetch_failed",
		"publish_failed",
		"job_timeout",
		"internal_error",
		"claude_rate_limited",
		"claude_timeout",
		"claude_overloaded",
		"claude_invalid_json",
		"claude_schema_invalid",
		"claude_domain_invalid",
		"claude_unavailable":
		return true
	default:
		return false
	}
}

func formatTime(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func formatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := formatTime(*t)
	return &s
}
