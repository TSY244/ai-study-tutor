// Package proc 实现 Worker 外部子进程执行器（03 §8 / 05 §7.2）。
//
// 安全约束：
//   - 仅以参数数组调用（exec.CommandContext），永不经过 shell；
//   - context 取消/超时会杀掉子进程（取消语义见 03 §7.3）；
//   - stdout/stderr 采集有界，超出部分截断；非零退出时 Result 与 ExitError
//     一并返回（Result 携带有界采集输出），错误信息本身只携带 stderr 尾部
//     摘要，不携带完整输出（可能含学习材料片段）。
package proc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// 输出采集上限：stdout 用于解析器文本输出（pdftotext），上限给足；
// stderr 只用于诊断摘要。
const (
	maxStdoutBytes = 32 << 20
	maxStderrBytes = 1 << 20
	stderrTailLen  = 300
)

// Cmd 描述一次子进程调用。Dir 是工作目录（Job 隔离临时目录）。
// Env 非空时完整替换子进程环境（白名单放行，形如 KEY=VALUE）；
// 为空时继承父进程环境。
type Cmd struct {
	Name string
	Args []string
	Dir  string
	Env  []string
	// OnStderrLine receives complete newline-delimited records during execution.
	// Oversized records are discarded; bounded stderr capture remains unchanged.
	OnStderrLine func([]byte)
}

// Result 是有界采集的子进程输出。
type Result struct {
	Stdout []byte
	Stderr []byte
	// StdoutTruncated/StderrTruncated 标记输出达到采集上限被截断。
	StdoutTruncated bool
	StderrTruncated bool
}

// ExitError：子进程以非零状态退出。StderrTail 为有界尾部摘要。
// Run 返回 ExitError 时仍可能同时返回非 nil 的 *Result（有界采集的
// stdout/stderr，例如 CLI 在非零退出前写出的 JSON 信封）；错误本身只
// 携带 stderr 尾部摘要。调用方不得以 Result 非 nil 判定执行成功。
type ExitError struct {
	Code       int
	StderrTail string
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("子进程退出码 %d: %s", e.Code, e.StderrTail)
}

// MissingToolError：外部二进制不存在（未安装或未在 PATH）。
type MissingToolError struct {
	Tool string
}

func (e *MissingToolError) Error() string {
	return fmt.Sprintf("外部工具不可用: %s", e.Tool)
}

// Runner 是子进程执行面；测试注入 Fake。
type Runner interface {
	Run(ctx context.Context, cmd Cmd) (*Result, error)
}

// ExecRunner 是生产实现：参数数组、无 shell、取消即杀进程。
type ExecRunner struct{}

// boundedWriter 采集至多 max 字节，并记录是否发生截断。
type boundedWriter struct {
	buf       bytes.Buffer
	max       int
	truncated bool
}

func (w *boundedWriter) Write(p []byte) (int, error) {
	remain := w.max - w.buf.Len()
	if remain > 0 {
		if len(p) > remain {
			w.buf.Write(p[:remain])
			w.truncated = true
		} else {
			w.buf.Write(p)
		}
	} else {
		w.truncated = true
	}
	return len(p), nil
}

// Run 执行命令。取消/超时返回 ctx.Err()（exec.CommandContext 已杀掉子进程）。
// 非零退出返回 *ExitError，并同时返回携带有界采集输出的非 nil *Result。
func (ExecRunner) Run(ctx context.Context, cmd Cmd) (*Result, error) {
	if cmd.Name == "" {
		return nil, errors.New("proc: 命令名不能为空")
	}
	c := exec.CommandContext(ctx, cmd.Name, cmd.Args...)
	if cmd.Dir != "" {
		c.Dir = cmd.Dir
	}
	if cmd.Env != nil {
		c.Env = cmd.Env
	}
	stdout := &boundedWriter{max: maxStdoutBytes}
	stderr := &boundedWriter{max: maxStderrBytes}
	c.Stdout = stdout
	c.Stderr = stderr
	if cmd.OnStderrLine != nil {
		c.Stderr = io.MultiWriter(stderr, &lineWriter{onLine: cmd.OnStderrLine})
	}
	// 取消传播：CommandContext 在 ctx 结束时 Kill 子进程。
	if err := c.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// 非零退出时进程可能已把结构化结果（如 CLI JSON 信封）写到
			// stdout：Result 与 ExitError 一并返回，错误本身仍只携带
			// stderr 尾部摘要。
			return &Result{
				Stdout:          stdout.buf.Bytes(),
				Stderr:          stderr.buf.Bytes(),
				StdoutTruncated: stdout.truncated,
				StderrTruncated: stderr.truncated,
			}, &ExitError{Code: exitErr.ExitCode(), StderrTail: tailOf(stderr.buf.Bytes(), stderrTailLen)}
		}
		var pathErr *os.PathError
		if errors.As(err, &pathErr) || errors.Is(err, exec.ErrNotFound) || isNotFound(err) {
			return nil, &MissingToolError{Tool: cmd.Name}
		}
		return nil, fmt.Errorf("proc: 启动子进程失败: %w", err)
	}
	return &Result{
		Stdout:          stdout.buf.Bytes(),
		Stderr:          stderr.buf.Bytes(),
		StdoutTruncated: stdout.truncated,
		StderrTruncated: stderr.truncated,
	}, nil
}

func isNotFound(err error) bool {
	return errors.Is(err, syscall.ENOENT)
}

// tailOf 返回输出的最后 n 个字节（按 UTF-8 安全截断），用于错误摘要。
func tailOf(b []byte, n int) string {
	if len(b) <= n {
		return strings.TrimSpace(string(b))
	}
	s := string(b[len(b)-n:])
	// 截断点可能落在多字节字符中间，丢弃第一个不完整字符。
	if i := strings.IndexAny(s, " \n"); i >= 0 && i < 4 {
		s = s[i+1:]
	}
	return "…" + strings.TrimSpace(s)
}
