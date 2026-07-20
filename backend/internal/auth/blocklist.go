package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"sync"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/store"
)

// blocklistBucket is the store bucket for revoked tokens: sha256(token) hex →
// expiry unix seconds. Only hashes touch flash — raw tokens are bearer
// credentials.
const blocklistBucket = "blocklist"

// TokenBlocklist maintains a set of blocked (revoked) JWT tokens, keyed by
// token hash. With a store attached, revocations survive backend restarts
// (previously a restart resurrected logged-out tokens for up to 24h).
type TokenBlocklist struct {
	mu       sync.RWMutex
	tokens   map[string]time.Time // sha256(token) hex -> expiry
	db       *store.Store         // nil = in-memory only
	stopCh   chan struct{}
	stopOnce sync.Once
}

// NewTokenBlocklist creates an in-memory TokenBlocklist.
func NewTokenBlocklist() *TokenBlocklist {
	return &TokenBlocklist{
		tokens: make(map[string]time.Time),
		stopCh: make(chan struct{}),
	}
}

// NewTokenBlocklistWithStore creates a TokenBlocklist backed by a persistent
// store. Unexpired entries are loaded immediately; expired ones are pruned.
func NewTokenBlocklistWithStore(db *store.Store) *TokenBlocklist {
	b := NewTokenBlocklist()
	b.db = db
	now := time.Now()
	var stale []string
	_ = db.ForEach(blocklistBucket, func(k, v []byte) error {
		unix, err := strconv.ParseInt(string(v), 10, 64)
		if err != nil {
			stale = append(stale, string(k))
			return nil
		}
		expiry := time.Unix(unix, 0)
		if now.After(expiry) {
			stale = append(stale, string(k))
			return nil
		}
		b.tokens[string(k)] = expiry
		return nil
	})
	for _, k := range stale {
		_ = db.Delete(blocklistBucket, k)
	}
	return b
}

func hashToken(tokenString string) string {
	sum := sha256.Sum256([]byte(tokenString))
	return hex.EncodeToString(sum[:])
}

// Block adds a token to the blocklist with its expiry time.
func (b *TokenBlocklist) Block(tokenString string, expiry time.Time) {
	key := hashToken(tokenString)
	b.mu.Lock()
	b.tokens[key] = expiry
	b.mu.Unlock()
	if b.db != nil {
		// Logout is rare — a per-revocation flash write is acceptable.
		_ = b.db.Put(blocklistBucket, key, []byte(strconv.FormatInt(expiry.Unix(), 10)))
	}
}

// IsBlocked checks if a token is in the blocklist.
func (b *TokenBlocklist) IsBlocked(tokenString string) bool {
	key := hashToken(tokenString)
	b.mu.RLock()
	defer b.mu.RUnlock()
	_, ok := b.tokens[key]
	return ok
}

// Cleanup removes expired tokens from the blocklist and the store.
func (b *TokenBlocklist) Cleanup() {
	now := time.Now()
	var expired []string
	b.mu.Lock()
	for key, expiry := range b.tokens {
		if now.After(expiry) {
			delete(b.tokens, key)
			expired = append(expired, key)
		}
	}
	b.mu.Unlock()
	if b.db != nil {
		for _, key := range expired {
			_ = b.db.Delete(blocklistBucket, key)
		}
	}
}

// StartCleanup starts a goroutine that periodically cleans up expired tokens.
// Call Stop() to shut it down.
func (b *TokenBlocklist) StartCleanup(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				b.Cleanup()
			case <-b.stopCh:
				return
			}
		}
	}()
}

// Stop stops the cleanup goroutine. Safe to call multiple times.
func (b *TokenBlocklist) Stop() {
	b.stopOnce.Do(func() { close(b.stopCh) })
}
