// Package database owns the PostgreSQL connection pool (pgx/v5) and the
// readiness probe. SQL lives in module repositories (B2+); this package
// only provides the pool and connectivity checks.
package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool is the minimal pool surface used by readiness checks and future
// repositories. pgxpool.Pool satisfies it; tests use fakes.
type Pool interface {
	Ping(ctx context.Context) error
	Close()
}

// Checker is the readiness dependency of the HTTP layer. Keeping it narrow
// lets /readyz tests run without a real database.
type Checker interface {
	Check(ctx context.Context) error
}

// Querier 是 repository 使用的最小执行面：*pgxpool.Pool 与 pgx.Tx 都满足它，
// 使 repository 方法既能独立执行也能组合进调用方的事务边界。
type Querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Connect creates a connection pool. It does not dial eagerly: an
// unreachable database must not prevent process start, because /healthz
// must stay 200 and /readyz reports the dependency state instead.
func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, errors.New("数据库连接串无法解析（检查 DATABASE_URL/DB_* 配置）")
	}
	cfg.MaxConns = 4
	cfg.MinConns = 0
	cfg.MaxConnIdleTime = 5 * time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("数据库连接池创建失败")
	}
	return pool, nil
}

// PingChecker adapts a Pool to Checker with a bounded timeout.
type PingChecker struct {
	Pool    Pool
	Timeout time.Duration
}

func (c PingChecker) Check(ctx context.Context) error {
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.Pool.Ping(ctx)
}
