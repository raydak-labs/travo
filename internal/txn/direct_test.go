package txn

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDirectApplyCommitsAndReloads(t *testing.T) {
	t.Parallel()
	m, d := setup(t, time.Second)
	_ = d.Device.SetType(ctx, "dhcp", "s", "x")
	_ = d.Device.Commit(ctx, "dhcp")
	if err := m.Apply(ctx, []string{"dhcp"}, stageSet(d, "dhcp")); err != nil {
		t.Fatal(err)
	}
	if p, _ := d.Pending(ctx, "dhcp"); p || d.Reloads != 1 {
		t.Fatal("not committed or not reloaded")
	}
}

func TestDirectApplyRefusesConnectivityConfigs(t *testing.T) {
	t.Parallel()
	m, d := setup(t, time.Second)
	for _, c := range []string{"wireless", "network", "firewall"} {
		if err := m.Apply(ctx, []string{"dhcp", c}, stageSet(d, c)); err == nil {
			t.Errorf("%s applied without watchdog", c)
		}
	}
	if d.Reloads != 0 {
		t.Fatal("reloaded")
	}
}

func TestDirectApplyRevertsOnStageAndCommitError(t *testing.T) {
	t.Parallel()
	m, d := setup(t, time.Second)
	_ = d.Device.SetType(ctx, "dhcp", "s", "x")
	_ = d.Device.Commit(ctx, "dhcp")
	err := m.Apply(ctx, []string{"dhcp"}, func(ctx context.Context) error {
		_ = stageSet(d, "dhcp")(ctx)
		return errors.New("bad")
	})
	if err == nil {
		t.Fatal("want error")
	}
	if p, _ := d.Pending(ctx, "dhcp"); p {
		t.Fatal("stage error left delta")
	}
	d.commitErr = errors.New("disk")
	if err := m.Apply(ctx, []string{"dhcp"}, stageSet(d, "dhcp")); err == nil {
		t.Fatal("want error")
	}
	if p, _ := d.Pending(ctx, "dhcp"); p {
		t.Fatal("commit error left delta")
	}
}

func TestDirectApplyRefusesForeignStaged(t *testing.T) {
	t.Parallel()
	m, d := setup(t, time.Second)
	_ = d.Device.SetType(ctx, "dhcp", "s", "x")
	if err := m.Apply(ctx, []string{"dhcp"}, stageSet(d, "dhcp")); !errors.Is(err, ErrForeignStaged) {
		t.Fatal(err)
	}
	if p, _ := d.Pending(ctx, "dhcp"); !p {
		t.Fatal("reverted someone else's delta")
	}
}
