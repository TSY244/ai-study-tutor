package backup

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestVerifyBundleAcceptsChecksummedFiles(t *testing.T) {
	root := t.TempDir()
	name := "backup-20260722T090000Z"
	dir := filepath.Join(root, name)
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, DatabaseDumpName), []byte("database dump"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, FilesArchiveName), []byte("published objects"), 0o600); err != nil {
		t.Fatal(err)
	}

	want := Manifest{
		FormatVersion:   1,
		BackupID:        "4e87d572-c4c4-4ed0-8477-658f193615d0",
		AppVersion:      "test",
		DatabaseVersion: "17.4",
		CreatedAt:       time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC),
		FileCount:       2,
		FileBytes:       32,
		ActiveJobs:      0,
	}
	if _, err := WriteBundleManifest(dir, want); err != nil {
		t.Fatal(err)
	}

	got, gotDir, err := VerifyBundle(root, name)
	if err != nil {
		t.Fatal(err)
	}
	if gotDir != dir {
		t.Fatalf("bundle path = %q, want %q", gotDir, dir)
	}
	if got.BackupID != want.BackupID || got.FormatVersion != want.FormatVersion {
		t.Fatalf("manifest = %+v, want %+v", got, want)
	}
}

func TestVerifyBundleRejectsTamperedArchive(t *testing.T) {
	root := t.TempDir()
	name := "backup-20260722T090000Z"
	dir := filepath.Join(root, name)
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, DatabaseDumpName), []byte("database dump"), 0o600); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, FilesArchiveName)
	if err := os.WriteFile(archive, []byte("published objects"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteBundleManifest(dir, Manifest{
		FormatVersion: 1,
		BackupID:      "4e87d572-c4c4-4ed0-8477-658f193615d0",
		CreatedAt:     time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archive, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, _, err := VerifyBundle(root, name); err == nil {
		t.Fatal("tampered archive must be rejected")
	}
}

func TestVerifyBundleRejectsPathTraversalName(t *testing.T) {
	if _, _, err := VerifyBundle(t.TempDir(), "../outside"); err == nil {
		t.Fatal("path traversal bundle name must be rejected")
	}
}

func TestVerifyBundleRejectsSymlinkedDirectory(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	name := "backup-20260722T090000Z"
	if err := os.WriteFile(filepath.Join(outside, DatabaseDumpName), []byte("database dump"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, FilesArchiveName), []byte("published objects"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteBundleManifest(outside, Manifest{
		FormatVersion: 1,
		BackupID:      "4e87d572-c4c4-4ed0-8477-658f193615d0",
		CreatedAt:     time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, name)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := VerifyBundle(root, name); err == nil {
		t.Fatal("symlinked backup directory must be rejected")
	}
}
