package systemlog

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

func capture(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../../testdata/device/axt1800-25.12.6/logs/logread.txt")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestParseLineShapes(t *testing.T) {
	for _, tc := range []struct {
		line string
		want LogEntry
	}{
		{"Tue Nov 17 21:58:39 2026 daemon.notice wpa_supplicant[1866]: Successfully initialized wpa_supplicant",
			LogEntry{Time: "2026-11-17T21:58:39", Facility: "daemon", Level: "notice", Program: "wpa_supplicant", PID: 1866, Message: "Successfully initialized wpa_supplicant"}},
		{"Tue Nov 17 21:58:34 2026 user.notice : Added device handler type: vrf",
			LogEntry{Time: "2026-11-17T21:58:34", Facility: "user", Level: "notice", Message: "Added device handler type: vrf"}},
		{"Tue Nov 17 21:58:32 2026 kern.info kernel: [    0.000000] Machine model: GL.iNet GL-AXT1800",
			LogEntry{Time: "2026-11-17T21:58:32", Facility: "kern", Level: "info", Program: "kernel", Message: "[    0.000000] Machine model: GL.iNet GL-AXT1800"}},
		{"Sun Nov  1 09:00:00 2026 daemon.warn odhcpd[2006]: x",
			LogEntry{Time: "2026-11-01T09:00:00", Facility: "daemon", Level: "warn", Program: "odhcpd", PID: 2006, Message: "x"}},
		{"garbage line", LogEntry{Message: "garbage line"}},
	} {
		if got := Parse(tc.line); got != tc.want {
			t.Errorf("%q\n got %+v\nwant %+v", tc.line, got, tc.want)
		}
	}
}

func TestEveryCapturedLineParses(t *testing.T) {
	for _, line := range strings.Split(strings.TrimRight(capture(t), "\n"), "\n") {
		e := Parse(line)
		if e.Time == "" || e.Level == "" || e.Facility == "" {
			t.Errorf("did not parse: %q", line)
		}
	}
}

func TestSelectNewestFirstWithLimit(t *testing.T) {
	r := Select(capture(t), Query{Lines: 3})
	if len(r.Entries) != 3 || !r.Truncated {
		t.Fatalf("%d entries, truncated %v", len(r.Entries), r.Truncated)
	}
	if r.Entries[0].Program != "tailscaled" || r.Entries[0].Time < r.Entries[2].Time {
		t.Fatalf("not newest first: %+v", r.Entries)
	}
}

func TestSelectFilters(t *testing.T) {
	text := capture(t)
	r := Select(text, Query{Lines: MaxLines, MinLevel: "warn"})
	for _, e := range r.Entries {
		if e.Level != "warn" && e.Level != "err" && e.Level != "alert" && e.Level != "crit" && e.Level != "emerg" {
			t.Fatalf("level %q passed warn", e.Level)
		}
	}
	if len(r.Entries) == 0 || r.Truncated {
		t.Fatalf("%d entries truncated=%v", len(r.Entries), r.Truncated)
	}
	r = Select(text, Query{Contains: "DROPBEAR"})
	if len(r.Entries) != 6 {
		t.Fatalf("dropbear lines: %d", len(r.Entries))
	}
	if r := Select(text, Query{Contains: "no such thing"}); len(r.Entries) != 0 || r.Entries == nil {
		t.Fatalf("%+v", r)
	}
}

func TestSelectClampsAndCutsOversizedText(t *testing.T) {
	line := "Tue Nov 17 21:58:39 2026 daemon.info x[1]: " + strings.Repeat("a", 1000) + "\n"
	text := strings.Repeat(line, 2000) // ~2 MB
	r := Select(text, Query{Lines: 5000})
	if len(r.Entries) != MaxLines || !r.Truncated {
		t.Fatalf("%d %v", len(r.Entries), r.Truncated)
	}
	if r := Select(text, Query{}); len(r.Entries) != DefaultLines {
		t.Fatalf("default %d", len(r.Entries))
	}
}

type reader struct {
	text string
	err  error
}

func (r reader) ReadSystemLog(context.Context) (string, error) { return r.text, r.err }

func TestServiceReturnsReadError(t *testing.T) {
	s := &Service{Log: reader{err: errors.New("timeout")}}
	if _, err := s.Query(context.Background(), Query{}); err == nil {
		t.Fatal("want error")
	}
}
