package system

import (
	"strings"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/wifi"
)

func TestValidateButtonName(t *testing.T) {
	discovered := []string{"POWER", "volume_up", "reset-factory"}
	cases := []struct {
		name    string
		dis     []string
		wantErr bool
	}{
		{"POWER", discovered, false},
		{"volume_up", discovered, false},
		{"reset-factory", discovered, false},
		{"", discovered, true},
		{"NOT_A_BUTTON", discovered, true},
		{"POWER; reboot", discovered, true},
		{"POWER)\n  reboot)\n    reboot\n  x)", discovered, true},
		{"POWER\n", discovered, true},
		{"POWER", nil, false}, // no devicetree labels: charset check only
		{"POWER; reboot", nil, true},
	}
	for _, tc := range cases {
		err := ValidateButtonName(tc.name, tc.dis)
		if (err != nil) != tc.wantErr {
			t.Errorf("ValidateButtonName(%q, %v) err=%v, wantErr=%v", tc.name, tc.dis, err, tc.wantErr)
		}
	}
}

func TestAddSSHKeyRejectsMultilineAndGarbage(t *testing.T) {
	s := New(nil, nil, nil)
	if err := s.AddSSHKey("ssh-ed25519 AAAA user@host\nssh-rsa AAAA evil@host"); err == nil {
		t.Error("expected multi-line key to be rejected")
	}
	if err := s.AddSSHKey("not a key"); err == nil {
		t.Error("expected malformed key to be rejected")
	}
}

func TestBuildButtonHotplugScriptHasNoWifiCommands(t *testing.T) {
	script := buildButtonHotplugScript([]models.HardwareButton{
		{Name: "POWER", Action: models.ButtonActionWifiToggle},
		{Name: "volume_up", Action: models.ButtonActionVPNToggle},
	})
	for _, forbidden := range []string{"wifi up", "wifi down", "wifi reload"} {
		if strings.Contains(script, forbidden) {
			t.Errorf("hotplug script must not contain %q:\n%s", forbidden, script)
		}
	}
	if !strings.Contains(script, wifi.WirelessToggleScriptPath) {
		t.Errorf("hotplug script should delegate to the toggle helper:\n%s", script)
	}
}
