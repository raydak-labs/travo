package auth

import (
	"sync"
	"time"
)

// RateLimiter tracks login attempts per IP address.
type RateLimiter struct {
	mu          sync.Mutex
	attempts    map[string][]time.Time
	maxAttempts int
	window      time.Duration
	stopCh      chan struct{}
	stopOnce    sync.Once
}

// NewRateLimiter creates a new RateLimiter.
// maxAttempts is the maximum number of failed attempts allowed within window.
func NewRateLimiter(maxAttempts int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		attempts:    make(map[string][]time.Time),
		maxAttempts: maxAttempts,
		window:      window,
		stopCh:      make(chan struct{}),
	}
}

// Cleanup removes expired attempts for all IPs. Without this sweep the map
// grows unbounded when many distinct source IPs record attempts once and
// never return (per-IP cleanup only runs on that IP's next Allow call).
func (r *RateLimiter) Cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for ip := range r.attempts {
		r.cleanupIP(ip)
	}
}

// StartCleanup starts a goroutine that periodically sweeps stale attempts.
// Call Stop() to shut it down.
func (r *RateLimiter) StartCleanup(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				r.Cleanup()
			case <-r.stopCh:
				return
			}
		}
	}()
}

// Stop stops the cleanup goroutine. Safe to call multiple times.
func (r *RateLimiter) Stop() {
	r.stopOnce.Do(func() { close(r.stopCh) })
}

// Allow returns true if the IP is under the rate limit.
func (r *RateLimiter) Allow(ip string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cleanupIP(ip)
	return len(r.attempts[ip]) < r.maxAttempts
}

// Record records a failed login attempt for the IP.
func (r *RateLimiter) Record(ip string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.attempts[ip] = append(r.attempts[ip], time.Now())
}

// Reset clears all attempts for the IP (called on successful login).
func (r *RateLimiter) Reset(ip string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.attempts, ip)
}

// cleanupIP removes expired attempts for the given IP. Must be called with lock held.
func (r *RateLimiter) cleanupIP(ip string) {
	cutoff := time.Now().Add(-r.window)
	attempts := r.attempts[ip]
	valid := attempts[:0]
	for _, t := range attempts {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	if len(valid) == 0 {
		delete(r.attempts, ip)
	} else {
		r.attempts[ip] = valid
	}
}
