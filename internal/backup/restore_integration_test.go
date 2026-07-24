package backup_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/TSY244/ai-study-tutor/internal/backup"
	"github.com/TSY244/ai-study-tutor/internal/platform/databasetest"
)

func TestRestoreRequiresVerifiedBundleAndPublishesStagedObjects(t *testing.T) {
	pool := databasetest.New(t)
	backupID := uuid.New()
	backupDir, name := restoreFixture(t, backupID)
	dataDir := t.TempDir()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO backups (id, status, display_name) VALUES ($1, 'creating', $2)`, backupID, name); err != nil {
		t.Fatal(err)
	}
	runner := &restoreRunner{}
	service := backup.RestoreService{
		Pool:        pool,
		DataDir:     dataDir,
		BackupDir:   backupDir,
		DatabaseURL: "postgres://tutor:test@localhost:5432/tutor",
		Confirmed:   true,
		Runner:      runner,
	}

	if err := service.Restore(context.Background(), name); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(runner.commands, ","); got != "tar,pg_restore" {
		t.Fatalf("commands = %q", got)
	}
	if !containsArg(runner.pgRestoreArgs, "--dbname=tutor") {
		t.Fatalf("pg_restore must target the configured database, args = %q", runner.pgRestoreArgs)
	}
	if !containsArg(runner.pgRestoreArgs, "--single-transaction") {
		t.Fatalf("pg_restore must be atomic, args = %q", runner.pgRestoreArgs)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "objects", "aa", "restored")); err != nil {
		t.Fatalf("published object missing: %v", err)
	}
	var status string
	var maintenance bool
	if err := pool.QueryRow(context.Background(), `SELECT status FROM backups WHERE id = $1`, backupID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), `SELECT maintenance_mode FROM system_state WHERE singleton = true`).Scan(&maintenance); err != nil {
		t.Fatal(err)
	}
	if status != "ready" || maintenance {
		t.Fatalf("backup=%s maintenance=%v", status, maintenance)
	}
}

func TestRestoreRejectsNonEmptyDataVolume(t *testing.T) {
	pool := databasetest.New(t)
	backupDir, name := restoreFixture(t, uuid.New())
	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, "do-not-overwrite"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	service := backup.RestoreService{
		Pool:        pool,
		DataDir:     dataDir,
		BackupDir:   backupDir,
		DatabaseURL: "postgres://tutor:test@localhost:5432/tutor",
		Confirmed:   true,
		Runner:      &restoreRunner{},
	}
	if err := service.Restore(context.Background(), name); err == nil {
		t.Fatal("restore to non-empty data volume must be rejected")
	}
}

func restoreFixture(t *testing.T, backupID uuid.UUID) (string, string) {
	t.Helper()
	root := t.TempDir()
	name := "backup-20260722T090000Z"
	dir := filepath.Join(root, name)
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, backup.DatabaseDumpName), []byte("database dump"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, backup.FilesArchiveName), []byte("files archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := backup.WriteBundleManifest(dir, backup.Manifest{
		FormatVersion: 1,
		BackupID:      backupID.String(),
		CreatedAt:     time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	return root, name
}

type restoreRunner struct {
	commands      []string
	pgRestoreArgs []string
}

func (r *restoreRunner) Run(_ context.Context, name string, args []string, _ []string) error {
	r.commands = append(r.commands, name)
	if name == "pg_restore" {
		r.pgRestoreArgs = append([]string(nil), args...)
	}
	if name != "tar" {
		return nil
	}
	for _, arg := range args {
		if dir, ok := strings.CutPrefix(arg, "--directory="); ok {
			object := filepath.Join(dir, "objects", "aa", "restored")
			if err := os.MkdirAll(filepath.Dir(object), 0o700); err != nil {
				return err
			}
			return os.WriteFile(object, []byte("restored"), 0o600)
		}
	}
	return nil
}

func containsArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}
