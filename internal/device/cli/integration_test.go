//go:build integration

package cli

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

func TestRealUCIWirelessKey(t *testing.T) {
	ctx := context.Background()
	d := New()
	const config = "/etc/config/wireless"
	if _, err := os.Stat(config); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(config, nil, 0600); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Remove(config) })
	}
	if err := d.SetType(ctx, "wireless", "travo_test", "wifi-iface"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := d.Revert(ctx, "wireless"); err != nil {
			t.Error(err)
		}
	})
	const key = ` it's "quoted" \ # $`
	if err := d.SetWirelessKey(ctx, "travo_test", key); err != nil {
		t.Fatal(err)
	}
	if err := d.SetWirelessKey(ctx, "travo_missing", key); !errors.Is(err, errStageWirelessKey) {
		t.Fatalf("write on missing section returned %v", err)
	}
}

// Runs against the real uci inside an OpenWrt container (see mise task test:integration).
func TestRealUCIMissingConfig(t *testing.T) {
	if _, err := New().Show(context.Background(), "nosuch"); !errors.Is(err, device.ErrNotFound) {
		t.Fatalf("Show on missing config: %v", err)
	}
}

func TestRealUCIRoundTrip(t *testing.T) {
	ctx := context.Background()
	d := New()

	if err := d.Set(ctx, "dhcp", "nosuch", "x", "1"); !errors.Is(err, device.ErrNotFound) {
		t.Fatalf("Set on missing section: %v", err)
	}
	for i := 0; i < 2; i++ { // upsert must not error
		if err := d.SetType(ctx, "dhcp", "travo_test", "dnsmasq"); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.Set(ctx, "dhcp", "travo_test", "note", "it's \"quoted\" $x"); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"1.1.1.1", "/lan/127.0.0.1#53"} {
		if err := d.AddToList(ctx, "dhcp", "travo_test", "server", s); err != nil {
			t.Fatal(err)
		}
	}
	if p, err := d.Pending(ctx, "dhcp"); err != nil || !p {
		t.Fatalf("pending after set: %v %v", p, err)
	}
	secs, err := d.Show(ctx, "dhcp")
	if err != nil {
		t.Fatal(err)
	}
	var got *device.Section
	for i := range secs {
		if secs[i].Name == "travo_test" {
			got = &secs[i]
		}
	}
	if got == nil {
		t.Fatalf("staged section not visible: %+v", secs)
	}
	if got.Options["note"] != `it's "quoted" $x` {
		t.Errorf("quoting lost: %q", got.Options["note"])
	}
	if !reflect.DeepEqual(got.Lists["server"], []string{"1.1.1.1", "/lan/127.0.0.1#53"}) {
		t.Errorf("list: %v", got.Lists["server"])
	}

	if err := d.Revert(ctx, "dhcp"); err != nil {
		t.Fatal(err)
	}
	if p, _ := d.Pending(ctx, "dhcp"); p {
		t.Fatal("pending after revert")
	}
	secs, _ = d.Show(ctx, "dhcp")
	for _, s := range secs {
		if s.Name == "travo_test" {
			t.Fatal("revert did not drop staged section")
		}
	}

	if err := d.SetType(ctx, "dhcp", "travo_test", "dnsmasq"); err != nil {
		t.Fatal(err)
	}
	if err := d.Commit(ctx, "dhcp"); err != nil {
		t.Fatal(err)
	}
	if err := d.Revert(ctx, "dhcp"); err != nil { // no-op after commit
		t.Fatal(err)
	}
	secs, _ = d.Show(ctx, "dhcp")
	found := false
	for _, s := range secs {
		found = found || s.Name == "travo_test"
	}
	if !found {
		t.Fatal("revert after commit removed committed data")
	}
	if err := d.DeleteSection(ctx, "dhcp", "travo_test"); err != nil {
		t.Fatal(err)
	}
	if err := d.Commit(ctx, "dhcp"); err != nil { // leave no staged state for later tests
		t.Fatal(err)
	}
}
