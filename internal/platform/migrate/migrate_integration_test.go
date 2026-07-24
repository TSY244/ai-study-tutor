package migrate_test

import (
	"context"
	"testing"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/jackc/pgx/v5/pgxpool"

	"fmt"
	"io"
	"net"
	"os"

	"github.com/TSY244/ai-study-tutor/internal/platform/migrate"
)

// migrate 测试需要"未迁移"的数据库，不复用 databasetest（它已执行迁移）。
func TestMain(m *testing.M) { os.Exit(m.Run()) }

func newRawPostgres(t *testing.T) *pgxpool.Pool {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := uint32(l.Addr().(*net.TCPAddr).Port)
	_ = l.Close()
	ep := embeddedpostgres.NewDatabase(
		embeddedpostgres.DefaultConfig().
			Version(embeddedpostgres.V17).
			Port(port).
			RuntimePath(t.TempDir()).
			Logger(io.Discard))
	if err := ep.Start(); err != nil {
		t.Fatalf("启动测试 PostgreSQL 失败: %v", err)
	}
	t.Cleanup(func() { _ = ep.Stop() })
	dsn := fmt.Sprintf("postgres://postgres:postgres@127.0.0.1:%d/postgres?sslmode=disable", port)
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// 空库迁移 -> status -> 幂等重跑 -> down -> 再 up。
func TestMigrateRoundTrip(t *testing.T) {
	pool := newRawPostgres(t)
	ctx := context.Background()

	applied, err := migrate.Up(ctx, pool)
	if err != nil {
		t.Fatalf("首次迁移失败: %v", err)
	}
	if len(applied) == 0 {
		t.Fatal("空库应应用全部迁移")
	}
	// 关键不变量抽查：表与部分唯一索引存在。
	for _, q := range []string{
		`SELECT 1 FROM courses LIMIT 0`,
		`SELECT 1 FROM jobs LIMIT 0`,
		`SELECT 1 FROM file_objects LIMIT 0`,
		`SELECT 1 FROM system_state WHERE singleton = true`,
	} {
		if _, err := pool.Exec(ctx, q); err != nil {
			t.Fatalf("迁移后对象缺失（%s）: %v", q, err)
		}
	}
	// 幂等重跑。
	applied, err = migrate.Up(ctx, pool)
	if err != nil || len(applied) != 0 {
		t.Fatalf("重复迁移应为空集: %v %v", applied, err)
	}
	// status。
	statuses, err := migrate.Current(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range statuses {
		if !s.Applied {
			t.Fatalf("迁移 %s 应为 applied", s.Version)
		}
	}
	// down 后表消失，可再次 up。
	if err := migrate.Down(ctx, pool); err != nil {
		t.Fatalf("回滚失败: %v", err)
	}
	if _, err := pool.Exec(ctx, `SELECT 1 FROM courses LIMIT 0`); err == nil {
		t.Fatal("回滚后 courses 应不存在")
	}
	if _, err := migrate.Up(ctx, pool); err != nil {
		t.Fatalf("回滚后重新迁移失败: %v", err)
	}
}

// 并发迁移互斥：两个并发 Up 不报错且只应用一次。
func TestMigrateConcurrent(t *testing.T) {
	pool := newRawPostgres(t)
	ctx := context.Background()
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			_, err := migrate.Up(ctx, pool)
			errs <- err
		}()
	}
	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("并发迁移失败: %v", err)
		}
	}
	statuses, _ := migrate.Current(ctx, pool)
	for _, s := range statuses {
		if !s.Applied {
			t.Fatalf("迁移 %s 应为 applied", s.Version)
		}
	}
}
