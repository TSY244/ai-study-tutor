// Package gate 实现 03 §6.7 的两把 PostgreSQL advisory lock：
//
//   - Business Gate：所有 API mutation、Job claim、结果发布事务先获取
//     transaction-scoped shared lock，再检查维护状态；维护切换与 quiesced
//     备份窗口使用 exclusive lock 排空写入。
//   - Maintenance Owner Lock：备份进程用 session-scoped exclusive lock
//     持有整个生命周期；连接断开或 SIGKILL 后由 PostgreSQL 自动释放。
//
// 锁键集中在 maintenance.go，避免与迁移锁（734000）冲突。
package gate

import (
	"context"
	"errors"
	"fmt"

	"github.com/TSY244/ai-study-tutor/internal/platform/database"
)

// Advisory lock 键空间（734000 = 迁移锁，见 platform/migrate）。
const (
	BusinessGateKey     int64 = 734001
	MaintenanceOwnerKey int64 = 734002
)

// ErrMaintenanceMode 表示系统处于维护模式，新业务写入被拒绝。
// HTTP 层映射为 503 maintenance_mode。
var ErrMaintenanceMode = errors.New("系统维护中，暂时拒绝写入")

// LockBusinessShared 在当前事务内获取 Business Gate shared lock。
func LockBusinessShared(ctx context.Context, tx database.Querier) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock_shared($1)`, BusinessGateKey)
	return err
}

// LockBusinessExclusive 在当前事务内获取 Business Gate exclusive lock。
// 用于维护阶段切换与 quiesced 备份窗口。
func LockBusinessExclusive(ctx context.Context, tx database.Querier) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, BusinessGateKey)
	return err
}

// AssertBusinessOpen 获取 Business Gate shared lock 并检查维护状态；
// 处于维护模式时返回 ErrMaintenanceMode。所有业务写入事务入口必须调用。
func AssertBusinessOpen(ctx context.Context, tx database.Querier) error {
	if err := LockBusinessShared(ctx, tx); err != nil {
		return fmt.Errorf("获取业务门禁锁失败: %w", err)
	}
	var maintenance bool
	if err := tx.QueryRow(ctx,
		`SELECT maintenance_mode FROM system_state WHERE singleton = true`).Scan(&maintenance); err != nil {
		return fmt.Errorf("读取系统维护状态失败: %w", err)
	}
	if maintenance {
		return ErrMaintenanceMode
	}
	return nil
}

// TryMaintenanceOwner 在专用连接上尝试获取 session-scoped Maintenance
// Owner Lock。返回 false 表示仍有活跃维护进程持有该锁。
// conn 必须是会话级连接（*pgx.Conn 或 *pgxpool.Conn），不能使用事务。
func TryMaintenanceOwner(ctx context.Context, conn database.Querier) (bool, error) {
	var acquired bool
	if err := conn.QueryRow(ctx,
		`SELECT pg_try_advisory_lock($1)`, MaintenanceOwnerKey).Scan(&acquired); err != nil {
		return false, fmt.Errorf("尝试维护 Owner 锁失败: %w", err)
	}
	return acquired, nil
}

// UnlockMaintenanceOwner 显式释放 Maintenance Owner Lock；连接关闭时
// PostgreSQL 也会自动释放。
func UnlockMaintenanceOwner(ctx context.Context, conn database.Querier) error {
	_, err := conn.Exec(ctx, `SELECT pg_advisory_unlock($1)`, MaintenanceOwnerKey)
	return err
}
