package cookiecheck

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/TSY244/ai-study-tutor/backend/internal/sitecookie"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker"
	"github.com/TSY244/ai-study-tutor/backend/internal/worker/proc"
)

func TestQRLoginWaitsForMobileConfirmation(t *testing.T) {
	sessions := sitecookie.NewLoginSessions(t.TempDir())
	fake := proc.NewFake()
	id := uuid.New()
	fake.Enqueue(func(_ context.Context, cmd proc.Cmd) (*proc.Result, error) {
		if cmd.Name != "bili" || len(cmd.Args) < 3 || cmd.Args[0] != "auth" || cmd.Args[1] != "login" {
			t.Fatal("expected bili auth login")
		}
		stat, err := os.Stat(sessions.Profile(id))
		if err != nil || stat.Mode().Perm() != 0o700 {
			t.Fatal("login profile must be private")
		}
		return &proc.Result{Stdout: []byte(`{"ok":true,"data":{"expires_in":180}}`)}, nil
	})
	fake.Enqueue(func(_ context.Context, _ proc.Cmd) (*proc.Result, error) {
		st, err := sessions.Read(id)
		if err != nil || st.State != "waiting_scan" {
			t.Fatal("QR must be available before waiting for mobile confirmation")
		}
		return &proc.Result{Stdout: []byte(`{"ok":false,"error":{"code":"login_pending","detail":{"scanned":true}}}`)}, &proc.ExitError{Code: 9}
	})
	fake.Enqueue(func(_ context.Context, _ proc.Cmd) (*proc.Result, error) {
		st, err := sessions.Read(id)
		if err != nil || st.State != "scanned" {
			t.Fatal("scanned must not be mistaken for logged in")
		}
		return &proc.Result{Stdout: []byte(`{"ok":true,"data":{"logged_in":true}}`)}, nil
	})
	h := LoginHandler{Sessions: sessions, Proc: fake}
	if err := h.login(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	st, err := sessions.Read(id)
	if err != nil || st.State != "confirmed" {
		t.Fatal("only confirmed logins may be imported by API")
	}
	if _, err := os.Stat(filepath.Join(sessions.Profile(id), "credential.json")); err == nil {
		t.Fatal("test must never create a real credential")
	}
}

func TestQRLoginFailureAndCancellationRemovePrivateState(t *testing.T) {
	for _, scenario := range []string{"expired", "cancelled", "missing_tool"} {
		t.Run(scenario, func(t *testing.T) {
			sessions := sitecookie.NewLoginSessions(t.TempDir())
			fake := proc.NewFake()
			if scenario == "missing_tool" {
				fake.Enqueue(proc.Static(nil, &proc.MissingToolError{Tool: "bili"}))
			} else {
				fake.Enqueue(proc.Static(&proc.Result{Stdout: []byte(`{"ok":true,"data":{"expires_in":180}}`)}, nil))
				if scenario == "cancelled" {
					fake.Enqueue(proc.BlockUntilDone())
				} else {
					fake.Enqueue(proc.Static(&proc.Result{Stdout: []byte(`{"ok":false,"error":{"code":"login_expired","message":"SECRET-COOKIE"}}`)}, &proc.ExitError{Code: 10, StderrTail: "SECRET-COOKIE"}))
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
			defer cancel()
			id := uuid.New()
			h := LoginHandler{Sessions: sessions, Proc: fake}
			err := h.login(ctx, id)
			if err == nil || strings.Contains(err.Error(), "SECRET-COOKIE") {
				t.Fatalf("unexpected unsafe error: %v", err)
			}
			if scenario == "cancelled" && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatal("cancellation must propagate")
			}
			if scenario == "expired" {
				var e *worker.Error
				if !errors.As(err, &e) || e.Code != ErrLoginExpired || e.Retryable {
					t.Fatal("expired human login must not auto-retry")
				}
			}
			if _, err := os.Stat(sessions.Dir(id)); !os.IsNotExist(err) {
				t.Fatal("failed login must remove temporary credentials")
			}
		})
	}
}
