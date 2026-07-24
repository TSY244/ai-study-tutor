package proc

import (
	"context"
	"fmt"
	"sync"
)

// Fake 是可编排的 Runner 桩：按 FIFO 队列回放响应，队列耗尽后回落到
// OnRun。所有调用按顺序记录（Calls）供断言。响应函数可以阻塞等待
// ctx.Done（模拟超时/取消），也可以在 cmd.Dir 内写文件（模拟 yt-dlp
// 落盘产物）。
type Fake struct {
	mu    sync.Mutex
	calls []Cmd
	queue []RespondFunc
	OnRun RespondFunc
}

// RespondFunc 生成一次调用的响应。
type RespondFunc func(ctx context.Context, cmd Cmd) (*Result, error)

// NewFake 创建空的 Fake；未编排的调用返回错误。
func NewFake() *Fake { return &Fake{} }

// Enqueue 追加一组 FIFO 响应。
func (f *Fake) Enqueue(fns ...RespondFunc) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queue = append(f.queue, fns...)
}

// Calls 返回按顺序记录的全部调用（副本）。
func (f *Fake) Calls() []Cmd {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Cmd, len(f.calls))
	copy(out, f.calls)
	return out
}

// Static 生成固定响应。
func Static(res *Result, err error) RespondFunc {
	return func(context.Context, Cmd) (*Result, error) { return res, err }
}

// BlockUntilDone 阻塞直到 ctx 取消（模拟挂起的子进程）。
func BlockUntilDone() RespondFunc {
	return func(ctx context.Context, _ Cmd) (*Result, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
}

// Run 实现 Runner。
func (f *Fake) Run(ctx context.Context, cmd Cmd) (*Result, error) {
	f.mu.Lock()
	f.calls = append(f.calls, cmd)
	var fn RespondFunc
	if len(f.queue) > 0 {
		fn = f.queue[0]
		f.queue = f.queue[1:]
	} else {
		fn = f.OnRun
	}
	f.mu.Unlock()
	if fn == nil {
		return nil, fmt.Errorf("proc.Fake: 未编排的调用 %s %v", cmd.Name, cmd.Args)
	}
	return fn(ctx, cmd)
}
