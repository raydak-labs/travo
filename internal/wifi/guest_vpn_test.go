package wifi

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/vpnkill"
)

// writeSections stages sections the way vpnkill writes them, then commits.
func writeSections(t *testing.T, d *mock.Device, sections []device.Section) {
	t.Helper()
	ctx := context.Background()
	for _, x := range sections {
		if err := d.SetType(ctx, "firewall", x.Name, x.Type); err != nil {
			t.Fatal(err)
		}
		for k, v := range x.Options {
			if err := d.Set(ctx, "firewall", x.Name, k, v); err != nil {
				t.Fatal(err)
			}
		}
		for k, vs := range x.Lists {
			for _, v := range vs {
				if err := d.AddToList(ctx, "firewall", x.Name, k, v); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	if err := d.Commit(ctx, "firewall"); err != nil {
		t.Fatal(err)
	}
}

func travoWrites(writes []uciWrite) []uciWrite {
	return slices.DeleteFunc(slices.Clone(writes), func(w uciWrite) bool { return !strings.HasPrefix(w.section, "travo_") })
}

func TestPlanGuestWithoutVPNWritesNoVPNSections(t *testing.T) {
	c, live := guestFixtures(t)
	writes, _, _, err := planGuest(c, live, GuestRequest{SSID: "Guests", Key: "example-only", Enabled: true})
	if err != nil || len(travoWrites(writes)) != 0 {
		t.Fatalf("%v %v", travoWrites(writes), err)
	}
}

// Invariants §11: creating the guest AP while the tunnel zone or the kill switch is on
// keeps the same guarantees for guests in the same transaction.
func TestPlanGuestFollowsTheVPNState(t *testing.T) {
	for _, tc := range []struct {
		name       string
		killSwitch bool
		want       []uciWrite
	}{
		{"tunnel zone only", false, []uciWrite{
			{"type", "firewall", vpnkill.GuestForward, "", "forwarding"},
			{"set", "firewall", vpnkill.GuestForward, "src", "guest"},
			{"set", "firewall", vpnkill.GuestForward, "dest", vpnkill.ZoneName},
		}},
		{"kill switch on", true, []uciWrite{
			{"set", "firewall", vpnkill.GuestForward, "src", "guest"},
			{"type", "firewall", vpnkill.RulePrefix + "guest_wan", "", "rule"},
			{"set", "firewall", vpnkill.RulePrefix + "guest_wan", "src", "guest"},
			{"set", "firewall", vpnkill.RulePrefix + "guest_wan", "dest", "wan"},
			{"set", "firewall", vpnkill.RulePrefix + "guest_wan", "target", "REJECT"},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, live := guestFixtures(t)
			vpn := vpnkill.VPNZone(c.firewall)
			if tc.killSwitch {
				var err error
				if vpn, err = vpnkill.Desired(c.firewall); err != nil {
					t.Fatal(err)
				}
			}
			c.firewall = append(c.firewall, vpn...)
			writes, _, _, err := planGuest(c, live, GuestRequest{SSID: "Guests", Key: "example-only", Enabled: true})
			if err != nil {
				t.Fatal(err)
			}
			for _, w := range tc.want {
				if !slices.Contains(writes, w) {
					t.Errorf("missing %v", w)
				}
			}
			hasRule := slices.ContainsFunc(writes, func(w uciWrite) bool { return strings.HasPrefix(w.section, vpnkill.RulePrefix) })
			if hasRule != tc.killSwitch {
				t.Fatalf("kill switch rules written: %v", hasRule)
			}
			// Only guest-sourced owned sections are added; lan's stay as they were.
			for _, w := range travoWrites(writes) {
				if w.section == vpnkill.ZoneName || w.section == vpnkill.LANForward || w.section == vpnkill.RulePrefix+"wan" {
					t.Fatalf("rewrote %v", w)
				}
			}
		})
	}
}

func TestPlanGuestRefusesForeignSectionOnOwnedName(t *testing.T) {
	c, live := guestFixtures(t)
	c.firewall = append(c.firewall, vpnkill.VPNZone(c.firewall)...)
	c.firewall = append(c.firewall, device.Section{Name: vpnkill.GuestForward, Type: "rule", Options: map[string]string{}})
	if _, _, _, err := planGuest(c, live, GuestRequest{SSID: "Guests", Key: "example-only", Enabled: true}); err != ErrGuestConflict {
		t.Fatal(err)
	}
}

// After the guest transaction stages, the firewall matches what the kill switch and the
// tunnel enable want, so neither finds anything left to change.
func TestGuestWithKillSwitchMatchesVPNDesired(t *testing.T) {
	ctx := context.Background()
	d := mock.New()
	seedConfigs(t, d, "uci-show-wireless.txt", nil)
	base, err := d.Show(ctx, "firewall")
	if err != nil {
		t.Fatal(err)
	}
	want, err := vpnkill.Desired(base)
	if err != nil {
		t.Fatal(err)
	}
	writeSections(t, d, want)
	s, _, _ := apService(t, apUbus(t))
	s.join.UCI, s.join.Keys = d, d
	_, live := guestFixtures(t)
	c, err := s.readGuestConfigs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	req := GuestRequest{SSID: "Guests", Key: "example-only", Enabled: true}
	writes, _, _, err := planGuest(c, live, req)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.stageWrites(ctx, writes, req.Key); err != nil {
		t.Fatal(err)
	}
	firewall, err := d.Show(ctx, "firewall")
	if err != nil {
		t.Fatal(err)
	}
	if !vpnkill.HasGuestZone(firewall) {
		t.Fatal("the guest code's zone is not the one vpnkill detects")
	}
	again, err := vpnkill.Desired(firewall)
	if err != nil || !vpnkill.Matches(firewall, again) {
		t.Fatalf("kill switch would still change: %v", err)
	}
	if missing, err := vpnkill.ZoneMissing(firewall); err != nil || missing {
		t.Fatalf("tunnel enable would still change: %v %v", missing, err)
	}
	if c, err = s.readGuestConfigs(ctx); err != nil {
		t.Fatal(err)
	}
	if writes, _, _, err := planGuest(c, live, req); err != nil || len(writes) != 0 {
		t.Fatalf("re-apply staged %v %v", writes, err)
	}
}
