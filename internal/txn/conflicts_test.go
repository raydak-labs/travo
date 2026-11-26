package txn

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/guard"
)

func txnGuarded(t *testing.T, m *Manager) bool {
	t.Helper()
	st, err := m.Guards.Stale()
	if err != nil {
		t.Fatal(err)
	}
	return slices.ContainsFunc(st, func(g guard.Info) bool { return g.Name == GuardName })
}

func TestBeginAndApplyRefuseTerminalAndPackageOpGuards(t *testing.T) {
	t.Parallel()
	for _, held := range []string{"firmware-upgrade-in-progress", "factory-reset-in-progress", "reboot-in-progress", "update-in-progress", "catalog-package-in-progress"} {
		m, d := setup(t, 60*time.Second)
		if err := m.Guards.Acquire(held); err != nil {
			t.Fatal(err)
		}
		if _, err := m.Begin(ctx, "t", []string{"network"}, stageSet(d, "network"), nil); !errors.Is(err, ErrGuarded) || !errors.Is(err, ErrActive) {
			t.Errorf("%s: Begin got %v, want ErrGuarded (a 409)", held, err)
		}
		if txnGuarded(t, m) {
			t.Errorf("%s: refusal left the txn guard", held)
		}
		if err := m.Apply(ctx, []string{"dhcp"}, stageSet(d, "dhcp")); !errors.Is(err, ErrGuarded) {
			t.Errorf("%s: Apply got %v", held, err)
		}
		if content(t, m, "network") != "original network\n" || d.Reloads != 0 {
			t.Errorf("%s: a refused change reached the device", held)
		}
	}
}

func TestBeginRunsBesideOtherGuards(t *testing.T) {
	t.Parallel()
	m, d := setup(t, 60*time.Second)
	_ = m.Guards.Acquire("dns-layer-in-progress")
	if _, err := m.Begin(ctx, "t", []string{"network"}, stageSet(d, "network"), nil); err != nil {
		t.Fatal(err)
	}
}

func TestRefuseAnyGuardSkipsWhileAnyGuardIsHeld(t *testing.T) {
	t.Parallel()
	m, d := setup(t, 60*time.Second)
	_ = m.Guards.Acquire("interface-restart-in-progress")
	if _, err := m.Begin(RefuseAnyGuard(ctx), "t", []string{"network"}, stageSet(d, "network"), nil); !errors.Is(err, ErrGuardHeld) {
		t.Fatalf("got %v, want ErrGuardHeld", err)
	}
	if txnGuarded(t, m) {
		t.Fatal("refusal left the txn guard")
	}
}
