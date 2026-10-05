package services

import (
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

func ifaceStats(iface string, rx, tx int64) []models.NetworkInterfaceStats {
	return []models.NetworkInterfaceStats{{Interface: iface, RxBytes: rx, TxBytes: tx}}
}

// The ring is capped by COUNT, so a stalled broadcast loop cannot grow memory:
// past the cap the oldest point is dropped, newest kept.
func TestTrafficHistory_DropsOldestPointPastCap(t *testing.T) {
	h := NewTrafficHistoryService(3)
	for i := range 5 {
		h.Append(ifaceStats("br-lan", int64(i), int64(100+i)))
	}

	pts := h.History()
	if len(pts) != 3 {
		t.Fatalf("expected 3 retained points, got %d", len(pts))
	}
	if pts[0].RxBytes != 2 || pts[2].RxBytes != 4 {
		t.Errorf("expected oldest point dropped (rx 2..4), got rx %d..%d", pts[0].RxBytes, pts[2].RxBytes)
	}
}

// Each interface gets its own budget; one chatty interface must not evict
// another interface's history.
func TestTrafficHistory_CapIsPerInterface(t *testing.T) {
	h := NewTrafficHistoryService(2)
	for i := range 4 {
		h.Append(ifaceStats("br-lan", int64(i), 0))
		h.Append(ifaceStats("eth0", int64(100+i), 0))
	}

	pts := h.History()
	if len(pts) != 4 {
		t.Fatalf("expected 2 points per interface (4 total), got %d", len(pts))
	}
	seen := map[string][]int64{}
	for _, p := range pts {
		seen[p.IfName] = append(seen[p.IfName], p.RxBytes)
	}
	if got := len(seen["br-lan"]); got != 2 {
		t.Errorf("expected 2 br-lan points, got %d", got)
	}
	if got := seen["eth0"][1]; got != 103 {
		t.Errorf("expected newest eth0 point kept (rx 103), got %d", got)
	}
}

// An empty buffer is valid right after boot and must serialise as [] rather
// than null so the dashboard can paint from it unconditionally.
func TestTrafficHistory_EmptyHistoryHasNoPoints(t *testing.T) {
	h := NewTrafficHistoryService(10)
	if got := len(h.History()); got != 0 {
		t.Errorf("expected empty history, got %d points", got)
	}
	if got := h.RetainedSeconds(); got != 0 {
		t.Errorf("expected 0 retained seconds before the first sample, got %d", got)
	}
}

// retained_seconds must report the span actually held, so a chart can show how
// much of its window the server could really fill.
func TestTrafficHistory_RetainedSeconds(t *testing.T) {
	h := NewTrafficHistoryService(10)
	now := int64(1000)
	for i := range 5 {
		h.appendAt(ifaceStats("br-lan", int64(i), 0), now+int64(i)*2)
	}

	if got := h.RetainedSeconds(); got != 8 {
		t.Errorf("expected 8 retained seconds across 5 points 2s apart, got %d", got)
	}
}
