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
}

// NewRateLimiter creates a new RateLimiter.
// maxAttempts is the maximum number of failed attempts allowed within window.
func NewRateLimiter(maxAttempts int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		attempts:    make(map[string][]time.Time),
		maxAttempts: maxAttempts,
		window:      window,
	}
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
