package uplink

import (
	"errors"
	"slices"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/platform"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

func TestSetSplitTunnel_CustomWithoutRoutesIsRejected(t *testing.T) {
	u := uci.NewMockUCI()
	_ = u.AddSection("network", "wg0_peer0", "wireguard_wg0")
	_ = u.Set("network", "wg0_peer0", "allowed_ips", "10.0.0.0/24")
	svc := NewVpnService(VpnDeps{UCI: u, Cmd: &platform.MockCommandRunner{}})

	err := svc.SetSplitTunnel(models.SplitTunnelConfig{Mode: "custom"})
	if err == nil {
		t.Fatal("expected a validation error for custom split tunnel with no routes")
	}
	allowed, _ := u.Get("network", "wg0_peer0", "allowed_ips")
	if allowed != "10.0.0.0/24" {
		t.Errorf("peer allowed_ips must be untouched, got %q", allowed)
	}
}

func TestSetSplitTunnel_UnknownModeIsRejected(t *testing.T) {
	u := uci.NewMockUCI()
	svc := NewVpnService(VpnDeps{UCI: u, Cmd: &platform.MockCommandRunner{}})
	if err := svc.SetSplitTunnel(models.SplitTunnelConfig{Mode: "sideways"}); err == nil {
		t.Fatal("expected a validation error for an unknown split tunnel mode")
	}
}

func TestSplitTunnelAllowedIPs(t *testing.T) {
	t.Parallel()

	got, err := splitTunnelAllowedIPs(models.SplitTunnelConfig{Mode: "all"})
	if err != nil {
		t.Fatalf("mode all: %v", err)
	}
	if !slices.Equal(got, []string{"0.0.0.0/0", "::/0"}) {
		t.Errorf("mode all = %v", got)
	}

	got, err = splitTunnelAllowedIPs(models.SplitTunnelConfig{Mode: "custom", Routes: []string{" 10.0.0.0/8 ", ""}})
	if err != nil {
		t.Fatalf("mode custom: %v", err)
	}
	if !slices.Equal(got, []string{"10.0.0.0/8"}) {
		t.Errorf("mode custom = %v", got)
	}

	if _, err := splitTunnelAllowedIPs(models.SplitTunnelConfig{Mode: "custom", Routes: []string{"  "}}); err == nil {
		t.Error("expected an error for custom mode with only blank routes")
	}
}

func TestSetSplitTunnel_RevertsStagedDeltasOnWriteError(t *testing.T) {
	base := uci.NewMockUCI()
	_ = base.AddSection("network", "wg0_peer0", "wireguard_wg0")
	_ = base.Set("network", "wg0_peer0", "allowed_ips", "10.0.0.0/24")
	_ = base.AddSection("network", "wg0_peer1", "wireguard_wg0")
	_ = base.Set("network", "wg0_peer1", "allowed_ips", "192.168.0.0/16")
	fu := &failingUCI{UCI: base, addListErrOnce: map[string]error{
		"network.wg0_peer1.allowed_ips": errors.New("uci add_list failed"),
	}}
	svc := NewVpnService(VpnDeps{UCI: fu, Cmd: &platform.MockCommandRunner{}})

	err := svc.SetSplitTunnel(models.SplitTunnelConfig{Mode: "custom", Routes: []string{"0.0.0.0/0"}})
	if err == nil {
		t.Fatal("expected SetSplitTunnel to fail when a peer cannot be written")
	}
	peer0, _ := base.Get("network", "wg0_peer0", "allowed_ips")
	if peer0 != "10.0.0.0/24" {
		t.Errorf("expected wg0_peer0 allowed_ips reverted to 10.0.0.0/24, got %q", peer0)
	}
	peer1, _ := base.Get("network", "wg0_peer1", "allowed_ips")
	if peer1 != "192.168.0.0/16" {
		t.Errorf("expected wg0_peer1 allowed_ips reverted to 192.168.0.0/16, got %q", peer1)
	}
}
