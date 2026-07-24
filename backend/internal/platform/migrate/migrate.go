// Package migrate 提供版本化数据库迁移：嵌入式 SQL 文件、
// schema_migrations 版本表、迁移互斥锁。每个迁移在独立事务中执行，
// 失败即停，不留半迁移状态。
package migrate

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// migrationLockKey 防止多个进程并发执行迁移（独立于业务/维护锁键）。
const migrationLockKey int64 = 734000

// Migration 是一个已解析的迁移文件对。
type Migration struct {
	Version string // 例如 "0001"
	Name    string // 例如 "init"
	UpSQL   string
	DownSQL string
}

// Status 报告单个迁移的应用状态。
type Status struct {
	Version   string
	Name      string
	Applied   bool
	AppliedAt string
}

// Load 从嵌入式 FS 解析全部迁移，按版本升序返回。
func Load() ([]Migration, error) {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("读取迁移目录失败: %w", err)
	}
	ups := map[string]string{}
	downs := map[string]string{}
	for _, e := range entries {
		base := e.Name()
		key, isUp, ok := parseFilename(base)
		if !ok {
			return nil, fmt.Errorf("迁移文件名不合法: %s（期望 NNNN_name.up.sql / NNNN_name.down.sql）", base)
		}
		body, err := migrationsFS.ReadFile("migrations/" + base)
		if err != nil {
			return nil, fmt.Errorf("读取迁移失败: %w", err)
		}
		if isUp {
			ups[key] = string(body)
		} else {
			downs[key] = string(body)
		}
	}
	if len(ups) == 0 {
		return nil, errors.New("未发现任何迁移文件")
	}
	out := make([]Migration, 0, len(ups))
	for key, up := range ups {
		down, ok := downs[key]
		if !ok {
			return nil, fmt.Errorf("迁移 %s 缺少 down 文件", key)
		}
		version, name, _ := strings.Cut(key, "_")
		out = append(out, Migration{Version: version, Name: name, UpSQL: up, DownSQL: down})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

func parseFilename(base string) (key string, isUp bool, ok bool) {
	if strings.HasSuffix(base, ".up.sql") {
		return strings.TrimSuffix(base, ".up.sql"), true, len(base) > len(".up.sql")
	}
	if strings.HasSuffix(base, ".down.sql") {
		return strings.TrimSuffix(base, ".down.sql"), false, len(base) > len(".down.sql")
	}
	return "", false, false
}

func ensureVersionTable(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT PRIMARY KEY,
			name       TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`)
	return err
}

func appliedVersions(ctx context.Context, tx pgx.Tx) (map[string]bool, error) {
	rows, err := tx.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	applied := map[string]bool{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

// Up 应用全部未执行的迁移。返回本次应用的版本列表（空表示已是最新）。
// 通过 session 级 advisory lock 保证并发安全。
func Up(ctx context.Context, pool *pgxpool.Pool) ([]string, error) {
	migrations, err := Load()
	if err != nil {
		return nil, err
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取迁移连接失败: %w", err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, migrationLockKey); err != nil {
		return nil, fmt.Errorf("获取迁移锁失败: %w", err)
	}
	defer func() { _, _ = conn.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, migrationLockKey) }()

	var appliedNow []string
	for _, m := range migrations {
		tx, err := conn.Begin(ctx)
		if err != nil {
			return nil, err
		}
		if err := ensureVersionTable(ctx, tx); err != nil {
			_ = tx.Rollback(ctx)
			return nil, fmt.Errorf("创建版本表失败: %w", err)
		}
		applied, err := appliedVersions(ctx, tx)
		if err != nil {
			_ = tx.Rollback(ctx)
			return nil, err
		}
		if applied[m.Version] {
			if err := tx.Rollback(ctx); err != nil {
				return nil, err
			}
			continue
		}
		if _, err := tx.Exec(ctx, m.UpSQL); err != nil {
			_ = tx.Rollback(ctx)
			return appliedNow, fmt.Errorf("迁移 %s_%s 执行失败: %w", m.Version, m.Name, err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`, m.Version, m.Name); err != nil {
			_ = tx.Rollback(ctx)
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return appliedNow, fmt.Errorf("迁移 %s_%s 提交失败: %w", m.Version, m.Name, err)
		}
		appliedNow = append(appliedNow, m.Version)
	}
	return appliedNow, nil
}

// Down 回滚全部已应用迁移（逆序）。仅供测试和本机维护使用。
func Down(ctx context.Context, pool *pgxpool.Pool) error {
	migrations, err := Load()
	if err != nil {
		return err
	}
	for i := len(migrations) - 1; i >= 0; i-- {
		m := migrations[i]
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		if err := ensureVersionTable(ctx, tx); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		applied, err := appliedVersions(ctx, tx)
		if err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if !applied[m.Version] {
			if err := tx.Rollback(ctx); err != nil {
				return err
			}
			continue
		}
		if _, err := tx.Exec(ctx, m.DownSQL); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("回滚 %s_%s 失败: %w", m.Version, m.Name, err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM schema_migrations WHERE version = $1`, m.Version); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Current 返回每个迁移的应用状态。
func Current(ctx context.Context, pool *pgxpool.Pool) ([]Status, error) {
	migrations, err := Load()
	if err != nil {
		return nil, err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := ensureVersionTable(ctx, tx); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT version, applied_at::text FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	appliedAt := map[string]string{}
	for rows.Next() {
		var v, at string
		if err := rows.Scan(&v, &at); err != nil {
			rows.Close()
			return nil, err
		}
		appliedAt[v] = at
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]Status, 0, len(migrations))
	for _, m := range migrations {
		at, ok := appliedAt[m.Version]
		out = append(out, Status{Version: m.Version, Name: m.Name, Applied: ok, AppliedAt: at})
	}
	return out, nil
}
