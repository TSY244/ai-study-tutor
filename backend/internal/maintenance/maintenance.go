// Package maintenance 实现系统维护状态与崩溃恢复协议（03 §6.7）：
//
//   - Maintenance Owner Lock：session-scoped advisory lock，备份进程持有
//     整个生命周期；SIGKILL 后由 PostgreSQL 自动释放。
//   - Business Gate：业务写入/Job claim/发布用 shared lock；维护切换与
//     quiesced 备份窗口用 exclusive lock（见 platform/gate）。
//   - 租约与心跳使用数据库时间，主机时钟不参与比较。
//   - maintenance-repair：Owner Lock + Business Gate exclusive 下先回收
//     过期 Job lease 与 temporary 文件，再清理遗留维护状态。
package maintenance

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/backend/internal/filestore"
	"github.com/TSY244/ai-study-tutor/backend/internal/job"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/database"
	"github.com/TSY244/ai-study-tutor/backend/internal/platform/gate"
)

// 维护阶段。
const (
	PhaseNormal   = "normal"
	PhaseDraining = "draining"
	PhaseQuiesced = "quiesced"
)

var (
	// ErrMaintenanceActive：Maintenance Owner Lock 被占用，仍有活跃维护进程。
	ErrMaintenanceActive = errors.New("仍有活跃维护进程，拒绝修复")
	// ErrLeaseNotExpired：维护租约按数据库时间尚未过期，拒绝修复。
	ErrLeaseNotExpired = errors.New("维护租约尚未过期，拒绝修复")
	// ErrActiveLeasesExist：仍有未过期的 Job lease，拒绝修复。
	ErrActiveLeasesExist = errors.New("存在尚未过期的 Job 租约，拒绝修复")
	// ErrNotInMaintenance：系统不在维护模式，无需修复。
	ErrNotInMaintenance = errors.New("系统不在维护模式")
)

// SystemState 是 system_state 单行表的领域表示。
type SystemState struct {
	MaintenanceMode           bool
	Phase                     string
	MaintenanceReason         *string
	MaintenanceOwner          *string
	MaintenanceStartedAt      *time.Time
	MaintenanceLeaseExpiresAt *time.Time
	MaintenanceHeartbeatAt    *time.Time
	Version                   int64
	UpdatedAt                 time.Time
}

// Repository 提供 system_state 读写。所有方法接受 database.Querier，
// 由调用方控制事务边界（维护切换在 Business Gate exclusive lock 下执行）。
type Repository struct{}

// Get 读取单行系统状态。
func (Repository) Get(ctx context.Context, q database.Querier) (*SystemState, error) {
	var s SystemState
	err := q.QueryRow(ctx, `
		SELECT maintenance_mode, phase, maintenance_reason, maintenance_owner,
		       maintenance_started_at, maintenance_lease_expires_at,
		       maintenance_heartbeat_at, version, updated_at
		FROM system_state WHERE singleton = true`).Scan(
		&s.MaintenanceMode, &s.Phase, &s.MaintenanceReason, &s.MaintenanceOwner,
		&s.MaintenanceStartedAt, &s.MaintenanceLeaseExpiresAt,
		&s.MaintenanceHeartbeatAt, &s.Version, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("读取系统状态失败: %w", err)
	}
	return &s, nil
}

// EnterMaintenance 进入维护模式（draining）。租约时间由数据库 now() 计算，
// 主机时钟不参与。调用方必须先持有 Business Gate exclusive lock。
func (Repository) EnterMaintenance(ctx context.Context, tx database.Querier, owner, reason string, lease time.Duration) error {
	tag, err := tx.Exec(ctx, `
		UPDATE system_state
		SET maintenance_mode = true, phase = 'draining',
		    maintenance_reason = $2, maintenance_owner = $1,
		    maintenance_started_at = now(), maintenance_heartbeat_at = now(),
		    maintenance_lease_expires_at = now() + $3::interval,
		    version = version + 1, updated_at = now()
		WHERE singleton = true AND maintenance_mode = false`,
		owner, reason, lease.String())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("系统已在维护模式")
	}
	return nil
}

// SetPhase 更新维护阶段（draining -> quiesced）。
func (Repository) SetPhase(ctx context.Context, tx database.Querier, phase string) error {
	_, err := tx.Exec(ctx, `
		UPDATE system_state SET phase = $1, version = version + 1, updated_at = now()
		WHERE singleton = true AND maintenance_mode = true`, phase)
	return err
}

// Heartbeat 用独立数据库连接续租（03 §6.7：每 10 秒，使用数据库时间）。
// 只有当前 Owner 可续租。
func (Repository) Heartbeat(ctx context.Context, conn database.Querier, owner string, lease time.Duration) error {
	tag, err := conn.Exec(ctx, `
		UPDATE system_state
		SET maintenance_heartbeat_at = now(),
		    maintenance_lease_expires_at = now() + $2::interval,
		    updated_at = now()
		WHERE singleton = true AND maintenance_mode = true AND maintenance_owner = $1`,
		owner, lease.String())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("维护 Owner 已变更或维护已结束，心跳失败")
	}
	return nil
}

// ClearMaintenance 退出维护模式并清除 Owner/租约字段。
func (Repository) ClearMaintenance(ctx context.Context, tx database.Querier) error {
	_, err := tx.Exec(ctx, `
		UPDATE system_state
		SET maintenance_mode = false, phase = 'normal',
		    maintenance_reason = NULL, maintenance_owner = NULL,
		    maintenance_started_at = NULL, maintenance_lease_expires_at = NULL,
		    maintenance_heartbeat_at = NULL,
		    version = version + 1, updated_at = now()
		WHERE singleton = true`)
	return err
}

// RepairReport 是 maintenance-repair 的处置结果（恢复审计）。
type RepairReport struct {
	JobsRequeued     int
	JobsFailed       int
	JobsCancelled    int
	FilesCompleted   int
	FilesQuarantined int
	FilesDeleted     int
	FilesOrphaned    int
	BackupsFailed    int
}

// Repairer 执行崩溃恢复协议（03 §6.7 / 05 §10.2）。只有 maintenance-repair
// 一次性服务（doctor --repair-stale-maintenance）应调用 Repair。
type Repairer struct {
	Pool  *pgxpool.Pool
	Jobs  *job.Repository
	Files *filestore.Store
	Log   *slog.Logger
}

// Repair 修复 SIGKILL/主机重启遗留的维护状态：
//
//  1. pg_try_advisory_lock 获取 Maintenance Owner Lock；被占用则拒绝；
//  2. Business Gate exclusive lock 下复查 maintenance_mode 且租约已按
//     数据库时间过期；存在未过期 Job lease 时拒绝；
//  3. 回收过期 Job lease（cancel_requested→cancelled；未达上限→queued；
//     达上限→failed(worker_lost)）；
//  4. 按同一文件 Recovery 完成/隔离/删除过期 temporary 对象；
//  5. 复查无活动 lease/temporary 后，把该 Owner 的 creating 备份标为
//     failed(maintenance_lease_expired)，把 system_state 归一为 normal；
//  6. 任一步骤失败保持 maintenance 并返回可行动错误。
func (r *Repairer) Repair(ctx context.Context) (*RepairReport, error) {
	log := r.Log
	if log == nil {
		log = slog.Default()
	}
	// Owner Lock 需要 session 级专用连接，不能经 pool 复用。
	conn, err := r.Pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取修复连接失败: %w", err)
	}
	defer conn.Release()
	acquired, err := gate.TryMaintenanceOwner(ctx, conn)
	if err != nil {
		return nil, err
	}
	if !acquired {
		return nil, ErrMaintenanceActive
	}
	defer func() {
		// 连接释放时 PostgreSQL 自动解锁；显式解锁便于早释放。
		_ = gate.UnlockMaintenanceOwner(context.Background(), conn)
	}()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := gate.LockBusinessExclusive(ctx, tx); err != nil {
		return nil, fmt.Errorf("获取业务门禁独占锁失败: %w", err)
	}

	stateRepo := Repository{}
	state, err := stateRepo.Get(ctx, tx)
	if err != nil {
		return nil, err
	}
	if !state.MaintenanceMode {
		return nil, ErrNotInMaintenance
	}
	var leaseExpired bool
	if err := tx.QueryRow(ctx, `
		SELECT maintenance_lease_expires_at IS NOT NULL
		   AND maintenance_lease_expires_at <= now()
		FROM system_state WHERE singleton = true`).Scan(&leaseExpired); err != nil {
		return nil, err
	}
	if !leaseExpired {
		return nil, ErrLeaseNotExpired
	}
	var activeJobLeases bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM jobs
			WHERE status IN ('running','validating','cancel_requested')
			  AND lease_expires_at > now())`).Scan(&activeJobLeases); err != nil {
		return nil, err
	}
	if activeJobLeases {
		return nil, ErrActiveLeasesExist
	}

	// 步骤 3：回收过期 Job lease。
	jobRepo := r.Jobs
	if jobRepo == nil {
		jobRepo = &job.Repository{}
	}
	requeued, failed, cancelled, err := jobRepo.RecoverExpiredLeases(ctx, tx, 500)
	if err != nil {
		return nil, fmt.Errorf("回收过期 Job 租约失败: %w", err)
	}

	// 步骤 4：文件 Recovery。
	var filesRes filestore.RecoverResult
	if r.Files != nil {
		filesRes, err = r.Files.Recover(ctx, tx, 500)
		if err != nil {
			return nil, fmt.Errorf("文件恢复失败: %w", err)
		}
	}

	// 步骤 5 前置复查：不允许残留活动 lease 或未决 temporary。
	var remaining int
	if err := tx.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM jobs
		        WHERE status IN ('running','validating','cancel_requested'))
		     + (SELECT count(*) FROM file_objects WHERE state = 'temporary')`).Scan(&remaining); err != nil {
		return nil, err
	}
	if remaining > 0 {
		return nil, fmt.Errorf("仍有 %d 个活动任务或未决文件对象，保持维护状态", remaining)
	}

	// 同一 Owner 的 creating 备份 → failed(maintenance_lease_expired)。
	owner := ""
	if state.MaintenanceOwner != nil {
		owner = *state.MaintenanceOwner
	}
	tag, err := tx.Exec(ctx, `
		UPDATE backups
		SET status = 'failed', error_code = $1,
		    error_detail = '维护 Owner 租约过期，备份由 maintenance-repair 回收',
		    completed_at = now()
		WHERE status = 'creating' AND maintenance_owner = $2`,
		job.ErrCodeMaintenanceLeaseExpired, owner)
	if err != nil {
		return nil, err
	}

	// 归一系统状态。
	if err := stateRepo.ClearMaintenance(ctx, tx); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	report := &RepairReport{
		JobsRequeued: requeued, JobsFailed: failed, JobsCancelled: cancelled,
		FilesCompleted: filesRes.Completed, FilesQuarantined: filesRes.Quarantined,
		FilesDeleted: filesRes.Deleted, FilesOrphaned: filesRes.Orphaned,
		BackupsFailed: int(tag.RowsAffected()),
	}
	// 恢复审计：结构化日志（不含敏感内容）。
	log.Info("maintenance_repair_completed",
		slog.Int("jobs_requeued", report.JobsRequeued),
		slog.Int("jobs_failed", report.JobsFailed),
		slog.Int("jobs_cancelled", report.JobsCancelled),
		slog.Int("files_completed", report.FilesCompleted),
		slog.Int("files_quarantined", report.FilesQuarantined),
		slog.Int("files_deleted", report.FilesDeleted),
		slog.Int("files_orphaned", report.FilesOrphaned),
		slog.Int("backups_failed", report.BackupsFailed),
		slog.String("previous_owner", owner))
	return report, nil
}

// Status 是 doctor 只读检查的结果。
type Status struct {
	MaintenanceMode  bool
	Phase            string
	LeaseExpired     bool
	ActiveJobLeases  int
	ExpiredTemporary int
	CreatingBackups  int
}

// Check 只读巡检：不获取任何锁，不改变任何状态。
func Check(ctx context.Context, q database.Querier) (*Status, error) {
	var st Status
	err := q.QueryRow(ctx, `
		SELECT maintenance_mode, phase,
		       maintenance_lease_expires_at IS NOT NULL
		           AND maintenance_lease_expires_at <= now()
		FROM system_state WHERE singleton = true`).Scan(&st.MaintenanceMode, &st.Phase, &st.LeaseExpired)
	if err != nil {
		return nil, err
	}
	if err := q.QueryRow(ctx, `
		SELECT count(*) FROM jobs
		WHERE status IN ('running','validating','cancel_requested')
		  AND lease_expires_at > now()`).Scan(&st.ActiveJobLeases); err != nil {
		return nil, err
	}
	if err := q.QueryRow(ctx, `
		SELECT count(*) FROM file_objects
		WHERE state = 'temporary' AND temporary_expires_at <= now()`).Scan(&st.ExpiredTemporary); err != nil {
		return nil, err
	}
	if err := q.QueryRow(ctx,
		`SELECT count(*) FROM backups WHERE status = 'creating'`).Scan(&st.CreatingBackups); err != nil {
		return nil, err
	}
	return &st, nil
}
