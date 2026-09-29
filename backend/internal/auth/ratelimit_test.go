package auth

import (
	"fmt"
	"testing"
	"time"
)

func TestRateLimiter_AllowsUnderLimit(t *testing.T) {
	rl := NewRateLimiter(5, time.Minute)
	for range 5 {
		rl.Record("192.168.1.1")
	}
	// 5 records, but Allow checks BEFORE recording, so 5 records means blocked
	// Let's test with 4 records instead
	rl2 := NewRateLimiter(5, time.Minute)
	for range 4 {
		rl2.Record("192.168.1.1")
	}
	if !rl2.Allow("192.168.1.1") {
		t.Error("expected to allow under limit (4 of 5)")
	}
}

func TestRateLimiter_BlocksOverLimit(t *testing.T) {
	rl := NewRateLimiter(5, time.Minute)
	for range 5 {
		rl.Record("192.168.1.1")
	}
	if rl.Allow("192.168.1.1") {
		t.Error("expected to block over limit (5 of 5)")
	}
}

func TestRateLimiter_ResetClearsAttempts(t *testing.T) {
	rl := NewRateLimiter(5, time.Minute)
	for range 5 {
		rl.Record("192.168.1.1")
	}
	if rl.Allow("192.168.1.1") {
		t.Error("expected to block before reset")
	}
	rl.Reset("192.168.1.1")
	if !rl.Allow("192.168.1.1") {
		t.Error("expected to allow after reset")
	}
}

func TestRateLimiter_DifferentIPs(t *testing.T) {
	rl := NewRateLimiter(5, time.Minute)
	for range 5 {
		rl.Record("192.168.1.1")
	}
	if rl.Allow("192.168.1.1") {
		t.Error("expected IP1 to be blocked")
	}
	if !rl.Allow("192.168.1.2") {
		t.Error("expected IP2 to be allowed (independent limits)")
	}
}

func TestRateLimiter_WindowExpiry(t *testing.T) {
	rl := NewRateLimiter(5, 20*time.Millisecond)
	for range 5 {
		rl.Record("192.168.1.1")
	}
	if rl.Allow("192.168.1.1") {
		t.Error("expected to block before window expires")
	}
	// Poll instead of sleeping a fixed margin: a 10ms over-sleep is plenty on a
	// quiet machine and far too tight on a loaded CI runner, where a timer can
	// be delayed by tens of milliseconds.
	if !waitFor(2*time.Second, func() bool { return rl.Allow("192.168.1.1") }) {
		t.Error("expected to allow after window expires")
	}
}

func TestRateLimiter_CleanupSweepsStaleIPs(t *testing.T) {
	rl := NewRateLimiter(5, 20*time.Millisecond)
	for i := range 100 {
		rl.Record(fmt.Sprintf("10.0.0.%d", i))
	}
	// Every entry must be older than the window before a sweep can drop it;
	// wait for that rather than assuming a fixed sleep elapsed.
	waitFor(2*time.Second, func() bool {
		rl.mu.Lock()
		defer rl.mu.Unlock()
		for _, stamps := range rl.attempts {
			for _, at := range stamps {
				if time.Since(at) < 20*time.Millisecond {
					return false
				}
			}
		}
		return true
	})
	rl.Cleanup()

	rl.mu.Lock()
	size := len(rl.attempts)
	rl.mu.Unlock()
	if size != 0 {
		t.Errorf("expected all stale IPs swept, %d remain", size)
	}
}

func TestRateLimiter_StartCleanupSweepsPeriodically(t *testing.T) {
	rl := NewRateLimiter(5, 20*time.Millisecond)
	rl.StartCleanup(20 * time.Millisecond)
	defer rl.Stop()

	rl.Record("10.0.0.1")
	// The sweeper runs on a background goroutine; under -race on a busy runner a
	// 60ms sleep is not a reliable expectation for it having fired.
	waitFor(5*time.Second, func() bool {
		rl.mu.Lock()
		defer rl.mu.Unlock()
		return len(rl.attempts) == 0
	})

	rl.mu.Lock()
	size := len(rl.attempts)
	rl.mu.Unlock()
	if size != 0 {
		t.Errorf("expected background sweep to clear stale IPs, %d remain", size)
	}
}

func TestRateLimiter_StopIsIdempotent(t *testing.T) {
	rl := NewRateLimiter(5, time.Minute)
	rl.StartCleanup(time.Minute)
	rl.Stop()
	rl.Stop() // must not panic
}

// waitFor polls cond until it holds or the timeout elapses. Used instead of a
// fixed sleep wherever a background goroutine or a short window is involved:
// a fixed sleep tuned on a quiet machine flakes on a loaded CI runner.
func waitFor(timeout time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(timeout)
	for {
		if cond() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(5 * time.Millisecond)
	}
}
