package auth

import (
	"fmt"
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

func TestBlocklist_Concurrent(t *testing.T) {
	bl := NewTokenBlocklist()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
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
