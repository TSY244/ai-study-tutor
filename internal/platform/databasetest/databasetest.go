// Package databasetest 提供集成测试基础设施：在随机端口启动真实的
// 嵌入式 PostgreSQL（binaries 缓存于用户目录，首次运行需网络下载），
// 应用全部迁移。无需 Docker；CI 与本机行为一致。
//
// 测试数据库启动方式（B2 交接物）——每个测试包共享一个实例：
//
//	func TestMain(m *testing.M) { databasetest.Main(m) }
//
//	func TestXxx(t *testing.T) {
//	    pool := databasetest.New(t)   // 清空全部业务表，返回共享连接池
//	    ...
//	}
//
// 共享实例意味着同一包内测试不能 t.Parallel()。
package databasetest

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TSY244/ai-study-tutor/internal/platform/migrate"
)

var (
	shared struct {
		sync.Once
		pool *pgxpool.Pool
		err  error
	}
	stop func()
)

// Main 是测试包的 TestMain 入口：启动共享实例、运行测试、停止实例。
func Main(m *testing.M) {
	ensureStarted()
	code := m.Run()
	if stop != nil {
		stop()
	}
	os.Exit(code)
}

// New 清空全部业务表并返回共享连接池。system_state 单行与
// schema_migrations 版本表会保留/重建。
func New(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ensureStarted()
	if shared.err != nil {
		t.Fatalf("测试 PostgreSQL 启动失败: %v", shared.err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := shared.pool.Exec(ctx, `
		TRUNCATE courses, lessons, file_objects, materials, documents,
			questions, quizzes, quiz_items, attempts, attempt_answers,
			weaknesses, mastery_snapshots, summary_evaluations,
			jobs, job_events, ai_runs,
			idempotency_records, backups,
			legacy_import_batches, legacy_import_records CASCADE`); err != nil {
		t.Fatalf("清空测试表失败: %v", err)
	}
	if _, err := shared.pool.Exec(ctx, `
		UPDATE system_state
		SET maintenance_mode = false, phase = 'normal',
		    maintenance_reason = NULL, maintenance_owner = NULL,
		    maintenance_started_at = NULL, maintenance_lease_expires_at = NULL,
		    maintenance_heartbeat_at = NULL
		WHERE singleton = true`); err != nil {
		t.Fatalf("重置 system_state 失败: %v", err)
	}
	return shared.pool
}

func ensureStarted() {
	shared.Do(func() {
		port := freePort()
		runtimeDir, err := os.MkdirTemp("", "databasetest-runtime-*")
		if err != nil {
			shared.err = err
			return
		}
		ep := embeddedpostgres.NewDatabase(
			embeddedpostgres.DefaultConfig().
				Version(embeddedpostgres.V17).
				Port(port).
				RuntimePath(runtimeDir).
				Logger(io.Discard))
		if err := ep.Start(); err != nil {
			shared.err = fmt.Errorf("启动测试 PostgreSQL 失败: %w", err)
			return
		}
		dsn := fmt.Sprintf("postgres://postgres:postgres@127.0.0.1:%d/postgres?sslmode=disable", port)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			shared.err = fmt.Errorf("创建连接池失败: %w", err)
			_ = ep.Stop()
			return
		}
		if err := pool.Ping(ctx); err != nil {
			shared.err = fmt.Errorf("测试数据库不可达: %w", err)
			_ = ep.Stop()
			return
		}
		if _, err := migrate.Up(ctx, pool); err != nil {
			shared.err = fmt.Errorf("应用迁移失败: %w", err)
			_ = ep.Stop()
			return
		}
		shared.pool = pool
		stop = func() {
			pool.Close()
			_ = ep.Stop()
			_ = os.RemoveAll(runtimeDir)
		}
	})
}

// freePort 让内核分配一个空闲端口后释放，交给 PostgreSQL 使用。
// 存在微小的端口竞争窗口，对本机单用户测试可接受。
func freePort() uint32 {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 15433
	}
	defer func() { _ = l.Close() }()
	return uint32(l.Addr().(*net.TCPAddr).Port)
}
