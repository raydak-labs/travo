package ws

import (
	"sync"
	"testing"
	"time"
)

func TestTicketStore_SingleUse(t *testing.T) {
	store := NewTicketStore(time.Minute)
	ticket, expiresIn, err := store.Issue("session-jwt")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if ticket == "" || expiresIn != 60 {
		t.Fatalf("ticket = %q, expiresIn = %d", ticket, expiresIn)
	}
	if got, ok := store.Consume(ticket); !ok || got != "session-jwt" {
		t.Fatalf("first Consume = %q, %v", got, ok)
	}
	if _, ok := store.Consume(ticket); ok {
		t.Fatal("a ticket must not authenticate two upgrades")
	}
}

func TestTicketStore_Expires(t *testing.T) {
	store := NewTicketStore(time.Minute)
	now := time.Now()
	store.now = func() time.Time { return now }
	ticket, _, err := store.Issue("session-jwt")
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Minute)
	if _, ok := store.Consume(ticket); ok {
		t.Fatal("expired ticket was accepted")
	}
}

func TestTicketStore_BoundsPendingTickets(t *testing.T) {
	store := NewTicketStore(time.Minute)
	for range maxPendingTickets {
		if _, _, err := store.Issue("session-jwt"); err != nil {
			t.Fatalf("Issue below cap: %v", err)
		}
	}
	if _, _, err := store.Issue("session-jwt"); err == nil {
		t.Fatal("ticket issuance above the cap must fail")
	}
}

func TestTicketStore_ConcurrentConsumeOnlyWinsOnce(t *testing.T) {
	store := NewTicketStore(time.Minute)
	ticket, _, err := store.Issue("session-jwt")
	if err != nil {
		t.Fatal(err)
	}
	const attempts = 16
	var wg sync.WaitGroup
	var mu sync.Mutex
	wins := 0
	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, ok := store.Consume(ticket); ok {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("successful consumes = %d, want exactly 1", wins)
	}
}
