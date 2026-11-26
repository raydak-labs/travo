// Package auth holds the pure session core: tokens, epochs, rate limits, password policy.
// Time and randomness come from the caller; nothing here touches the disk or the device.
package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"sync"
	"time"
)

// Lifetime is how long a session lives (invariants §3).
const Lifetime = 24 * time.Hour

// Clock is injected so tests control time. Production returns time.Now(), which carries
// the monotonic reading; expiry uses only Sub/After so wall-clock jumps cannot revive a session.
type Clock interface{ Now() time.Time }

// SystemClock is the production Clock.
type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now() }

// Session is the server-side record behind one cookie.
type Session struct {
	Epoch   uint64
	CSRF    string
	Expires time.Time
}

// Store keeps sessions in memory, keyed by sha256(token): a memory dump holds no usable cookie.
type Store struct {
	rng      io.Reader
	mu       sync.Mutex
	epoch    uint64
	sessions map[[sha256.Size]byte]Session
}

// NewStore takes the entropy source (crypto/rand.Reader in production).
func NewStore(rng io.Reader) *Store {
	return &Store{rng: rng, sessions: map[[sha256.Size]byte]Session{}}
}

func (s *Store) random() (string, error) {
	b := make([]byte, 32)
	if _, err := io.ReadFull(s.rng, b); err != nil {
		return "", fmt.Errorf("entropy: %w", err) // no fallback: a guessable token is worse than no login
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hash(token string) [sha256.Size]byte { return sha256.Sum256([]byte(token)) }

// Create issues a session valid for Lifetime. The raw token is returned once and never stored.
func (s *Store) Create(now time.Time) (string, Session, error) {
	token, err := s.random()
	if err != nil {
		return "", Session{}, err
	}
	csrf, err := s.random()
	if err != nil {
		return "", Session{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sess := Session{Epoch: s.epoch, CSRF: csrf, Expires: now.Add(Lifetime)}
	s.sessions[hash(token)] = sess
	return token, sess, nil
}

// Lookup returns the session when the token is known, unexpired and of the current epoch.
func (s *Store) Lookup(token string, now time.Time) (Session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := hash(token)
	sess, ok := s.sessions[key]
	if !ok {
		return Session{}, false
	}
	if sess.Epoch != s.epoch || !now.Before(sess.Expires) {
		delete(s.sessions, key)
		return Session{}, false
	}
	return sess, true
}

func (s *Store) Revoke(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, hash(token))
}

func (s *Store) RevokeAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	clear(s.sessions)
}

// Sweep drops expired and stale-epoch sessions so the map cannot grow without bound.
func (s *Store) Sweep(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, sess := range s.sessions {
		if sess.Epoch != s.epoch || !now.Before(sess.Expires) {
			delete(s.sessions, k)
		}
	}
}

// Len reports stored sessions (tests and diagnostics).
func (s *Store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sessions)
}

// Epoch is the current epoch; sessions of any other epoch are invalid.
func (s *Store) Epoch() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.epoch
}

// SetEpoch loads the persisted epoch at startup.
func (s *Store) SetEpoch(n uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.epoch = n
}

// BumpEpoch invalidates every existing session and returns the new epoch.
func (s *Store) BumpEpoch() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.epoch++
	return s.epoch
}

// ExpiresIn is the remaining lifetime in whole seconds; clients never see absolute times.
func ExpiresIn(s Session, now time.Time) int {
	d := s.Expires.Sub(now)
	if d <= 0 {
		return 0
	}
	return int(d / time.Second)
}
