package auth

import (
	"sync"
	"time"
)

const (
	rateBurst   = 5
	rateWindow  = time.Minute // rateBurst attempts per window, refilled continuously
	rateIdleTTL = 10 * time.Minute
)

type bucket struct {
	tokens float64
	last   time.Time
}

// RateLimiter is a per-IP token bucket for login and first-run.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
}

func NewRateLimiter() *RateLimiter { return &RateLimiter{buckets: map[string]*bucket{}} }

// Allow spends one token for ip; false means the caller is over the limit.
func (r *RateLimiter) Allow(ip string, now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.buckets[ip]
	if !ok {
		b = &bucket{tokens: rateBurst, last: now}
		r.buckets[ip] = b
	}
	if el := now.Sub(b.last); el > 0 {
		b.tokens = min(rateBurst, b.tokens+float64(el)/float64(rateWindow)*rateBurst)
		b.last = now
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// Sweep drops entries idle for more than rateIdleTTL.
func (r *RateLimiter) Sweep(now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for ip, b := range r.buckets {
		if now.Sub(b.last) > rateIdleTTL {
			delete(r.buckets, ip)
		}
	}
}

func (r *RateLimiter) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.buckets)
}
