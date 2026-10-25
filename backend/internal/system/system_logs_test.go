package system

import (
	"testing"
)

func TestParseLogOutput_Normal(t *testing.T) {
	input := "line one\nline two\nline three"
	result := parseLogOutput("syslog", input, "", "")

	if result.Source != "syslog" {
		t.Errorf("expected source 'syslog', got %q", result.Source)
	}
	if result.Total != 3 {
		t.Errorf("expected 3 lines, got %d", result.Total)
	}
	if len(result.Lines) != 3 {
		t.Fatalf("expected 3 line entries, got %d", len(result.Lines))
	}
	if result.Lines[0].Line != "line one" {
		t.Errorf("expected 'line one', got %q", result.Lines[0].Line)
	}
	if result.Lines[2].Line != "line three" {
		t.Errorf("expected 'line three', got %q", result.Lines[2].Line)
	}
}

func TestParseLogOutput_Empty(t *testing.T) {
	result := parseLogOutput("kernel", "", "", "")

	if result.Source != "kernel" {
		t.Errorf("expected source 'kernel', got %q", result.Source)
	}
	if result.Total != 0 {
		t.Errorf("expected 0 lines, got %d", result.Total)
	}
	if len(result.Lines) != 0 {
		t.Errorf("expected empty lines slice, got %d entries", len(result.Lines))
	}
}

func TestParseLogOutput_BlankLines(t *testing.T) {
	input := "first\n\nsecond\n\n\nthird\n"
	result := parseLogOutput("syslog", input, "", "")

	if result.Total != 3 {
		t.Errorf("expected 3 non-blank lines, got %d", result.Total)
	}
	if len(result.Lines) != 3 {
		t.Fatalf("expected 3 line entries, got %d", len(result.Lines))
	}
	if result.Lines[1].Line != "second" {
		t.Errorf("expected 'second', got %q", result.Lines[1].Line)
	}
}

func TestParseLogOutput_ServiceFilter(t *testing.T) {
	input := `Tue Mar 11 09:17:52 2026 daemon.info dnsmasq[1234]: query from 192.168.8.100
Tue Mar 11 09:17:53 2026 daemon.info AdGuardHome[3732]: blocked ad.example.com
Tue Mar 11 09:17:54 2026 daemon.info dnsmasq[1234]: forwarded google.com
Tue Mar 11 09:17:55 2026 kern.info netifd[456]: interface up`

	// Filter by dnsmasq
	result := parseLogOutput("syslog", input, "dnsmasq", "")
	if result.Total != 2 {
		t.Errorf("expected 2 dnsmasq lines, got %d", result.Total)
	}

	// Filter by AdGuardHome (case-insensitive)
	result = parseLogOutput("syslog", input, "adguardhome", "")
	if result.Total != 1 {
		t.Errorf("expected 1 AdGuardHome line, got %d", result.Total)
	}

	// No filter returns all
	result = parseLogOutput("syslog", input, "", "")
	if result.Total != 4 {
		t.Errorf("expected 4 lines with no filter, got %d", result.Total)
	}

	// Non-matching filter returns none
	result = parseLogOutput("syslog", input, "wireguard", "")
	if result.Total != 0 {
		t.Errorf("expected 0 lines for wireguard filter, got %d", result.Total)
	}
}

func TestExtractLevel(t *testing.T) {
	tests := []struct {
		line     string
		expected string
	}{
		{"Tue Mar 11 09:17:52 2026 daemon.info dnsmasq[1234]: query", "info"},
		{"Tue Mar 11 09:17:52 2026 kern.err kernel: error occurred", "err"},
		{"Tue Mar 11 09:17:52 2026 daemon.warning dnsmasq[1234]: warn", "warning"},
		{"Tue Mar 11 09:17:52 2026 kern.crit kernel: critical", "crit"},
		{"Tue Mar 11 09:17:52 2026 user.notice netifd: up", "notice"},
		{"Tue Mar 11 09:17:52 2026 daemon.debug dnsmasq: debug", "debug"},
		{"Tue Mar 11 09:17:52 2026 auth.emerg sshd: emergency", "emerg"},
		{"Tue Mar 11 09:17:52 2026 kern.alert kernel: alert", "alert"},
		{"short line", ""},
		{"", ""},
		{"no facility field at all", ""},
	}
	for _, tt := range tests {
		got := extractLevel(tt.line)
		if got != tt.expected {
			t.Errorf("extractLevel(%q) = %q, want %q", tt.line, got, tt.expected)
		}
	}
}

func TestParseLogOutput_LevelFilter(t *testing.T) {
	input := `Tue Mar 11 09:17:50 2026 daemon.debug dnsmasq[1234]: debug msg
Tue Mar 11 09:17:51 2026 daemon.info dnsmasq[1234]: info msg
Tue Mar 11 09:17:52 2026 daemon.notice dnsmasq[1234]: notice msg
Tue Mar 11 09:17:53 2026 daemon.warning dnsmasq[1234]: warning msg
Tue Mar 11 09:17:54 2026 daemon.err dnsmasq[1234]: error msg
Tue Mar 11 09:17:55 2026 daemon.crit dnsmasq[1234]: critical msg
Tue Mar 11 09:17:56 2026 daemon.alert dnsmasq[1234]: alert msg
Tue Mar 11 09:17:57 2026 daemon.emerg dnsmasq[1234]: emergency msg`

	// No level filter returns all 8
	result := parseLogOutput("syslog", input, "", "")
	if result.Total != 8 {
		t.Errorf("expected 8 lines, got %d", result.Total)
	}

	// Filter: err and above (emerg, alert, crit, err) = 4
	result = parseLogOutput("syslog", input, "", "err")
	if result.Total != 4 {
		t.Errorf("expected 4 lines for err filter, got %d", result.Total)
	}

	// Filter: warning and above = 5
	result = parseLogOutput("syslog", input, "", "warning")
	if result.Total != 5 {
		t.Errorf("expected 5 lines for warning filter, got %d", result.Total)
	}

	// Filter: info and above = 7 (all except debug)
	result = parseLogOutput("syslog", input, "", "info")
	if result.Total != 7 {
		t.Errorf("expected 7 lines for info filter, got %d", result.Total)
	}

	// Filter: emerg = 1
	result = parseLogOutput("syslog", input, "", "emerg")
	if result.Total != 1 {
		t.Errorf("expected 1 line for emerg filter, got %d", result.Total)
	}

	// Filter: debug = all 8
	result = parseLogOutput("syslog", input, "", "debug")
	if result.Total != 8 {
		t.Errorf("expected 8 lines for debug filter, got %d", result.Total)
	}
}

func TestParseLogOutput_LevelExtracted(t *testing.T) {
	input := "Tue Mar 11 09:17:52 2026 daemon.err dnsmasq[1234]: error msg"
	result := parseLogOutput("syslog", input, "", "")
	if result.Total != 1 {
		t.Fatalf("expected 1 line, got %d", result.Total)
	}
	if result.Lines[0].Level != "err" {
		t.Errorf("expected level 'err', got %q", result.Lines[0].Level)
	}
}

func TestParseLogOutput_LevelAndServiceFilter(t *testing.T) {
	input := `Tue Mar 11 09:17:50 2026 daemon.debug dnsmasq[1234]: debug msg
Tue Mar 11 09:17:51 2026 daemon.err dnsmasq[1234]: error msg
Tue Mar 11 09:17:52 2026 kern.err netifd[456]: kernel error
Tue Mar 11 09:17:53 2026 daemon.info dnsmasq[1234]: info msg`

	// Filter: dnsmasq + err level = only the dnsmasq err line
	result := parseLogOutput("syslog", input, "dnsmasq", "err")
	if result.Total != 1 {
		t.Errorf("expected 1 line for dnsmasq+err, got %d", result.Total)
	}
}

// Level extraction must not depend on a fixed column: BusyBox logread and
// dmesg disagree about the weekday prefix, and the old index-based parse
// returned "" for every line without it (so ?level=err returned everything).
func TestExtractLevel_PositionIndependent(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{"weekday prefix", "Tue Mar 11 09:17:52 2026 daemon.err dnsmasq[1]: boom", "err"},
		{"no weekday prefix", "daemon.err dnsmasq[1]: boom", "err"},
		{"no year, short date", "Mar 11 09:17:52 daemon.warning dnsmasq[1]: slow", "warning"},
		{"no timestamp at all", "user.notice netifd: up", "notice"},
		{"kern facility", "kern.crit kernel: critical", "crit"},
		{"warn alias", "daemon.warn dnsmasq[1]: hmm", "warning"},
		{"error alias", "daemon.error dnsmasq[1]: bad", "err"},
		{"plain text, no level", "something happened at 10.1.2.3", ""},
		{"dmesg style", "[ 12.345678] ath11k firmware crashed", ""},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractLevel(tt.line); got != tt.want {
				t.Errorf("extractLevel(%q) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}

func TestParseLogOutput_LevelFilterWorksWithoutWeekday(t *testing.T) {
	input := `daemon.debug dnsmasq[1]: debug msg
daemon.info dnsmasq[1]: info msg
daemon.err dnsmasq[1]: error msg
kern.crit kernel: critical`
	res := parseLogOutput("syslog", input, "", "err")
	if res.Total != 2 {
		t.Fatalf("expected 2 lines at err or above, got %d", res.Total)
	}
	for _, l := range res.Lines {
		sev, ok := logLevelSeverity[l.Level]
		if !ok {
			t.Fatalf("line %q kept with unknown level %q", l.Line, l.Level)
		}
		if sev > logLevelSeverity["err"] {
			t.Errorf("line %q (level %q) should have been filtered out", l.Line, l.Level)
		}
	}
}
