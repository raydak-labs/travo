package auth

import (
	"sync"
	"time"
)

// TokenBlocklist maintains a set of blocked (revoked) JWT tokens.
type TokenBlocklist struct {
	mu     sync.RWMutex
	tokens map[string]time.Time // token -> expiry
}

// NewTokenBlocklist creates a new TokenBlocklist.
func NewTokenBlocklist() *TokenBlocklist {
	return &TokenBlocklist{
		tokens: make(map[string]time.Time),
	}
}

// Block adds a token to the blocklist with its expiry time.
func (b *TokenBlocklist) Block(tokenString string, expiry time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tokens[tokenString] = expiry
}

// IsBlocked checks if a token is in the blocklist.
func (b *TokenBlocklist) IsBlocked(tokenString string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	_, ok := b.tokens[tokenString]
	return ok
}

// Cleanup removes expired tokens from the blocklist.
func (b *TokenBlocklist) Cleanup() {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	for token, expiry := range b.tokens {
		if now.After(expiry) {
			delete(b.tokens, token)
		}
	}
}

// StartCleanup starts a goroutine that periodically cleans up expired tokens.
func (b *TokenBlocklist) StartCleanup(interval time.Duration, stop <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				b.Cleanup()
			case <-stop:
				return
			}
		}
	}()
}
