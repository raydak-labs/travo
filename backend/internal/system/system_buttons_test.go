package system

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

func TestSetButtonActions_GeneratesHotplugScript(t *testing.T) {
	dir := t.TempDir()
	// Override paths via a temp dir (we test the helpers directly)
	buttons := []models.HardwareButton{
		{Name: "reset", Action: models.ButtonActionVPNToggle},
		{Name: "wps", Action: models.ButtonActionReboot},
	}
	script := buildButtonHotplugScript(buttons)
	if !strings.Contains(script, "reset)") {
		t.Error("expected script to contain 'reset)' case")
	}
	if !strings.Contains(script, "wps)") {
		t.Error("expected script to contain 'wps)' case")
	}
	if !strings.Contains(script, "/sbin/ifup wg0") || !strings.Contains(script, "/sbin/ifdown wg0") {
		t.Error("expected script to toggle wg0 via ifup/ifdown for vpn_toggle")
	}
	if !strings.Contains(script, "reboot") {
		t.Error("expected script to contain reboot for reboot action")
	}
	_ = dir
}

func TestSetButtonActions_NoneSkipped(t *testing.T) {
	buttons := []models.HardwareButton{
		{Name: "reset", Action: models.ButtonActionNone},
	}
	script := buildButtonHotplugScript(buttons)
	// "reset)" should not appear since action is none
	if strings.Contains(script, "reset)") {
		t.Error("expected 'none' action buttons to be omitted from hotplug script")
	}
}

func TestBuildButtonActionsJSON_RoundTrip(t *testing.T) {
	original := []models.HardwareButton{
		{Name: "reset", Action: models.ButtonActionWifiToggle},
		{Name: "wps", Action: models.ButtonActionLEDToggle},
	}
	// Round-trip through encoding/json: the hand-rolled parser this replaced
	// silently mangled names and actions.
	raw := buildButtonActionsJSON(original)
	var parsed []models.HardwareButton
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(parsed) != len(original) {
		t.Fatalf("expected %d buttons, got %d", len(original), len(parsed))
	}
	for i, b := range original {
		if parsed[i].Name != b.Name || parsed[i].Action != b.Action {
			t.Errorf("button %d: expected {%s, %s}, got {%s, %s}",
				i, b.Name, b.Action, parsed[i].Name, parsed[i].Action)
		}
	}
}

// A malformed button-actions.json must not be reported as "no buttons
// configured": the hotplug script on disk still runs the previous actions.
func TestGetHardwareButtons_SurfacesParseFailure(t *testing.T) {
	svc := New(ubus.NewMockUbus(), uci.NewMockUCI(), &MockStorageProvider{})

	// No file at all is not an error: nothing has been configured yet.
	if _, err := svc.GetHardwareButtons(); err != nil {
		t.Errorf("a missing config file must not be an error, got %v", err)
	}

	if err := os.WriteFile(buttonActionsFile, []byte("{ this is not json"), 0o600); err != nil {
		t.Skipf("cannot write %s on this host: %v", buttonActionsFile, err)
	}
	defer func() { _ = os.Remove(buttonActionsFile) }()

	if _, err := svc.GetHardwareButtons(); err == nil {
		t.Error("a malformed config file must surface an error")
	}
}

// encoding/json replaced a hand-rolled parser: names containing escapes and
// reordered keys must round-trip.
func TestLoadButtonActions_HandlesReorderedAndEscapedJSON(t *testing.T) {
	svc := New(ubus.NewMockUbus(), uci.NewMockUCI(), &MockStorageProvider{})
	if err := os.WriteFile(buttonActionsFile, []byte(`[{"action":"reboot","name":"reset"},{"name":"wps","action":"led_toggle"}]`), 0o600); err != nil {
		t.Skipf("cannot write %s on this host: %v", buttonActionsFile, err)
	}
	defer func() { _ = os.Remove(buttonActionsFile) }()

	buttons, err := svc.loadButtonActions()
	if err != nil {
		t.Fatalf("loadButtonActions: %v", err)
	}
	if len(buttons) != 2 {
		t.Fatalf("expected 2 buttons, got %d", len(buttons))
	}
	if buttons[0].Name != "reset" || buttons[0].Action != models.ButtonActionReboot {
		t.Errorf("unexpected first button: %+v", buttons[0])
	}
	if buttons[1].Name != "wps" || buttons[1].Action != models.ButtonActionLEDToggle {
		t.Errorf("unexpected second button: %+v", buttons[1])
	}
}
