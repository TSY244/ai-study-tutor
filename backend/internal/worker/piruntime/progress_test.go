package piruntime

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/TSY244/ai-study-tutor/backend/internal/worker/proc"
)

func TestProgressIsLiveAndSanitized(t *testing.T) {
	ctx := context.Background()
	var messages []string
	fake := proc.NewFake()
	fake.OnRun = func(ctx context.Context, cmd proc.Cmd) (*proc.Result, error) {
		if cmd.OnStderrLine == nil {
			t.Fatal("no live callback")
		}
		cmd.OnStderrLine([]byte(`{"type":"pi_progress","phase":"generating","characters":42,"turns":1,"message":"private-source-and-key"}`))
		cmd.OnStderrLine([]byte(`{"type":"pi_progress","phase":"tool_start","tool":"write","characters":42,"turns":1}`))
		if len(messages) != 2 {
			t.Fatal("progress did not arrive before Run returned")
		}
		return &proc.Result{Stdout: []byte(`{"type":"result"}`)}, nil
	}
	err := WithProgress(ctx, func(ctx context.Context, message string) error { messages = append(messages, message); return nil }, func(ctx context.Context) error {
		result, err := Run(ctx, fake, proc.Cmd{Name: "node"})
		if result == nil || string(result.Stdout) != `{"type":"result"}` {
			t.Fatal("final envelope changed")
		}
		return err
	})
	if err != nil || !strings.Contains(messages[0], "42") || messages[1] != "Pi 正在写入结果" || strings.Contains(strings.Join(messages, ""), "private") {
		t.Fatalf("unsafe progress: %q %v", messages, err)
	}
	for _, line := range []string{`garbage`, `{"type":"pi_progress","phase":"private-secret","turns":1}`, `{"type":"pi_progress","phase":"tool_start","tool":"private-secret","turns":1}`, `{"type":"pi_progress","phase":"generating","turns":-1}`, strings.Repeat("x", 1025)} {
		if progressMessage([]byte(line)) != "" {
			t.Fatalf("invalid progress accepted: %q", line)
		}
	}
}

func TestProgressFailureCancelsAndPreservesCause(t *testing.T) {
	want := errors.New("lease lost")
	fake := proc.NewFake()
	fake.OnRun = func(ctx context.Context, cmd proc.Cmd) (*proc.Result, error) {
		cmd.OnStderrLine([]byte(`{"type":"pi_progress","phase":"started","turns":1}`))
		if ctx.Err() != context.Canceled {
			t.Fatal("failed progress did not cancel child")
		}
		return nil, ctx.Err()
	}
	err := WithProgress(context.Background(), func(context.Context, string) error { return want }, func(ctx context.Context) error { _, err := Run(ctx, fake, proc.Cmd{Name: "node"}); return err })
	if !errors.Is(err, want) {
		t.Fatalf("progress error disguised: %v", err)
	}
}
