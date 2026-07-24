package job_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/backend/internal/job"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/databasetest"
)

var repo = &job.Repository{}

func TestMain(m *testing.M) { databasetest.Main(m) }

func newJob(aggID uuid.UUID) *job.Job {
	return &job.Job{
		Kind:          job.KindLessonImport,
		AggregateType: "lesson",
		AggregateID:   aggID,
		Payload:       []byte(`{"lesson_id":"` + aggID.String() + `"}`),
	}
}

func createJob(t *testing.T, pool *pgxpool.Pool, j *job.Job) *job.Job {
	t.Helper()
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := repo.Create(context.Background(), tx, j); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	return j
}

func claim(t *testing.T, pool *pgxpool.Pool, workerID string, lease time.Duration) *job.Job {
	t.Helper()
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	j, err := repo.Claim(context.Background(), tx, workerID, lease)
	if err != nil {
		t.Fatal(err)
	}
	if j == nil {
		return nil
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	return j
}

func inTx(t *testing.T, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) {
	t.Helper()
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := fn(tx); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// 并发领取：N 个 Job、M 个 Worker 并发 Claim，每个 Job 只被领取一次。
func TestClaimConcurrentNoDoubleClaim(t *testing.T) {
	pool := databasetest.New(t)
	const jobCount, workerCount = 20, 8
	want := map[uuid.UUID]bool{}
	for i := 0; i < jobCount; i++ {
		j := createJob(t, pool, newJob(uuid.New()))
		want[j.ID] = true
	}

	var mu sync.Mutex
	claimed := map[uuid.UUID]string{}
	var wg sync.WaitGroup
	for w := 0; w < workerCount; w++ {
		wg.Add(1)
		go func(workerID string) {
			defer wg.Done()
			for {
				j := claim(t, pool, workerID, time.Minute)
				if j == nil {
					return
				}
				mu.Lock()
				if prev, dup := claimed[j.ID]; dup {
					t.Errorf("Job %s 被重复领取: %s 与 %s", j.ID, prev, workerID)
				}
				claimed[j.ID] = workerID
				mu.Unlock()
			}
		}(uuid.NewString())
	}
	wg.Wait()
	if len(claimed) != jobCount {
		t.Fatalf("领取数 = %d，期望 %d", len(claimed), jobCount)
	}
	for id := range want {
		if _, ok := claimed[id]; !ok {
			t.Errorf("Job %s 未被领取", id)
		}
	}
}

// 维护模式下 Claim 返回空（Business Gate + NOT EXISTS 检查）。
func TestClaimBlockedByMaintenance(t *testing.T) {
	pool := databasetest.New(t)
	createJob(t, pool, newJob(uuid.New()))
	if _, err := pool.Exec(context.Background(),
		`UPDATE system_state SET maintenance_mode = true WHERE singleton = true`); err != nil {
		t.Fatal(err)
	}
	if j := claim(t, pool, "w1", time.Minute); j != nil {
		t.Fatalf("维护模式下不应领取到任务，得到 %s", j.ID)
	}
	if _, err := pool.Exec(context.Background(),
		`UPDATE system_state SET maintenance_mode = false WHERE singleton = true`); err != nil {
		t.Fatal(err)
	}
	if j := claim(t, pool, "w1", time.Minute); j == nil {
		t.Fatal("退出维护后应能领取任务")
	}
}

// 优先级与租约字段：高优先级先领取，领取后持有 owner/lease。
func TestClaimPriorityAndLeaseFields(t *testing.T) {
	pool := databasetest.New(t)
	low := createJob(t, pool, newJob(uuid.New()))
	high := newJob(uuid.New())
	high.Priority = 10
	createJob(t, pool, high)

	first := claim(t, pool, "worker-a", 90*time.Second)
	if first == nil || first.ID != high.ID {
		t.Fatalf("应先领取高优先级任务，得到 %+v", first)
	}
	if first.LeaseOwner == nil || *first.LeaseOwner != "worker-a" {
		t.Fatalf("lease_owner 不正确: %+v", first.LeaseOwner)
	}
	if first.LeaseExpiresAt == nil || time.Until(*first.LeaseExpiresAt) < 80*time.Second {
		t.Fatalf("lease_expires_at 不正确: %+v", first.LeaseExpiresAt)
	}
	if first.AttemptCount != 1 {
		t.Fatalf("attempt_count = %d，期望 1", first.AttemptCount)
	}
	second := claim(t, pool, "worker-a", time.Minute)
	if second == nil || second.ID != low.ID {
		t.Fatalf("应领取低优先级任务，得到 %+v", second)
	}
}

// 心跳：只有租约持有方在租约有效时可续租。
func TestHeartbeatOwnership(t *testing.T) {
	pool := databasetest.New(t)
	j := createJob(t, pool, newJob(uuid.New()))
	claimed := claim(t, pool, "owner-a", time.Minute)
	if claimed.ID != j.ID {
		t.Fatal("领取的不是创建的任务")
	}
	if err := repo.Heartbeat(context.Background(), pool, j.ID, "owner-b", time.Minute); !errors.Is(err, job.ErrLeaseLost) {
		t.Fatalf("非持有方心跳应返回 ErrLeaseLost，得到 %v", err)
	}
	if err := repo.Heartbeat(context.Background(), pool, j.ID, "owner-a", 2*time.Minute); err != nil {
		t.Fatalf("持有方心跳失败: %v", err)
	}
	// 强制租约过期后心跳失败。
	if _, err := pool.Exec(context.Background(),
		`UPDATE jobs SET lease_expires_at = now() - interval '1 second' WHERE id = $1`, j.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.Heartbeat(context.Background(), pool, j.ID, "owner-a", time.Minute); !errors.Is(err, job.ErrLeaseLost) {
		t.Fatalf("过期租约心跳应返回 ErrLeaseLost，得到 %v", err)
	}
}

// 自动重试：可重试失败回到 queued（退避），次数耗尽进入 failed。
func TestFailRetryBackoffAndExhaustion(t *testing.T) {
	pool := databasetest.New(t)
	backoffRepo := &job.Repository{Backoff: func(attempt int32) time.Duration { return time.Hour }}
	j := createJob(t, pool, newJob(uuid.New()))

	// 第 1 次失败：回到 queued，available_at 在未来（退避生效）。
	claim(t, pool, "w", time.Minute)
	if err := backoffRepo.Fail(context.Background(), pool, j.ID, "w", "transient_error", "网络超时", true); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetByID(context.Background(), pool, j.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != job.StatusQueued {
		t.Fatalf("可重试失败后 status = %s，期望 queued", got.Status)
	}
	if time.Until(got.AvailableAt) < 50*time.Minute {
		t.Fatalf("退避未生效，available_at = %v", got.AvailableAt)
	}
	if got.ErrorCode == nil || *got.ErrorCode != "transient_error" {
		t.Fatalf("error_code 未保留: %+v", got.ErrorCode)
	}
	events, err := repo.ListEvents(context.Background(), pool, j.ID, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if last := events[len(events)-1]; last.EventType != job.EventQueued {
		t.Fatalf("自动重试事件 = %s，期望 %s，以便 SSE 发布 queued 状态", last.EventType, job.EventQueued)
	}
	// 退避期间不可领取。
	if c := claim(t, pool, "w", time.Minute); c != nil {
		t.Fatal("退避中的任务不应被领取")
	}
	// 退避结束后再次领取并连续失败到上限 → failed。
	if _, err := pool.Exec(context.Background(),
		`UPDATE jobs SET available_at = now() - interval '1 second' WHERE id = $1`, j.ID); err != nil {
		t.Fatal(err)
	}
	for i := int32(2); i <= got.MaxAttempts; i++ {
		c := claim(t, pool, "w", time.Minute)
		if c == nil {
			t.Fatalf("第 %d 次应能领取", i)
		}
		if err := backoffRepo.Fail(context.Background(), pool, j.ID, "w", "transient_error", "再次超时", true); err != nil {
			t.Fatal(err)
		}
		if got.Status == job.StatusQueued {
			if _, err := pool.Exec(context.Background(),
				`UPDATE jobs SET available_at = now() - interval '1 second' WHERE id = $1`, j.ID); err != nil {
				t.Fatal(err)
			}
		}
	}
	final, err := repo.GetByID(context.Background(), pool, j.ID)
	if err != nil {
		t.Fatal(err)
	}
	if final.Status != job.StatusFailed {
		t.Fatalf("次数耗尽后 status = %s，期望 failed", final.Status)
	}
	if final.AttemptCount != final.MaxAttempts {
		t.Fatalf("attempt_count = %d，期望 %d", final.AttemptCount, final.MaxAttempts)
	}
}

// 租约回收：cancel_requested → cancelled；未达上限 → queued；达上限 → failed(worker_lost)。
func TestRecoverExpiredLeases(t *testing.T) {
	pool := databasetest.New(t)
	noBackoff := &job.Repository{Backoff: func(int32) time.Duration { return 0 }}

	running := createJob(t, pool, newJob(uuid.New()))
	cancelling := createJob(t, pool, newJob(uuid.New()))
	exhausted := createJob(t, pool, newJob(uuid.New()))
	fresh := createJob(t, pool, newJob(uuid.New()))

	claim(t, pool, "w", time.Minute) // running
	claim(t, pool, "w", time.Minute) // cancelling
	c := claim(t, pool, "w", time.Minute)
	if c == nil || c.ID != exhausted.ID {
		t.Fatal("第三个领取的应是 exhausted")
	}
	// cancelling：用户已请求取消。
	if err := repo.RequestCancel(context.Background(), pool, cancelling.ID); err != nil {
		t.Fatal(err)
	}
	// exhausted：达重试上限。
	if _, err := pool.Exec(context.Background(),
		`UPDATE jobs SET attempt_count = max_attempts WHERE id = $1`, exhausted.ID); err != nil {
		t.Fatal(err)
	}
	// fresh 保持 queued 不动。其余三个强制租约过期。
	if _, err := pool.Exec(context.Background(),
		`UPDATE jobs SET lease_expires_at = now() - interval '1 second'
		 WHERE id IN ($1,$2,$3)`, running.ID, cancelling.ID, exhausted.ID); err != nil {
		t.Fatal(err)
	}

	var requeued, failed, cancelled int
	inTx(t, pool, func(tx pgx.Tx) error {
		var err error
		requeued, failed, cancelled, err = noBackoff.RecoverExpiredLeases(context.Background(), tx, 100)
		return err
	})
	if requeued != 1 || failed != 1 || cancelled != 1 {
		t.Fatalf("回收计数 = (%d,%d,%d)，期望 (1,1,1)", requeued, failed, cancelled)
	}
	assertStatus := func(id uuid.UUID, want, wantErrCode string) {
		t.Helper()
		j, err := repo.GetByID(context.Background(), pool, id)
		if err != nil {
			t.Fatal(err)
		}
		if j.Status != want {
			t.Fatalf("Job %s status = %s，期望 %s", id, j.Status, want)
		}
		if wantErrCode != "" {
			if j.ErrorCode == nil || *j.ErrorCode != wantErrCode {
				t.Fatalf("Job %s error_code = %+v，期望 %s", id, j.ErrorCode, wantErrCode)
			}
		}
	}
	assertStatus(running.ID, job.StatusQueued, job.ErrCodeWorkerLost)
	assertStatus(exhausted.ID, job.StatusFailed, job.ErrCodeWorkerLost)
	assertStatus(cancelling.ID, job.StatusCancelled, "")
	assertStatus(fresh.ID, job.StatusQueued, "")
	// 回收后可重新领取。
	if c := claim(t, pool, "w2", time.Minute); c == nil || c.ID != running.ID {
		t.Fatalf("回收后的任务应可被重新领取，得到 %+v", c)
	}
}

// lesson_generation 族部分唯一索引：同 Lesson 拒绝第二个活动任务。
func TestLessonGenerationFamilyPartialUnique(t *testing.T) {
	pool := databasetest.New(t)
	lessonID := uuid.New()
	family := job.GenerationFamilyLessonGeneration

	first := newJob(lessonID)
	first.GenerationFamily = &family
	createJob(t, pool, first)

	second := newJob(lessonID)
	second.Kind = job.KindQuizGenerate
	second.GenerationFamily = &family
	var createErr error
	inTxExpectError(t, pool, func(tx pgx.Tx) error {
		createErr = repo.Create(context.Background(), tx, second)
		return createErr
	})
	if !errors.Is(createErr, job.ErrActiveGenerationJobExists) {
		t.Fatalf("同族活动任务应冲突，得到 %v", createErr)
	}

	// 冲突时可查询现有活动 Job（409 resource_busy 的 active_job_id）。
	active, err := repo.GetActiveGenerationJob(context.Background(), pool, lessonID)
	if err != nil || active == nil || active.ID != first.ID {
		t.Fatalf("应返回活动任务 %s，得到 %+v, %v", first.ID, active, err)
	}

	// 不同 Lesson 不受影响。
	other := newJob(uuid.New())
	other.GenerationFamily = &family
	createJob(t, pool, other)

	// 原任务进入终态后可创建新任务。
	claimed := claim(t, pool, "w", time.Minute)
	if claimed == nil {
		t.Fatal("应能领取任务")
	}
	if err := repo.Complete(context.Background(), pool, first.ID, "w", []byte(`{"ok":true}`)); err != nil {
		t.Fatal(err)
	}
	third := newJob(lessonID)
	third.GenerationFamily = &family
	createJob(t, pool, third)
}

// jobs 级幂等唯一键。
func TestJobIdempotencyUnique(t *testing.T) {
	pool := databasetest.New(t)
	scope, key := "quiz_generate", uuid.NewString()
	first := newJob(uuid.New())
	first.IdempotencyScope, first.IdempotencyKey = &scope, &key
	createJob(t, pool, first)

	second := newJob(uuid.New())
	second.IdempotencyScope, second.IdempotencyKey = &scope, &key
	var createErr error
	inTxExpectError(t, pool, func(tx pgx.Tx) error {
		createErr = repo.Create(context.Background(), tx, second)
		return createErr
	})
	if !errors.Is(createErr, job.ErrDuplicateJob) {
		t.Fatalf("相同幂等键应冲突，得到 %v", createErr)
	}
}

// 取消语义：queued 直接取消；running 置 cancel_requested 后由持有方确认；终态拒绝。
func TestCancelFlow(t *testing.T) {
	pool := databasetest.New(t)

	queued := createJob(t, pool, newJob(uuid.New()))
	if err := repo.RequestCancel(context.Background(), pool, queued.ID); err != nil {
		t.Fatal(err)
	}
	if j, _ := repo.GetByID(context.Background(), pool, queued.ID); j.Status != job.StatusCancelled {
		t.Fatalf("queued 任务应直接取消，得到 %s", j.Status)
	}

	running := createJob(t, pool, newJob(uuid.New()))
	claim(t, pool, "w", time.Minute)
	if err := repo.RequestCancel(context.Background(), pool, running.ID); err != nil {
		t.Fatal(err)
	}
	if j, _ := repo.GetByID(context.Background(), pool, running.ID); j.Status != job.StatusCancelRequested || !j.CancelRequested {
		t.Fatalf("running 任务应置 cancel_requested，得到 %+v", j)
	}
	if err := repo.UpdateProgress(context.Background(), pool, running.ID, "w", "validating", 80, "正在校验"); err != nil {
		t.Fatal(err)
	}
	if j, _ := repo.GetByID(context.Background(), pool, running.ID); j.Status != job.StatusCancelRequested {
		t.Fatalf("取消请求后的进度更新不能把状态回退为 %s", j.Status)
	}
	events, err := repo.ListEvents(context.Background(), pool, running.ID, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if last := events[len(events)-1]; last.EventType != job.EventCancelRequested {
		t.Fatalf("取消后的进度事件 = %s，期望保持 %s", last.EventType, job.EventCancelRequested)
	}
	if err := repo.CancelByOwner(context.Background(), pool, running.ID, "w"); err != nil {
		t.Fatal(err)
	}
	if j, _ := repo.GetByID(context.Background(), pool, running.ID); j.Status != job.StatusCancelled {
		t.Fatalf("确认后应为 cancelled，得到 %s", j.Status)
	}
	if err := repo.RequestCancel(context.Background(), pool, running.ID); !errors.Is(err, job.ErrNotCancellable) {
		t.Fatalf("终态任务应拒绝取消，得到 %v", err)
	}
}

// 取消与 Worker 终态提交并发时，不能在已完成 Job 后追加伪 cancel_requested 事件。
func TestRequestCancelDoesNotAppendAfterConcurrentCompletion(t *testing.T) {
	pool := databasetest.New(t)
	j := createJob(t, pool, newJob(uuid.New()))
	claim(t, pool, "w", time.Minute)

	completeTx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = completeTx.Rollback(context.Background()) }()
	if err := repo.Complete(context.Background(), completeTx, j.ID, "w", []byte(`{"ok":true}`)); err != nil {
		t.Fatal(err)
	}

	cancelTx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cancelTx.Rollback(context.Background()) }()
	var cancelPID int32
	if err := cancelTx.QueryRow(context.Background(), `SELECT pg_backend_pid()`).Scan(&cancelPID); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		err := repo.RequestCancel(context.Background(), cancelTx, j.ID)
		if err == nil {
			err = cancelTx.Commit(context.Background())
		} else {
			_ = cancelTx.Rollback(context.Background())
		}
		result <- err
	}()

	deadline := time.Now().Add(3 * time.Second)
	for {
		var waitType *string
		if err := pool.QueryRow(context.Background(),
			`SELECT wait_event_type FROM pg_stat_activity WHERE pid = $1`, cancelPID).Scan(&waitType); err != nil {
			t.Fatal(err)
		}
		if waitType != nil && *waitType == "Lock" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("取消事务未等待终态事务的行锁")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := completeTx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := <-result; !errors.Is(err, job.ErrNotCancellable) {
		t.Fatalf("并发终态后的取消应返回 ErrNotCancellable，得到 %v", err)
	}

	events, err := repo.ListEvents(context.Background(), pool, j.ID, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if event.EventType == job.EventCancelRequested {
			t.Fatalf("已完成任务出现伪取消事件: %+v", event)
		}
	}
}

// 事件：创建/领取产生持久化事件，id 单调递增，支持 afterID 回放。
func TestJobEventsLifecycle(t *testing.T) {
	pool := databasetest.New(t)
	j := createJob(t, pool, newJob(uuid.New()))
	claim(t, pool, "w", time.Minute)

	events, err := repo.ListEvents(context.Background(), pool, j.ID, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].EventType != job.EventQueued || events[1].EventType != job.EventStarted {
		t.Fatalf("事件序列不符合预期: %+v", events)
	}
	if events[1].ID <= events[0].ID {
		t.Fatal("事件 id 应单调递增")
	}
	replay, err := repo.ListEvents(context.Background(), pool, j.ID, events[0].ID, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(replay) != 1 || replay[0].EventType != job.EventStarted {
		t.Fatalf("afterID 回放不符合预期: %+v", replay)
	}
}

// 手动重试：failed 任务派生新 Job，retry_of_job_id 关联。
func TestRetryFailedCreatesLinkedJob(t *testing.T) {
	pool := databasetest.New(t)
	j := createJob(t, pool, newJob(uuid.New()))
	claim(t, pool, "w", time.Minute)
	if err := repo.Fail(context.Background(), pool, j.ID, "w", "input_error", "不支持的文件", false); err != nil {
		t.Fatal(err)
	}
	var retry *job.Job
	inTx(t, pool, func(tx pgx.Tx) error {
		var err error
		retry, err = repo.RetryFailed(context.Background(), tx, j.ID, nil, nil)
		return err
	})
	if retry.RetryOfJobID == nil || *retry.RetryOfJobID != j.ID {
		t.Fatalf("retry_of_job_id 未关联: %+v", retry.RetryOfJobID)
	}
	if retry.Status != job.StatusQueued {
		t.Fatalf("重试任务应为 queued，得到 %s", retry.Status)
	}
}

func inTxExpectError(t *testing.T, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) {
	t.Helper()
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	_ = fn(tx)
}
