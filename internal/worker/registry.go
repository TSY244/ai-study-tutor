package worker

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/internal/filestore"
	"github.com/TSY244/ai-study-tutor/internal/job"
)

// PublishFunc 在 Job 成功的同一事务中发布领域结果，并返回写入
// jobs.result 的小体积摘要 JSON（不得含学习材料正文）。
// 与 repo.Complete 同事务，保证“发布领域结果和切换终态原子完成”（03 §6.4）。
type PublishFunc func(ctx context.Context, tx pgx.Tx) ([]byte, error)

// FinalizeFunc 在 Job 进入终态（failed/cancelled）的同一事务中整理
// 业务侧状态（如素材/课时状态回写）。
type FinalizeFunc func(ctx context.Context, tx pgx.Tx, j *job.Job) error

// FailFinalizeFunc 同 FinalizeFunc，额外携带终态错误码。
type FailFinalizeFunc func(ctx context.Context, tx pgx.Tx, j *job.Job, code string) error

// Outcome 是 Handler 成功执行后的产物。
type Outcome struct {
	// Publish 在 Complete 事务中执行领域发布并返回 result JSON。
	Publish PublishFunc
}

// Handler 实现一种 Job kind 的执行。约定：
//
//   - Execute 收到 context，必须在阶段边界响应取消；
//   - 阶段推进经 Execution.Stage 上报（稳定 stage code）；
//   - 错误用 *worker.Error 分类（重试策略）；
//   - Execute 返回错误时 Outcome 被忽略；失败/取消的侧写由
//     FinalizeFail/FinalizeCancel 在终态事务中完成；
//   - FinalizeFail 只在 Job 进入 failed 终态时运行（自动重试 requeue 不运行）。
type Handler interface {
	Execute(ctx context.Context, ex *Execution) (*Outcome, error)
	FinalizeFail(ctx context.Context, tx pgx.Tx, j *job.Job, code string) error
	FinalizeCancel(ctx context.Context, tx pgx.Tx, j *job.Job) error
}

// Registry 按 Job kind 分发 Handler。
type Registry struct {
	handlers map[string]Handler
}

// NewRegistry 创建空 Registry。
func NewRegistry() *Registry {
	return &Registry{handlers: map[string]Handler{}}
}

// Register 注册 Handler；重复注册同一 kind 会 panic（编程错误）。
func (r *Registry) Register(kind string, h Handler) {
	if _, exists := r.handlers[kind]; exists {
		panic("worker: 重复注册 Job kind " + kind)
	}
	r.handlers[kind] = h
}

// Lookup 查找 kind 对应 Handler。
func (r *Registry) Lookup(kind string) (Handler, bool) {
	h, ok := r.handlers[kind]
	return h, ok
}

// Execution 是 Handler 的执行上下文。
type Execution struct {
	Job     *job.Job
	WorkDir string // 数据目录 tmp/{job_id}/，任务结束由 Runner 清理
	Files   *filestore.Store
	Pool    *pgxpool.Pool
	Logger  *slog.Logger

	stageFn func(ctx context.Context, stage string, progress int16, message string) error
}

// Stage 上报阶段进度（短事务写 jobs + job.progress 事件）。
// message 只含阶段摘要，不含学习材料正文。租约丢失返回 job.ErrLeaseLost，
// Handler 必须中止执行。
func (e *Execution) Stage(ctx context.Context, stage string, progress int16, message string) error {
	return e.stageFn(ctx, stage, progress, message)
}
