package vpnkill

import (
	"errors"
	"slices"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
)

// guestZone is the zone section internal/wifi planGuest writes (invariants §10).
func guestZone() device.Section {
	return device.Section{Name: GuestZone, Type: "zone", Options: map[string]string{
		"name": GuestZone, "network": GuestZone, "input": "REJECT", "output": "ACCEPT", "forward": "REJECT",
	}, Lists: map[string][]string{}}
}

func names(secs []device.Section) []string {
	var out []string
	for _, s := range secs {
		out = append(out, s.Name)
	}
	return out
}

func TestDesiredCoversTheGuestZone(t *testing.T) {
	want, err := Desired(append(fixture(t), guestZone()))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(names(want), []string{ZoneName, LANForward, GuestForward, RulePrefix + "wan", RulePrefix + "guest_wan"}) {
		t.Fatalf("%v", names(want))
	}
	if f := want[2].Options; f["src"] != GuestZone || f["dest"] != ZoneName {
		t.Fatalf("%+v", f)
	}
	if r := want[4].Options; r["src"] != GuestZone || r["dest"] != "wan" || r["target"] != "REJECT" || r["proto"] != "all" {
		t.Fatalf("%+v", r)
	}
}

func TestUserZoneNamedGuestIsNotAdopted(t *testing.T) {
	user := guestZone()
	user.Name = "cfg0abc"
	want, err := Desired(append(fixture(t), user))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(names(want), []string{ZoneName, LANForward, RulePrefix + "wan"}) {
		t.Fatalf("%v", names(want))
	}
}

func TestDesiredRefusesRuleNameCollision(t *testing.T) {
	secs := append(fixture(t), guestZone(), device.Section{Name: "cfg77", Type: "zone",
		Options: map[string]string{"name": "guest_wan"}, Lists: map[string][]string{"network": {"wwan"}}})
	if _, err := Desired(secs); !errors.Is(err, ErrUnsupported) {
		t.Fatal(err)
	}
}

func TestGuestSectionsFollowTheVPNState(t *testing.T) {
	base := append(fixture(t), guestZone())
	if got, err := GuestSections(base); err != nil || got != nil {
		t.Fatalf("no VPN: %v %v", got, err)
	}
	if got, err := GuestSections(fixture(t)); err != nil || got != nil {
		t.Fatalf("no guest zone: %v %v", got, err)
	}
	withZone := append(slices.Clone(base), VPNZone(fixture(t))...)
	if got, _ := GuestSections(withZone); !slices.Equal(names(got), []string{GuestForward}) {
		t.Fatalf("tunnel zone: %v", names(got))
	}
	desired, err := Desired(fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	withSwitch := append(slices.Clone(base), desired...)
	if got, _ := GuestSections(withSwitch); !slices.Equal(names(got), []string{GuestForward, RulePrefix + "guest_wan"}) {
		t.Fatalf("kill switch: %v", names(got))
	}
}

func TestStageZoneAddsAndDropsTheGuestForwarding(t *testing.T) {
	d := mock.New()
	if err := d.Load(ctx, "firewall", append(append(fixture(t), guestZone()), VPNZone(fixture(t))...)); err != nil {
		t.Fatal(err)
	}
	current := show(t, d)
	if missing, err := ZoneMissing(current); err != nil || !missing {
		t.Fatalf("guest forwarding absent must count as missing: %v %v", missing, err)
	}
	if err := StageZone(ctx, d, current); err != nil {
		t.Fatal(err)
	}
	if err := d.Commit(ctx, "firewall"); err != nil {
		t.Fatal(err)
	}
	current = show(t, d)
	if missing, err := ZoneMissing(current); err != nil || missing {
		t.Fatalf("after staging: %v %v", missing, err)
	}

	// The guest zone goes away: its forwarding is stale and the next stage removes it.
	if err := d.DeleteSection(ctx, "firewall", GuestZone); err != nil {
		t.Fatal(err)
	}
	if err := d.Commit(ctx, "firewall"); err != nil {
		t.Fatal(err)
	}
	current = show(t, d)
	if missing, _ := ZoneMissing(current); !missing {
		t.Fatal("stale guest forwarding must count")
	}
	if err := StageZone(ctx, d, current); err != nil {
		t.Fatal(err)
	}
	if slices.Contains(names(show(t, d)), GuestForward) {
		t.Fatal("stale guest forwarding kept")
	}
}

func TestKillSwitchRoundTripWithGuestZone(t *testing.T) {
	svc, d, _, _ := setup(t)
	if err := d.Load(ctx, "firewall", append(fixture(t), guestZone())); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Set(ctx, true); err != nil {
		t.Fatal(err)
	}
	got := names(show(t, d))
	for _, want := range []string{GuestForward, RulePrefix + "guest_wan", RulePrefix + "wan"} {
		if !slices.Contains(got, want) {
			t.Fatalf("missing %s in %v", want, got)
		}
	}
	if _, err := svc.Set(ctx, false); err != nil {
		t.Fatal(err)
	}
	got = names(show(t, d))
	if Enabled(show(t, d)) || !slices.Contains(got, GuestForward) {
		t.Fatalf("disable removes the rules and keeps the guest forwarding: %v", got)
	}
}
