package mock

import (
	"context"
	"errors"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

var ctx = context.Background()

func TestSetTypeUpsertsAndSetNeedsSection(t *testing.T) {
	d := New()
	if err := d.Set(ctx, "network", "wwan", "proto", "dhcp"); !errors.Is(err, device.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
	for i := 0; i < 2; i++ { // upsert: second call must not error
		if err := d.SetType(ctx, "network", "wwan", "interface"); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.Set(ctx, "network", "wwan", "proto", "dhcp"); err != nil {
		t.Fatal(err)
	}
}

func TestRevertAfterCommitIsNoOp(t *testing.T) {
	d := New()
	_ = d.SetType(ctx, "wireless", "sta0", "wifi-iface")
	_ = d.Set(ctx, "wireless", "sta0", "disabled", "0")
	_ = d.Commit(ctx, "wireless")
	_ = d.Revert(ctx, "wireless")
	s, _ := d.Show(ctx, "wireless")
	if len(s) != 1 || s[0].Options["disabled"] != "0" {
		t.Fatalf("revert undid committed data: %+v", s)
	}
}

func TestRevertDropsStaged(t *testing.T) {
	d := New()
	_ = d.SetType(ctx, "firewall", "z", "zone")
	_ = d.Commit(ctx, "firewall")
	_ = d.SetType(ctx, "firewall", "y", "zone")
	_ = d.Revert(ctx, "firewall")
	s, _ := d.Show(ctx, "firewall")
	if len(s) != 1 {
		t.Fatalf("%+v", s)
	}
}

func TestUnscriptedUbusIsNotFound(t *testing.T) {
	if _, err := New().Call(ctx, "x", "y", nil); !errors.Is(err, device.ErrNotFound) {
		t.Fatal(err)
	}
}

func TestLatestHandshakesIsNotFound(t *testing.T) {
	if _, err := New().LatestHandshakes(ctx, "wg0"); !errors.Is(err, device.ErrNotFound) {
		t.Fatal(err)
	}
}
