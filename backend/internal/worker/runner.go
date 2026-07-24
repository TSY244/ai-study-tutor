package worker

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/backend/internal/filestore"
	"github.com/TSY244/ai-study-tutor/backend/internal/job"
)

// Runner 领取并执行 Job。一个进程内可按并发度运行多个 Runner 循环；
// 同一 Job 不会重复领取（SKIP LOCKED + 租约，03 §7.1）。
type Runner struct {
	Pool     *pgxpool.Pool
	Registry *Registry
	Files    *filestore.Store
	Repo     *job.Repository

	WorkerID   string
	Lease      time.Duration // 租约时长（默认 60s）
	JobTimeout time.Duration // 单任务总时限（默认 30m，03 §8.2）

	// 可选调参（测试可缩短）。
	PollInterval       time.Duration // 无任务时的轮询间隔（默认 2s）
	HeartbeatInterval  time.Duration // 租约心跳间隔（默认 Lease/3）
	CancelPollInterval time.Duration // cancel_requested 轮询间隔（默认 2s）

	Logger *slog.Logger
}

func (r *Runner) withDefaults() {
	if r.Repo == nil {
		r.Repo = &job.Repository{}
	}
	if r.Lease <= 0 {
		r.Lease = 60 * time.Second
	}
	if r.JobTimeout <= 0 {
		r.JobTimeout = 30 * time.Minute
	}
	if r.PollInterval <= 0 {
		r.PollInterval = 2 * time.Second
	}
	if r.HeartbeatInterval <= 0 {
		r.HeartbeatInterval = r.Lease / 3
	}
	if r.CancelPollInterval <= 0 {
		r.CancelPollInterval = 2 * time.Second
	}
	if r.Logger == nil {
		r.Logger = slog.Default()
	}
}

// Run 是常驻领取循环：有任务立即执行，无任务按 PollInterval 轮询；
// ctx 取消后不再领取新任务并返回。
func (r *Runner) Run(ctx context.Context) {
	r.withDefaults()
	for {
		if ctx.Err() != nil {
			return
		}
		claimed, err := r.RunOnce(ctx)
		if err != nil {
			r.Logger.Warn("job_claim_failed", slog.String("error", err.Error()))
		}
		if !claimed {
			select {
			case <-ctx.Done():
				return
			case <-time.After(r.PollInterval):
			}
		}
	}
}

// RunOnce 领取并执行至多一个 Job；返回是否领取到任务。
// 任务级结果（成功/失败/取消）在内部完成落库，不作为 error 返回；
// error 仅表示队列级故障（如数据库不可达）。
func (r *Runner) RunOnce(ctx context.Context) (bool, error) {
	r.withDefaults()
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	j, err := r.Repo.Claim(ctx, tx, r.WorkerID, r.Lease)
	if err != nil {
		_ = tx.Rollback(ctx)
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	if j == nil {
		return false, nil
	}
	r.execute(ctx, j)
	return true, nil
}

// execute 编排单个 Job 的生命周期：临时目录、租约守护、Handler 执行、
// 终态事务。所有路径都保证不 panic；租约丢失时放弃任务由 Recovery 接管。
func (r *Runner) execute(ctx context.Context, j *job.Job) {
	logger := r.Logger.With(slog.String("job_id", j.ID.String()), slog.String("kind", j.Kind))
	h, ok := r.Registry.Lookup(j.Kind)
	if !ok {
		r.failJob(ctx, j, nil, NewError(ErrCodeUnsupportedKind,
			"该任务类型尚未实现，请等待后续版本或取消相关操作。", false))
		return
	}

	workDir := r.Files.TmpDirFor(j.ID)
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		r.failJob(ctx, j, h, WrapError(ErrCodeInternal, "创建任务临时目录失败，将自动重试。", true, err))
		return
	}
	defer func() { _ = os.RemoveAll(workDir) }()

	jctx, cancel := context.WithTimeout(ctx, r.JobTimeout)
	defer cancel()

	guard := newLeaseGuard(r.Pool, r.Repo, j.ID, r.WorkerID, r.Lease,
		r.HeartbeatInterval, r.CancelPollInterval, cancel, logger)
	guard.start()
	defer guard.stop()

	ex := &Execution{
		Job: j, WorkDir: workDir, Files: r.Files, Pool: r.Pool, Logger: logger,
		stageFn: r.stageFunc(j),
	}
	outcome, herr := h.Execute(jctx, ex)
	guard.stop()

	switch {
	case guard.leaseLost() || errors.Is(herr, job.ErrLeaseLost):
		// 租约已不属于本 Worker：任务状态由 Recovery 处置，不再写入。
		logger.Warn("job_lease_lost")
	case guard.cancelRequested():
		r.cancelJob(j, h, logger)
	case ctx.Err() != nil:
		// 进程关停：不触碰任务状态，租约到期后由 Recovery 回收重试。
		logger.Info("job_abandoned_on_shutdown")
	case herr == nil:
		r.completeJob(ctx, j, outcome, logger)
	default:
		werr := Classify(herr)
		if errors.Is(herr, context.DeadlineExceeded) || jctx.Err() == context.DeadlineExceeded {
			werr = NewError(ErrCodeJobTimeout, "任务超过总时限，将自动重试。", true)
		}
		r.failJob(ctx, j, h, werr)
	}
}

// stageFunc 生成 Execution.Stage 的实现：短事务更新阶段并写事件。
func (r *Runner) stageFunc(j *job.Job) func(ctx context.Context, stage string, progress int16, message string) error {
	return func(ctx context.Context, stage string, progress int16, message string) error {
		tx, err := r.Pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback(ctx) }()
		if err := r.Repo.UpdateProgress(ctx, tx, j.ID, r.WorkerID, stage, progress, message); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
}

// completeJob 成功路径：领域发布与 Complete 同事务（03 §6.4）。
// 发布失败按 publish_failed 可重试处理；此时领域写入已随事务回滚。
func (r *Runner) completeJob(ctx context.Context, j *job.Job, outcome *Outcome, logger *slog.Logger) {
	fctx, cancel := finalCtx(ctx)
	defer cancel()
	tx, err := r.Pool.Begin(fctx)
	if err != nil {
		logger.Warn("job_complete_begin_failed", slog.String("error", err.Error()))
		return
	}
	defer func() { _ = tx.Rollback(fctx) }()
	var result []byte
	if outcome != nil && outcome.Publish != nil {
		if result, err = outcome.Publish(fctx, tx); err != nil {
			logger.Warn("job_publish_failed", slog.String("error", err.Error()))
			r.failJob(ctx, j, nil, WrapError(ErrCodePublishFailed, "发布任务结果失败，将自动重试。", true, err))
			return
		}
	}
	if err := r.Repo.Complete(fctx, tx, j.ID, r.WorkerID, result); err != nil {
		if !errors.Is(err, job.ErrLeaseLost) {
			logger.Warn("job_complete_failed", slog.String("error", err.Error()))
		}
		return
	}
	if err := tx.Commit(fctx); err != nil {
		logger.Warn("job_complete_commit_failed", slog.String("error", err.Error()))
	}
}

// failJob 失败路径：repo.Fail 内部决定 requeue（指数退避）或终态 failed；
// 仅终态时在同一事务运行 Handler.FinalizeFail。
func (r *Runner) failJob(ctx context.Context, j *job.Job, h Handler, werr *Error) {
	fctx, cancel := finalCtx(ctx)
	defer cancel()
	tx, err := r.Pool.Begin(fctx)
	if err != nil {
		r.Logger.Warn("job_fail_begin_failed", slog.String("error", err.Error()))
		return
	}
	defer func() { _ = tx.Rollback(fctx) }()
	if err := r.Repo.Fail(fctx, tx, j.ID, r.WorkerID, werr.Code, werr.Detail, werr.Retryable); err != nil {
		if !errors.Is(err, job.ErrLeaseLost) {
			r.Logger.Warn("job_fail_update_failed", slog.String("error", err.Error()))
		}
		return
	}
	updated, err := r.Repo.GetByID(fctx, tx, j.ID)
	if err != nil {
		r.Logger.Warn("job_fail_readback_failed", slog.String("error", err.Error()))
		return
	}
	if updated.Status == job.StatusFailed && h != nil {
		if err := h.FinalizeFail(fctx, tx, updated, werr.Code); err != nil {
			r.Logger.Warn("job_fail_finalize_failed", slog.String("error", err.Error()))
			return
		}
	}
	if err := tx.Commit(fctx); err != nil {
		r.Logger.Warn("job_fail_commit_failed", slog.String("error", err.Error()))
	}
}

// cancelJob 取消路径：确认取消并运行 Handler.FinalizeCancel。
func (r *Runner) cancelJob(j *job.Job, h Handler, logger *slog.Logger) {
	fctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tx, err := r.Pool.Begin(fctx)
	if err != nil {
		logger.Warn("job_cancel_begin_failed", slog.String("error", err.Error()))
		return
	}
	defer func() { _ = tx.Rollback(fctx) }()
	if err := r.Repo.CancelByOwner(fctx, tx, j.ID, r.WorkerID); err != nil {
		if !errors.Is(err, job.ErrLeaseLost) {
			logger.Warn("job_cancel_failed", slog.String("error", err.Error()))
		}
		return
	}
	if h != nil {
		if err := h.FinalizeCancel(fctx, tx, j); err != nil {
			logger.Warn("job_cancel_finalize_failed", slog.String("error", err.Error()))
			return
		}
	}
	if err := tx.Commit(fctx); err != nil {
		logger.Warn("job_cancel_commit_failed", slog.String("error", err.Error()))
	}
}

// finalCtx 派生终态事务上下文：父 ctx 已取消时（进程关停路径除外，
// 该路径不进入终态事务）仍给予短时限完成落库。
func finalCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx.Err() == nil {
		return context.WithTimeout(ctx, 60*time.Second)
	}
	return context.WithTimeout(context.Background(), 30*time.Second)
}

// leaseGuard 是执行期间的租约守护：周期心跳续租 + cancel_requested 轮询。
// 任一触发即取消任务上下文。
type leaseGuard struct {
	pool       *pgxpool.Pool
	repo       *job.Repository
	jobID      uuid.UUID
	owner      string
	lease      time.Duration
	hbInterval time.Duration
	poll       time.Duration
	cancel     context.CancelFunc
	logger     *slog.Logger

	lost      atomic.Bool
	cancelReq atomic.Bool
	stopCh    chan struct{}
	stopOnce  sync.Once
	wg        sync.WaitGroup
}

func newLeaseGuard(pool *pgxpool.Pool, repo *job.Repository, jobID uuid.UUID, owner string,
	lease, hbInterval, poll time.Duration, cancel context.CancelFunc, logger *slog.Logger) *leaseGuard {
	return &leaseGuard{
		pool: pool, repo: repo, jobID: jobID, owner: owner, lease: lease,
		hbInterval: hbInterval, poll: poll, cancel: cancel, logger: logger,
		stopCh: make(chan struct{}),
	}
}

func (g *leaseGuard) start() {
	g.wg.Add(2)
	go g.heartbeatLoop()
	go g.cancelLoop()
}

func (g *leaseGuard) stop() {
	g.stopOnce.Do(func() { close(g.stopCh) })
	g.wg.Wait()
}

func (g *leaseGuard) leaseLost() bool       { return g.lost.Load() }
func (g *leaseGuard) cancelRequested() bool { return g.cancelReq.Load() }

func (g *leaseGuard) heartbeatLoop() {
	defer g.wg.Done()
	t := time.NewTicker(g.hbInterval)
	defer t.Stop()
	for {
		select {
		case <-g.stopCh:
			return
		case <-t.C:
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := g.repo.Heartbeat(ctx, g.pool, g.jobID, g.owner, g.lease)
		cancel()
		if err == nil {
			continue
		}
		if errors.Is(err, job.ErrLeaseLost) {
			g.lost.Store(true)
			g.cancel()
			return
		}
		g.logger.Warn("job_heartbeat_failed", slog.String("error", err.Error()))
	}
}

func (g *leaseGuard) cancelLoop() {
	defer g.wg.Done()
	t := time.NewTicker(g.poll)
	defer t.Stop()
	for {
		select {
		case <-g.stopCh:
			return
		case <-t.C:
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		requested, err := g.repo.CancelRequested(ctx, g.pool, g.jobID)
		cancel()
		if err != nil {
			g.logger.Warn("job_cancel_poll_failed", slog.String("error", err.Error()))
			continue
		}
		if requested {
			g.cancelReq.Store(true)
			g.cancel()
			return
		}
	}
}
