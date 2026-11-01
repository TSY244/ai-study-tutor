// service.go 是 Job 用例的应用服务（B4 范围：失败任务手动重试）。
// 列表/详情/取消/事件/SSE 在 B7 落地。
package job

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/backend/internal/httpx"
	"github.com/TSY244/ai-study-tutor/backend/internal/idempotency"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/gate"
)

// Accepted 是 202 AsyncAccepted 响应体（契约：{"job": Job}）。
type Accepted struct {
	Job DTO `json:"job"`
}

// Service 编排 Job 用例；事务边界在本层。
type Service struct {
	pool *pgxpool.Pool
	repo *Repository
	idem *idempotency.Service

	// SSE 可调参数；零值使用 B7 默认。
	ReplayRetention  time.Duration
	StreamPoll       time.Duration
	StreamHeartbeat  time.Duration
	StreamBatchLimit int
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, repo: &Repository{}, idem: idempotency.NewService(pool)}
}

// CreateConnectivity creates the Worker-owned Claude connectivity job. The
// API never reads provider credentials or calls Claude directly.
func (s *Service) CreateConnectivity(ctx context.Context, rec *idempotency.Record) (int, *Accepted, error) {
	return s.createSystemJob(ctx, rec, KindClaudeConnectivityTest, []byte(`{}`))
}

// CreateKeyConnectivity creates the same claude_connectivity_test job kind
// targeting a single Claude gateway key (payload {"key_id": ...}).
// The payload never contains the key material itself.
func (s *Service) CreateKeyConnectivity(ctx context.Context, rec *idempotency.Record, keyID string) (int, *Accepted, error) {
	payload, err := json.Marshal(map[string]string{"key_id": keyID})
	if err != nil {
		return 0, nil, err
	}
	return s.createSystemJob(ctx, rec, KindClaudeConnectivityTest, payload)
}

// CreateBilibiliCookieTest creates the Worker-owned B 站 Cookie 可用性检测
// 任务。API 进程没有外网 egress，也不直接接触目标站点。
func (s *Service) CreateBilibiliCookieTest(ctx context.Context, rec *idempotency.Record) (int, *Accepted, error) {
	return s.createSystemJob(ctx, rec, KindBilibiliCookieTest, []byte(`{}`))
}

func (s *Service) CreateBilibiliLogin(ctx context.Context, rec *idempotency.Record) (int, *Accepted, error) {
	return s.createSystemJob(ctx, rec, KindBilibiliLogin, []byte(`{}`))
}

func (s *Service) createSystemJob(ctx context.Context, rec *idempotency.Record, kind string, payload []byte) (int, *Accepted, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := gate.AssertBusinessOpen(ctx, tx); err != nil {
		return 0, nil, err
	}
	j := &Job{
		Kind:             kind,
		AggregateType:    "system",
		AggregateID:      uuid.Nil,
		Payload:          payload,
		IdempotencyScope: &rec.Scope,
		IdempotencyKey:   &rec.Key,
	}
	if err := s.repo.Create(ctx, tx, j); err != nil {
		return 0, nil, err
	}
	body := &Accepted{Job: ToDTO(j, nil)}
	headers := map[string]string{"Location": "/api/v1/jobs/" + j.ID.String()}
	if err := httpx.CompleteJSON(ctx, tx, s.idem, rec.ID, http.StatusAccepted, body, headers, nil, nil, &j.ID); err != nil {
		return 0, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, nil, err
	}
	return http.StatusAccepted, body, nil
}

// List 返回最近 Job 列表。active=true 与 status 互斥（04 §13）。
func (s *Service) List(ctx context.Context, f ListFilter) ([]JobWithMessage, bool, error) {
	if f.Active != nil && *f.Active && f.Status != nil {
		return nil, false, httpx.NewProblemError(http.StatusBadRequest, "bad_request",
			"请求无效", "active=true 不能与 status 同时使用。")
	}
	if f.Limit <= 0 {
		f.Limit = 20
	}
	fetch := f
	fetch.Limit = f.Limit + 1
	rows, err := s.repo.List(ctx, s.pool, fetch)
	if err != nil {
		return nil, false, err
	}
	hasMore := len(rows) > f.Limit
	if hasMore {
		rows = rows[:f.Limit]
	}
	return rows, hasMore, nil
}

// Get 读取 Job 详情。
func (s *Service) Get(ctx context.Context, id uuid.UUID) (DTO, error) {
	j, err := s.repo.GetWithMessage(ctx, s.pool, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return DTO{}, httpx.NewProblemError(http.StatusNotFound, "not_found",
				"资源不存在", "请求的资源不存在或已删除。")
		}
		return DTO{}, err
	}
	return ToDTO(&j.Job, j.Message), nil
}

// Events 返回单个 Job 的审计事件分页。
func (s *Service) Events(ctx context.Context, jobID uuid.UUID, afterID int64, limit int) ([]Event, bool, error) {
	if _, err := s.repo.GetByID(ctx, s.pool, jobID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, false, httpx.NewProblemError(http.StatusNotFound, "not_found",
				"资源不存在", "请求的资源不存在或已删除。")
		}
		return nil, false, err
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.repo.ListEvents(ctx, s.pool, jobID, afterID, limit+1)
	if err != nil {
		return nil, false, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	return rows, hasMore, nil
}

// Retry 为终态失败任务创建手动重试 Job（03 §6.4：retry_of_job_id 指向原
// 任务，保留失败历史）。attempt_grade 任务重试成功时同事务把关联的
// grading_failed 作答重新置为 grading（04 §10.5：批阅失败使用 Job Retry，
// 不创建新 Attempt）。
func (s *Service) Retry(ctx context.Context, rec *idempotency.Record, jobID uuid.UUID) (int, *Accepted, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := gate.AssertBusinessOpen(ctx, tx); err != nil {
		return 0, nil, err
	}
	orig, err := s.repo.GetByID(ctx, tx, jobID)
	if errors.Is(err, ErrNotFound) {
		return 0, nil, httpx.NewProblemError(http.StatusNotFound, "not_found",
			"资源不存在", "请求的资源不存在或已删除。")
	}
	if err != nil {
		return 0, nil, err
	}
	if orig.Status != StatusFailed {
		return 0, nil, notRetryableProblem(&ErrNotRetryable{Status: orig.Status})
	}
	if err := s.repo.LockRetryTarget(ctx, tx, orig); err != nil {
		if errors.Is(err, ErrNotFound) {
			return 0, nil, httpx.NewProblemError(http.StatusNotFound, "not_found",
				"资源不存在", "任务关联的学习内容已删除，不能重试。")
		}
		return 0, nil, err
	}
	nj, err := s.repo.RetryFailed(ctx, tx, jobID, &rec.Scope, &rec.Key)
	if err != nil {
		var nr *ErrNotRetryable
		switch {
		case errors.Is(err, ErrNotFound):
			return 0, nil, httpx.NewProblemError(http.StatusNotFound, "not_found",
				"资源不存在", "请求的资源不存在或已删除。")
		case errors.As(err, &nr):
			return 0, nil, notRetryableProblem(nr)
		case errors.Is(err, ErrActiveGenerationJobExists):
			// 唯一冲突已中止当前事务，用连接池查询已有任务。
			active, readErr := s.repo.GetActiveGenerationJob(ctx, s.pool, orig.AggregateID)
			if readErr == nil && active != nil {
				return 0, nil, ResourceBusyError(active.ID)
			}
			return 0, nil, httpx.NewProblemError(http.StatusConflict, "resource_busy",
				"资源繁忙", "该课时已有进行中的生成任务，暂时不能重试。")
		case errors.Is(err, ErrActiveAttemptGradeExists):
			return 0, nil, httpx.NewProblemError(http.StatusConflict, "resource_busy",
				"资源繁忙", "该作答已有进行中的批阅任务，暂时不能重试。")
		default:
			return 0, nil, err
		}
	}
	if nj.Kind == KindAttemptGrade {
		// 关联作答处于 grading_failed 时恢复为 grading 并绑定新任务；
		// 其他状态（如旧隔离数据）不强行改写，由 Worker 侧对账。
		if _, err := tx.Exec(ctx, `
			UPDATE attempts
			SET status = 'grading', grading_job_id = $2,
			    grading_error_code = NULL, grading_error_detail = NULL,
			    version = version + 1
			WHERE grading_job_id = $1 AND status = 'grading_failed'`,
			jobID, nj.ID); err != nil {
			return 0, nil, err
		}
	}
	body := &Accepted{Job: ToDTO(nj, nil)}
	if err := httpx.CompleteJSON(ctx, tx, s.idem, rec.ID, http.StatusAccepted, body, nil, nil, nil, &nj.ID); err != nil {
		return 0, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, nil, err
	}
	return http.StatusAccepted, body, nil
}

// Cancel 请求取消任务，并在同一事务完成幂等记录。
func (s *Service) Cancel(ctx context.Context, rec *idempotency.Record, jobID uuid.UUID) (int, DTO, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, DTO{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := gate.AssertBusinessOpen(ctx, tx); err != nil {
		return 0, DTO{}, err
	}
	if err := s.repo.RequestCancel(ctx, tx, jobID); err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			return 0, DTO{}, httpx.NewProblemError(http.StatusNotFound, "not_found",
				"资源不存在", "请求的资源不存在或已删除。")
		case errors.Is(err, ErrNotCancellable):
			return 0, DTO{}, httpx.NewProblemError(http.StatusUnprocessableEntity, "validation_error",
				"任务不能取消", "只有排队中或运行中的任务可以取消。")
		default:
			return 0, DTO{}, err
		}
	}
	j, err := s.repo.GetWithMessage(ctx, tx, jobID)
	if err != nil {
		return 0, DTO{}, err
	}
	body := ToDTO(&j.Job, j.Message)
	if err := httpx.CompleteJSON(ctx, tx, s.idem, rec.ID, http.StatusAccepted, body, nil, nil, nil, &jobID); err != nil {
		return 0, DTO{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, DTO{}, err
	}
	return http.StatusAccepted, body, nil
}

// StreamSink 是 SSE 写出侧的窄接口，避免 Service 依赖 Gin/HTTP 类型。
type StreamSink interface {
	WriteFrame(StreamFrame) error
}

// Stream 根据 Last-Event-ID 执行 30 天回放、断档 resync、实时轮询和心跳。
func (s *Service) Stream(ctx context.Context, sink StreamSink, lastEventID int64) error {
	retention := s.replayRetention()
	high, minRetained, err := s.repo.StreamWatermark(ctx, s.pool, retention)
	if err != nil {
		return err
	}
	sentInitial := false
	if historyExpired(lastEventID, high, minRetained) {
		id := high
		if err := sink.WriteFrame(StreamFrame{
			ID:    &id,
			Event: "stream.resync_required",
			Data: JobResyncRequired{
				Reason:               "history_expired",
				HighWatermarkEventID: high,
				ActiveJobsURL:        "/api/v1/jobs?active=true",
			},
		}); err != nil {
			return err
		}
		lastEventID = high
		sentInitial = true
	} else {
		n, err := s.writeAvailableStreamEvents(ctx, sink, &lastEventID)
		if err != nil {
			return err
		}
		sentInitial = n > 0
	}
	if !sentInitial {
		if err := sink.WriteFrame(StreamFrame{
			Event: "heartbeat",
			Data:  HeartbeatStreamData{ServerTime: time.Now().UTC().Format(time.RFC3339)},
		}); err != nil {
			return err
		}
	}

	poll := time.NewTicker(s.streamPoll())
	defer poll.Stop()
	heartbeat := time.NewTicker(s.streamHeartbeat())
	defer heartbeat.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-poll.C:
			if _, err := s.writeAvailableStreamEvents(ctx, sink, &lastEventID); err != nil {
				return err
			}
		case <-heartbeat.C:
			if err := sink.WriteFrame(StreamFrame{
				Event: "heartbeat",
				Data:  HeartbeatStreamData{ServerTime: time.Now().UTC().Format(time.RFC3339)},
			}); err != nil {
				return err
			}
		}
	}
}

func (s *Service) writeAvailableStreamEvents(ctx context.Context, sink StreamSink, lastEventID *int64) (int, error) {
	total := 0
	for {
		events, err := s.repo.ListStreamEvents(ctx, s.pool, *lastEventID, s.replayRetention(), s.streamBatchLimit())
		if err != nil {
			return total, err
		}
		if len(events) == 0 {
			return total, nil
		}
		for _, ev := range events {
			id := ev.ID
			if err := sink.WriteFrame(StreamFrame{ID: &id, Event: ev.EventType, Data: ToJobStreamData(ev)}); err != nil {
				return total, err
			}
			*lastEventID = ev.ID
			total++
		}
		if len(events) < s.streamBatchLimit() {
			return total, nil
		}
	}
}

func historyExpired(lastEventID, highWatermarkID, minRetainedID int64) bool {
	if lastEventID <= 0 || highWatermarkID <= 0 {
		return false
	}
	if minRetainedID == 0 {
		return lastEventID <= highWatermarkID
	}
	return lastEventID < minRetainedID
}

func (s *Service) replayRetention() time.Duration {
	if s.ReplayRetention > 0 {
		return s.ReplayRetention
	}
	return 30 * 24 * time.Hour
}

func (s *Service) streamPoll() time.Duration {
	if s.StreamPoll > 0 {
		return s.StreamPoll
	}
	return 100 * time.Millisecond
}

func (s *Service) streamHeartbeat() time.Duration {
	if s.StreamHeartbeat > 0 {
		return s.StreamHeartbeat
	}
	return 15 * time.Second
}

func (s *Service) streamBatchLimit() int {
	if s.StreamBatchLimit > 0 {
		return s.StreamBatchLimit
	}
	return 100
}

// notRetryableProblem 区分映射：活动状态任务 → 409 resource_busy；
// 其他终态（succeeded/cancelled）→ 422 validation_error。
func notRetryableProblem(nr *ErrNotRetryable) *httpx.ProblemError {
	switch nr.Status {
	case StatusQueued, StatusRunning, StatusValidating, StatusCancelRequested:
		return httpx.NewProblemError(http.StatusConflict, "resource_busy",
			"任务仍在进行", "任务仍在进行，不能重试；请先等待完成或取消。")
	default:
		return httpx.NewProblemError(http.StatusUnprocessableEntity, "validation_error",
			"任务不能重试", "只有失败状态的任务可以手动重试。")
	}
}
