package guard

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newDir(t *testing.T) Dir {
	return Dir{Path: filepath.Join(t.TempDir(), "guards"), Now: func() time.Time { return time.Unix(1700000000, 0) }}
}

func TestAcquireTwiceIsActive(t *testing.T) {
	d := newDir(t)
	if err := d.Acquire("wifi-apply-in-progress"); err != nil {
		t.Fatal(err)
	}
	if err := d.Acquire("wifi-apply-in-progress"); !errors.Is(err, ErrActive) {
		t.Fatalf("got %v", err)
	}
}

func TestAcquireFailureAbortsWhenDirUnwritable(t *testing.T) {
	d := newDir(t)
	if err := os.WriteFile(filepath.Dir(d.Path)+"/guards", []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := d.Acquire("a-in-progress"); err == nil {
		t.Fatal("expected error, never degrade to unguarded")
	}
}

func TestClearByNameOnlyAndRejectsTraversal(t *testing.T) {
	d := newDir(t)
	_ = d.Acquire("a-in-progress")
	_ = d.Acquire("b-in-progress")
	if err := d.Clear("a-in-progress"); err != nil {
		t.Fatal(err)
	}
	st, _ := d.Stale()
	if len(st) != 1 || st[0].Name != "b-in-progress" {
		t.Fatalf("stale %v", st)
	}
	for _, bad := range []string{"../x-in-progress", "*", "a/b-in-progress", "plain"} {
		if err := d.Clear(bad); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

func TestUnreadableGuardStillReportedStale(t *testing.T) {
	d := newDir(t)
	_ = os.MkdirAll(d.Path, 0o700)
	_ = os.WriteFile(filepath.Join(d.Path, "x-in-progress"), []byte("{"), 0o600)
	st, _ := d.Stale()
	if len(st) != 1 {
		t.Fatalf("stale %v", st)
	}
}
