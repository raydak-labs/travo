package services

import (
	"testing"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

func newTestStatsHistory(interval time.Duration) *StatsHistoryService {
	svc := NewSystemService(ubus.NewMockUbus(), uci.NewMockUCI(), &MockStorageProvider{})
	return NewStatsHistoryService(svc, interval, 10)
}

func TestStatsHistory_CollectsPoints(t *testing.T) {
	s := newTestStatsHistory(10 * time.Millisecond)
	s.Start()
	defer s.Stop()

	time.Sleep(50 * time.Millisecond)
	if len(s.GetHistory()) == 0 {
		t.Error("expected collected points after start")
	}
}

func TestStatsHistory_StopEndsCollection(t *testing.T) {
	s := newTestStatsHistory(10 * time.Millisecond)
	s.Start()
	time.Sleep(30 * time.Millisecond)
	s.Stop()
	// Allow an already-selected tick to finish before snapshotting.
	time.Sleep(20 * time.Millisecond)

	n := len(s.GetHistory())
	time.Sleep(50 * time.Millisecond)
	if got := len(s.GetHistory()); got != n {
		t.Errorf("expected no collection after Stop, points grew from %d to %d", n, got)
	}
}

func TestStatsHistory_StopIsIdempotent(t *testing.T) {
	s := newTestStatsHistory(time.Minute)
	s.Start()
	s.Stop()
	s.Stop() // must not panic
}
