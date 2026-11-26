package auth

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"
)

// seqReader yields a counter-filled stream so tokens are distinct and predictable.
type seqReader struct{ n byte }

func (r *seqReader) Read(p []byte) (int, error) {
	for i := range p {
		r.n++
		p[i] = r.n
	}
	return len(p), nil
}

type failReader struct{}

func (failReader) Read([]byte) (int, error) { return 0, errors.New("no entropy") }

func newStore() (*Store, time.Time) { return NewStore(&seqReader{}), time.Now() }

func TestLookupHonoursLifetimeOnTheMonotonicClock(t *testing.T) {
	s, now := newStore()
	tok, sess, err := s.Create(now)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sess.Expires.String(), "m=") {
		t.Fatal("expiry lost its monotonic reading; wall-clock jumps could revive sessions")
	}
	if _, ok := s.Lookup(tok, now.Add(Lifetime-time.Second)); !ok {
		t.Fatal("session should be valid just before expiry")
	}
	if _, ok := s.Lookup(tok, now.Add(Lifetime)); ok {
		t.Fatal("session valid at expiry")
	}
	if _, ok := s.Lookup(tok, now); ok {
		t.Fatal("expired session was dropped on lookup and must stay gone")
	}
}

func TestExpiresInIsRelativeSeconds(t *testing.T) {
	s, now := newStore()
	_, sess, _ := s.Create(now)
	if got := ExpiresIn(sess, now.Add(time.Hour)); got != 23*3600 {
		t.Fatalf("got %d", got)
	}
	if got := ExpiresIn(sess, now.Add(48*time.Hour)); got != 0 {
		t.Fatalf("past expiry must clamp to 0, got %d", got)
	}
}

func TestEpochBumpInvalidatesOlderSessions(t *testing.T) {
	s, now := newStore()
	s.SetEpoch(7)
	old, _, _ := s.Create(now)
	if e := s.BumpEpoch(); e != 8 {
		t.Fatalf("epoch %d", e)
	}
	if _, ok := s.Lookup(old, now); ok {
		t.Fatal("session from an older epoch still valid")
	}
	fresh, sess, _ := s.Create(now)
	if sess.Epoch != 8 {
		t.Fatalf("new session epoch %d", sess.Epoch)
	}
	if _, ok := s.Lookup(fresh, now); !ok {
		t.Fatal("new-epoch session rejected")
	}
}

func TestRestartWithSameEpochDoesNotRevive(t *testing.T) {
	s, now := newStore()
	tok, _, _ := s.Create(now)
	restarted := NewStore(&seqReader{})
	restarted.SetEpoch(s.Epoch())
	if _, ok := restarted.Lookup(tok, now); ok {
		t.Fatal("memory-only store must not know old tokens")
	}
}

func TestRawTokenIsNeverStored(t *testing.T) {
	s, now := newStore()
	tok, _, _ := s.Create(now)
	raw, err := base64.RawURLEncoding.DecodeString(tok)
	if err != nil || len(raw) != 32 {
		t.Fatalf("token should be 32 bytes base64url: %v %d", err, len(raw))
	}
	sum := sha256.Sum256([]byte(tok))
	s.mu.Lock()
	defer s.mu.Unlock()
	for k := range s.sessions {
		if k != sum {
			t.Fatal("unexpected key")
		}
		if bytes.Contains(k[:], []byte(tok)) || bytes.Contains(k[:], raw) {
			t.Fatal("raw token in map key")
		}
	}
}

func TestCSRFIsSeparateRandom(t *testing.T) {
	s, now := newStore()
	tok, sess, _ := s.Create(now)
	if sess.CSRF == "" || sess.CSRF == tok {
		t.Fatalf("csrf %q token %q", sess.CSRF, tok)
	}
}

func TestEntropyFailureIsAnError(t *testing.T) {
	s := NewStore(failReader{})
	tok, _, err := s.Create(time.Now())
	if err == nil || tok != "" {
		t.Fatalf("got token %q err %v", tok, err)
	}
	if s.Len() != 0 {
		t.Fatal("a session was stored without entropy")
	}
}

func TestRevokeRevokeAllAndSweep(t *testing.T) {
	s, now := newStore()
	a, _, _ := s.Create(now)
	b, _, _ := s.Create(now)
	s.Revoke(a)
	if _, ok := s.Lookup(a, now); ok {
		t.Fatal("revoked session valid")
	}
	if _, ok := s.Lookup(b, now); !ok {
		t.Fatal("other session lost")
	}
	s.Create(now.Add(-2 * Lifetime))
	s.Sweep(now)
	if s.Len() != 1 {
		t.Fatalf("sweep left %d", s.Len())
	}
	s.RevokeAll()
	if s.Len() != 0 {
		t.Fatal("RevokeAll left sessions")
	}
}

func TestRateLimiterBurstRefillAndSweep(t *testing.T) {
	r, now := NewRateLimiter(), time.Now()
	for i := 0; i < 5; i++ {
		if !r.Allow("1.2.3.4", now) {
			t.Fatalf("attempt %d denied inside burst", i)
		}
	}
	if r.Allow("1.2.3.4", now) {
		t.Fatal("6th attempt allowed")
	}
	if !r.Allow("5.6.7.8", now) {
		t.Fatal("limit must be per IP")
	}
	if r.Allow("1.2.3.4", now.Add(5*time.Second)) {
		t.Fatal("refill is 5/min; 5 s is not enough for a full token")
	}
	if !r.Allow("1.2.3.4", now.Add(13*time.Second)) {
		t.Fatal("one token should be back after 12 s")
	}
	r.Sweep(now.Add(11 * time.Minute))
	if r.Len() != 0 {
		t.Fatalf("idle entries left: %d", r.Len())
	}
}

func TestValidatePassword(t *testing.T) {
	cases := []struct {
		name, pw string
		ok       bool
	}{
		{"min", "12345678", true},
		{"short", "1234567", false},
		{"max", strings.Repeat("a", 128), true},
		{"too long", strings.Repeat("a", 129), false},
		{"nul", "abcdefg\x00h", false},
		{"newline", "abcdefg\nh", false},
		{"carriage return", "abcdefg\rh", false},
		{"length counts bytes", "äääää", true}, // 5 runes, 10 bytes
		{"spaces ok", "pass word!", true},
	}
	for _, c := range cases {
		err := ValidatePassword(c.pw)
		if (err == nil) != c.ok {
			t.Errorf("%s: err=%v want ok=%v", c.name, err, c.ok)
		}
		if err != nil && c.pw != "" && strings.Contains(err.Error(), c.pw) {
			t.Errorf("%s: error echoes the password", c.name)
		}
	}
}
