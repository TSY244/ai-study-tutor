// 本机诊断与维护修复命令（05 §10.2）。
//
//	doctor                            只读巡检：不获取锁、不改变状态
//	doctor --repair-stale-maintenance 修复过期维护状态（maintenance-repair
//	                                  一次性服务内部调用；需数据卷写权限）
//
// 退出码：0 正常/修复成功；1 发现异常或修复被拒绝；2 配置或数据库不可达。
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/internal/filestore"
	"github.com/TSY244/ai-study-tutor/internal/maintenance"
	"github.com/TSY244/ai-study-tutor/internal/platform/config"
	"github.com/TSY244/ai-study-tutor/internal/platform/database"
	"github.com/TSY244/ai-study-tutor/internal/platform/logging"
)

func main() {
	repair := len(os.Args) > 1 && os.Args[1] == "--repair-stale-maintenance"
	if len(os.Args) > 1 && !repair {
		fmt.Fprintln(os.Stderr, "用法：doctor [--repair-stale-maintenance]")
		os.Exit(2)
	}

	cfg, err := config.Load(config.ServiceDoctor)
	if err != nil {
		fmt.Fprintf(os.Stderr, "doctor 启动失败：%v\n", err)
		os.Exit(2)
	}
	logger := logging.New(cfg.Service, cfg.Version, cfg.LogLevel)

	pool, err := database.Connect(context.Background(), cfg.DB.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "doctor 数据库配置无效：%v\n", err)
		os.Exit(2)
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "doctor 数据库不可达")
		os.Exit(2)
	}

	if repair {
		runRepair(ctx, pool, cfg, logger)
		return
	}

	st, err := maintenance.Check(ctx, pool)
	if err != nil {
		fmt.Fprintf(os.Stderr, "doctor 巡检失败：%v\n", err)
		os.Exit(2)
	}
	fmt.Printf("maintenance_mode=%v phase=%s lease_expired=%v active_job_leases=%d expired_temporary=%d creating_backups=%d\n",
		st.MaintenanceMode, st.Phase, st.LeaseExpired,
		st.ActiveJobLeases, st.ExpiredTemporary, st.CreatingBackups)

	abnormal := st.LeaseExpired || st.ExpiredTemporary > 0 ||
		(st.MaintenanceMode && st.CreatingBackups == 0 && st.ActiveJobLeases == 0)
	if abnormal {
		fmt.Fprintln(os.Stderr, "发现异常：存在过期维护/临时状态。修复命令：doctor --repair-stale-maintenance")
		os.Exit(1)
	}
	fmt.Println("doctor 巡检正常")
}

func runRepair(ctx context.Context, pool *pgxpool.Pool, cfg config.Config, logger *slog.Logger) {
	files, err := filestore.New(pool, cfg.DataDir, 24*time.Hour)
	if err != nil {
		fmt.Fprintf(os.Stderr, "doctor 修复初始化失败：%v\n", err)
		os.Exit(2)
	}
	repairer := &maintenance.Repairer{Pool: pool, Files: files, Log: logger}
	report, err := repairer.Repair(ctx)
	if err != nil {
		switch {
		case errors.Is(err, maintenance.ErrNotInMaintenance):
			fmt.Println("系统不在维护模式，无需修复")
		case errors.Is(err, maintenance.ErrMaintenanceActive):
			fmt.Fprintln(os.Stderr, "修复被拒绝：仍有活跃维护进程持有 Owner Lock")
		case errors.Is(err, maintenance.ErrLeaseNotExpired):
			fmt.Fprintln(os.Stderr, "修复被拒绝：维护租约按数据库时间尚未过期")
		case errors.Is(err, maintenance.ErrActiveLeasesExist):
			fmt.Fprintln(os.Stderr, "修复被拒绝：存在尚未过期的 Job 租约")
		default:
			fmt.Fprintf(os.Stderr, "修复失败：%v\n", err)
		}
		os.Exit(1)
	}
	fmt.Printf("修复完成：jobs(requeued=%d failed=%d cancelled=%d) files(completed=%d quarantined=%d deleted=%d orphaned=%d) backups_failed=%d\n",
		report.JobsRequeued, report.JobsFailed, report.JobsCancelled,
		report.FilesCompleted, report.FilesQuarantined, report.FilesDeleted, report.FilesOrphaned,
		report.BackupsFailed)
}
