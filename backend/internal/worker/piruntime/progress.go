package piruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/TSY244/ai-study-tutor/backend/internal/worker/proc"
)

type progressKey struct{}
type progressCallback func(string)

// WithProgress forwards safe status messages while run is still executing.
// A failed stage update cancels the subprocess and preserves the original error
// (including lease loss) rather than disguising it as a provider failure.
func WithProgress(ctx context.Context, report func(context.Context, string) error, run func(context.Context) error) error {
	callCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var mu sync.Mutex
	var reportErr error
	callback := progressCallback(func(message string) {
		if callCtx.Err() != nil {
			return
		}
		reportCtx, stop := context.WithTimeout(callCtx, 3*time.Second)
		err := report(reportCtx, message)
		stop()
		if err != nil {
			mu.Lock()
			reportErr = err
			mu.Unlock()
			cancel()
		}
	})
	err := run(context.WithValue(callCtx, progressKey{}, callback))
	mu.Lock()
	defer mu.Unlock()
	if reportErr != nil {
		return reportErr
	}
	return err
}

// Run keeps stdout's single final envelope intact and consumes only a small,
// allowlisted stderr protocol. Arbitrary child output never becomes a Job event.
func Run(ctx context.Context, runner proc.Runner, cmd proc.Cmd) (*proc.Result, error) {
	if callback, ok := ctx.Value(progressKey{}).(progressCallback); ok {
		count := 0
		cmd.OnStderrLine = func(line []byte) {
			if count >= 1000 || ctx.Err() != nil {
				return
			}
			if message := progressMessage(line); message != "" {
				count++
				callback(message)
			}
		}
	}
	return runner.Run(ctx, cmd)
}

func progressMessage(line []byte) string {
	var event struct {
		Type       string `json:"type"`
		Phase      string `json:"phase"`
		Tool       string `json:"tool"`
		Characters int64  `json:"characters"`
		Turns      int    `json:"turns"`
	}
	if len(line) > 1024 || json.Unmarshal(line, &event) != nil || event.Type != "pi_progress" || event.Characters < 0 || event.Characters > 1_000_000_000 || event.Turns < 1 || event.Turns > 40 {
		return ""
	}
	switch event.Phase {
	case "started":
		return "Pi 流式会话已启动，正在等待模型输出"
	case "generating":
		return fmt.Sprintf("Pi 正在流式生成 · 第 %d 轮 · 已接收 %d 字符", event.Turns, event.Characters)
	case "tool_start", "tool_end":
		label := map[string]string{"read": "读取资料", "write": "写入结果", "bash": "执行处理工具"}[event.Tool]
		if label == "" {
			return ""
		}
		if event.Phase == "tool_start" {
			return "Pi 正在" + label
		}
		return "Pi 工具执行已结束：" + label
	}
	return ""
}
