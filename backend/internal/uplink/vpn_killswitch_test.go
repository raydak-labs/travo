package uplink

import (
	"errors"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/platform"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

func TestGetKillSwitch_DisabledByDefault(t *testing.T) {
	u := uci.NewMockUCI()
	svc := NewVpnService(VpnDeps{UCI: u})
	ks, err := svc.GetKillSwitch()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ks.Enabled {
		t.Error("expected kill switch disabled by default")
	}
}

func TestSetKillSwitch_Enable(t *testing.T) {
	u := uci.NewMockUCI()
	svc := NewVpnService(VpnDeps{UCI: u})
	if err := svc.SetKillSwitch(true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ks, err := svc.GetKillSwitch()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ks.Enabled {
		t.Error("expected kill switch enabled")
	}
}

func TestSetKillSwitch_Disable(t *testing.T) {
	u := uci.NewMockUCI()
	svc := NewVpnService(VpnDeps{UCI: u})
	// Enable first, then disable
	_ = svc.SetKillSwitch(true)
	if err := svc.SetKillSwitch(false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ks, err := svc.GetKillSwitch()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ks.Enabled {
		t.Error("expected kill switch disabled after disabling")
	}
}

func TestSetKillSwitch_PropagatesAddSectionError(t *testing.T) {
	base := uci.NewMockUCI()
	fu := &failingUCI{UCI: base, addSectionErr: errors.New("uci readonly")}
	svc := NewVpnService(VpnDeps{UCI: fu, Cmd: &platform.MockCommandRunner{}})

	if err := svc.SetKillSwitch(true); err == nil {
		t.Fatal("expected SetKillSwitch to fail when the rule cannot be created")
	}
	if _, err := base.GetAll("firewall", "vpn_killswitch"); err == nil {
		t.Error("must not create a partial kill switch rule")
	}
}

func TestSetKillSwitch_PropagatesSetErrorWithoutCommit(t *testing.T) {
	base := uci.NewMockUCI()
	fu := &failingUCI{UCI: base, setErr: map[string]error{
		"firewall.vpn_killswitch.target": errors.New("uci set failed"),
	}}
	svc := NewVpnService(VpnDeps{UCI: fu, Cmd: &platform.MockCommandRunner{}})

	if err := svc.SetKillSwitch(true); err == nil {
		t.Fatal("expected SetKillSwitch to fail when an option cannot be written")
	}
	if fu.commits != 0 {
		t.Errorf("must not commit a partial kill switch rule, commits = %d", fu.commits)
	}
}

func TestSetKillSwitch_MarksUserOwnership(t *testing.T) {
	u := uci.NewMockUCI()
	svc := NewVpnService(VpnDeps{UCI: u, Cmd: &platform.MockCommandRunner{}})

	if err := svc.SetKillSwitch(true); err != nil {
		t.Fatalf("SetKillSwitch(true): %v", err)
	}
	opts, err := u.GetAll("firewall", "vpn_killswitch")
	if err != nil {
		t.Fatalf("expected kill switch rule: %v", err)
	}
	if opts["target"] != "REJECT" {
		t.Errorf("expected target=REJECT, got %q", opts["target"])
	}
	if opts[vpnKillSwitchOwnerOption] != vpnKillSwitchOwnerUser {
		t.Errorf("expected %s=%s, got %q", vpnKillSwitchOwnerOption, vpnKillSwitchOwnerUser, opts[vpnKillSwitchOwnerOption])
	}
}

func TestSetKillSwitch_DisableWithoutRuleIsNoop(t *testing.T) {
	u := uci.NewMockUCI()
	svc := NewVpnService(VpnDeps{UCI: u, Cmd: &platform.MockCommandRunner{}})

	if err := svc.SetKillSwitch(false); err != nil {
		t.Fatalf("SetKillSwitch(false) on a clean router: %v", err)
	}
}
