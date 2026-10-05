package services

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/store"
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

func openStatsStore(t *testing.T, dir string) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(dir, "travo.db"))
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// Stats history must survive a backend restart (previously RAM-only: every
// reboot wiped the 6h dashboard history).
func TestStatsHistory_PersistsAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	db := openStatsStore(t, dir)
	svc := NewSystemService(ubus.NewMockUbus(), uci.NewMockUCI(), &MockStorageProvider{})

	s := NewStatsHistoryServiceWithStore(svc, time.Minute, 10, db)
	s.collect()
	s.collect()
	s.Flush()

	s2 := NewStatsHistoryServiceWithStore(svc, time.Minute, 10, db)
	if got := len(s2.GetHistory()); got != 2 {
		t.Errorf("expected 2 restored points, got %d", got)
	}
}

// Stop must flush so a graceful shutdown (incl. reboot via init) keeps history.
func TestStatsHistory_StopFlushes(t *testing.T) {
	dir := t.TempDir()
	db := openStatsStore(t, dir)
	svc := NewSystemService(ubus.NewMockUbus(), uci.NewMockUCI(), &MockStorageProvider{})

	s := NewStatsHistoryServiceWithStore(svc, time.Minute, 10, db)
	s.collect()
	s.Stop()

	s2 := NewStatsHistoryServiceWithStore(svc, time.Minute, 10, db)
	if got := len(s2.GetHistory()); got != 1 {
		t.Errorf("expected 1 restored point after Stop, got %d", got)
	}
}

// Restored history must respect the ring-buffer capacity.
func TestStatsHistory_RestoreRespectsMaxLen(t *testing.T) {
	dir := t.TempDir()
	db := openStatsStore(t, dir)
	svc := NewSystemService(ubus.NewMockUbus(), uci.NewMockUCI(), &MockStorageProvider{})

	s := NewStatsHistoryServiceWithStore(svc, time.Minute, 10, db)
	for range 8 {
		s.collect()
	}
	s.Flush()

	s2 := NewStatsHistoryServiceWithStore(svc, time.Minute, 3, db)
	if got := len(s2.GetHistory()); got != 3 {
		t.Errorf("expected restore capped at maxLen=3, got %d", got)
	}
}

// A failing flash write must be visible and must not silently retire the
// pending batch: the counter is only reset once the write landed, so the next
// batch tries again instead of the history being dropped forever.
func TestStatsHistory_FailedFlushKeepsPendingPoints(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	checker := newTestStatsHistory(time.Minute).checker
	s := NewStatsHistoryServiceWithStore(checker, time.Minute, 10, db)

	s.collect()
	if s.sinceFlush != 1 {
		t.Fatalf("precondition: expected one unflushed point, got %d", s.sinceFlush)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	s.Flush()

	if s.sinceFlush != 1 {
		t.Errorf("a failed flush must not reset the flush counter, got %d", s.sinceFlush)
	}
	if len(s.GetHistory()) != 1 {
		t.Errorf("a failed flush must not drop the in-memory points, got %d", len(s.GetHistory()))
	}
}
