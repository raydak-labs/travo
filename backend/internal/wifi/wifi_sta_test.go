package wifi

import (
	"errors"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

// A section netifd cannot be asked about proves nothing. Skipping it (as this
// used to) could empty the want-list and turn "the AP never came up" into
// "nothing to prove".
func TestEnabledAPSections_APWithoutDeviceIsUnprovable(t *testing.T) {
	sections := map[string]map[string]string{
		"radio0":  {"type": "mac80211", "disabled": "0"},
		"orphan":  {"mode": "ap", "disabled": "0"},
		"radio1":  {"type": "mac80211", "disabled": "0"},
		"default": {"mode": "ap", "device": "radio1", "disabled": "0"},
	}
	if _, err := enabledAPSections(sections); !errors.Is(err, ErrWirelessNotUp) {
		t.Fatalf("an enabled AP section with no device must be unprovable, got %v", err)
	}
	delete(sections, "orphan")
	names, err := enabledAPSections(sections)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 1 || names[0] != "default" {
		t.Fatalf("expected only the provable AP, got %v", names)
	}
}

// Finding 3: nextSTASectionName must not fall back to a name that may exist.
func TestNextSTASectionName_PropagatesReadFailure(t *testing.T) {
	svc := New(Deps{UCI: &uci.FailingSectionsUCI{MockUCI: uci.NewMockUCI()}, Ubus: ubus.NewMockUbus(), Reloader: &NoopWifiReloader{}})

	name, err := svc.nextSTASectionName()
	if err == nil {
		t.Fatal("expected an error when the wireless sections cannot be read")
	}
	if name != "" {
		t.Errorf("expected no fallback name, got %q (writing it would hijack an existing section)", name)
	}
}
