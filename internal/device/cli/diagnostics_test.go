package cli

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

// diagCapture splits a diagnostics/ capture into the tool's text and the exit code line
// the capture appended (diagnostics/PROVENANCE.md).
func diagCapture(t *testing.T, name string) (string, string) {
	t.Helper()
	b, err := os.ReadFile("../../../testdata/device/axt1800-25.12.6/diagnostics/" + name)
	if err != nil {
		t.Fatal(err)
	}
	text, exit, ok := strings.Cut(string(b), "exit=")
	if !ok {
		t.Fatal("no exit line")
	}
	return text, strings.TrimSpace(exit)
}

func TestDiagnoseArgvAndTiers(t *testing.T) {
	for _, tc := range []struct {
		tool string
		tier time.Duration
		argv []string
	}{
		{"ping", TierPing, []string{"ping", "-c", "4", "-W", "2", "10.0.1.1"}},
		{"traceroute", TierTraceroute, []string{"traceroute", "-n", "-w", "2", "-q", "1", "-m", "20", "10.0.1.1"}},
		{"lookup", TierLookup, []string{"nslookup", "10.0.1.1", "127.0.0.1"}},
	} {
		var got []string
		var budget time.Duration
		d := Device{Run: func(ctx context.Context, name string, args ...string) (string, int, error) {
			got = append([]string{name}, args...)
			deadline, _ := ctx.Deadline()
			budget = time.Until(deadline)
			return "", 0, nil
		}}
		run, err := d.Diagnose(t.Context(), tc.tool, "10.0.1.1")
		if err != nil || !run.OK || !slices.Equal(got, tc.argv) || budget > tc.tier || budget < tc.tier-time.Second {
			t.Errorf("%s: %v %+v %v %v", tc.tool, err, run, got, budget)
		}
	}
}

func TestDiagnoseOutcomesFromCaptures(t *testing.T) {
	okText, _ := diagCapture(t, "ping-ok.txt")
	d := Device{Run: func(context.Context, string, ...string) (string, int, error) { return okText, 0, nil }}
	if run, err := d.Diagnose(t.Context(), "ping", "10.0.1.1"); err != nil || !run.OK || run.Output != okText {
		t.Fatalf("ok: %+v %v", run, err)
	}
	// Stdout-only failure: the statistics stay, OK is false.
	noReply, exit := diagCapture(t, "ping-no-reply.txt")
	d.Run = func(context.Context, string, ...string) (string, int, error) {
		return noReply, 1, failedCommand("ping", "")
	}
	if run, err := d.Diagnose(t.Context(), "ping", "10.0.1.254"); err != nil || run.OK || run.Output != noReply || exit != "1" {
		t.Fatalf("no reply: %+v %v", run, err)
	}
	// Stderr-only failure: the diagnostic is the output.
	bad, _ := diagCapture(t, "ping-bad-host.txt")
	d.Run = func(context.Context, string, ...string) (string, int, error) {
		return "", 1, failedCommand("ping", strings.TrimSpace(bad))
	}
	if run, err := d.Diagnose(t.Context(), "ping", "does-not-exist.invalid"); err != nil || run.OK || run.Output != strings.TrimSpace(bad) {
		t.Fatalf("bad host: %+v %v", run, err)
	}
}

func TestDiagnoseTimeoutAndStartFailure(t *testing.T) {
	d := Device{Run: func(ctx context.Context, _ string, _ ...string) (string, int, error) {
		<-ctx.Done()
		return "partial\n", -1, failedCommand("traceroute", "")
	}}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()
	if run, err := d.Diagnose(ctx, "traceroute", "10.0.1.1"); err != nil || run.OK || !run.TimedOut || run.Output != "partial\n" {
		t.Fatalf("timeout: %+v %v", run, err)
	}
	d.Run = func(context.Context, string, ...string) (string, int, error) {
		return "", 0, errors.New("exec: not found")
	}
	if _, err := d.Diagnose(t.Context(), "lookup", "example.com"); err == nil {
		t.Fatal("start failure must be an error")
	}
}

func TestDiagnoseRefusesBadInputBeforeRunning(t *testing.T) {
	d := Device{Run: func(context.Context, string, ...string) (string, int, error) {
		t.Fatal("ran with invalid input")
		return "", 0, nil
	}}
	for _, tc := range [][2]string{{"ping", "-c100"}, {"ping", "a;reboot"}, {"ping", ""}, {"curl", "example.com"}} {
		if _, err := d.Diagnose(t.Context(), tc[0], tc[1]); err == nil {
			t.Fatal(tc)
		}
	}
}
