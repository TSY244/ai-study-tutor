package lessonimport

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TSY244/ai-study-tutor/backend/internal/worker"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker/proc"
)

func TestAudioTranscribesBeforeTextIsReturned(t *testing.T) {
	for _, language := range []string{"", "zh-CN", "en"} {
		t.Run(language, func(t *testing.T) {
			dir := t.TempDir()
			fake := proc.NewFake()
			calls := 0
			fake.OnRun = func(ctx context.Context, cmd proc.Cmd) (*proc.Result, error) {
				calls++
				if calls == 1 {
					return nil, &proc.ExitError{Code: 1, StderrTail: "no subtitle stream"}
				}
				if calls == 2 {
					if cmd.Name != "ffmpeg" || !strings.Contains(strings.Join(cmd.Args, " "), "-ar 16000 -ac 1") {
						t.Fatalf("missing PCM conversion: %v", cmd)
					}
					return &proc.Result{}, nil
				}
				if cmd.Name != "whisper-cli" {
					t.Fatalf("expected local ASR: %v", cmd)
				}
				expected := language
				if expected == "" {
					expected = "auto"
				}
				if expected == "zh-CN" {
					expected = "zh"
				}
				if !strings.Contains(strings.Join(cmd.Args, " "), "-l "+expected) {
					t.Fatalf("wrong recognition language: %v", cmd.Args)
				}
				if err := os.WriteFile(filepath.Join(dir, "transcript.txt"), []byte("\n今天学习二分查找。\n二分查找要求输入有序。\n"), 0600); err != nil {
					t.Fatal(err)
				}
				return &proc.Result{}, nil
			}
			h := &Handler{Proc: fake, FFmpegBin: "ffmpeg", WhisperBin: "whisper-cli", WhisperModelPath: "/models/base.bin"}
			text, version, err := h.extract(context.Background(), "lecture.mp3", TypeAudio, "whisper_local", dir, language)
			if err != nil || !strings.Contains(text, "二分查找要求输入有序") || version != mediaVersion || calls != 3 {
				t.Fatalf("text=%q version=%s calls=%d err=%v", text, version, calls, err)
			}
		})
	}
}
func TestAudioRejectsEmptyTranscriptAndPreservesCancellation(t *testing.T) {
	for _, mode := range []string{"empty", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			fake := proc.NewFake()
			fake.OnRun = func(ctx context.Context, cmd proc.Cmd) (*proc.Result, error) {
				if mode == "cancelled" {
					return nil, context.Canceled
				}
				if err := os.WriteFile(filepath.Join(dir, "transcript.txt"), []byte(" \n"), 0600); err != nil {
					t.Fatal(err)
				}
				return &proc.Result{}, nil
			}
			h := &Handler{Proc: fake, WhisperBin: "whisper-cli", WhisperModelPath: "/models/base.bin"}
			_, err := h.runWhisper(context.Background(), "audio.wav", dir, "")
			var we *worker.Error
			if mode == "empty" && (!errors.As(err, &we) || we.Code != worker.ErrCodeEmptyContent) {
				t.Fatalf("must reject empty transcript: %v", err)
			}
			if mode == "cancelled" && !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation must survive: %v", err)
			}
		})
	}
}
