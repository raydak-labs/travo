package wifi

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

func guestFixtures(t *testing.T) (guestConfigs, []facts.Interface) {
	t.Helper()
	live, err := facts.ParseInterfaceDump(fixture(t, "interface-dump.json"))
	if err != nil {
		t.Fatal(err)
	}
	return guestConfigs{
		wireless: configFixture(t, "wireless", "uci-show-wireless.txt"),
		network:  configFixture(t, "network", "uci-show-network.txt"),
		firewall: configFixture(t, "firewall", "uci-show-firewall.txt"),
		dhcp:     configFixture(t, "dhcp", "uci-show-dhcp.txt"),
	}, live
}

func TestPlanGuestCreatesTheFullSurface(t *testing.T) {
	c, live := guestFixtures(t)
	writes, before, after, err := planGuest(c, live, GuestRequest{SSID: "Guests", Key: "example-only", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []uciWrite{
		{"type", "network", "guest_dev", "", "device"},
		{"set", "network", "guest", "ipaddr", "192.168.2.1/24"},
		{"set", "dhcp", "guest", "interface", "guest"},
		{"set", "firewall", "guest", "input", "REJECT"},
		{"set", "firewall", "guest", "forward", "REJECT"},
		{"set", "firewall", "guest_wan", "dest", "wan"},
		{"set", "firewall", "guest_dns", "dest_port", "53"},
		// The STA holds radio0 in this capture; the guest AP goes on radio1.
		{"set", "wireless", "guest", "device", "radio1"},
		{"set", "wireless", "guest", "isolate", "1"},
		{"key", "wireless", "guest", "key", ""},
		{"set", "wireless", "guest", "disabled", "0"},
	} {
		if !slices.Contains(writes, want) {
			t.Errorf("missing %v", want)
		}
	}
	// No path from guest into lan.
	if slices.ContainsFunc(writes, func(w uciWrite) bool { return w.config == "firewall" && w.value == "lan" }) {
		t.Fatal("guest reaches lan")
	}
	if len(after) != len(before)+1 || !after[len(after)-1].Guest {
		t.Fatalf("lockout layout %+v", after)
	}
}

func TestPlanGuestReapplyStagesNothing(t *testing.T) {
	ctx := context.Background()
	d := mock.New()
	seedConfigs(t, d, "uci-show-wireless.txt", nil)
	s, _, _ := apService(t, apUbus(t))
	s.join.UCI, s.join.Keys = d, d
	req := GuestRequest{SSID: "Guests", Key: "example-only", Enabled: true}
	_, live := guestFixtures(t)
	c, err := s.readGuestConfigs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	writes, _, _, err := planGuest(c, live, req)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.stageWrites(ctx, writes, req.Key); err != nil {
		t.Fatal(err)
	}
	if c, err = s.readGuestConfigs(ctx); err != nil {
		t.Fatal(err)
	}
	for _, again := range []GuestRequest{req, {SSID: "Guests", Enabled: true}} {
		if writes, _, _, err := planGuest(c, live, again); err != nil || len(writes) != 0 {
			t.Fatalf("re-apply staged %v %v", writes, err)
		}
	}
	got, err := s.Guest(ctx)
	if err != nil || got != (GuestConfig{Configured: true, Enabled: true, SSID: "Guests", Radio: "radio1", Subnet: "192.168.2.0/24"}) {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestPlanGuestRefusals(t *testing.T) {
	ok := GuestRequest{SSID: "Guests", Key: "example-only", Enabled: true}
	for _, tc := range []struct {
		name   string
		mutate func(*guestConfigs, *[]facts.Interface)
		req    GuestRequest
		err    error
	}{
		{"uplink lease in the guest subnet", func(_ *guestConfigs, live *[]facts.Interface) {
			*live = append(*live, facts.Interface{Name: "wwan", Up: true, IPv4: []netip.Prefix{netip.MustParsePrefix("192.168.2.77/24")}})
		}, ok, ErrGuestOverlap},
		{"configured netmask form", func(c *guestConfigs, _ *[]facts.Interface) {
			c.network = append(c.network, device.Section{Name: "office", Type: "interface", Options: map[string]string{"ipaddr": "192.168.0.1", "netmask": "255.255.252.0"}})
		}, ok, ErrGuestOverlap},
		{"first creation without key", nil, GuestRequest{SSID: "Guests", Enabled: true}, ValidationError{"wifi_key_required"}},
		{"STA radio requested", nil, GuestRequest{SSID: "Guests", Key: "example-only", Enabled: true, Radio: "radio0"}, ErrRadioInUse},
		{"unknown radio", nil, GuestRequest{SSID: "Guests", Key: "example-only", Enabled: true, Radio: "radio9"}, ErrNoGuestRadio},
		{"foreign guest zone", func(c *guestConfigs, _ *[]facts.Interface) {
			c.firewall = append(c.firewall, device.Section{Name: "cfg0abc", Type: "zone", Options: map[string]string{"name": "guest"}})
		}, ok, ErrGuestConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, live := guestFixtures(t)
			if tc.mutate != nil {
				tc.mutate(&c, &live)
			}
			if _, _, _, err := planGuest(c, live, tc.req); !errors.Is(err, tc.err) && err != tc.err {
				t.Fatal(err)
			}
		})
	}
}

func TestSetGuestOverlapWritesNothing(t *testing.T) {
	ubus := apUbus(t)
	ubus.replies[ubusKey{"network.interface", "dump", ""}] = mutatedFixture(t, "interface-dump.json", func(p map[string]any) {
		// Same capture with the lan prefix moved onto the guest subnet.
		for _, raw := range p["interface"].([]any) {
			iface := raw.(map[string]any)
			if iface["interface"] == "lan" {
				iface["ipv4-address"].([]any)[0].(map[string]any)["address"] = "192.168.2.1"
			}
		}
	})
	s, d, begin := apService(t, ubus)
	_, err := s.SetGuest(context.Background(), GuestRequest{SSID: "Guests", Key: "example-only", Enabled: true})
	pending, _ := d.Pending(context.Background(), "network")
	if !errors.Is(err, ErrGuestOverlap) || begin.calls != 0 || pending {
		t.Fatal(err)
	}
}

func TestSetGuestFailingProbeRollsBackEveryConfig(t *testing.T) {
	ctx := context.Background()
	ubus := apUbus(t)
	base := mock.New()
	seedConfigs(t, base, "uci-show-wireless.txt", nil)
	// netifd never lists the guest AP, so it cannot be proven up.
	manager, configDir := fileTxnManager(t, base, func() {})
	s := New(ctx, ubus, nil).WithJoin(JoinDeps{UCI: manager.Device, Ubus: ubus, Keys: base, Neighbours: base, Txns: manager, ProbeBudget: 30 * time.Millisecond, ProbeEvery: time.Millisecond})
	st, err := s.SetGuest(ctx, GuestRequest{SSID: "Guests", Key: "example-only", Enabled: true})
	if !errors.Is(err, txn.ErrProbeFailed) || st.State != txn.RolledBack {
		t.Fatal(st, err)
	}
	for _, config := range []string{"wireless", "network", "firewall", "dhcp"} {
		content, err := os.ReadFile(filepath.Join(configDir, config))
		if err != nil || string(content) != "original "+config {
			t.Fatalf("%s not restored: %v", config, err)
		}
	}
}

func TestGuestProbeProvesEachAPIndependently(t *testing.T) {
	c, _ := guestFixtures(t)
	expect := apExpectations(c.wireless, guestName, true, "Guests")
	if len(expect) != 1 || expect[0].section != "default_radio1" {
		t.Fatalf("main AP not expected: %+v", expect)
	}
}
