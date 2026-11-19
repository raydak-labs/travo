package statshistory

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestRecordAddsDayTotalsAndUpList(t *testing.T) {
	h := NewHistory(time.Time{}, nil, nil, PersistenceOK)
	late := time.Date(2026, 11, 18, 23, 50, 0, 0, time.UTC)
	h.Record(obs(late, time.Hour, map[string]Counter{"wwan": {"phy0-sta0", 0, 0}}))
	h.Record(obs(late.Add(5*time.Minute), time.Hour+5*time.Minute, map[string]Counter{"wwan": {"phy0-sta0", 100, 10}, "wan": {"eth0", 1, 1}}))
	h.Record(obs(late.Add(15*time.Minute), time.Hour+15*time.Minute, map[string]Counter{"wwan": {"phy0-sta0", 300, 30}, "wan": {"eth0", 2, 3}}))
	days, _ := h.Days()
	want := []Day{
		{Date: "2026-11-18", Uplinks: []UplinkBytes{{Name: "wwan", RXBytes: 100, TXBytes: 10}}},
		{Date: "2026-11-19", Uplinks: []UplinkBytes{{Name: "wan", RXBytes: 1, TXBytes: 2}, {Name: "wwan", RXBytes: 200, TXBytes: 20}}},
	}
	if !slices.EqualFunc(days, want, func(a, b Day) bool { return a.Date == b.Date && slices.Equal(a.Uplinks, b.Uplinks) }) {
		t.Fatalf("days = %+v", days)
	}
	points, _ := h.Points(time.Time{})
	if !slices.Equal(points[0].Up, []string{"wwan"}) || !slices.Equal(points[2].Up, []string{"wan", "wwan"}) {
		t.Fatalf("up = %v %v", points[0].Up, points[2].Up)
	}
}

func TestDaysOutliveThePointRingAndPrune(t *testing.T) {
	h := NewHistory(time.Time{}, nil, []Day{{Date: "2026-08-01", Uplinks: []UplinkBytes{}}, {Date: "2026-11-01", Uplinks: []UplinkBytes{{Name: "wwan", RXBytes: 5}}}}, PersistenceOK)
	h.Record(obs(t0, time.Hour, nil))
	days, _ := h.Days()
	if len(days) != 1 || days[0].Date != "2026-11-01" {
		t.Fatalf("days = %+v (93 day retention from %s)", days, t0)
	}
}

func TestDaysPersistAndAreRebuiltFromOldFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stats-history.json")
	old := `{"version":1,"interval_seconds":600,"points":[{"at":1763460000,"uptime_seconds":1,"clients":null,"uplinks":[{"name":"wwan","rx_bytes":7,"tx_bytes":3}]}]}`
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	s := Open(path, time.Time{}, time.Now)
	days, p := s.Days()
	if p != PersistenceOK || len(days) != 1 || days[0].Uplinks[0].RXBytes != 7 {
		t.Fatalf("seeded days = %+v %v", days, p)
	}
	points, _ := s.Points(time.Time{})
	if points[0].Up != nil {
		t.Fatal("an old point's up list is unknown, not empty")
	}
	s.Record(obs(t0, time.Hour, nil))
	s.Flush()
	if err := s.Reset(); err != nil {
		t.Fatal(err)
	}
	if days, _ := s.Days(); len(days) != 0 {
		t.Fatal("reset clears the day totals")
	}
}
