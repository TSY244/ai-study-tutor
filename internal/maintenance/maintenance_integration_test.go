package maintenance_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/internal/filestore"
	"github.com/TSY244/ai-study-tutor/internal/job"
	"github.com/TSY244/ai-study-tutor/internal/maintenance"
	"github.com/TSY244/ai-study-tutor/internal/platform/databasetest"
	"github.com/TSY244/ai-study-tutor/internal/platform/gate"
)

func TestMain(m *testing.M) { databasetest.Main(m) }

var stateRepo = maintenance.Repository{}

// enterMaintenance 以备份进程姿势进入维护模式（Owner Lock + Gate exclusive）。
func enterMaintenance(t *testing.T, pool *pgxpool.Pool, owner string, lease time.Duration) {
	t.Helper()
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := gate.LockBusinessExclusive(context.Background(), tx); err != nil {
		t.Fatal(err)
	}
	if err := stateRepo.EnterMaintenance(context.Background(), tx, owner, "一致性备份", lease); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func seedJobWithLease(t *testing.T, pool *pgxpool.Pool, status string, attempts int32, expired bool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	leaseExpr := "now() + interval '5 minutes'"
	if expired {
		leaseExpr = "now() - interval '1 second'"
	}
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO jobs (id, kind, aggregate_type, aggregate_id, status,
			attempt_count, max_attempts, lease_owner, lease_expires_at, started_at)
		VALUES ($1,'lesson_import','lesson',$2,$3,$4,3,'dead-worker',`+leaseExpr+`, now())`,
		id, uuid.New(), status, attempts); err != nil {
		t.Fatal(err)
	}
	return id
}

// 维护模式：进入/心跳/退出与业务写入拒绝。
func TestMaintenanceLifecycle(t *testing.T) {
	pool := databasetest.New(t)
	ctx := context.Background()

	enterMaintenance(t, pool, "backup-1", time.Minute)

	st, err := stateRepo.Get(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if !st.MaintenanceMode || st.Phase != maintenance.PhaseDraining {
		t.Fatalf("维护状态不正确: %+v", st)
	}
	if st.MaintenanceOwner == nil || *st.MaintenanceOwner != "backup-1" {
		t.Fatalf("Owner 不正确: %+v", st.MaintenanceOwner)
	}

	// 维护模式拒绝新业务写入。
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := gate.AssertBusinessOpen(ctx, tx); !errors.Is(err, gate.ErrMaintenanceMode) {
		t.Fatalf("维护中应拒绝业务写入，得到 %v", err)
	}
	_ = tx.Rollback(ctx)

	// Owner 心跳续租（数据库时间）。
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	if err := stateRepo.Heartbeat(ctx, conn, "backup-1", 2*time.Minute); err != nil {
		t.Fatal(err)
	}
	var valid bool
	if err := conn.QueryRow(ctx,
		`SELECT maintenance_lease_expires_at > now() + interval '90 seconds'
		 FROM system_state WHERE singleton = true`).Scan(&valid); err != nil {
		t.Fatal(err)
	}
	if !valid {
		t.Fatal("心跳未按数据库时间续租")
	}
	// 非 Owner 心跳失败。
	if err := stateRepo.Heartbeat(ctx, conn, "intruder", time.Minute); err == nil {
		t.Fatal("非 Owner 心跳应失败")
	}

	// 阶段推进与退出。
	tx2, _ := pool.Begin(ctx)
	if err := stateRepo.SetPhase(ctx, tx2, maintenance.PhaseQuiesced); err != nil {
		t.Fatal(err)
	}
	if err := tx2.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	tx3, _ := pool.Begin(ctx)
	if err := stateRepo.ClearMaintenance(ctx, tx3); err != nil {
		t.Fatal(err)
	}
	if err := tx3.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	st, _ = stateRepo.Get(ctx, pool)
	if st.MaintenanceMode || st.Phase != maintenance.PhaseNormal || st.MaintenanceOwner != nil {
		t.Fatalf("退出维护后状态未归一: %+v", st)
	}
}

// SIGKILL 场景：备份进程持锁设置维护状态后连接被强杀（Owner Lock 自动释放，
// 维护行残留），maintenance-repair 完成恢复。
func TestRepairAfterOwnerSIGKILL(t *testing.T) {
	pool := databasetest.New(t)
	st, err := filestore.New(pool, t.TempDir(), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	// --- 模拟备份进程：持 Owner Lock + 维护状态 + creating 备份 ---
	victim, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	ok, err := gate.TryMaintenanceOwner(ctx, victim)
	if err != nil || !ok {
		t.Fatalf("备份进程应获得 Owner Lock: %v %v", ok, err)
	}
	enterMaintenance(t, pool, "backup-sigkill", time.Hour)
	backupID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO backups (id, status, maintenance_owner) VALUES ($1,'creating','backup-sigkill')`, backupID); err != nil {
		t.Fatal(err)
	}
	// 备份窗口内的在途任务与 temporary 对象（Owner 已死）。
	jobRetry := seedJobWithLease(t, pool, "running", 1, true)
	jobCancel := seedJobWithLease(t, pool, "cancel_requested", 1, true)
	jobDead := seedJobWithLease(t, pool, "running", 3, true)
	fileID, tmpKey := uuid.New(), filepath.Join("tmp", jobDead.String(), uuid.NewString())
	tmpAbs := filepath.Join(st.DataDir(), tmpKey)
	if err := os.MkdirAll(filepath.Dir(tmpAbs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tmpAbs, []byte("half published"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO file_objects (id, storage_key, size_bytes, state,
			temporary_owner_type, temporary_owner_id, temporary_expires_at)
		VALUES ($1,$2,14,'temporary','job',$3, now() - interval '1 second')`,
		fileID, tmpKey, jobDead); err != nil {
		t.Fatal(err)
	}

	// --- SIGKILL：连接直接消失，Owner Lock 由 PostgreSQL 释放，维护行残留 ---
	victim.Hijack().Close(ctx)

	// 租约尚未过期时拒绝修复。
	repairer := &maintenance.Repairer{Pool: pool, Files: st}
	if _, err := repairer.Repair(ctx); !errors.Is(err, maintenance.ErrLeaseNotExpired) {
		t.Fatalf("租约未过期应拒绝修复，得到 %v", err)
	}
	// 时间流逝（数据库时间），租约过期。
	if _, err := pool.Exec(ctx, `
		UPDATE system_state SET maintenance_lease_expires_at = now() - interval '1 second'
		WHERE singleton = true`); err != nil {
		t.Fatal(err)
	}

	report, err := repairer.Repair(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if report.JobsRequeued != 1 || report.JobsFailed != 1 || report.JobsCancelled != 1 {
		t.Fatalf("Job 回收计数不符: %+v", report)
	}
	if report.FilesQuarantined != 1 {
		t.Fatalf("文件回收计数不符: %+v", report)
	}
	if report.BackupsFailed != 1 {
		t.Fatalf("备份回收计数不符: %+v", report)
	}

	// Job 终局。
	assertJob := func(id uuid.UUID, wantStatus, wantCode string) {
		t.Helper()
		j, err := (&job.Repository{}).GetByID(ctx, pool, id)
		if err != nil {
			t.Fatal(err)
		}
		if j.Status != wantStatus {
			t.Fatalf("Job %s status = %s，期望 %s", id, j.Status, wantStatus)
		}
		if wantCode != "" && (j.ErrorCode == nil || *j.ErrorCode != wantCode) {
			t.Fatalf("Job %s error_code = %+v，期望 %s", id, j.ErrorCode, wantCode)
		}
	}
	assertJob(jobRetry, "queued", job.ErrCodeWorkerLost)
	assertJob(jobCancel, "cancelled", "")
	assertJob(jobDead, "failed", job.ErrCodeWorkerLost)

	// 文件被隔离且真实移动。
	fo, err := st.GetByID(ctx, pool, fileID)
	if err != nil {
		t.Fatal(err)
	}
	if fo.State != filestore.StateQuarantined {
		t.Fatalf("文件状态 = %s，期望 quarantined", fo.State)
	}
	if _, err := os.Stat(filepath.Join(st.DataDir(), fo.StorageKey)); err != nil {
		t.Fatalf("隔离文件应存在: %v", err)
	}

	// 备份标为 failed(maintenance_lease_expired)。
	var bStatus, bCode string
	if err := pool.QueryRow(ctx,
		`SELECT status, error_code FROM backups WHERE id = $1`, backupID).Scan(&bStatus, &bCode); err != nil {
		t.Fatal(err)
	}
	if bStatus != "failed" || bCode != job.ErrCodeMaintenanceLeaseExpired {
		t.Fatalf("备份 = %s/%s，期望 failed/%s", bStatus, bCode, job.ErrCodeMaintenanceLeaseExpired)
	}

	// 系统状态归一。
	final, _ := stateRepo.Get(ctx, pool)
	if final.MaintenanceMode || final.Phase != maintenance.PhaseNormal || final.MaintenanceOwner != nil {
		t.Fatalf("修复后系统状态未归一: %+v", final)
	}
	// 业务重新开放。
	tx, _ := pool.Begin(ctx)
	if err := gate.AssertBusinessOpen(ctx, tx); err != nil {
		t.Fatalf("修复后业务应开放: %v", err)
	}
	_ = tx.Rollback(ctx)
}

// Owner Lock 被活跃进程持有时拒绝修复。
func TestRepairRefusedWhileOwnerLockHeld(t *testing.T) {
	pool := databasetest.New(t)
	ctx := context.Background()
	enterMaintenance(t, pool, "backup-alive", time.Hour)
	if _, err := pool.Exec(ctx, `
		UPDATE system_state SET maintenance_lease_expires_at = now() - interval '1 second'
		WHERE singleton = true`); err != nil {
		t.Fatal(err)
	}
	holder, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Release()
	ok, err := gate.TryMaintenanceOwner(ctx, holder)
	if err != nil || !ok {
		t.Fatal("测试应能持有 Owner Lock")
	}
	repairer := &maintenance.Repairer{Pool: pool}
	if _, err := repairer.Repair(ctx); !errors.Is(err, maintenance.ErrMaintenanceActive) {
		t.Fatalf("Owner Lock 被持有应拒绝修复，得到 %v", err)
	}
	// 状态未被改动。
	st, _ := stateRepo.Get(ctx, pool)
	if !st.MaintenanceMode {
		t.Fatal("拒绝修复后维护状态不应变化")
	}
}

// 非维护模式拒绝修复。
func TestRepairRefusedWhenNotInMaintenance(t *testing.T) {
	pool := databasetest.New(t)
	repairer := &maintenance.Repairer{Pool: pool}
	if _, err := repairer.Repair(context.Background()); !errors.Is(err, maintenance.ErrNotInMaintenance) {
		t.Fatalf("非维护模式应返回 ErrNotInMaintenance，得到 %v", err)
	}
}

// doctor 只读巡检。
func TestDoctorCheck(t *testing.T) {
	pool := databasetest.New(t)
	enterMaintenance(t, pool, "backup-x", time.Hour)
	seedJobWithLease(t, pool, "running", 1, false)
	seedJobWithLease(t, pool, "running", 1, true) // 过期但未被 repair 前可见
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO backups (id, status, maintenance_owner) VALUES ($1,'creating','backup-x')`, uuid.New()); err != nil {
		t.Fatal(err)
	}
	st, err := maintenance.Check(context.Background(), pool)
	if err != nil {
		t.Fatal(err)
	}
	if !st.MaintenanceMode || st.Phase != maintenance.PhaseDraining {
		t.Fatalf("巡检维护状态不符: %+v", st)
	}
	if st.ActiveJobLeases != 1 {
		t.Fatalf("活动租约 = %d，期望 1", st.ActiveJobLeases)
	}
	if st.CreatingBackups != 1 {
		t.Fatalf("creating 备份 = %d，期望 1", st.CreatingBackups)
	}
	if st.LeaseExpired {
		t.Fatal("租约未过期")
	}
}
