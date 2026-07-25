package claudegateway

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func syntheticSecret(label string) string {
	return strings.Join([]string{"sk", "ant", "test", label, "0123456789ABCDEF"}, "-")
}

func TestSafeKeyJSONMasksAndFingerprintsWithoutSecretFields(t *testing.T) {
	secret := syntheticSecret("safe-view")
	now := time.Date(2026, 7, 25, 9, 30, 0, 0, time.UTC)
	view := safeView(Key{
		ID: "ck_safe", Name: "primary", APIKey: secret, Model: "claude-test",
		Enabled: true, Priority: 10, Weight: 100, CreatedAt: now, UpdatedAt: now,
	}, KeyState{Status: StatusHealthy})

	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(`"api_key"`)) {
		t.Fatal("safe JSON contains the secret field name")
	}
	if bytes.Contains(raw, []byte(secret)) {
		t.Fatal("safe JSON contains the synthetic secret")
	}
	wantMask := secret[:7] + "..." + secret[len(secret)-4:]
	if view.MaskedKey != wantMask {
		t.Fatalf("masked key mismatch: got %q want %q", view.MaskedKey, wantMask)
	}
	sum := sha256.Sum256([]byte(secret))
	wantFingerprint := "sha256:" + hex.EncodeToString(sum[:])[:12]
	if view.Fingerprint != wantFingerprint {
		t.Fatalf("fingerprint mismatch: got %q want %q", view.Fingerprint, wantFingerprint)
	}
}

func TestStoreWritesKeysAtomicallyWithPrivatePermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "gateway")
	path := filepath.Join(dir, "keys.json")
	store := NewStore(path)
	created, err := store.Create(Key{
		Name: "primary", APIKey: syntheticSecret("atomic"), Model: "claude-test", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := before.Mode().Perm(); got != 0o600 {
		t.Fatalf("keys.json permissions = %o, want 600", got)
	}
	if info, err := os.Stat(dir); err != nil {
		t.Fatal(err)
	} else if got := info.Mode().Perm(); got != 0o700 {
		t.Fatalf("gateway directory permissions = %o, want 700", got)
	}

	name := "renamed"
	if _, err := store.Update(created.ID, KeyPatch{Name: &name}); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(before, after) {
		t.Fatal("keys.json update did not replace the destination file atomically")
	}
	if got := after.Mode().Perm(); got != 0o600 {
		t.Fatalf("replacement keys.json permissions = %o, want 600", got)
	}
	leftovers, err := filepath.Glob(filepath.Join(dir, ".claude-gateway-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(leftovers) != 0 {
		t.Fatalf("atomic write left %d temporary files", len(leftovers))
	}
}

func TestStoreReloadsAfterSameMetadataFileReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.json")
	firstSecret := syntheticSecret("reload-a")
	secondSecret := syntheticSecret("reload-b")
	first := configFile{Keys: []Key{{ID: "ck_reload", Name: "reload-a", APIKey: firstSecret}}}
	second := configFile{Keys: []Key{{ID: "ck_reload", Name: "reload-b", APIKey: secondSecret}}}
	firstRaw, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	secondRaw, err := json.Marshal(second)
	if err != nil {
		t.Fatal(err)
	}
	if len(firstRaw) != len(secondRaw) {
		t.Fatal("test fixture replacements must have identical sizes")
	}
	if err := writeFileAtomic(path, firstRaw); err != nil {
		t.Fatal(err)
	}
	store := NewStore(path)
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || loaded[0].APIKey != firstSecret {
		t.Fatal("store did not load the initial config")
	}
	oldInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".external-replacement-*.tmp")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmp.Name()
	t.Cleanup(func() { _ = os.Remove(tmpPath) })
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		t.Fatal(err)
	}
	if _, err := tmp.Write(secondRaw); err != nil {
		_ = tmp.Close()
		t.Fatal(err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(tmpPath, oldInfo.ModTime(), oldInfo.ModTime()); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		t.Fatal(err)
	}

	loaded, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || loaded[0].APIKey != secondSecret || loaded[0].Name != "reload-b" {
		t.Fatal("store returned stale data after keys.json was atomically replaced")
	}
}

func TestStoreUpdatePreservesOmittedSecretAndRejectsBlankReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.json")
	secret := syntheticSecret("patch")
	store := NewStore(path)
	created, err := store.Create(Key{
		Name: "before", APIKey: secret, Model: "claude-test", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	name := "after"
	updated, err := store.Update(created.ID, KeyPatch{Name: &name})
	if err != nil {
		t.Fatal(err)
	}
	if updated.APIKey != secret {
		t.Fatal("PATCH without api_key did not preserve the exact stored secret")
	}
	loaded, err := store.Get(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.APIKey != secret {
		t.Fatal("persisted secret changed after PATCH omitted api_key")
	}

	blank := " \t\n "
	_, err = store.Update(created.ID, KeyPatch{APIKey: &blank})
	if !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("blank replacement error = %v, want ErrInvalidKey", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatal("blank replacement error exposed the stored secret")
	}
	loaded, err = store.Get(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.APIKey != secret {
		t.Fatal("blank replacement mutated the persisted secret")
	}
}

func TestStoreWriteFailureDoesNotPublishUnpersistedMutation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Store, Key) error
		check  func(*testing.T, []Key, Key)
	}{
		{
			name: "create",
			mutate: func(store *Store, _ Key) error {
				_, err := store.Create(Key{
					Name: "unpersisted", APIKey: syntheticSecret("failed-create"), Model: "claude-test",
				})
				return err
			},
			check: func(t *testing.T, keys []Key, original Key) {
				t.Helper()
				if len(keys) != 1 || keys[0].ID != original.ID {
					t.Fatal("failed create became visible through the cache")
				}
			},
		},
		{
			name: "update",
			mutate: func(store *Store, original Key) error {
				name := "unpersisted"
				_, err := store.Update(original.ID, KeyPatch{Name: &name})
				return err
			},
			check: func(t *testing.T, keys []Key, original Key) {
				t.Helper()
				if len(keys) != 1 || keys[0].Name != original.Name {
					t.Fatal("failed update became visible through the cache")
				}
			},
		},
		{
			name: "delete",
			mutate: func(store *Store, original Key) error {
				return store.Delete(original.ID)
			},
			check: func(t *testing.T, keys []Key, original Key) {
				t.Helper()
				if len(keys) != 1 || keys[0].ID != original.ID {
					t.Fatal("failed delete became visible through the cache")
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := NewStore(filepath.Join(t.TempDir(), "keys.json"))
			original, err := store.Create(Key{
				Name: "persisted", APIKey: syntheticSecret("persisted"), Model: "claude-test", Enabled: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			store.writeFile = func(string, []byte) error { return errors.New("injected write failure") }
			if err := tc.mutate(store, original); err == nil {
				t.Fatal("expected injected write failure")
			}
			store.writeFile = nil
			keys, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			tc.check(t, keys, original)
		})
	}
}
