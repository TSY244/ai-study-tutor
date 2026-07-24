package backup_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TSY244/ai-study-tutor/internal/backup"
	"github.com/TSY244/ai-study-tutor/internal/platform/databasetest"
)

func TestCreatePublishesReadyBundleAndLeavesSystemOpen(t *testing.T) {
	pool := databasetest.New(t)
	dataDir := t.TempDir()
	object := filepath.Join(dataDir, "objects", "ab", "object-1")
	if err := os.MkdirAll(filepath.Dir(object), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(object, []byte("ready"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{}
	service := backup.Service{
		Pool:         pool,
		DataDir:      dataDir,
		BackupDir:    t.TempDir(),
		DatabaseURL:  "postgres://tutor:test@localhost:5432/tutor",
		AppVersion:   "test",
		Lease:        time.Minute,
		Heartbeat:    10 * time.Millisecond,
		DrainTimeout: time.Second,
		PollInterval: time.Millisecond,
		Runner:       runner,
		Now:          func() time.Time { return time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC) },
	}

	result, err := service.Create(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	manifest, _, err := backup.VerifyBundle(service.BackupDir, result.Name)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.BackupID != result.ID.String() || manifest.FileCount != 1 || manifest.FileBytes != 5 {
		t.Fatalf("manifest = %+v", manifest)
	}
	if got := strings.Join(runner.commands, ","); got != "pg_dump,tar" {
		t.Fatalf("commands = %q", got)
	}

	var status string
	var maintenance bool
	if err := pool.QueryRow(context.Background(), `SELECT status FROM backups WHERE id = $1`, result.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), `SELECT maintenance_mode FROM system_state WHERE singleton = true`).Scan(&maintenance); err != nil {
		t.Fatal(err)
	}
	if status != "ready" || maintenance {
		t.Fatalf("backup=%s maintenance=%v", status, maintenance)
	}
}

func TestCreateMarksFailureAndClearsMaintenanceWhenDumpFails(t *testing.T) {
	pool := databasetest.New(t)
	service := backup.Service{
		Pool:         pool,
		DataDir:      t.TempDir(),
		BackupDir:    t.TempDir(),
		DatabaseURL:  "postgres://tutor:test@localhost:5432/tutor",
		Lease:        time.Minute,
		DrainTimeout: time.Second,
		PollInterval: time.Millisecond,
		Runner:       &fakeRunner{err: errors.New("pg_dump failed")},
	}

	if _, err := service.Create(context.Background()); err == nil {
		t.Fatal("dump failure must be returned")
	}
	var status string
	var maintenance bool
	if err := pool.QueryRow(context.Background(), `SELECT status FROM backups ORDER BY created_at DESC LIMIT 1`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), `SELECT maintenance_mode FROM system_state WHERE singleton = true`).Scan(&maintenance); err != nil {
		t.Fatal(err)
	}
	if status != "failed" || maintenance {
		t.Fatalf("backup=%s maintenance=%v", status, maintenance)
	}
}

type fakeRunner struct {
	commands []string
	err      error
}

func (r *fakeRunner) Run(_ context.Context, name string, args []string, _ []string) error {
	r.commands = append(r.commands, name)
	if r.err != nil {
		return r.err
	}
	for _, arg := range args {
		if path, ok := strings.CutPrefix(arg, "--file="); ok {
			if err := os.WriteFile(path, []byte(name+" output"), 0o600); err != nil {
				return err
			}
		}
	}
	return nil
}
