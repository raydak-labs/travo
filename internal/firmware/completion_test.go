package firmware

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"testing"
	"time"
)

// newerBoard changes only the verbatim capture's revision: the image that was flashed.
func newerBoard(t *testing.T) []byte {
	t.Helper()
	b := fixture(t, "device/axt1800-25.12.6/system-board.json")
	out := bytes.Replace(b, []byte(`"revision": "r33311-981c978eff"`), []byte(`"revision": "r33400-0123456789"`), 1)
	if bytes.Equal(out, b) {
		t.Fatal("fixture shape changed")
	}
	return out
}

// flashed applies an upgrade, then returns a fresh service over the same guards and
// device, as the daemon sees them after the reboot.
func flashed(t *testing.T) (*Service, *device) {
	t.Helper()
	s, d := setup(t)
	img := staged(t, s)
	if err := s.Apply(context.Background(), img.SHA256, true, Phrase); err != nil {
		t.Fatal(err)
	}
	held, err := d.guards.Stale()
	if err != nil || len(held) != 1 {
		t.Fatalf("%v %v", held, err)
	}
	want := map[string]string{DetailBootID: "boot-before", DetailRevision: "r33311-981c978eff", DetailSHA256: img.SHA256}
	for k, v := range want {
		if held[0].Detail[k] != v {
			t.Fatalf("guard detail %v, want %v", held[0].Detail, want)
		}
	}
	at := time.Date(2026, 11, 19, 9, 0, 0, 0, time.UTC)
	return &Service{Device: d, Guards: d.guards, Now: func() time.Time { return at }}, d
}

func TestCompleteReleasesTheGuardOfAFinishedUpgradeAndAlertsOnce(t *testing.T) {
	s, d := flashed(t)
	d.bootID, d.board = "boot-after", newerBoard(t)
	out, err := s.Complete(context.Background())
	if err != nil || out != OutcomeCompleted {
		t.Fatalf("%v %v", out, err)
	}
	if slices.Contains(heldNames(t, s), GuardName) {
		t.Fatal("guard kept after a finished upgrade")
	}
	if at := s.Completed(); at == nil || at.Hour() != 9 {
		t.Fatalf("completed %v", at)
	}
	// The next daemon start finds no guard and raises nothing.
	next := &Service{Device: d, Guards: d.guards}
	if out, err := next.Complete(context.Background()); err != nil || out != OutcomeNone || next.Completed() != nil {
		t.Fatalf("%v %v %v", out, err, next.Completed())
	}
}

func TestCompleteKeepsTheGuardOfAnInterruptedFlash(t *testing.T) {
	for name, boot := range map[string]string{"rebooted, old firmware": "boot-after", "same boot": "boot-before"} {
		s, d := flashed(t)
		d.bootID = boot
		if name == "same boot" {
			d.board = newerBoard(t)
		}
		out, err := s.Complete(context.Background())
		if err != nil || out != OutcomeInterrupted {
			t.Fatalf("%s: %v %v", name, out, err)
		}
		if !slices.Contains(heldNames(t, s), GuardName) || s.Completed() != nil {
			t.Fatalf("%s: guard released", name)
		}
	}
}

func TestCompleteFailsClosedWhenSomethingIsUnreadable(t *testing.T) {
	s, d := flashed(t)
	d.bootID, d.boardErr = "boot-after", errors.New("ubus: timeout")
	if out, err := s.Complete(context.Background()); err != nil || out != OutcomeUnknown {
		t.Fatalf("%v %v", out, err)
	}
	if !slices.Contains(heldNames(t, s), GuardName) || s.Completed() != nil {
		t.Fatal("guard released without the running revision")
	}
}

func TestDecide(t *testing.T) {
	rec := map[string]string{DetailBootID: "a", DetailRevision: "r1"}
	for _, c := range []struct {
		detail    map[string]string
		boot, rev string
		want      Outcome
	}{
		{rec, "b", "r2", OutcomeCompleted},
		{rec, "a", "r2", OutcomeInterrupted},
		{rec, "b", "r1", OutcomeInterrupted},
		{rec, "", "r2", OutcomeUnknown},
		{rec, "b", "", OutcomeUnknown},
		{nil, "b", "r2", OutcomeUnknown}, // a guard written before ADR 0030
		{map[string]string{DetailBootID: "a"}, "b", "r2", OutcomeUnknown},
	} {
		if got := Decide(c.detail, c.boot, c.rev); got != c.want {
			t.Errorf("%v %q %q: %v, want %v", c.detail, c.boot, c.rev, got, c.want)
		}
	}
}
