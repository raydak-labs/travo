package auth

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/store"
)

func openBlocklistStore(t *testing.T, dir string) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(dir, "travo.db"))
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// A blocked token must stay revoked after a backend restart — this was the
// main gap in the auth hardening plan (restart resurrected revoked tokens).
func TestBlocklist_PersistsAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	s := openBlocklistStore(t, dir)

	bl := NewTokenBlocklistWithStore(s)
	bl.Block("some-jwt-token", time.Now().Add(time.Hour))
	if !bl.IsBlocked("some-jwt-token") {
		t.Fatal("expected token blocked")
	}

	// Simulate restart: fresh blocklist over the same store.
	bl2 := NewTokenBlocklistWithStore(s)
	if !bl2.IsBlocked("some-jwt-token") {
		t.Error("expected token to stay blocked after restart")
	}
	if bl2.IsBlocked("other-token") {
		t.Error("expected unrelated token to be allowed")
	}
}

func TestBlocklist_ExpiredEntriesNotLoaded(t *testing.T) {
	dir := t.TempDir()
	s := openBlocklistStore(t, dir)

	bl := NewTokenBlocklistWithStore(s)
	bl.Block("stale-token", time.Now().Add(-time.Hour))

	bl2 := NewTokenBlocklistWithStore(s)
	if bl2.IsBlocked("stale-token") {
		t.Error("expected expired entry to be dropped on load")
	}
}

// Raw tokens are bearer credentials — only hashes may touch flash.
func TestBlocklist_StoresHashesNotRawTokens(t *testing.T) {
	dir := t.TempDir()
	s := openBlocklistStore(t, dir)

	bl := NewTokenBlocklistWithStore(s)
	bl.Block("super-secret-raw-token", time.Now().Add(time.Hour))

	err := s.ForEach("blocklist", func(k, v []byte) error {
		if strings.Contains(string(k), "super-secret-raw-token") {
			t.Error("raw token found as store key")
		}
		if strings.Contains(string(v), "super-secret-raw-token") {
			t.Error("raw token found as store value")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ForEach failed: %v", err)
	}
}

func TestBlocklist_CleanupPrunesStore(t *testing.T) {
	dir := t.TempDir()
	s := openBlocklistStore(t, dir)

	bl := NewTokenBlocklistWithStore(s)
	bl.Block("stale-token", time.Now().Add(-time.Hour))
	bl.Cleanup()

	entries := 0
	_ = s.ForEach("blocklist", func(k, v []byte) error {
		entries++
		return nil
	})
	if entries != 0 {
		t.Errorf("expected store pruned after Cleanup, %d entries remain", entries)
	}
}

func TestBlocklist_WorksWithoutStore(t *testing.T) {
	bl := NewTokenBlocklist()
	bl.Block("tok", time.Now().Add(time.Hour))
	if !bl.IsBlocked("tok") {
		t.Error("expected in-memory blocklist to work without a store")
	}
}
