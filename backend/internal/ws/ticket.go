package ws

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

const (
	defaultTicketTTL  = 30 * time.Second
	maxPendingTickets = 1024
)

type ticketRecord struct {
	sessionToken string
	expiresAt    time.Time
}

// TicketStore issues bounded, short-lived, single-use WebSocket tickets.
// Tickets are hashed at rest; only the associated session JWT is retained for
// keepalive revalidation. No goroutine or wall-clock state crosses a restart.
type TicketStore struct {
	mu      sync.Mutex
	tickets map[[32]byte]ticketRecord
	ttl     time.Duration
	now     func() time.Time
}

func NewTicketStore(ttl time.Duration) *TicketStore {
	if ttl <= 0 {
		ttl = defaultTicketTTL
	}
	return &TicketStore{tickets: make(map[[32]byte]ticketRecord), ttl: ttl, now: time.Now}
}

func (s *TicketStore) Issue(sessionToken string) (string, int64, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", 0, fmt.Errorf("issue WebSocket ticket: %w", err)
	}
	ticket := hex.EncodeToString(random)
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.deleteExpired(now)
	if len(s.tickets) >= maxPendingTickets {
		return "", 0, fmt.Errorf("too many pending WebSocket tickets")
	}
	s.tickets[sha256.Sum256([]byte(ticket))] = ticketRecord{
		sessionToken: sessionToken,
		expiresAt:    now.Add(s.ttl),
	}
	return ticket, int64(s.ttl.Seconds()), nil
}

func (s *TicketStore) Consume(ticket string) (string, bool) {
	key := sha256.Sum256([]byte(ticket))
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.tickets[key]
	delete(s.tickets, key)
	if !ok || !s.now().Before(record.expiresAt) {
		return "", false
	}
	return record.sessionToken, true
}

func (s *TicketStore) deleteExpired(now time.Time) {
	for key, record := range s.tickets {
		if !now.Before(record.expiresAt) {
			delete(s.tickets, key)
		}
	}
}
