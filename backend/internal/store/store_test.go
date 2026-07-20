package store

import (
	"os"
	"path/filepath"
	"testing"
)

func openTestStore(t *testing.T, dir string) *Store {
	t.Helper()
	s, err := Open(filepath.Join(dir, "travo.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	return s
}

func TestStore_PutGetDelete(t *testing.T) {
	s := openTestStore(t, t.TempDir())
	defer s.Close()

	if err := s.Put("bucket", "k", []byte("v")); err != nil {
		t.Fatalf("Put failed: %v", err)
	}
	got, err := s.Get("bucket", "k")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if string(got) != "v" {
		t.Errorf("expected v, got %q", got)
	}

	if err := s.Delete("bucket", "k"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	got, err = s.Get("bucket", "k")
	if err != nil {
		t.Fatalf("Get after delete failed: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil after delete, got %q", got)
	}
}

func TestStore_GetMissingBucketAndKey(t *testing.T) {
	s := openTestStore(t, t.TempDir())
	defer s.Close()

	if got, err := s.Get("nope", "k"); err != nil || got != nil {
		t.Errorf("missing bucket: expected nil,nil got %q,%v", got, err)
	}
}

func TestStore_PersistsAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	s := openTestStore(t, dir)
	if err := s.Put("bucket", "k", []byte("v")); err != nil {
		t.Fatalf("Put failed: %v", err)
	}
	s.Close()

	s2 := openTestStore(t, dir)
	defer s2.Close()
	got, err := s2.Get("bucket", "k")
	if err != nil || string(got) != "v" {
		t.Errorf("expected persisted value v, got %q (err %v)", got, err)
	}
}

func TestStore_ForEach(t *testing.T) {
	s := openTestStore(t, t.TempDir())
	defer s.Close()

	_ = s.Put("b", "k1", []byte("v1"))
	_ = s.Put("b", "k2", []byte("v2"))

	seen := map[string]string{}
	err := s.ForEach("b", func(k, v []byte) error {
		seen[string(k)] = string(v)
		return nil
	})
	if err != nil {
		t.Fatalf("ForEach failed: %v", err)
	}
	if len(seen) != 2 || seen["k1"] != "v1" || seen["k2"] != "v2" {
		t.Errorf("unexpected contents: %v", seen)
	}

	// Missing bucket iterates nothing without error.
	if err := s.ForEach("missing", func(k, v []byte) error { return nil }); err != nil {
		t.Errorf("ForEach on missing bucket: %v", err)
	}
}

func TestStore_FilePermissions(t *testing.T) {
	dir := t.TempDir()
	s := openTestStore(t, dir)
	defer s.Close()

	info, err := os.Stat(filepath.Join(dir, "travo.db"))
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("expected 0600 permissions, got %v", info.Mode().Perm())
	}
}
