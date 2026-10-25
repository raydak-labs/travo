package wifi

import (
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

func TestDeriveWifiMode_ReadsFromModeFile(t *testing.T) {
	tmpFile := t.TempDir() + "/wifi-mode"
	svc, _ := newTestWifiServiceWithModeFile(tmpFile)

	// Both STA and AP are enabled in mock UCI, so UCI-only detection would return "repeater".
	// After SetMode("client"), the mode file should make deriveWifiMode return "client".
	_, err := svc.SetMode("client", LockoutRequest{})
	if err != nil {
		t.Fatalf("SetMode error: %v", err)
	}

	mode := svc.deriveWifiMode()
	if mode != "client" {
		t.Errorf("expected deriveWifiMode to return 'client' from mode file, got %q", mode)
	}
}

func TestDeriveWifiMode_FallsBackToUCIWhenNoModeFile(t *testing.T) {
	// No mode file — uses UCI detection.
	svc, _ := newTestWifiServiceWithModeFile("")

	mode := svc.deriveWifiMode()
	// Mock UCI has both ap and sta sections enabled, so UCI detection returns "repeater".
	if mode != "repeater" && mode != "ap" && mode != "client" {
		t.Errorf("unexpected mode %q from UCI fallback", mode)
	}
}

// Finding 4: ensureNamedSection must correct a wrong section type.
func TestEnsureNamedSection_FixesWrongType(t *testing.T) {
	svc, u := newTestWifiService()

	// A section that already exists with the wrong type.
	if err := u.AddSection("wireless", "guest", "wifi-device"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ensureNamedSection("wireless", "guest", "wifi-iface"); err != nil {
		t.Fatalf("ensureNamedSection: %v", err)
	}
	opts, err := u.GetAll("wireless", "guest")
	if err != nil {
		t.Fatal(err)
	}
	if opts[".type"] != "wifi-iface" {
		t.Errorf("expected the section type to be corrected to wifi-iface, got %q", opts[".type"])
	}
}

func TestEnsureNamedSection_CreatesMissingSection(t *testing.T) {
	svc, u := newTestWifiService()

	if err := svc.ensureNamedSection("dhcp", "guest", "dhcp"); err != nil {
		t.Fatalf("ensureNamedSection: %v", err)
	}
	opts, err := u.GetAll("dhcp", "guest")
	if err != nil {
		t.Fatalf("expected the section to be created: %v", err)
	}
	if opts[".type"] != "dhcp" {
		t.Errorf("expected type dhcp, got %q", opts[".type"])
	}
}

// A read failure must not be mistaken for a missing section.
func TestEnsureNamedSection_PropagatesReadFailure(t *testing.T) {
	u := uci.NewMockUCI()
	svc := New(Deps{UCI: &uci.FailingSectionsUCI{MockUCI: u}, Ubus: ubus.NewMockUbus(), Reloader: &NoopWifiReloader{}})

	if err := svc.ensureNamedSection("wireless", "guest", "wifi-iface"); err == nil {
		t.Fatal("expected the read failure to surface instead of silently creating a section")
	}
}
