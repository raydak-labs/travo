package datausage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/statshistory"
)

func day(d string, u ...statshistory.UplinkBytes) statshistory.Day {
	return statshistory.Day{Date: d, Uplinks: u}
}

func bytes(name string, rx, tx uint64) statshistory.UplinkBytes {
	return statshistory.UplinkBytes{Name: name, RXBytes: rx, TXBytes: tx}
}

func TestBillingPeriod(t *testing.T) {
	cases := []struct {
		now        string
		start      int
		from, till string
	}{
		{"2026-11-19T10:00:00Z", 1, "2026-11-01", "2026-12-01"},
		{"2026-11-19T10:00:00Z", 19, "2026-11-19", "2026-12-19"},
		{"2026-11-19T10:00:00Z", 20, "2026-10-20", "2026-11-20"},
		{"2026-01-05T00:00:00Z", 28, "2025-12-28", "2026-01-28"},
		{"2026-12-31T23:59:59Z", 15, "2026-12-15", "2027-01-15"},
	}
	for _, c := range cases {
		now, _ := time.Parse(time.RFC3339, c.now)
		s, e := BillingPeriod(now, c.start)
		if s.Format(time.DateOnly) != c.from || e.Format(time.DateOnly) != c.till {
			t.Errorf("%s day %d: %s..%s", c.now, c.start, s, e)
		}
	}
}

func TestSumUsedAndEvaluate(t *testing.T) {
	days := []statshistory.Day{
		day("2026-10-31", bytes("wwan", 1000, 1000)),
		day("2026-11-01", bytes("wwan", 300, 100), bytes("usbtether", 50, 50)),
		day("2026-11-19", bytes("wwan", 200, 0)),
	}
	start, end := BillingPeriod(time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC), 1)
	p := Sum(days, start, end)
	if !slices.Equal(p.Uplinks, []statshistory.UplinkBytes{bytes("usbtether", 50, 50), bytes("wwan", 500, 100)}) {
		t.Fatalf("period = %+v", p)
	}
	if got := Used(p, []string{"wwan"}); got != 600 {
		t.Fatalf("used = %d", got)
	}
	b := Budget{Enabled: true, LimitBytes: 1000, WarnPercent: 80}
	for used, want := range map[uint64]State{0: StateOK, 799: StateOK, 800: StateWarning, 1000: StateExceeded, 5000: StateExceeded} {
		if got := Evaluate(b, used); got != want {
			t.Errorf("used %d: %s, want %s", used, got, want)
		}
	}
	if Evaluate(Budget{LimitBytes: 1}, 99) != StateOff {
		t.Fatal("a disabled budget is off")
	}
}

func TestBudgetValidate(t *testing.T) {
	ok := Budget{Enabled: true, LimitBytes: MinLimitBytes, WarnPercent: 80, StartDay: 1, Uplinks: []string{"wwan"}}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := DefaultBudget().Validate(); err != nil {
		t.Fatal(err)
	}
	for field, mut := range map[string]func(*Budget){
		"start_day":    func(b *Budget) { b.StartDay = 29 },
		"warn_percent": func(b *Budget) { b.WarnPercent = 0 },
		"limit_bytes":  func(b *Budget) { b.LimitBytes = 10 },
		"uplinks":      func(b *Budget) { b.Uplinks = []string{"wwan", "wwan"} },
	} {
		b := ok
		b.Uplinks = slices.Clone(ok.Uplinks)
		mut(&b)
		var fe *FieldError
		if err := b.Validate(); !errors.As(err, &fe) || fe.Field != field {
			t.Errorf("%s: %v", field, err)
		}
	}
}

func up(names ...string) []string { return append([]string{}, names...) }

func TestDeriveUptime(t *testing.T) {
	points := []statshistory.Point{
		{At: 1000, UptimeSeconds: 5000, Up: nil}, // recorded before the up list existed
		{At: 1600, UptimeSeconds: 5600, Up: up("wwan")},
		{At: 2200, UptimeSeconds: 6200, Up: up()},
		{At: 2800, UptimeSeconds: 300, Up: up("wan", "wwan")},
		{At: 3400, UptimeSeconds: 900, Up: up("wan", "wwan")},
	}
	log := DeriveUptime(points)
	want := []Event{
		{At: 2200, Kind: EventDown, Uplink: "wwan"},
		{At: 2500, Kind: EventBoot},
		{At: 2800, Kind: EventUp, Uplink: "wan"},
		{At: 2800, Kind: EventUp, Uplink: "wwan"},
	}
	if !slices.Equal(log.Events, want) {
		t.Fatalf("events = %+v", log.Events)
	}
	wantAvail := []Availability{{Name: "wan", UpSamples: 2, Samples: 4}, {Name: "wwan", UpSamples: 3, Samples: 4}}
	if !slices.Equal(log.Availability, wantAvail) {
		t.Fatalf("availability = %+v", log.Availability)
	}
}

type fakeHistory struct{ days []statshistory.Day }

func (f fakeHistory) Points(time.Time) ([]statshistory.Point, statshistory.Persistence) {
	return []statshistory.Point{}, statshistory.PersistenceOK
}

func (f fakeHistory) Days() ([]statshistory.Day, statshistory.Persistence) {
	return f.days, statshistory.PersistenceOK
}

func TestServiceBudgetFileFailsClosed(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "data-budget.json")
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	s := &Service{History: fakeHistory{days: []statshistory.Day{day("2026-11-19", bytes("wwan", 900<<20, 100<<20))}}, Path: path, Now: func() time.Time { return now }}
	v, err := s.View(ctx)
	if err != nil || v.BudgetState != StateOff || v.Today.Uplinks[0].RXBytes != 900<<20 || v.BudgetUsedBytes != 1000<<20 {
		t.Fatalf("absent file: %+v %v", v, err)
	}
	v, err = s.SetBudget(ctx, Budget{Enabled: true, LimitBytes: 1 << 30, WarnPercent: 90, StartDay: 1, Uplinks: []string{"wwan"}})
	if err != nil || v.BudgetState != StateWarning {
		t.Fatalf("set: %+v %v", v.BudgetState, err)
	}
	if st, _ := s.BudgetState(ctx); st != StateWarning {
		t.Fatalf("alert source = %s", st)
	}
	torn := []byte(`{"enabled":tr`)
	if err := os.WriteFile(path, torn, 0o600); err != nil {
		t.Fatal(err)
	}
	if v, _ := s.View(ctx); v.BudgetState != StateUnreadable {
		t.Fatalf("torn file reads as %s", v.BudgetState)
	}
	if _, err := s.SetBudget(ctx, DefaultBudget()); !errors.Is(err, ErrUnreadable) {
		t.Fatalf("set over a torn file: %v", err)
	}
	if b, _ := os.ReadFile(path); string(b) != string(torn) {
		t.Fatal("the torn file was overwritten")
	}
	if v, err := s.ResetBudget(ctx); err != nil || v.BudgetState != StateOff {
		t.Fatalf("reset: %+v %v", v.BudgetState, err)
	}
}
