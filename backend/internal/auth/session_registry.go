package auth

import (
	"sync"
	"time"
)

// expiredRetention keeps expired sessions in the registry for a grace period so
// their TTL verdict keeps winning over the wall-clock exp fallback (e.g. when
// the clock was set backwards). After the grace period the entry is swept and
// the token falls back to exp validation, same as after a restart.
const expiredRetention = time.Hour

// SessionRegistry tracks active sessions by JWT ID (jti) using the process
// monotonic clock. time.Since on a stored time.Time uses the monotonic reading,
// so session validity is immune to wall-clock jumps (NTP, time-sync, timezone
// fixes). The registry is in-memory: after a backend restart tokens fall back
// to standard exp validation.
type SessionRegistry struct {
	mu       sync.Mutex
	sessions map[string]time.Time // jti -> issuedAt (carries monotonic reading)
	ttl      time.Duration
}

// NewSessionRegistry creates a registry whose sessions live for ttl.
func NewSessionRegistry(ttl time.Duration) *SessionRegistry {
	return &SessionRegistry{
		sessions: make(map[string]time.Time),
		ttl:      ttl,
	}
}

// TTL returns the configured session lifetime.
func (r *SessionRegistry) TTL() time.Duration {
	return r.ttl
}

// Register records a new session for the given jti, issued now.
func (r *SessionRegistry) Register(jti string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sweepLocked()
	r.sessions[jti] = time.Now()
}

// Remove deletes a session (logout).
func (r *SessionRegistry) Remove(jti string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, jti)
}

// RevokeAll drops every live session and returns the revoked jtis. A plain
// Remove would leave the tokens valid through the exp fallback (an unknown jti
// is judged by wall-clock exp), so callers block the returned jtis as well.
func (r *SessionRegistry) RevokeAll() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	revoked := make([]string, 0, len(r.sessions))
	for jti := range r.sessions {
		revoked = append(revoked, jti)
		delete(r.sessions, jti)
	}
	return revoked
}

// Status reports the remaining lifetime of a session. known is false when the
// jti was never registered (or has been swept); remaining <= 0 means the
// session is known and expired.
func (r *SessionRegistry) Status(jti string) (remaining time.Duration, known bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	issued, ok := r.sessions[jti]
	if !ok {
		return 0, false
	}
	return r.ttl - time.Since(issued), true
}

// sweepLocked drops sessions expired for longer than the retention grace.
// Called with the mutex held; bounded by the number of logins, so lazy
// sweeping on Register is enough — no background goroutine needed.
func (r *SessionRegistry) sweepLocked() {
	for jti, issued := range r.sessions {
		if time.Since(issued) > r.ttl+expiredRetention {
			delete(r.sessions, jti)
		}
	}
}
