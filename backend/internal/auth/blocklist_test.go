package auth

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestBlocklist_BlockAndCheck(t *testing.T) {
	bl := NewTokenBlocklist()
	bl.Block("token123", time.Now().Add(1*time.Hour))
	if !bl.IsBlocked("token123") {
		t.Error("expected token to be blocked")
	}
}

func TestBlocklist_UnblockedToken(t *testing.T) {
	bl := NewTokenBlocklist()
	if bl.IsBlocked("nonexistent") {
		t.Error("expected unblocked token to return false")
	}
}

func TestBlocklist_Cleanup(t *testing.T) {
	bl := NewTokenBlocklist()
	bl.Block("expired", time.Now().Add(-1*time.Hour))
	bl.Block("valid", time.Now().Add(1*time.Hour))
	bl.Cleanup()
	if bl.IsBlocked("expired") {
		t.Error("expected expired token to be cleaned up")
	}
	if !bl.IsBlocked("valid") {
		t.Error("expected valid token to still be blocked")
	}
}

func TestBlocklist_JTIBlocking(t *testing.T) {
	bl := NewTokenBlocklist()
	bl.BlockJTI("jti-1", time.Now().Add(1*time.Hour))
	if !bl.IsJTIBlocked("jti-1") {
		t.Error("expected jti to be blocked")
	}
	if bl.IsJTIBlocked("jti-2") {
		t.Error("expected unrelated jti to be allowed")
	}
	// A jti entry must not block a raw token string, and vice versa.
	if bl.IsBlocked("jti-1") {
		t.Error("jti entry must not match a token lookup")
	}
	bl.Block("raw-token", time.Now().Add(1*time.Hour))
	if bl.IsJTIBlocked("raw-token") {
		t.Error("token entry must not match a jti lookup")
	}
}

func TestBlocklist_JTIBlockedPersistsAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	s := openBlocklistStore(t, dir)

	bl := NewTokenBlocklistWithStore(s)
	bl.BlockJTI("jti-persist", time.Now().Add(time.Hour))

	bl2 := NewTokenBlocklistWithStore(s)
	if !bl2.IsJTIBlocked("jti-persist") {
		t.Error("expected blocked jti to survive a restart")
	}
}

func TestBlocklist_JTICleanupPrunesExpired(t *testing.T) {
	bl := NewTokenBlocklist()
	bl.BlockJTI("stale-jti", time.Now().Add(-time.Hour))
	bl.BlockJTI("live-jti", time.Now().Add(time.Hour))
	bl.Cleanup()
	if bl.IsJTIBlocked("stale-jti") {
		t.Error("expected expired jti entry to be cleaned up")
	}
	if !bl.IsJTIBlocked("live-jti") {
		t.Error("expected unexpired jti entry to survive cleanup")
	}
}

// Raw tokens are bearer credentials — only hashes may touch flash. The same
// holds for jtis, which key the store alongside token hashes.
func TestBlocklist_JTIStoresHashesNotRawJTI(t *testing.T) {
	dir := t.TempDir()
	s := openBlocklistStore(t, dir)

	bl := NewTokenBlocklistWithStore(s)
	bl.BlockJTI("raw-jti-value", time.Now().Add(time.Hour))

	err := s.ForEach(blocklistBucket, func(k, v []byte) error {
		if strings.Contains(string(k), "raw-jti-value") {
			t.Error("raw jti found as store key")
		}
		if strings.Contains(string(v), "raw-jti-value") {
			t.Error("raw jti found as store value")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ForEach failed: %v", err)
	}
}

func TestBlocklist_Concurrent(t *testing.T) {
	bl := NewTokenBlocklist()
	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(2)
		token := fmt.Sprintf("token%d", i)
		go func() {
			defer wg.Done()
			bl.Block(token, time.Now().Add(1*time.Hour))
		}()
		go func() {
			defer wg.Done()
			bl.IsBlocked(token)
		}()
	}
	wg.Wait()
}
