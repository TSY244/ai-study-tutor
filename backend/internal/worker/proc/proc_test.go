package proc

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestExecRunnerEcho(t *testing.T) {
	res, err := ExecRunner{}.Run(context.Background(), Cmd{Name: "/bin/echo", Args: []string{"你好"}})
	if err != nil {
		t.Fatal(err)
	}
	if string(res.Stdout) != "你好\n" {
		t.Fatalf("stdout 错误: %q", res.Stdout)
	}
}

func TestExecRunnerNonZeroExit(t *testing.T) {
	_, err := ExecRunner{}.Run(context.Background(), Cmd{Name: "/bin/sh", Args: []string{"-c", "echo boom >&2; exit 3"}})
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("期望 ExitError，得到 %v", err)
	}
	if exitErr.Code != 3 || exitErr.StderrTail == "" {
		t.Fatalf("退出码/摘要错误: %+v", exitErr)
	}
}

func TestExecRunnerTimeoutKillsProcess(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := ExecRunner{}.Run(ctx, Cmd{Name: "/bin/sleep", Args: []string{"10"}})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("期望 DeadlineExceeded，得到 %v", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("子进程未被及时终止")
	}
}

func TestExecRunnerMissingTool(t *testing.T) {
	_, err := ExecRunner{}.Run(context.Background(), Cmd{Name: "definitely-not-exists-bin-xyz"})
	var missing *MissingToolError
	if !errors.As(err, &missing) {
		t.Fatalf("期望 MissingToolError，得到 %v", err)
	}
}

func TestFakeQueueAndCalls(t *testing.T) {
	f := NewFake()
	f.Enqueue(
		Static(&Result{Stdout: []byte("1")}, nil),
		func(ctx context.Context, cmd Cmd) (*Result, error) {
			return &Result{Stdout: []byte(cmd.Args[0])}, nil
		},
	)
	ctx := context.Background()
	if res, err := f.Run(ctx, Cmd{Name: "a"}); err != nil || string(res.Stdout) != "1" {
		t.Fatalf("第一个响应错误: %v %v", res, err)
	}
	if res, err := f.Run(ctx, Cmd{Name: "b", Args: []string{"x"}}); err != nil || string(res.Stdout) != "x" {
		t.Fatalf("第二个响应错误: %v %v", res, err)
	}
	if _, err := f.Run(ctx, Cmd{Name: "c"}); err == nil {
		t.Fatal("队列耗尽后未编排调用应报错")
	}
	calls := f.Calls()
	if len(calls) != 3 || calls[1].Name != "b" {
		t.Fatalf("调用记录错误: %+v", calls)
	}
}

func TestFakeBlockUntilDone(t *testing.T) {
	f := NewFake()
	f.OnRun = BlockUntilDone()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := f.Run(ctx, Cmd{Name: "slow"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("期望 ctx 错误，得到 %v", err)
	}
}
