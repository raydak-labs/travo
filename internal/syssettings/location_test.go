package syssettings

import (
	"testing"
	"time"
)

func TestLocationAppliesThePOSIXRuleWithDST(t *testing.T) {
	berlin, _ := POSIXTimezone("Europe/Berlin")
	loc, err := Location(berlin)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		at   time.Time
		name string
		off  int
	}{
		{time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC), "CET", 3600},
		{time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC), "CEST", 7200},
		// 2026-03-29 01:00 UTC switches to summer time, 2026-10-25 01:00 UTC back.
		{time.Date(2026, 3, 29, 0, 59, 59, 0, time.UTC), "CET", 3600},
		{time.Date(2026, 3, 29, 1, 0, 0, 0, time.UTC), "CEST", 7200},
		{time.Date(2026, 10, 25, 0, 59, 59, 0, time.UTC), "CEST", 7200},
		{time.Date(2026, 10, 25, 1, 0, 0, 0, time.UTC), "CET", 3600},
	} {
		if name, off := c.at.In(loc).Zone(); name != c.name || off != c.off {
			t.Errorf("%v: %s %d, want %s %d", c.at, name, off, c.name, c.off)
		}
	}
}

func TestLocationHalfHourAndWestern(t *testing.T) {
	for zone, want := range map[string]int{"Asia/Kolkata": 19800, "America/New_York": -5 * 3600} {
		tz, ok := POSIXTimezone(zone)
		if !ok {
			t.Fatalf("%s missing", zone)
		}
		loc, err := Location(tz)
		if err != nil {
			t.Fatal(err)
		}
		if _, off := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC).In(loc).Zone(); off != want {
			t.Errorf("%s: %d, want %d", zone, off, want)
		}
	}
}

func TestLocationUTCAndRejects(t *testing.T) {
	for _, tz := range []string{"", "UTC", "UTC0", utcPOSIX} {
		loc, err := Location(tz)
		if err != nil {
			t.Fatalf("%q: %v", tz, err)
		}
		if _, off := time.Now().In(loc).Zone(); off != 0 {
			t.Errorf("%q: offset %d", tz, off)
		}
	}
	for _, tz := range []string{"CET-1CEST,M3.5.0,M10.5.0/3\n", "1nonsense", "CET-1\nX"} {
		if _, err := Location(tz); err == nil {
			t.Errorf("%q accepted", tz)
		}
	}
}
