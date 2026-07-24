package job

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/TSY244/ai-study-tutor/internal/platform/database"
	"github.com/TSY244/ai-study-tutor/internal/platform/gate"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// 哨兵错误：由调用方映射到稳定错误码（B3 起映射 Problem）。
var (
	// ErrActiveGenerationJobExists：同一 Lesson 已有活动 lesson_generation Job
	// （部分唯一索引 uq_jobs_active_lesson_generation_family 冲突），映射 409 resource_busy。
	ErrActiveGenerationJobExists = errors.New("该课时已有进行中的生成任务")
	// ErrActiveAttemptGradeExists：同一 Attempt 已有活动批阅 Job
	// （部分唯一索引 uq_jobs_active_attempt_grade 冲突），映射 409 resource_busy。
	ErrActiveAttemptGradeExists = errors.New("该作答已有进行中的批阅任务")
	// ErrDuplicateJob：jobs 级幂等键冲突（uq_jobs_idempotency）。
	ErrDuplicateJob = errors.New("相同幂等键的任务已存在")
	// ErrLeaseLost：租约不属于调用方或已过期，Worker 必须放弃该 Job。
	ErrLeaseLost = errors.New("任务租约已丢失或过期")
	// ErrNotCancellable：终态任务不能取消。
	ErrNotCancellable = errors.New("任务已结束，不能取消")
	// ErrNotFound：任务不存在。
	ErrNotFound = errors.New("任务不存在")
)

// BackoffFunc 计算第 attempt 次执行（从 1 计）失败后的重试延迟。
type BackoffFunc func(attempt int32) time.Duration

// DefaultBackoff 返回指数退避（5s 起步、封顶 5min、加 0–5s 抖动）。
func DefaultBackoff(attempt int32) time.Duration {
	const base = 5 * time.Second
	const cap = 5 * time.Minute
	d := base << max(attempt-1, 0)
	if d > cap || d <= 0 {
		d = cap
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(base)))
	if err == nil {
		d += time.Duration(n.Int64())
	}
	return d
}

// Repository 是无状态 Job 队列 Repository；所有方法接受 database.Querier，
// 由调用方决定独立执行还是组合进更大的事务边界。
type Repository struct {
	Backoff BackoffFunc
}

func (r *Repository) backoff() BackoffFunc {
	if r.Backoff != nil {
		return r.Backoff
	}
	return DefaultBackoff
}

const columns = `id, kind, aggregate_type, aggregate_id, generation_family,
	status, stage, progress, payload, result, error_code, error_detail, error_retryable,
	priority, attempt_count, max_attempts, available_at,
	lease_owner, lease_expires_at, cancel_requested, retry_of_job_id,
	idempotency_scope, idempotency_key,
	created_at, started_at, completed_at, updated_at`

func scanJob(row pgx.Row) (*Job, error) {
	var j Job
	err := row.Scan(
		&j.ID, &j.Kind, &j.AggregateType, &j.AggregateID, &j.GenerationFamily,
		&j.Status, &j.Stage, &j.Progress, &j.Payload, &j.Result, &j.ErrorCode, &j.ErrorDetail, &j.ErrorRetryable,
		&j.Priority, &j.AttemptCount, &j.MaxAttempts, &j.AvailableAt,
		&j.LeaseOwner, &j.LeaseExpiresAt, &j.CancelRequested, &j.RetryOfJobID,
		&j.IdempotencyScope, &j.IdempotencyKey,
		&j.CreatedAt, &j.StartedAt, &j.CompletedAt, &j.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &j, nil
}

// JobWithMessage 是 Job 与最近一条事件摘要的查询视图。
type JobWithMessage struct {
	Job
	Message *string
}

func scanJobWithMessage(row pgx.Row) (*JobWithMessage, error) {
	var j JobWithMessage
	err := row.Scan(
		&j.ID, &j.Kind, &j.AggregateType, &j.AggregateID, &j.GenerationFamily,
		&j.Status, &j.Stage, &j.Progress, &j.Payload, &j.Result, &j.ErrorCode, &j.ErrorDetail, &j.ErrorRetryable,
		&j.Priority, &j.AttemptCount, &j.MaxAttempts, &j.AvailableAt,
		&j.LeaseOwner, &j.LeaseExpiresAt, &j.CancelRequested, &j.RetryOfJobID,
		&j.IdempotencyScope, &j.IdempotencyKey,
		&j.CreatedAt, &j.StartedAt, &j.CompletedAt, &j.UpdatedAt,
		&j.Message,
	)
	if err != nil {
		return nil, err
	}
	return &j, nil
}

func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == constraint
}

// Create 在调用方事务中创建 Job 并写入 job.queued 事件。
// 创建事务直接依赖 lesson_generation 部分唯一索引（03 §6.4）。
func (r *Repository) Create(ctx context.Context, q database.Querier, j *Job) error {
	if j.ID == uuid.Nil {
		j.ID = uuid.New()
	}
	if j.MaxAttempts <= 0 {
		j.MaxAttempts = 3
	}
	j.Status = StatusQueued
	j.Stage = StatusQueued
	j.Progress = 0
	_, err := q.Exec(ctx, `
		INSERT INTO jobs (id, kind, aggregate_type, aggregate_id, generation_family,
			payload, priority, max_attempts, retry_of_job_id,
			idempotency_scope, idempotency_key)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		j.ID, j.Kind, j.AggregateType, j.AggregateID, j.GenerationFamily,
		j.Payload, j.Priority, j.MaxAttempts, j.RetryOfJobID,
		j.IdempotencyScope, j.IdempotencyKey)
	if err != nil {
		switch {
		case isUniqueViolation(err, "uq_jobs_active_lesson_generation_family"):
			return ErrActiveGenerationJobExists
		case isUniqueViolation(err, "uq_jobs_active_attempt_grade"):
			return ErrActiveAttemptGradeExists
		case isUniqueViolation(err, "uq_jobs_idempotency"):
			return ErrDuplicateJob
		default:
			return fmt.Errorf("创建任务失败: %w", err)
		}
	}
	return r.addEvent(ctx, q, j.ID, EventQueued, StatusQueued, 0, "任务已入队", nil)
}

// GetByID 读取单个 Job。
func (r *Repository) GetByID(ctx context.Context, q database.Querier, id uuid.UUID) (*Job, error) {
	j, err := scanJob(q.QueryRow(ctx, `SELECT `+columns+` FROM jobs WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return j, err
}

// GetWithMessage 读取单个 Job，并附带最近事件摘要（Job.message DTO）。
func (r *Repository) GetWithMessage(ctx context.Context, q database.Querier, id uuid.UUID) (*JobWithMessage, error) {
	j, err := scanJobWithMessage(q.QueryRow(ctx, `
		SELECT `+columnsPrefixed("j")+`, ev.message
		FROM jobs j
		LEFT JOIN LATERAL (
		  SELECT message FROM job_events
		  WHERE job_id = j.id
		  ORDER BY id DESC
		  LIMIT 1
		) ev ON true
		WHERE j.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return j, err
}

// ListFilter 描述任务列表查询条件；排序为 created_at DESC, id DESC。
type ListFilter struct {
	Status          *string
	Active          *bool
	Kind            *string
	AggregateType   *string
	AggregateID     *uuid.UUID
	BeforeCreatedAt *time.Time
	BeforeID        *uuid.UUID
	Limit           int
}

// List 按最近创建优先返回 Job 列表，并附带最近事件摘要。
func (r *Repository) List(ctx context.Context, q database.Querier, f ListFilter) ([]JobWithMessage, error) {
	var (
		conds []string
		args  []any
	)
	if f.Status != nil {
		args = append(args, *f.Status)
		conds = append(conds, fmt.Sprintf("j.status = $%d", len(args)))
	}
	if f.Active != nil && *f.Active {
		conds = append(conds, "j.status IN ('queued','running','validating','cancel_requested')")
	}
	if f.Kind != nil {
		args = append(args, *f.Kind)
		conds = append(conds, fmt.Sprintf("j.kind = $%d", len(args)))
	}
	if f.AggregateType != nil {
		args = append(args, *f.AggregateType)
		conds = append(conds, fmt.Sprintf("j.aggregate_type = $%d", len(args)))
	}
	if f.AggregateID != nil {
		args = append(args, *f.AggregateID)
		conds = append(conds, fmt.Sprintf("j.aggregate_id = $%d", len(args)))
	}
	if f.BeforeCreatedAt != nil && f.BeforeID != nil {
		args = append(args, *f.BeforeCreatedAt, *f.BeforeID)
		conds = append(conds, fmt.Sprintf("(j.created_at, j.id) < ($%d, $%d)", len(args)-1, len(args)))
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	if f.Limit <= 0 {
		f.Limit = 20
	}
	args = append(args, f.Limit)
	rows, err := q.Query(ctx, `
		SELECT `+columnsPrefixed("j")+`, ev.message
		FROM jobs j
		LEFT JOIN LATERAL (
		  SELECT message FROM job_events
		  WHERE job_id = j.id
		  ORDER BY id DESC
		  LIMIT 1
		) ev ON true
		`+where+`
		ORDER BY j.created_at DESC, j.id DESC
		LIMIT $`+fmt.Sprintf("%d", len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]JobWithMessage, 0, min(f.Limit, 128))
	for rows.Next() {
		var j JobWithMessage
		if err := rows.Scan(
			&j.ID, &j.Kind, &j.AggregateType, &j.AggregateID, &j.GenerationFamily,
			&j.Status, &j.Stage, &j.Progress, &j.Payload, &j.Result, &j.ErrorCode, &j.ErrorDetail, &j.ErrorRetryable,
			&j.Priority, &j.AttemptCount, &j.MaxAttempts, &j.AvailableAt,
			&j.LeaseOwner, &j.LeaseExpiresAt, &j.CancelRequested, &j.RetryOfJobID,
			&j.IdempotencyScope, &j.IdempotencyKey,
			&j.CreatedAt, &j.StartedAt, &j.CompletedAt, &j.UpdatedAt,
			&j.Message,
		); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// GetActiveAttemptGradeJob 返回指定 Attempt 的活动批阅 Job（用于
// 409 resource_busy 的 active_job_id 扩展字段）；不存在时返回 nil, nil。
func (r *Repository) GetActiveAttemptGradeJob(ctx context.Context, q database.Querier, attemptID uuid.UUID) (*Job, error) {
	j, err := scanJob(q.QueryRow(ctx, `
		SELECT `+columns+` FROM jobs
		WHERE aggregate_type = 'attempt' AND aggregate_id = $1
		  AND kind = 'attempt_grade'
		  AND status IN ('queued','running','validating','cancel_requested')`, attemptID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return j, err
}

// GetActiveGenerationJob 返回指定 Lesson 的 lesson_generation 族活动 Job（用于
// 409 resource_busy 的 active_job_id 扩展字段）；不存在时返回 nil, nil。
func (r *Repository) GetActiveGenerationJob(ctx context.Context, q database.Querier, lessonID uuid.UUID) (*Job, error) {
	j, err := scanJob(q.QueryRow(ctx, `
		SELECT `+columns+` FROM jobs
		WHERE aggregate_type = 'lesson' AND aggregate_id = $1
		  AND generation_family = 'lesson_generation'
		  AND status IN ('queued','running','validating','cancel_requested')`, lessonID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return j, err
}

// Claim 领取一个可执行 Job：Business Gate shared lock + 维护模式检查 +
// FOR UPDATE SKIP LOCKED（03 §7.1）。同一 Job 不会被两个 Worker 重复领取。
// 返回 (nil, nil) 表示当前无可领取任务。
func (r *Repository) Claim(ctx context.Context, q database.Querier, workerID string, lease time.Duration) (*Job, error) {
	tx, ok := q.(pgx.Tx)
	if !ok {
		return nil, errors.New("Claim 必须在事务中执行")
	}
	if err := gate.LockBusinessShared(ctx, tx); err != nil {
		return nil, fmt.Errorf("获取业务门禁锁失败: %w", err)
	}
	j, err := scanJob(tx.QueryRow(ctx, `
		WITH candidate AS (
		  SELECT id FROM jobs
		  WHERE status = 'queued'
		    AND available_at <= now()
		    AND NOT EXISTS (
		      SELECT 1 FROM system_state
		      WHERE singleton = true AND maintenance_mode = true
		    )
		  ORDER BY priority DESC, created_at ASC
		  FOR UPDATE SKIP LOCKED
		  LIMIT 1
		)
		UPDATE jobs j
		SET status = 'running',
		    lease_owner = $1,
		    lease_expires_at = now() + $2::interval,
		    started_at = COALESCE(started_at, now()),
		    attempt_count = attempt_count + 1,
		    updated_at = now()
		FROM candidate
		WHERE j.id = candidate.id
		RETURNING `+columnsPrefixed("j"), workerID, lease.String()))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("领取任务失败: %w", err)
	}
	if err := r.addEvent(ctx, tx, j.ID, EventStarted, j.Stage, j.Progress, "任务已开始执行", nil); err != nil {
		return nil, err
	}
	return j, nil
}

// Heartbeat 续租：只允许当前租约持有方在租约未过期时续约（03 §7.2）。
func (r *Repository) Heartbeat(ctx context.Context, q database.Querier, id uuid.UUID, owner string, lease time.Duration) error {
	tag, err := q.Exec(ctx, `
		UPDATE jobs
		SET lease_expires_at = now() + $3::interval, updated_at = now()
		WHERE id = $1 AND lease_owner = $2
		  AND status IN ('running','validating','cancel_requested')
		  AND lease_expires_at > now()`,
		id, owner, lease.String())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrLeaseLost
	}
	return nil
}

// UpdateProgress 由租约持有方更新阶段/进度并写 job.progress 或
// job.validating 事件。message 只含阶段摘要，不含学习材料正文。
func (r *Repository) UpdateProgress(ctx context.Context, q database.Querier, id uuid.UUID, owner string, stage string, progress int16, message string) error {
	var currentStatus string
	err := q.QueryRow(ctx, `
			UPDATE jobs
			SET stage = $3, progress = $4,
			    status = CASE
			      WHEN status = 'cancel_requested' THEN status
			      WHEN $3 = 'validating' THEN 'validating'
			      ELSE status
			    END,
			    updated_at = now()
			WHERE id = $1 AND lease_owner = $2
			  AND status IN ('running','validating','cancel_requested')
			  AND lease_expires_at > now()
			RETURNING status`,
		id, owner, stage, progress).Scan(&currentStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrLeaseLost
	}
	if err != nil {
		return err
	}
	eventType := EventProgress
	if currentStatus == StatusCancelRequested {
		eventType = EventCancelRequested
	} else if stage == StatusValidating {
		eventType = EventValidating
	}
	return r.addEvent(ctx, q, id, eventType, stage, progress, message, nil)
}

// Complete 由租约持有方把 Job 置为 succeeded 并写 job.succeeded 事件。
// 领域结果发布必须与该方法处于同一事务（03 §6.4）。
func (r *Repository) Complete(ctx context.Context, q database.Querier, id uuid.UUID, owner string, result []byte) error {
	tag, err := q.Exec(ctx, `
		UPDATE jobs
		SET status = 'succeeded', progress = 100,
		    result = $3, error_code = NULL, error_detail = NULL, error_retryable = NULL,
		    lease_owner = NULL, lease_expires_at = NULL,
		    completed_at = now(), updated_at = now()
		WHERE id = $1 AND lease_owner = $2
		  AND status IN ('running','validating','cancel_requested')
		  AND lease_expires_at > now()`,
		id, owner, result)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrLeaseLost
	}
	return r.addEvent(ctx, q, id, EventSucceeded, "succeeded", 100, "任务完成", nil)
}

// Fail 由租约持有方报告失败。retryable 且未达次数上限时回到 queued
// （指数退避），否则进入 failed 终态（03 §7.3 重试分类由 Handler 给出）。
func (r *Repository) Fail(ctx context.Context, q database.Querier, id uuid.UUID, owner string, code, detail string, retryable bool) error {
	j, err := r.GetByID(ctx, q, id)
	if err != nil {
		return err
	}
	requeue := retryable && j.AttemptCount < j.MaxAttempts
	var tag pgconn.CommandTag
	if requeue {
		delay := r.backoff()(j.AttemptCount)
		tag, err = q.Exec(ctx, `
			UPDATE jobs
			SET status = 'queued', available_at = now() + $3::interval,
			    error_code = $4, error_detail = $5, error_retryable = $6,
			    lease_owner = NULL, lease_expires_at = NULL, updated_at = now()
			WHERE id = $1 AND lease_owner = $2
			  AND status IN ('running','validating','cancel_requested')
			  AND lease_expires_at > now()`,
			id, owner, delay.String(), code, detail, retryable)
	} else {
		tag, err = q.Exec(ctx, `
			UPDATE jobs
			SET status = 'failed', error_code = $3, error_detail = $4, error_retryable = $5,
			    lease_owner = NULL, lease_expires_at = NULL,
			    completed_at = now(), updated_at = now()
			WHERE id = $1 AND lease_owner = $2
			  AND status IN ('running','validating','cancel_requested')
			  AND lease_expires_at > now()`,
			id, owner, code, detail, retryable)
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrLeaseLost
	}
	if requeue {
		return r.addEvent(ctx, q, id, EventQueued, j.Stage, j.Progress,
			fmt.Sprintf("任务失败（%s），等待重试", code), nil)
	}
	return r.addEvent(ctx, q, id, EventFailed, j.Stage, j.Progress, "任务失败: "+code, nil)
}

// RequestCancel 请求取消：queued 任务直接取消；运行中任务置
// cancel_requested，由 Worker 在下一检查点确认（CancelByOwner）。
func (r *Repository) RequestCancel(ctx context.Context, q database.Querier, id uuid.UUID) error {
	j, err := scanJob(q.QueryRow(ctx, `SELECT `+columns+` FROM jobs WHERE id = $1 FOR UPDATE`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	switch j.Status {
	case StatusQueued:
		tag, err := q.Exec(ctx, `
			UPDATE jobs
			SET status = 'cancelled', cancel_requested = true,
			    completed_at = now(), updated_at = now()
			WHERE id = $1 AND status = 'queued'`, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotCancellable
		}
		return r.addEvent(ctx, q, id, EventCancelled, j.Stage, j.Progress, "任务已取消", nil)
	case StatusRunning, StatusValidating, StatusCancelRequested:
		tag, err := q.Exec(ctx, `
				UPDATE jobs
				SET status = 'cancel_requested', cancel_requested = true, updated_at = now()
				WHERE id = $1 AND status IN ('running','validating','cancel_requested')`, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotCancellable
		}
		return r.addEvent(ctx, q, id, EventCancelRequested, j.Stage, j.Progress, "已请求取消", nil)
	default:
		return ErrNotCancellable
	}
}

// CancelByOwner 由租约持有方确认取消（Worker 检查到 cancel_requested 后调用）。
func (r *Repository) CancelByOwner(ctx context.Context, q database.Querier, id uuid.UUID, owner string) error {
	tag, err := q.Exec(ctx, `
		UPDATE jobs
		SET status = 'cancelled', lease_owner = NULL, lease_expires_at = NULL,
		    completed_at = now(), updated_at = now()
		WHERE id = $1 AND lease_owner = $2
		  AND status IN ('running','validating','cancel_requested')`,
		id, owner)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrLeaseLost
	}
	return r.addEvent(ctx, q, id, EventCancelled, "", 0, "任务已取消", nil)
}

// RecoverExpiredLeases 回收过期租约（03 §7.2 / §6.7 修复步骤 3）：
//   - cancel_requested → cancelled
//   - attempt_count < max_attempts → 清除租约回到 queued（指数退避）
//   - 达到上限 → failed(worker_lost)
//
// queued 任务不处理（退出维护后自然被领取）。返回各处置结果计数。
func (r *Repository) RecoverExpiredLeases(ctx context.Context, q database.Querier, batchSize int) (requeued, failed, cancelled int, err error) {
	tx, ok := q.(pgx.Tx)
	if !ok {
		return 0, 0, 0, errors.New("RecoverExpiredLeases 必须在事务中执行")
	}
	rows, err := tx.Query(ctx, `
		SELECT `+columns+` FROM jobs
		WHERE status IN ('running','validating','cancel_requested')
		  AND lease_expires_at <= now()
		ORDER BY lease_expires_at ASC
		FOR UPDATE SKIP LOCKED
		LIMIT $1`, batchSize)
	if err != nil {
		return 0, 0, 0, err
	}
	var expired []*Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			rows.Close()
			return 0, 0, 0, err
		}
		expired = append(expired, j)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, 0, 0, err
	}

	for _, j := range expired {
		switch {
		case j.Status == StatusCancelRequested || j.CancelRequested:
			if _, err := tx.Exec(ctx, `
				UPDATE jobs SET status = 'cancelled',
				    lease_owner = NULL, lease_expires_at = NULL,
				    completed_at = now(), updated_at = now()
				WHERE id = $1`, j.ID); err != nil {
				return requeued, failed, cancelled, err
			}
			if err := r.addEvent(ctx, tx, j.ID, EventCancelled, j.Stage, j.Progress, "Worker 丢失，取消已确认", nil); err != nil {
				return requeued, failed, cancelled, err
			}
			cancelled++
		case j.AttemptCount < j.MaxAttempts:
			delay := r.backoff()(j.AttemptCount)
			if _, err := tx.Exec(ctx, `
				UPDATE jobs SET status = 'queued',
				    available_at = now() + $2::interval,
				    error_code = $3, error_detail = 'Worker 租约过期，任务回收重试',
				    error_retryable = true,
				    lease_owner = NULL, lease_expires_at = NULL, updated_at = now()
				WHERE id = $1`, j.ID, delay.String(), ErrCodeWorkerLost); err != nil {
				return requeued, failed, cancelled, err
			}
			if err := r.addEvent(ctx, tx, j.ID, EventQueued, j.Stage, j.Progress, "Worker 丢失，任务回收等待重试", nil); err != nil {
				return requeued, failed, cancelled, err
			}
			requeued++
		default:
			if _, err := tx.Exec(ctx, `
				UPDATE jobs SET status = 'failed',
				    error_code = $2, error_detail = 'Worker 租约过期且重试次数耗尽',
				    error_retryable = true,
				    lease_owner = NULL, lease_expires_at = NULL,
				    completed_at = now(), updated_at = now()
				WHERE id = $1`, j.ID, ErrCodeWorkerLost); err != nil {
				return requeued, failed, cancelled, err
			}
			if err := r.addEvent(ctx, tx, j.ID, EventFailed, j.Stage, j.Progress, "任务失败: "+ErrCodeWorkerLost, nil); err != nil {
				return requeued, failed, cancelled, err
			}
			failed++
		}
	}
	return requeued, failed, cancelled, nil
}

// ErrNotRetryable：只有 failed 终态任务可以手动重试；Status 为当前状态，
// 由调用方区分映射（活动状态 → 409 resource_busy，其余终态 → 422）。
type ErrNotRetryable struct {
	Status string
}

func (e *ErrNotRetryable) Error() string {
	return "只有 failed 状态的任务可以手动重试（当前状态 " + e.Status + "）"
}

// RetryFailed 为终态失败任务创建手动重试 Job（retry_of_job_id 指向原任务，
// 03 §6.4：保留失败历史，不重置终态任务）。
func (r *Repository) RetryFailed(ctx context.Context, q database.Querier, id uuid.UUID, scope, key *string) (*Job, error) {
	orig, err := r.GetByID(ctx, q, id)
	if err != nil {
		return nil, err
	}
	if orig.Status != StatusFailed {
		return nil, &ErrNotRetryable{Status: orig.Status}
	}
	payload := orig.Payload
	if payload == nil {
		payload = []byte(`{}`)
	}
	nj := &Job{
		Kind:             orig.Kind,
		AggregateType:    orig.AggregateType,
		AggregateID:      orig.AggregateID,
		GenerationFamily: orig.GenerationFamily,
		Payload:          payload,
		Priority:         orig.Priority,
		MaxAttempts:      orig.MaxAttempts,
		RetryOfJobID:     &orig.ID,
		IdempotencyScope: scope,
		IdempotencyKey:   key,
	}
	if err := r.Create(ctx, q, nj); err != nil {
		return nil, err
	}
	return nj, nil
}

// CancelRequested 返回任务是否已被请求取消（Worker 取消轮询用，03 §7.2）。
func (r *Repository) CancelRequested(ctx context.Context, q database.Querier, id uuid.UUID) (bool, error) {
	var requested bool
	err := q.QueryRow(ctx, `SELECT cancel_requested FROM jobs WHERE id = $1`, id).Scan(&requested)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrNotFound
	}
	return requested, err
}

// AddEvent 对外暴露的事件写入（Worker Handler 自定义 message/data 时使用）。
// data 只含阶段、进度和脱敏错误摘要。
func (r *Repository) AddEvent(ctx context.Context, q database.Querier, jobID uuid.UUID, eventType, stage string, progress int16, message string, data []byte) error {
	return r.addEvent(ctx, q, jobID, eventType, stage, progress, message, data)
}

func (r *Repository) addEvent(ctx context.Context, q database.Querier, jobID uuid.UUID, eventType, stage string, progress int16, message string, data []byte) error {
	if data == nil {
		data = []byte(`{}`)
	}
	_, err := q.Exec(ctx, `
		INSERT INTO job_events (job_id, event_type, stage, progress, message, data)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		jobID, eventType, stage, progress, message, data)
	return err
}

// ListEvents 按 SSE Event ID 升序返回 afterID 之后的事件（回放语义）。
func (r *Repository) ListEvents(ctx context.Context, q database.Querier, jobID uuid.UUID, afterID int64, limit int) ([]Event, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := q.Query(ctx, `
		SELECT id, job_id, event_type, stage, progress, message, data, created_at
		FROM job_events
		WHERE job_id = $1 AND id > $2
		ORDER BY id ASC
		LIMIT $3`, jobID, afterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.JobID, &e.EventType, &e.Stage, &e.Progress, &e.Message, &e.Data, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ListStreamEvents 按全局 SSE Event ID 升序返回 afterID 之后仍在保留窗口内
// 的事件；不包含 job payload/result 或 event.data 原文。
func (r *Repository) ListStreamEvents(ctx context.Context, q database.Querier, afterID int64, retention time.Duration, limit int) ([]Event, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	retentionSeconds := int64(retention / time.Second)
	rows, err := q.Query(ctx, `
		SELECT id, job_id, event_type, stage, progress, message, data, created_at
		FROM job_events
		WHERE id > $1
		  AND created_at >= now() - ($2::bigint * interval '1 second')
		ORDER BY id ASC
		LIMIT $3`, afterID, retentionSeconds, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.JobID, &e.EventType, &e.Stage, &e.Progress, &e.Message, &e.Data, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// StreamWatermark 返回当前持久化事件高水位，以及仍在保留窗口内的最小事件 ID。
// minRetainedID 为 0 表示当前没有保留窗口内事件。
func (r *Repository) StreamWatermark(ctx context.Context, q database.Querier, retention time.Duration) (highWatermarkID, minRetainedID int64, err error) {
	retentionSeconds := int64(retention / time.Second)
	err = q.QueryRow(ctx, `
		SELECT COALESCE(max(id), 0),
		       COALESCE(min(id) FILTER (
		         WHERE created_at >= now() - ($1::bigint * interval '1 second')
		       ), 0)
		FROM job_events`, retentionSeconds).Scan(&highWatermarkID, &minRetainedID)
	return highWatermarkID, minRetainedID, err
}

// columnsPrefixed 生成带表前缀的列清单（用于 UPDATE ... RETURNING）。
func columnsPrefixed(prefix string) string {
	cols := []string{
		"id", "kind", "aggregate_type", "aggregate_id", "generation_family",
		"status", "stage", "progress", "payload", "result", "error_code", "error_detail", "error_retryable",
		"priority", "attempt_count", "max_attempts", "available_at",
		"lease_owner", "lease_expires_at", "cancel_requested", "retry_of_job_id",
		"idempotency_scope", "idempotency_key",
		"created_at", "started_at", "completed_at", "updated_at",
	}
	out := ""
	for i, c := range cols {
		if i > 0 {
			out += ", "
		}
		out += prefix + "." + c
	}
	return out
}
