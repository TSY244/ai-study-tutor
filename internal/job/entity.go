// Package job 实现 PostgreSQL 任务队列的领域模型与 Repository。
// 队列车机制见 03 §7：SKIP LOCKED 领取、租约/心跳、指数退避、
// 取消与 worker_lost 回收。Job Handler 编排在 B5 落地。
package job

import (
	"time"

	"github.com/google/uuid"
)

// Job kind（03 §6.4）。该列表是 API 契约，新增值向后兼容，不得改变既有语义。
const (
	KindClaudeConnectivityTest = "claude_connectivity_test"
	KindLessonImport           = "lesson_import"
	KindLessonRegenerate       = "lesson_regenerate"
	KindQuizGenerate           = "quiz_generate"
	KindReviewQuizGenerate     = "review_quiz_generate"
	KindAttemptGrade           = "attempt_grade"
	KindWrongAnswerAnalyze     = "wrong_answer_analyze"
	KindSummaryEvaluate        = "summary_evaluate"
	KindLessonExport           = "lesson_export"
)

// GenerationFamilyLessonImport 族：同一 Lesson 的内容变更与出题互斥。
const GenerationFamilyLessonGeneration = "lesson_generation"

// Job 状态机（Job 状态表，B2 交接物）：
//
//	queued           初始态；available_at 到达后可被领取
//	running          已被 Worker 领取，持有有效租约
//	validating       结果校验/发布窗口，仍持有租约
//	cancel_requested 用户请求取消，Worker 在下一检查点终止
//	succeeded        终态：成功
//	failed           终态：失败（error_code 给出稳定原因）
//	cancelled        终态：已取消
const (
	StatusQueued          = "queued"
	StatusRunning         = "running"
	StatusValidating      = "validating"
	StatusCancelRequested = "cancel_requested"
	StatusSucceeded       = "succeeded"
	StatusFailed          = "failed"
	StatusCancelled       = "cancelled"
)

// 稳定 Job 错误码（B2 子集；素材/Claude 错误码在 B5/B6 补充）。
// 前端与重试策略只依赖错误码，不解析自然语言 detail。
const (
	// ErrCodeWorkerLost：租约过期且自动重试次数耗尽，Worker 被判定丢失。
	ErrCodeWorkerLost = "worker_lost"
	// ErrCodeMaintenanceLeaseExpired：维护 Owner 租约过期，备份被回收标记。
	ErrCodeMaintenanceLeaseExpired = "maintenance_lease_expired"
)

// 持久化 Job 事件类型（04 §14 SSE 契约的子集可持久化部分）。
const (
	EventQueued          = "job.queued"
	EventStarted         = "job.started"
	EventProgress        = "job.progress"
	EventValidating      = "job.validating"
	EventCancelRequested = "job.cancel_requested"
	EventSucceeded       = "job.succeeded"
	EventFailed          = "job.failed"
	EventCancelled       = "job.cancelled"
)

// Job 是 jobs 表的领域表示。指针字段对应可空列。
type Job struct {
	ID               uuid.UUID
	Kind             string
	AggregateType    string
	AggregateID      uuid.UUID
	GenerationFamily *string
	Status           string
	Stage            string
	Progress         int16
	Payload          []byte // JSONB
	Result           []byte // JSONB，可空
	ErrorCode        *string
	ErrorDetail      *string
	ErrorRetryable   *bool
	Priority         int32
	AttemptCount     int32
	MaxAttempts      int32
	AvailableAt      time.Time
	LeaseOwner       *string
	LeaseExpiresAt   *time.Time
	CancelRequested  bool
	RetryOfJobID     *uuid.UUID
	IdempotencyScope *string
	IdempotencyKey   *string
	CreatedAt        time.Time
	StartedAt        *time.Time
	CompletedAt      *time.Time
	UpdatedAt        time.Time
}

// Event 是 job_events 表的领域表示；BIGSERIAL id 同时是 SSE Event ID。
type Event struct {
	ID        int64
	JobID     uuid.UUID
	EventType string
	Stage     string
	Progress  int16
	Message   string
	Data      []byte
	CreatedAt time.Time
}

// Active 返回 Job 是否处于活动状态（占用聚合根互斥约束的状态集合）。
func (j *Job) Active() bool {
	switch j.Status {
	case StatusQueued, StatusRunning, StatusValidating, StatusCancelRequested:
		return true
	default:
		return false
	}
}
