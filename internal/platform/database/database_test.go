package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakePool struct{ err error }

func (f fakePool) Ping(context.Context) error { return f.err }
func (f fakePool) Close()                     {}

func TestPingCheckerPropagatesFailure(t *testing.T) {
	if err := (PingChecker{Pool: fakePool{err: errors.New("down")}}).Check(context.Background()); err == nil {
		t.Fatal("数据库不可用应返回错误")
	}
	if err := (PingChecker{Pool: fakePool{}}).Check(context.Background()); err != nil {
		t.Fatalf("数据库可用不应返回错误：%v", err)
	}
}

func TestPingCheckerHasTimeout(t *testing.T) {
	checker := PingChecker{Pool: fakePool{}, Timeout: time.Second}
	ctx := context.Background()
	if err := checker.Check(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestConnectRejectsInvalidURL(t *testing.T) {
	if _, err := Connect(context.Background(), "://not-a-dsn"); err == nil {
		t.Fatal("非法 DSN 应报错")
	}
}

func TestConnectDoesNotDialEagerly(t *testing.T) {
	// 不可达地址也必须能创建池（healthz=200 依赖惰性连接）。
	pool, err := Connect(context.Background(), "postgres://u:x@127.0.0.1:1/db")
	if err != nil {
		t.Fatalf("惰性建池不应失败：%v", err)
	}
	defer pool.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if err := pool.Ping(ctx); err == nil {
		t.Fatal("不可达数据库 Ping 应失败")
	}
}
