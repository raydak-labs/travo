package txn

import (
	"errors"
	"testing"
	"time"
)

// argSpawner records the watchdog's window argument before the short spawner rewrites it.
type argSpawner struct {
	*shortSpawner
	window string
}

func (s *argSpawner) Start(script string, args ...string) (int, error) {
	s.window = args[2]
	return s.shortSpawner.Start(script, args...)
}

func TestBeginWindowArmsTheLongerWindow(t *testing.T) {
	t.Parallel()
	m, d := setup(t, time.Second)
	rec := &argSpawner{shortSpawner: m.Spawner.(*shortSpawner)}
	m.Spawner = rec
	st, err := m.BeginWindow(ctx, "lan", []string{"network"}, 180*time.Second, stageSet(d, "network"), nil)
	if err != nil || st.State != Pending {
		t.Fatal(st, err)
	}
	if st.WindowSeconds != 180 || rec.window != "180" {
		t.Fatalf("window: status %d, watchdog %q", st.WindowSeconds, rec.window)
	}
	if got, err := m.Get(st.ID); err != nil || got.WindowSeconds != 180 {
		t.Fatalf("persisted %+v %v", got, err)
	}
	if _, err := m.Confirm(st.ID); err != nil {
		t.Fatal(err)
	}
}

func TestBeginWindowRefusesAShorterWindow(t *testing.T) {
	t.Parallel()
	m, d := setup(t, time.Second)
	_, err := m.BeginWindow(ctx, "lan", []string{"network"}, m.Window-time.Second, stageSet(d, "network"), nil)
	if !errors.Is(err, ErrBadWindow) || guarded(t, m) {
		t.Fatalf("err = %v, guarded = %v", err, guarded(t, m))
	}
}
