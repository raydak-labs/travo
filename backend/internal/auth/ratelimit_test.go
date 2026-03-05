package auth

import (
	"testing"
	"time"
)

func TestRateLimiter_AllowsUnderLimit(t *testing.T) {
	rl := NewRateLimiter(5, time.Minute)
	for i := 0; i < 5; i++ {
		rl.Record("192.168.1.1")
	}
	// 5 records, but Allow checks BEFORE recording, so 5 records means blocked
	// Let's test with 4 records instead
	rl2 := NewRateLimiter(5, time.Minute)
	for i := 0; i < 4; i++ {
		rl2.Record("192.168.1.1")
	}
	if !rl2.Allow("192.168.1.1") {
		t.Error("expected to allow under limit (4 of 5)")
	}
}

func TestRateLimiter_BlocksOverLimit(t *testing.T) {
	rl := NewRateLimiter(5, time.Minute)
	for i := 0; i < 5; i++ {
		rl.Record("192.168.1.1")
	}
	if rl.Allow("192.168.1.1") {
		t.Error("expected to block over limit (5 of 5)")
	}
}

func TestRateLimiter_ResetClearsAttempts(t *testing.T) {
	rl := NewRateLimiter(5, time.Minute)
	for i := 0; i < 5; i++ {
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
	for i := 0; i < 5; i++ {
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
	rl := NewRateLimiter(5, 50*time.Millisecond)
	for i := 0; i < 5; i++ {
		rl.Record("192.168.1.1")
	}
	if rl.Allow("192.168.1.1") {
		t.Error("expected to block before window expires")
	}
	time.Sleep(60 * time.Millisecond)
	if !rl.Allow("192.168.1.1") {
		t.Error("expected to allow after window expires")
	}
}
