package services

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

func mockRunWireGuardEnableOK(name string, args ...string) ([]byte, error) {
	switch name {
	case "tailscale":
		return nil, nil
	case "/etc/init.d/firewall":
		return nil, nil
	case "/sbin/ubus":
		return []byte("{}"), nil
	case "/sbin/ifup", "/sbin/ifdown":
		return nil, nil
	case "/sbin/ip":
		if len(args) >= 4 && args[0] == "link" && args[1] == "show" && args[2] == "dev" && args[3] == "wg0" {
			return []byte("3: wg0: <POINTOPOINT,NOARP,UP,LOWER_UP> mtu 1420 qdisc noqueue state UNKNOWN"), nil
		}
	case "/usr/bin/wg":
		return []byte("PRIV\tPUB\t51820\toff\n"), nil
	}
	return nil, fmt.Errorf("unexpected command: %s %s", name, strings.Join(args, " "))
}

func TestGetVpnStatus(t *testing.T) {
	u := uci.NewMockUCI()
	svc := NewVpnService(u)

	statuses, err := svc.GetVpnStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(statuses) < 2 {
		t.Errorf("expected at least 2 statuses, got %d", len(statuses))
	}

	found := false
	for _, s := range statuses {
		if s.Type == "wireguard" {
			found = true
			// wg0 is disabled=1 in mock, so should not be enabled
			if s.Enabled {
				t.Error("expected wireguard not enabled (disabled=1 in mock)")
			}
		}
	}
	if !found {
		t.Error("expected wireguard status")
	}
}

func TestGetWireguardConfig(t *testing.T) {
	u := uci.NewMockUCI()
	svc := NewVpnService(u)

	config, err := svc.GetWireguardConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.PrivateKey == "" {
		t.Error("expected non-empty private key")
	}
	if config.Address == "" {
		t.Error("expected non-empty address")
	}
	if len(config.Peers) == 0 {
		t.Error("expected at least one peer")
	}
}

func TestToggleWireguard(t *testing.T) {
	u := uci.NewMockUCI()
	cmd := &MockCommandRunner{RunFunc: mockRunWireGuardEnableOK}
	svc := NewVpnServiceWithRunner(u, cmd)

	err := svc.ToggleWireguard(true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val, _ := u.Get("network", "wg0", "disabled")
	if val != "0" {
		t.Errorf("expected disabled=0, got %q", val)
	}
}

func TestToggleWireguard_Disable(t *testing.T) {
	u := uci.NewMockUCI()
	var calls []string
	kernelDefault := false
	cmd := &MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		calls = append(calls, name+" "+strings.Join(args, " "))
		if name == "/sbin/ip" && len(args) >= 3 && args[0] == "route" && args[1] == "show" && args[2] == "default" {
			if kernelDefault {
				return []byte("default via 10.0.1.1 dev phy1-sta0"), nil
			}
			return []byte(""), nil
		}
		if name == "/sbin/ubus" && len(args) >= 4 && args[0] == "-S" && args[1] == "call" && args[2] == "network.interface" && args[3] == "dump" {
			return []byte(`{"interface":[{"interface":"wwan","up":true,"route":[{"target":"0.0.0.0","mask":0,"nexthop":"10.0.1.1"}]}]}`), nil
		}
		if name == "/sbin/ubus" && len(args) >= 3 && args[0] == "call" && args[1] == "network.interface.wwan" && args[2] == "renew" {
			// After a renew + reload we assume default route comes back.
			kernelDefault = true
			return []byte("{}"), nil
		}
		return nil, nil
	}}
	svc := NewVpnServiceWithRunner(u, cmd)

	err := svc.ToggleWireguard(false)
	if err != nil {
		t.Fatalf("unexpected error disabling: %v", err)
	}
	val, _ := u.Get("network", "wg0", "disabled")
	if val != "1" {
		t.Errorf("expected disabled=1, got %q", val)
	}
	joined := strings.Join(calls, "\n")
	if !strings.Contains(joined, "/sbin/ifdown wg0") {
		t.Fatalf("expected /sbin/ifdown wg0 to be called, calls:\n%s", joined)
	}
	if !strings.Contains(joined, "/sbin/ubus call network reload") {
		t.Fatalf("expected /sbin/ubus call network reload to be called, calls:\n%s", joined)
	}
	// Ensure we reload network after ifdown (helps restore routes/DNS state).
	ifIdx := strings.Index(joined, "/sbin/ifdown wg0")
	ubusIdx := strings.Index(joined, "/sbin/ubus call network reload")
	if ifIdx < 0 || ubusIdx < 0 || ubusIdx < ifIdx {
		t.Fatalf("expected ifdown before ubus reload, calls:\n%s", joined)
	}
	// Netifd-managed recovery should attempt to bring WAN back up.
	if !strings.Contains(joined, "/sbin/ip route show default") {
		t.Fatalf("expected default route check, calls:\n%s", joined)
	}
	if !strings.Contains(joined, "/sbin/ubus -S call network.interface dump") {
		t.Fatalf("expected uplink discovery via ubus dump, calls:\n%s", joined)
	}
	if !strings.Contains(joined, "/sbin/ubus call network.interface.wwan renew") {
		t.Fatalf("expected wwan renew attempt, calls:\n%s", joined)
	}
}

func TestToggleWireguard_FailsWhenTunnelNotUp(t *testing.T) {
	prev := wireGuardVerifyTimeout
	wireGuardVerifyTimeout = 400 * time.Millisecond
	t.Cleanup(func() { wireGuardVerifyTimeout = prev })

	u := uci.NewMockUCI()
	cmd := &MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		switch name {
		case "tailscale", "/sbin/ubus", "/sbin/ifup", "/etc/init.d/firewall":
			return nil, nil
		case "/usr/bin/wg":
			return nil, fmt.Errorf("no interface")
		case "/sbin/ip":
			return []byte("3: wg0: state DOWN"), nil
		default:
			return nil, nil
		}
	}}
	svc := NewVpnServiceWithRunner(u, cmd)

	err := svc.ToggleWireguard(true)
	if err == nil {
		t.Fatal("expected error when wg0 does not come up")
	}
}

func TestToggleWireguard_FailsPreflightWhenMissingPrivateKey(t *testing.T) {
	u := uci.NewMockUCI()
	_ = u.Set("network", "wg0", "private_key", "")
	called := 0
	cmd := &MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		called++
		// tailscale is best-effort and may still be invoked before preflight validation.
		if name == "tailscale" || strings.HasSuffix(name, "/tailscale") {
			return nil, nil
		}
		return nil, fmt.Errorf("unexpected command during preflight failure: %s %v", name, args)
	}}
	svc := NewVpnServiceWithRunner(u, cmd)

	err := svc.ToggleWireguard(true)
	if err == nil {
		t.Fatal("expected error when wg0 private_key is missing")
	}
	val, _ := u.Get("network", "wg0", "disabled")
	if val != "1" {
		t.Errorf("expected disabled to remain '1' on preflight failure, got %q", val)
	}
	_ = called
}

func TestToggleWireguard_FailsPreflightWhenMissingPeerPublicKey(t *testing.T) {
	u := uci.NewMockUCI()
	_ = u.Set("network", "wg0_peer0", "public_key", "")
	cmd := &MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		if name == "tailscale" || strings.HasSuffix(name, "/tailscale") {
			return nil, nil
		}
		return nil, fmt.Errorf("unexpected command during preflight failure: %s %v", name, args)
	}}
	svc := NewVpnServiceWithRunner(u, cmd)

	err := svc.ToggleWireguard(true)
	if err == nil {
		t.Fatal("expected error when peer public_key is missing")
	}
	val, _ := u.Get("network", "wg0", "disabled")
	if val != "1" {
		t.Errorf("expected disabled to remain '1' on preflight failure, got %q", val)
	}
}

func TestToggleWireguard_FailsPreflightWhenEndpointPortInvalid(t *testing.T) {
	u := uci.NewMockUCI()
	_ = u.Set("network", "wg0_peer0", "endpoint_port", "abc")
	cmd := &MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		if name == "tailscale" || strings.HasSuffix(name, "/tailscale") {
			return nil, nil
		}
		return nil, fmt.Errorf("unexpected command during preflight failure: %s %v", name, args)
	}}
	svc := NewVpnServiceWithRunner(u, cmd)

	err := svc.ToggleWireguard(true)
	if err == nil {
		t.Fatal("expected error when endpoint port is invalid")
	}
	val, _ := u.Get("network", "wg0", "disabled")
	if val != "1" {
		t.Errorf("expected disabled to remain '1' on preflight failure, got %q", val)
	}
}

func TestToggleWireguard_FailsPreflightWhenRouteAllowedIPsNotOne(t *testing.T) {
	u := uci.NewMockUCI()
	_ = u.Set("network", "wg0_peer0", "route_allowed_ips", "0")
	cmd := &MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		if name == "tailscale" || strings.HasSuffix(name, "/tailscale") {
			return nil, nil
		}
		return nil, fmt.Errorf("unexpected command during preflight failure: %s %v", name, args)
	}}
	svc := NewVpnServiceWithRunner(u, cmd)

	err := svc.ToggleWireguard(true)
	if err == nil {
		t.Fatal("expected error when route_allowed_ips != 1")
	}
	val, _ := u.Get("network", "wg0", "disabled")
	if val != "1" {
		t.Errorf("expected disabled to remain '1' on preflight failure, got %q", val)
	}
}

func TestToggleWireguard_SetsProtoWireguard(t *testing.T) {
	u := uci.NewMockUCI()
	cmd := &MockCommandRunner{RunFunc: mockRunWireGuardEnableOK}
	svc := NewVpnServiceWithRunner(u, cmd)

	if err := svc.ToggleWireguard(true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	proto, _ := u.Get("network", "wg0", "proto")
	if proto != "wireguard" {
		t.Errorf("expected proto=wireguard, got %q", proto)
	}
}

func TestImportWireguardConfig_NormalizesPeerSections(t *testing.T) {
	u := uci.NewMockUCI()
	svc := NewVpnService(u)

	conf := `[Interface]
PrivateKey = abc123
Address = 10.66.0.2/32
DNS = 10.66.0.1

[Peer]
PublicKey = peerkey
Endpoint = vpn.example.com:51820
AllowedIPs = 0.0.0.0/0
`
	if err := svc.ImportWireguardConfig(conf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// wg0 must have proto=wireguard
	proto, _ := u.Get("network", "wg0", "proto")
	if proto != "wireguard" {
		t.Errorf("expected proto=wireguard after import, got %q", proto)
	}
	// peer section must exist with public_key
	pk, _ := u.Get("network", "wg0_peer0", "public_key")
	if pk != "peerkey" {
		t.Errorf("expected peer public_key peerkey, got %q", pk)
	}
	host, _ := u.Get("network", "wg0_peer0", "endpoint_host")
	if host != "vpn.example.com" {
		t.Errorf("expected endpoint_host vpn.example.com, got %q", host)
	}
	port, _ := u.Get("network", "wg0_peer0", "endpoint_port")
	if port != "51820" {
		t.Errorf("expected endpoint_port 51820, got %q", port)
	}
	ra, _ := u.Get("network", "wg0_peer0", "route_allowed_ips")
	if ra != "1" {
		t.Errorf("expected route_allowed_ips 1, got %q", ra)
	}
}

func TestWgRuntimeState_Disabled(t *testing.T) {
	u := uci.NewMockUCI()
	cmd := &MockCommandRunner{Err: fmt.Errorf("exit status 1")}
	svc := NewVpnServiceWithRunner(u, cmd)
	state := svc.wgRuntimeState(false)
	if state != "disabled" {
		t.Errorf("expected 'disabled', got %q", state)
	}
}

func TestWgRuntimeState_EnabledNotUp(t *testing.T) {
	u := uci.NewMockUCI()
	cmd := &MockCommandRunner{Err: fmt.Errorf("exit status 1")}
	svc := NewVpnServiceWithRunner(u, cmd)
	state := svc.wgRuntimeState(true)
	if state != "enabled_not_up" {
		t.Errorf("expected 'enabled_not_up', got %q", state)
	}
}

func TestWgRuntimeState_Connected(t *testing.T) {
	u := uci.NewMockUCI()
	dump := "PRIV\tPUB\t51820\toff\n" +
		"peerpub\t(none)\tvpn.example.com:51820\t0.0.0.0/0\t1740000000\t100\t200\t0\n"
	cmd := &MockCommandRunner{Output: []byte(dump)}
	svc := NewVpnServiceWithRunner(u, cmd)
	state := svc.wgRuntimeState(true)
	if state != "connected" {
		t.Errorf("expected 'connected', got %q", state)
	}
}

func TestGetTailscaleStatus(t *testing.T) {
	u := uci.NewMockUCI()
	cmd := &MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		if name == "which" && len(args) == 1 && args[0] == "tailscale" {
			return nil, fmt.Errorf("not found")
		}
		return nil, fmt.Errorf("unexpected command %s", name)
	}}
	svc := NewVpnServiceWithRunner(u, cmd)

	status, err := svc.GetTailscaleStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Test that the function runs without error and returns a valid status structure.
	// The installed status depends on the actual system environment.
	if status.Installed && status.Running {
		// If tailscale is installed and running, verify the status structure is populated correctly
		if status.LoggedIn {
			// When logged in, we should get peers
			if status.Peers == nil {
				t.Error("expected peers slice to be initialized")
			}
		}
	}
	// Test that we can call the function without panicking
	_ = status
}

func TestGetKillSwitch_DisabledByDefault(t *testing.T) {
	u := uci.NewMockUCI()
	svc := NewVpnService(u)
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
	svc := NewVpnService(u)
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
	svc := NewVpnService(u)
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

func TestGetWireGuardStatus_Success(t *testing.T) {
	u := uci.NewMockUCI()
	dump := "PRIVATE_KEY\tPUBLIC_KEY_IFACE\t51820\toff\n" +
		"PEER_PUB_KEY\t(none)\t1.2.3.4:51820\t0.0.0.0/0\t1710000000\t123456789\t987654321\toff\n"
	cmd := &MockCommandRunner{Output: []byte(dump)}
	svc := NewVpnServiceWithRunner(u, cmd)

	status, err := svc.GetWireGuardStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Interface != "wg0" {
		t.Errorf("expected interface wg0, got %q", status.Interface)
	}
	if status.PublicKey != "PUBLIC_KEY_IFACE" {
		t.Errorf("expected public key PUBLIC_KEY_IFACE, got %q", status.PublicKey)
	}
	if status.ListenPort != 51820 {
		t.Errorf("expected listen port 51820, got %d", status.ListenPort)
	}
	if len(status.Peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(status.Peers))
	}
	peer := status.Peers[0]
	if peer.PublicKey != "PEER_PUB_KEY" {
		t.Errorf("expected peer key PEER_PUB_KEY, got %q", peer.PublicKey)
	}
	if peer.Endpoint != "1.2.3.4:51820" {
		t.Errorf("expected endpoint 1.2.3.4:51820, got %q", peer.Endpoint)
	}
	if peer.LatestHandshake != 1710000000 {
		t.Errorf("expected handshake 1710000000, got %d", peer.LatestHandshake)
	}
	if peer.TransferRx != 123456789 {
		t.Errorf("expected rx 123456789, got %d", peer.TransferRx)
	}
	if peer.TransferTx != 987654321 {
		t.Errorf("expected tx 987654321, got %d", peer.TransferTx)
	}
	if peer.AllowedIPs != "0.0.0.0/0" {
		t.Errorf("expected allowed ips 0.0.0.0/0, got %q", peer.AllowedIPs)
	}
}

func TestGetWireGuardStatus_MultiplePeers(t *testing.T) {
	dump := "PRIV\tPUB\t51820\toff\n" +
		"PEER1\t(none)\t1.2.3.4:51820\t0.0.0.0/0\t1710000000\t100\t200\toff\n" +
		"PEER2\t(none)\t5.6.7.8:51821\t10.0.0.0/24\t1710000060\t300\t400\t25\n"
	status, err := ParseWgDump(dump)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(status.Peers) != 2 {
		t.Fatalf("expected 2 peers, got %d", len(status.Peers))
	}
	if status.Peers[1].Endpoint != "5.6.7.8:51821" {
		t.Errorf("expected second peer endpoint 5.6.7.8:51821, got %q", status.Peers[1].Endpoint)
	}
}

func TestGetWireGuardStatus_EmptyOutput(t *testing.T) {
	_, err := ParseWgDump("")
	if err == nil {
		t.Error("expected error for empty dump")
	}
}

func TestGetWireGuardStatus_CommandError(t *testing.T) {
	u := uci.NewMockUCI()
	cmd := &MockCommandRunner{Err: fmt.Errorf("exit status 1")}
	svc := NewVpnServiceWithRunner(u, cmd)

	status, err := svc.GetWireGuardStatus()
	if err != nil {
		t.Errorf("expected no error when wg command fails (tunnel not active), got: %v", err)
	}
	if status == nil {
		t.Fatal("expected empty status, got nil")
	}
	if status.PublicKey != "" || status.ListenPort != 0 || len(status.Peers) != 0 {
		t.Errorf("expected empty runtime status when tunnel not active, got: %+v", status)
	}
	if status.Interface != "wg0" {
		t.Errorf("expected interface wg0 placeholder, got %q", status.Interface)
	}
}

func TestParseWgDump_NoHandshake(t *testing.T) {
	dump := "PRIV\tPUB\t51820\toff\n" +
		"PEER1\t(none)\t1.2.3.4:51820\t0.0.0.0/0\t0\t0\t0\toff\n"
	status, err := ParseWgDump(dump)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Peers[0].LatestHandshake != 0 {
		t.Errorf("expected handshake 0, got %d", status.Peers[0].LatestHandshake)
	}
}

func newTestVpnService(t *testing.T) (*VpnService, string) {
	t.Helper()
	dir := t.TempDir()
	profilesPath := filepath.Join(dir, "wireguard_profiles.json")
	u := uci.NewMockUCI()
	cmd := &MockCommandRunner{Output: []byte("PRIV\tPUB\t51820\toff\n")}
	svc := NewVpnServiceWithProfilesPath(u, cmd, profilesPath)
	return svc, profilesPath
}

func TestGetProfiles_EmptyByDefault(t *testing.T) {
	svc, _ := newTestVpnService(t)
	profiles, err := svc.GetProfiles()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(profiles) != 0 {
		t.Errorf("expected 0 profiles, got %d", len(profiles))
	}
}

func TestAddProfile(t *testing.T) {
	svc, _ := newTestVpnService(t)

	conf := "[Interface]\nPrivateKey = dGVzdHByaXZhdGVrZXkxMjM0NTY3ODkwMTIzNDU2\nAddress = 10.0.0.2/32\n\n[Peer]\nPublicKey = dGVzdHB1YmxpY2tleTEyMzQ1Njc4OTAxMjM0NTY=\nEndpoint = vpn.example.com:51820\nAllowedIPs = 0.0.0.0/0\n"
	profile, err := svc.AddProfile("Test VPN", conf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile.Name != "Test VPN" {
		t.Errorf("expected name 'Test VPN', got %q", profile.Name)
	}
	if profile.ID == "" {
		t.Error("expected non-empty ID")
	}
	if profile.Active {
		t.Error("expected new profile to not be active")
	}

	profiles, _ := svc.GetProfiles()
	if len(profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(profiles))
	}
}

func TestAddProfile_InvalidConfig(t *testing.T) {
	svc, _ := newTestVpnService(t)
	_, err := svc.AddProfile("Bad", "not a valid config")
	if err == nil {
		t.Error("expected error for invalid config")
	}
}

func TestDeleteProfile(t *testing.T) {
	svc, _ := newTestVpnService(t)

	conf := "[Interface]\nPrivateKey = dGVzdHByaXZhdGVrZXkxMjM0NTY3ODkwMTIzNDU2\nAddress = 10.0.0.2/32\n\n[Peer]\nPublicKey = dGVzdHB1YmxpY2tleTEyMzQ1Njc4OTAxMjM0NTY=\nEndpoint = vpn.example.com:51820\nAllowedIPs = 0.0.0.0/0\n"
	profile, _ := svc.AddProfile("Test VPN", conf)

	err := svc.DeleteProfile(profile.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	profiles, _ := svc.GetProfiles()
	if len(profiles) != 0 {
		t.Errorf("expected 0 profiles after delete, got %d", len(profiles))
	}
}

func TestDeleteProfile_NotFound(t *testing.T) {
	svc, _ := newTestVpnService(t)
	err := svc.DeleteProfile("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent profile")
	}
}

func TestActivateProfile(t *testing.T) {
	svc, _ := newTestVpnService(t)

	conf := "[Interface]\nPrivateKey = dGVzdHByaXZhdGVrZXkxMjM0NTY3ODkwMTIzNDU2\nAddress = 10.0.0.2/32\n\n[Peer]\nPublicKey = dGVzdHB1YmxpY2tleTEyMzQ1Njc4OTAxMjM0NTY=\nEndpoint = vpn.example.com:51820\nAllowedIPs = 0.0.0.0/0\n"
	p1, _ := svc.AddProfile("VPN 1", conf)
	p2, _ := svc.AddProfile("VPN 2", conf)

	err := svc.ActivateProfile(p1.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	profiles, _ := svc.GetProfiles()
	for _, p := range profiles {
		if p.ID == p1.ID && !p.Active {
			t.Error("expected p1 to be active")
		}
		if p.ID == p2.ID && p.Active {
			t.Error("expected p2 to not be active")
		}
	}

	// Now activate p2
	err = svc.ActivateProfile(p2.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	profiles, _ = svc.GetProfiles()
	for _, p := range profiles {
		if p.ID == p1.ID && p.Active {
			t.Error("expected p1 to not be active after activating p2")
		}
		if p.ID == p2.ID && !p.Active {
			t.Error("expected p2 to be active")
		}
	}
}

func TestActivateProfile_NotFound(t *testing.T) {
	svc, _ := newTestVpnService(t)
	err := svc.ActivateProfile("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent profile")
	}
}

func TestProfilesPersistence(t *testing.T) {
	dir := t.TempDir()
	profilesPath := filepath.Join(dir, "wireguard_profiles.json")
	u := uci.NewMockUCI()
	cmd := &MockCommandRunner{Output: []byte("PRIV\tPUB\t51820\toff\n")}

	svc1 := NewVpnServiceWithProfilesPath(u, cmd, profilesPath)
	conf := "[Interface]\nPrivateKey = dGVzdHByaXZhdGVrZXkxMjM0NTY3ODkwMTIzNDU2\nAddress = 10.0.0.2/32\n\n[Peer]\nPublicKey = dGVzdHB1YmxpY2tleTEyMzQ1Njc4OTAxMjM0NTY=\nEndpoint = vpn.example.com:51820\nAllowedIPs = 0.0.0.0/0\n"
	_, _ = svc1.AddProfile("Persistent", conf)

	// Create a new service instance pointing to the same file
	svc2 := NewVpnServiceWithProfilesPath(u, cmd, profilesPath)
	profiles, err := svc2.GetProfiles()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(profiles) != 1 {
		t.Errorf("expected 1 profile persisted, got %d", len(profiles))
	}
	if profiles[0].Name != "Persistent" {
		t.Errorf("expected name 'Persistent', got %q", profiles[0].Name)
	}
}

func TestProfilesFilePermissions(t *testing.T) {
	svc, profilesPath := newTestVpnService(t)
	conf := "[Interface]\nPrivateKey = dGVzdHByaXZhdGVrZXkxMjM0NTY3ODkwMTIzNDU2\nAddress = 10.0.0.2/32\n\n[Peer]\nPublicKey = dGVzdHB1YmxpY2tleTEyMzQ1Njc4OTAxMjM0NTY=\nEndpoint = vpn.example.com:51820\nAllowedIPs = 0.0.0.0/0\n"
	_, _ = svc.AddProfile("Perm Test", conf)

	info, err := os.Stat(profilesPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Check file is only readable/writable by owner
	if info.Mode().Perm() != 0o600 {
		t.Errorf("expected file mode 0600, got %o", info.Mode().Perm())
	}
}

func TestRunDNSLeakTest_VPNActiveNoLeak(t *testing.T) {
	// Write a temp resolv.conf with a VPN DNS address.
	tmp := t.TempDir()
	resolvConf := filepath.Join(tmp, "resolv.conf")
	if err := os.WriteFile(resolvConf, []byte("nameserver 10.66.0.1\nnameserver 10.66.0.2\n"), 0600); err != nil {
		t.Fatal(err)
	}

	u := uci.NewMockUCI()
	// Set wg0 enabled with matching DNS.
	_ = u.Set("network", "wg0", "disabled", "0")
	_ = u.Set("network", "wg0", "dns", "10.66.0.1")

	svc := NewVpnService(u)

	// Override resolv.conf path by writing a temporary resolv.conf and reading it
	// via readResolvConfNameservers (tested indirectly by checking the helper).
	nameservers := readResolvConfNameserversFromPath(resolvConf)
	if len(nameservers) != 2 || nameservers[0] != "10.66.0.1" {
		t.Fatalf("unexpected nameservers: %v", nameservers)
	}
	_ = svc
}

func TestRunDNSLeakTest_PotentialLeak(t *testing.T) {
	u := uci.NewMockUCI()
	_ = u.Set("network", "wg0", "disabled", "0")
	_ = u.Set("network", "wg0", "dns", "10.66.0.1")
	svc := NewVpnService(u)

	result := svc.RunDNSLeakTest()
	// On test system /etc/resolv.conf likely doesn't contain 10.66.0.1,
	// so potentially_leaking should be true when VPN is active.
	if !result.VPNActive {
		t.Error("expected VPNActive=true")
	}
	if len(result.VPNDNSServers) != 1 || result.VPNDNSServers[0] != "10.66.0.1" {
		t.Errorf("unexpected VPNDNSServers: %v", result.VPNDNSServers)
	}
}

func TestRunDNSLeakTest_VPNDisabled(t *testing.T) {
	u := uci.NewMockUCI()
	// wg0 disabled=1 by default in mock
	svc := NewVpnService(u)
	result := svc.RunDNSLeakTest()
	if result.VPNActive {
		t.Error("expected VPNActive=false when disabled=1")
	}
	if result.PotentiallyLeaking {
		t.Error("expected PotentiallyLeaking=false when VPN not active")
	}
}

func TestReadResolvConfNameservers(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "resolv.conf")
	content := "# Generated by dnsmasq\nnameserver 127.0.0.1\nnameserver 8.8.8.8\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	servers := readResolvConfNameserversFromPath(path)
	if len(servers) != 2 {
		t.Fatalf("expected 2 servers, got %d: %v", len(servers), servers)
	}
	if servers[0] != "127.0.0.1" || servers[1] != "8.8.8.8" {
		t.Errorf("unexpected servers: %v", servers)
	}
}

func TestEffectiveNameserversForMerge(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		resolv    []string
		dnsmasq   []string
		wantFirst string
		wantLen   int
	}{
		{
			name:      "non_loopback_ignores_dnsmasq",
			resolv:    []string{"8.8.8.8"},
			dnsmasq:   []string{"10.66.0.1"},
			wantFirst: "8.8.8.8",
			wantLen:   1,
		},
		{
			name:      "loopback_only_uses_dnsmasq",
			resolv:    []string{"127.0.0.1"},
			dnsmasq:   []string{"10.66.0.1", "10.66.0.2"},
			wantFirst: "10.66.0.1",
			wantLen:   2,
		},
		{
			name:      "loopback_ipv6_only_uses_dnsmasq",
			resolv:    []string{"::1"},
			dnsmasq:   []string{"10.66.0.1"},
			wantFirst: "10.66.0.1",
			wantLen:   1,
		},
		{
			name:      "empty_resolv_uses_dnsmasq",
			resolv:    nil,
			dnsmasq:   []string{"1.1.1.1"},
			wantFirst: "1.1.1.1",
			wantLen:   1,
		},
		{
			name:      "strips_dnsmasq_hash_port",
			resolv:    []string{"127.0.0.1"},
			dnsmasq:   []string{"127.0.0.1#5353"},
			wantFirst: "127.0.0.1",
			wantLen:   1,
		},
		{
			name:      "loopback_fallback_when_no_dnsmasq",
			resolv:    []string{"127.0.0.1"},
			dnsmasq:   nil,
			wantFirst: "127.0.0.1",
			wantLen:   1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := effectiveNameserversForMerge(tt.resolv, tt.dnsmasq)
			if len(got) != tt.wantLen {
				t.Fatalf("len=%d want %d: %v", len(got), tt.wantLen, got)
			}
			if tt.wantLen > 0 && got[0] != tt.wantFirst {
				t.Errorf("first=%q want %q", got[0], tt.wantFirst)
			}
		})
	}
}

func TestRunDNSLeakTest_OpenWrtLoopbackDnsmasqMatchesVPN(t *testing.T) {
	u := uci.NewMockUCI()
	_ = u.Set("network", "wg0", "disabled", "0")
	_ = u.Set("network", "wg0", "dns", "10.66.0.1")

	cmd := &MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		if name == "uci" && len(args) >= 2 && args[0] == "get" && args[1] == "dhcp.@dnsmasq[0].server" {
			return []byte("10.66.0.1\n"), nil
		}
		return nil, fmt.Errorf("unexpected command")
	}}
	svc := NewVpnServiceWithRunner(u, cmd)

	// Simulate OpenWrt: resolv only lists local stub; upstream is dnsmasq server=.
	old := readResolvConfNameservers
	readResolvConfNameservers = func() []string {
		return []string{"127.0.0.1"}
	}
	t.Cleanup(func() { readResolvConfNameservers = old })

	result := svc.RunDNSLeakTest()
	if !result.VPNActive {
		t.Fatal("expected VPNActive=true")
	}
	if len(result.Nameservers) != 1 || result.Nameservers[0] != "10.66.0.1" {
		t.Fatalf("expected effective nameserver 10.66.0.1, got %v", result.Nameservers)
	}
	if result.PotentiallyLeaking {
		t.Fatal("expected PotentiallyLeaking=false when dnsmasq forwards to VPN DNS")
	}
}

func TestSplitWireGuardDNSOption(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want []string
	}{
		{"198.18.0.1,198.18.0.2", []string{"198.18.0.1", "198.18.0.2"}},
		{"10.0.0.1 10.0.0.2", []string{"10.0.0.1", "10.0.0.2"}},
		{"  1.1.1.1 , 8.8.8.8 ", []string{"1.1.1.1", "8.8.8.8"}},
		{"", nil},
	}
	for _, tt := range tests {
		got := splitWireGuardDNSOption(tt.in)
		if len(got) != len(tt.want) {
			t.Errorf("splitWireGuardDNSOption(%q) = %v; want %v", tt.in, got, tt.want)
			continue
		}
		for i := range tt.want {
			if got[i] != tt.want[i] {
				t.Errorf("splitWireGuardDNSOption(%q)[%d] = %q; want %q", tt.in, i, got[i], tt.want[i])
			}
		}
	}
}

func TestRunDNSLeakTest_DnsmasqOnlyAdGuardWhileVPNDNSConfiguredLeaks(t *testing.T) {
	// dnsmasq still forwards only to AdGuard while wg0 lists VPN DNS — LAN DNS is not using VPN DNS.
	u := uci.NewMockUCI()
	_ = u.Set("network", "wg0", "disabled", "0")
	_ = u.Set("network", "wg0", "dns", "1.1.1.1")

	cmd := &MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		if name == "uci" && len(args) >= 2 && args[0] == "get" && args[1] == "dhcp.@dnsmasq[0].server" {
			return []byte("127.0.0.1#5353\n"), nil
		}
		return nil, fmt.Errorf("unexpected command")
	}}
	svc := NewVpnServiceWithRunner(u, cmd)

	old := readResolvConfNameservers
	readResolvConfNameservers = func() []string {
		return []string{"127.0.0.1"}
	}
	t.Cleanup(func() { readResolvConfNameservers = old })

	result := svc.RunDNSLeakTest()
	if !result.VPNActive {
		t.Fatal("expected VPNActive=true")
	}
	if !result.PotentiallyLeaking {
		t.Fatal("expected PotentiallyLeaking=true when dnsmasq upstream is not VPN DNS")
	}
}

func TestSetupWireGuardFirewall(t *testing.T) {
	u := uci.NewMockUCI()
	svc := NewVpnService(u)

	if err := svc.setupWireGuardFirewall(); err != nil {
		t.Fatalf("setupWireGuardFirewall: %v", err)
	}

	zoneName, err := u.Get("firewall", "wg0_zone", "name")
	if err != nil || zoneName != "wg0" {
		t.Errorf("expected wg0_zone name='wg0', got %q (err=%v)", zoneName, err)
	}
	masq, _ := u.Get("firewall", "wg0_zone", "masq")
	if masq != "1" {
		t.Errorf("expected masq='1', got %q", masq)
	}
	src, _ := u.Get("firewall", "wg0_fwd", "src")
	dest, _ := u.Get("firewall", "wg0_fwd", "dest")
	if src != "lan" || dest != "wg0" {
		t.Errorf("expected wg0_fwd src=lan dest=wg0, got src=%q dest=%q", src, dest)
	}
}

func TestTeardownWireGuardFirewall(t *testing.T) {
	u := uci.NewMockUCI()
	svc := NewVpnService(u)

	// Set up then tear down.
	_ = svc.setupWireGuardFirewall()
	if err := svc.teardownWireGuardFirewall(); err != nil {
		t.Fatalf("teardownWireGuardFirewall: %v", err)
	}

	if _, err := u.Get("firewall", "wg0_zone", "name"); err == nil {
		t.Error("expected wg0_zone to be deleted")
	}
	if _, err := u.Get("firewall", "wg0_fwd", "src"); err == nil {
		t.Error("expected wg0_fwd to be deleted")
	}
}

func TestVerifyWireGuard(t *testing.T) {
	u := uci.NewMockUCI()
	// Set up firewall plumbing so VerifyWireGuard can find it.
	_ = u.AddSection("firewall", "wg0_zone", "zone")
	_ = u.Set("firewall", "wg0_zone", "name", "wg0")
	_ = u.Set("firewall", "wg0_zone", "network", "wg0")
	_ = u.AddSection("firewall", "wg0_fwd", "forwarding")
	_ = u.Set("firewall", "wg0_fwd", "src", "lan")
	_ = u.Set("firewall", "wg0_fwd", "dest", "wg0")

	// Command runner that simulates: wg0 is UP, handshake recent, route via wg0.
	now := "1000000000" // some epoch
	_ = now
	svc := NewVpnServiceWithRunner(u, &stubVerifyRunner{})

	result := svc.VerifyWireGuard()

	if !result.FirewallZoneOk {
		t.Error("expected FirewallZoneOk=true")
	}
	if !result.ForwardingOk {
		t.Error("expected ForwardingOk=true")
	}
	// InterfaceUp / HandshakeOk / RouteOk depend on stub output (false in stub).
	// Just ensure the function runs without panic.
}

func TestRunWireGuardSpeedTest_WireGuardDisabled(t *testing.T) {
	u := uci.NewMockUCI()
	svc := NewVpnServiceWithRunner(u, &FuncCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("unexpected command: %s", name)
	}})
	_, err := svc.RunWireGuardSpeedTest()
	if err == nil {
		t.Fatal("expected error when wireguard disabled")
	}
}

func TestRunWireGuardSpeedTest_InterfaceNotUp(t *testing.T) {
	u := uci.NewMockUCI()
	if err := u.Set("network", "wg0", "disabled", "0"); err != nil {
		t.Fatal(err)
	}
	svc := NewVpnServiceWithRunner(u, &FuncCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		if name == "/sbin/ip" && len(args) >= 4 && args[0] == "link" {
			return []byte(""), nil
		}
		return nil, fmt.Errorf("unexpected")
	}})
	_, err := svc.RunWireGuardSpeedTest()
	if err == nil || !strings.Contains(err.Error(), "not up") {
		t.Fatalf("expected not up error, got %v", err)
	}
}

func TestRunWireGuardSpeedTest_Success(t *testing.T) {
	u := uci.NewMockUCI()
	if err := u.Set("network", "wg0", "disabled", "0"); err != nil {
		t.Fatal(err)
	}
	svc := NewVpnServiceWithRunner(u, &FuncCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		switch name {
		case "/sbin/ip":
			if len(args) >= 4 && args[0] == "link" && args[1] == "show" {
				return []byte("2: wg0: <POINTOPOINT,UP,LOWER_UP> mtu 1420"), nil
			}
			if len(args) >= 2 && args[0] == "-4" {
				return []byte("wg0    inet 10.0.0.2/32 scope global wg0\n"), nil
			}
		case "wget":
			return nil, nil
		case "ping":
			return []byte("---\n"), nil
		}
		return nil, fmt.Errorf("unexpected %s %v", name, args)
	}})
	res, err := svc.RunWireGuardSpeedTest()
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if res.DownloadMbps <= 0 {
		t.Errorf("expected download > 0, got %v", res.DownloadMbps)
	}
}

// stubVerifyRunner returns canned output for ip/wg commands used by VerifyWireGuard.
type stubVerifyRunner struct{}

func (s *stubVerifyRunner) Run(name string, args ...string) ([]byte, error) {
	switch name {
	case "/sbin/ip":
		if len(args) >= 4 && args[0] == "link" && args[1] == "show" && args[2] == "dev" && args[3] == "wg0" {
			return []byte("2: wg0: <POINTOPOINT,UP,LOWER_UP> mtu 1420 state UP mode DEFAULT"), nil
		}
		if len(args) >= 2 && args[0] == "route" {
			return []byte("default via wg0 dev wg0 proto static"), nil
		}
	case "/usr/bin/wg":
		return []byte("PRIV\tPUB\t51820\toff\nPEERPUB\tnone\t1.2.3.4:51820\t0.0.0.0/0\t9999999999\t1000\t2000\t25\n"), nil
	}
	return nil, fmt.Errorf("stub: unhandled command %s %v", name, args)
}

// applyAndVerifyWireGuard must stop retrying ifup once wg0 is already up —
// blind fixed-count retries add ~1s of settle sleeps to every enable.
func TestApplyAndVerifyWireGuard_StopsWhenInterfaceIsUp(t *testing.T) {
	var ifupCalls, reloadCalls int
	cmd := &MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		switch name {
		case "/sbin/ubus":
			reloadCalls++
			return []byte("{}"), nil
		case "/sbin/ifup":
			ifupCalls++
			return nil, nil
		case "/usr/bin/wg":
			return []byte("PRIV\tPUB\t51820\toff\n"), nil
		case "/sbin/ip":
			return []byte("3: wg0: <POINTOPOINT,NOARP,UP,LOWER_UP> mtu 1420 state UNKNOWN"), nil
		}
		return nil, fmt.Errorf("unexpected command: %s %s", name, strings.Join(args, " "))
	}}
	svc := NewVpnServiceWithRunner(uci.NewMockUCI(), cmd)

	if err := svc.applyAndVerifyWireGuard(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ifupCalls > 1 {
		t.Errorf("expected at most 1 ifup when wg0 is already up, got %d", ifupCalls)
	}
	if reloadCalls > 1 {
		t.Errorf("expected no extra network reloads when wg0 is already up, got %d", reloadCalls)
	}
}

// When wg0 never comes up, the apply path must still retry and then fail.
func TestApplyAndVerifyWireGuard_RetriesThenFails(t *testing.T) {
	orig := wireGuardVerifyTimeout
	wireGuardVerifyTimeout = 300 * time.Millisecond
	defer func() { wireGuardVerifyTimeout = orig }()

	var ifupCalls int
	cmd := &MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		switch name {
		case "/sbin/ubus":
			return []byte("{}"), nil
		case "/sbin/ifup":
			ifupCalls++
			return nil, nil
		case "/usr/bin/wg":
			return nil, fmt.Errorf("no such device")
		case "/sbin/ip":
			return []byte("3: wg0: <POINTOPOINT,NOARP> mtu 1420 state DOWN"), nil
		}
		return nil, fmt.Errorf("unexpected command: %s %s", name, strings.Join(args, " "))
	}}
	svc := NewVpnServiceWithRunner(uci.NewMockUCI(), cmd)

	if err := svc.applyAndVerifyWireGuard(); err == nil {
		t.Fatal("expected error when wg0 never comes up")
	}
	if ifupCalls < 2 {
		t.Errorf("expected retries when wg0 stays down, got %d ifup calls", ifupCalls)
	}
}

// failingUCI wraps a real uci.UCI and injects write errors for safety tests.
type failingUCI struct {
	uci.UCI
	addSectionErr error
	setErr        map[string]error
	addListErr    map[string]error
	// addListErrOnce fails the first matching call only, so a retry (e.g. the
	// revert path) can succeed.
	addListErrOnce map[string]error
	commitErr      error
	commits        int
}

func (f *failingUCI) AddSection(config, section, stype string) error {
	if f.addSectionErr != nil {
		return f.addSectionErr
	}
	return f.UCI.AddSection(config, section, stype)
}

func (f *failingUCI) Set(config, section, option, value string) error {
	if err, ok := f.setErr[config+"."+section+"."+option]; ok && err != nil {
		return err
	}
	return f.UCI.Set(config, section, option, value)
}

func (f *failingUCI) AddList(config, section, option, value string) error {
	key := config + "." + section + "." + option
	if err, ok := f.addListErr[key]; ok && err != nil {
		return err
	}
	if err, ok := f.addListErrOnce[key]; ok && err != nil {
		delete(f.addListErrOnce, key)
		return err
	}
	return f.UCI.AddList(config, section, option, value)
}

func (f *failingUCI) Commit(config string) error {
	f.commits++
	if f.commitErr != nil {
		return f.commitErr
	}
	return f.UCI.Commit(config)
}

// newGuardedVpnService returns a service whose VPN crash guard lives in a temp dir.
func newGuardedVpnService(t *testing.T, u uci.UCI, cmd CommandRunner) (*VpnService, string) {
	t.Helper()
	svc := NewVpnServiceWithRunner(u, cmd)
	guard := filepath.Join(t.TempDir(), "vpn-in-progress")
	svc.guardFile = guard
	return svc, guard
}

func TestToggleWireguard_EnableVerifiesTunnelBeforeFirewallAndDNS(t *testing.T) {
	prev := wireGuardVerifyTimeout
	wireGuardVerifyTimeout = 300 * time.Millisecond
	t.Cleanup(func() { wireGuardVerifyTimeout = prev })

	u := uci.NewMockUCI()
	cmd := &MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		switch name {
		case "tailscale", "/etc/init.d/firewall", "/sbin/ubus", "/sbin/ifup", "/sbin/ifdown":
			return nil, nil
		case "/usr/bin/wg":
			return nil, fmt.Errorf("no interface")
		case "/sbin/ip":
			return []byte("3: wg0: state DOWN"), nil
		default:
			return nil, nil
		}
	}}
	svc := NewVpnServiceWithRunner(u, cmd)

	if err := svc.ToggleWireguard(true); err == nil {
		t.Fatal("expected error when wg0 does not come up")
	}

	// Dependent firewall plumbing must NOT exist: it would send LAN traffic to
	// a tunnel that is down.
	if _, err := u.GetAll("firewall", "wg0_zone"); err == nil {
		t.Error("wg0 firewall zone must not be created when the tunnel failed to start")
	}
	if _, err := u.GetAll("firewall", "wg0_fwd"); err == nil {
		t.Error("wg0 forwarding must not be created when the tunnel failed to start")
	}
	// wg0 must be disabled again.
	disabled, _ := u.Get("network", "wg0", "disabled")
	if disabled != "1" {
		t.Errorf("expected wg0 re-disabled after failed enable, got %q", disabled)
	}
}

func TestToggleWireguard_EnableRollsBackAfterTunnelFailure(t *testing.T) {
	prev := wireGuardVerifyTimeout
	wireGuardVerifyTimeout = 300 * time.Millisecond
	t.Cleanup(func() { wireGuardVerifyTimeout = prev })

	u := uci.NewMockUCI()
	// wg0 starts disabled=0 (user had it enabled in UCI but the link is dead).
	_ = u.AddSection("network", "wg0", "interface")
	_ = u.Set("network", "wg0", "disabled", "0")
	cmd := &MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		switch name {
		case "tailscale", "/etc/init.d/firewall", "/sbin/ubus", "/sbin/ifup", "/sbin/ifdown":
			return nil, nil
		case "/usr/bin/wg":
			return nil, fmt.Errorf("no interface")
		case "/sbin/ip":
			return []byte("3: wg0: state DOWN"), nil
		default:
			return nil, nil
		}
	}}
	svc, guard := newGuardedVpnService(t, u, cmd)

	if err := svc.ToggleWireguard(true); err == nil {
		t.Fatal("expected error when wg0 does not come up")
	}
	disabled, _ := u.Get("network", "wg0", "disabled")
	if disabled != "0" {
		t.Errorf("expected previous disabled=0 restored, got %q", disabled)
	}
	if _, err := os.Stat(guard); !os.IsNotExist(err) {
		t.Errorf("expected crash guard removed after a complete rollback, stat err = %v", err)
	}
}

func TestToggleWireguard_EnableFailsWithoutCrashGuard(t *testing.T) {
	u := uci.NewMockUCI()
	svc := NewVpnServiceWithRunner(u, &MockCommandRunner{RunFunc: mockRunWireGuardEnableOK})
	// A regular file cannot be a parent directory, so the guard cannot be created.
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatalf("write blocker file: %v", err)
	}
	svc.guardFile = filepath.Join(blocker, "vpn-in-progress")

	if err := svc.ToggleWireguard(true); err == nil {
		t.Fatal("expected enable to fail when the crash guard cannot be written")
	}
	disabled, _ := u.Get("network", "wg0", "disabled")
	if disabled == "0" {
		t.Error("must not enable the tunnel when the crash guard could not be written")
	}
}

func TestToggleWireguard_EnableRemovesCrashGuardOnSuccess(t *testing.T) {
	u := uci.NewMockUCI()
	svc, guard := newGuardedVpnService(t, u, &MockCommandRunner{RunFunc: mockRunWireGuardEnableOK})

	if err := svc.ToggleWireguard(true); err != nil {
		t.Fatalf("ToggleWireguard(true): %v", err)
	}
	if _, err := os.Stat(guard); !os.IsNotExist(err) {
		t.Errorf("expected crash guard removed after success, stat err = %v", err)
	}
	if _, err := u.GetAll("firewall", "wg0_zone"); err != nil {
		t.Error("expected wg0 firewall zone after a successful enable")
	}
}

func TestSetKillSwitch_PropagatesAddSectionError(t *testing.T) {
	base := uci.NewMockUCI()
	fu := &failingUCI{UCI: base, addSectionErr: errors.New("uci readonly")}
	svc := NewVpnServiceWithRunner(fu, &MockCommandRunner{})

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
	svc := NewVpnServiceWithRunner(fu, &MockCommandRunner{})

	if err := svc.SetKillSwitch(true); err == nil {
		t.Fatal("expected SetKillSwitch to fail when an option cannot be written")
	}
	if fu.commits != 0 {
		t.Errorf("must not commit a partial kill switch rule, commits = %d", fu.commits)
	}
}

func TestSetKillSwitch_MarksUserOwnership(t *testing.T) {
	u := uci.NewMockUCI()
	svc := NewVpnServiceWithRunner(u, &MockCommandRunner{})

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
	svc := NewVpnServiceWithRunner(u, &MockCommandRunner{})

	if err := svc.SetKillSwitch(false); err != nil {
		t.Fatalf("SetKillSwitch(false) on a clean router: %v", err)
	}
}

func TestToggleWireguard_DisableKeepsUserKillSwitch(t *testing.T) {
	u := uci.NewMockUCI()
	cmd := &MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		if name == "/sbin/ip" && len(args) >= 3 && args[0] == "route" && args[1] == "show" && args[2] == "default" {
			return []byte("default via 10.0.1.1 dev phy1-sta0"), nil
		}
		return nil, nil
	}}
	svc := NewVpnServiceWithRunner(u, cmd)

	if err := svc.SetKillSwitch(true); err != nil {
		t.Fatalf("SetKillSwitch(true): %v", err)
	}
	if err := svc.ToggleWireguard(false); err != nil {
		t.Fatalf("ToggleWireguard(false): %v", err)
	}
	if _, err := u.GetAll("firewall", "vpn_killswitch"); err != nil {
		t.Error("a user-configured kill switch must survive a VPN toggle")
	}
}

func TestToggleWireguard_DisableRemovesVPNOwnedKillSwitch(t *testing.T) {
	u := uci.NewMockUCI()
	_ = u.AddSection("firewall", "vpn_killswitch", "rule")
	_ = u.Set("firewall", "vpn_killswitch", "src", "lan")
	_ = u.Set("firewall", "vpn_killswitch", "dest", "wan")
	_ = u.Set("firewall", "vpn_killswitch", "target", "REJECT")
	_ = u.Set("firewall", "vpn_killswitch", vpnKillSwitchOwnerOption, vpnKillSwitchOwnerToggle)
	cmd := &MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		if name == "/sbin/ip" && len(args) >= 3 && args[0] == "route" && args[1] == "show" && args[2] == "default" {
			return []byte("default via 10.0.1.1 dev phy1-sta0"), nil
		}
		return nil, nil
	}}
	svc := NewVpnServiceWithRunner(u, cmd)

	if err := svc.ToggleWireguard(false); err != nil {
		t.Fatalf("ToggleWireguard(false): %v", err)
	}
	if _, err := u.GetAll("firewall", "vpn_killswitch"); err == nil {
		t.Error("a VPN-toggle owned kill switch must be removed when the tunnel is disabled")
	}
}

func TestSetSplitTunnel_CustomWithoutRoutesIsRejected(t *testing.T) {
	u := uci.NewMockUCI()
	_ = u.AddSection("network", "wg0_peer0", "wireguard_wg0")
	_ = u.Set("network", "wg0_peer0", "allowed_ips", "10.0.0.0/24")
	svc := NewVpnServiceWithRunner(u, &MockCommandRunner{})

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
	svc := NewVpnServiceWithRunner(u, &MockCommandRunner{})
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
	svc := NewVpnServiceWithRunner(fu, &MockCommandRunner{})

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

// wgDumpWithHandshake is a `wg show wg0 dump` with one peer that has completed
// a handshake, so wgRuntimeState reports "connected".
const wgDumpWithHandshake = "PRIV\tPUB\t51820\toff\n" +
	"PEER\t\t10.0.0.1:51820\t0.0.0.0/0\t1700000000\t1\t2\toff\n"

// newSnapshotScopedVpnService returns a service whose crash guard and DNS
// snapshot both live in a temp dir, so the snapshot lifecycle can be asserted
// without touching /etc/trafo.
func newSnapshotScopedVpnService(t *testing.T, u uci.UCI, cmd CommandRunner) (*VpnService, string) {
	t.Helper()
	svc, guard := newGuardedVpnService(t, u, cmd)
	dir := t.TempDir()
	svc.dnsSnapshotPath = filepath.Join(dir, "vpn-dns-snapshot.json")
	svc.legacyDnsSnapshotPath = filepath.Join(dir, "legacy-vpn-dns-snapshot.json")
	return svc, guard
}

// seedVpnDnsSnapshot writes the snapshot an earlier successful enable would have
// left behind, holding the pre-VPN dnsmasq list.
func seedVpnDnsSnapshot(t *testing.T, svc *VpnService) {
	t.Helper()
	err := svc.writeVpnDnsSnapshot(vpnDnsSnapshot{Servers: []string{"127.0.0.1#5353"}})
	if err != nil {
		t.Fatalf("seeding snapshot: %v", err)
	}
}

// dnsmasqState is a tiny model of dhcp.@dnsmasq[0] driven through the same
// `uci` shell-outs the service makes.
type dnsmasqState struct {
	servers  []string
	noresolv string
}

func newDnsmasqRunner(servers []string, wgUp bool) (*MockCommandRunner, *dnsmasqState) {
	state := &dnsmasqState{servers: servers}
	cmd := &MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		switch {
		case name == "uci" && len(args) == 2 && args[0] == "get" &&
			args[1] == "dhcp.@dnsmasq[0].server":
			return []byte(strings.Join(state.servers, " ")), nil
		case name == "uci" && len(args) == 2 && args[0] == "get" &&
			args[1] == "dhcp.@dnsmasq[0].noresolv":
			return []byte(state.noresolv), nil
		case name == "uci" && len(args) == 2 && args[0] == "delete" &&
			args[1] == "dhcp.@dnsmasq[0].server":
			state.servers = nil
			return nil, nil
		case name == "uci" && len(args) == 2 && args[0] == "add_list":
			_, value, _ := strings.Cut(args[1], "=")
			state.servers = append(state.servers, value)
			return nil, nil
		case name == "uci" && len(args) == 2 && args[0] == "set":
			_, value, _ := strings.Cut(args[1], "=")
			if strings.HasSuffix(args[1], ".noresolv=") || strings.Contains(args[1], ".noresolv=") {
				state.noresolv = value
			}
			return nil, nil
		case name == "uci" && len(args) == 2 && args[0] == "commit":
			return nil, nil
		case name == "/etc/init.d/dnsmasq":
			return nil, nil
		case name == "tailscale":
			return nil, nil
		case name == "/etc/init.d/firewall":
			return nil, nil
		case name == "/usr/bin/wg":
			if !wgUp {
				return nil, fmt.Errorf("Unable to access interface: No such device")
			}
			return []byte(wgDumpWithHandshake), nil
		case name == "/sbin/ip":
			if len(args) >= 3 && args[0] == "link" && args[1] == "show" {
				if !wgUp {
					return []byte("3: wg0: state DOWN"), nil
				}
				return []byte("3: wg0: <POINTOPOINT,NOARP,UP,LOWER_UP> state UNKNOWN"), nil
			}
			return nil, nil
		}
		return nil, nil
	}}
	return cmd, state
}

// Enabling twice must not overwrite the snapshot with the VPN's own resolvers.
// The dashboard toggle and the VPN page are independent entry points with no
// idempotency guard, so a second enable used to destroy the pre-VPN state and
// the later disable restored noresolv=1 pointing at 10.8.0.1.
func TestEnableVpnDNSForwarding_SecondEnableKeepsFirstSnapshot(t *testing.T) {
	u := uci.NewMockUCI()
	_ = u.Set("network", "wg0", "dns", "10.8.0.1")
	cmd, state := newDnsmasqRunner([]string{"127.0.0.1#5353"}, true)
	svc, _ := newSnapshotScopedVpnService(t, u, cmd)

	if err := svc.enableVpnDNSForwarding(); err != nil {
		t.Fatalf("first enable: %v", err)
	}
	if !slices.Equal(state.servers, []string{"10.8.0.1"}) || state.noresolv != "1" {
		t.Fatalf("first enable must point dnsmasq at the tunnel,"+
			" got %v noresolv=%q", state.servers, state.noresolv)
	}
	first, err := os.ReadFile(svc.dnsSnapshotPath)
	if err != nil {
		t.Fatalf("reading snapshot: %v", err)
	}
	if !strings.Contains(string(first), "127.0.0.1#5353") {
		t.Fatalf("the snapshot must hold the pre-VPN list, got %q", first)
	}

	// Second enable, now that dnsmasq already points at the tunnel.
	if err := svc.enableVpnDNSForwarding(); err != nil {
		t.Fatalf("second enable: %v", err)
	}
	second, err := os.ReadFile(svc.dnsSnapshotPath)
	if err != nil {
		t.Fatalf("reading snapshot: %v", err)
	}
	if string(second) != string(first) {
		t.Fatalf("a second enable must not overwrite the snapshot:\nfirst: %q\nsecond: %q", first, second)
	}

	// The disable still restores the pre-VPN list, not the tunnel's.
	svc.disableVpnDNSForwarding()
	if !slices.Equal(state.servers, []string{"127.0.0.1#5353"}) {
		t.Fatalf("disable must restore the pre-VPN list, got %v", state.servers)
	}
	if state.noresolv != "0" {
		t.Fatalf("disable must restore the pre-VPN noresolv, got %q", state.noresolv)
	}
}

// A snapshot that cannot be written means there is nothing to restore, so the
// forwarding must be aborted before dnsmasq is touched at all.
func TestEnableVpnDNSForwarding_SnapshotWriteFailureLeavesDnsmasqAlone(t *testing.T) {
	u := uci.NewMockUCI()
	_ = u.Set("network", "wg0", "dns", "10.8.0.1")
	cmd, state := newDnsmasqRunner([]string{"127.0.0.1#5353"}, true)
	svc, _ := newSnapshotScopedVpnService(t, u, cmd)
	// A regular file cannot be a parent directory, so the snapshot write fails.
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	svc.dnsSnapshotPath = filepath.Join(blocker, "vpn-dns-snapshot.json")

	if err := svc.enableVpnDNSForwarding(); err == nil {
		t.Fatal("expected the forwarding to abort when the snapshot cannot be written")
	}
	if !slices.Equal(state.servers, []string{"127.0.0.1#5353"}) {
		t.Errorf("dnsmasq must be untouched when the snapshot failed, got %v", state.servers)
	}
	if state.noresolv != "" {
		t.Errorf("noresolv must not be set when the snapshot failed, got %q", state.noresolv)
	}
}

// A downed tunnel with a snapshot on disk leaves dnsmasq at noresolv=1 pointing
// at resolvers reachable only through that tunnel. Polling GET /vpn/status must
// restore the pre-VPN state instead of waiting for an operator toggle.
func TestGetVpnStatus_SelfHealsDNSSnapshotWhenTunnelIsDown(t *testing.T) {
	u := uci.NewMockUCI()
	_ = u.Set("network", "wg0", "disabled", "0")
	cmd, state := newDnsmasqRunner([]string{"127.0.0.1#5353"}, false)
	svc, _ := newSnapshotScopedVpnService(t, u, cmd)

	// A snapshot left behind by an earlier enable, and dnsmasq pointed at the
	// tunnel's resolvers because the tunnel never came back.
	seedVpnDnsSnapshot(t, svc)
	state.servers = []string{"10.8.0.1"}
	state.noresolv = "1"

	if _, err := svc.GetVpnStatus(); err != nil {
		t.Fatalf("GetVpnStatus: %v", err)
	}
	if !slices.Equal(state.servers, []string{"127.0.0.1#5353"}) {
		t.Fatalf("expected dnsmasq restored to the pre-VPN list, got %v", state.servers)
	}
	if state.noresolv != "0" {
		t.Fatalf("expected noresolv cleared to 0, got %q", state.noresolv)
	}
	if _, err := os.Stat(svc.dnsSnapshotPath); !os.IsNotExist(err) {
		t.Errorf("the snapshot must be removed once restored, stat err = %v", err)
	}
}

// A healthy tunnel must not be healed: the VPN resolvers are the point.
func TestGetVpnStatus_KeepsVPNResolversWhenTunnelIsConnected(t *testing.T) {
	u := uci.NewMockUCI()
	_ = u.Set("network", "wg0", "disabled", "0")
	cmd, state := newDnsmasqRunner([]string{"10.8.0.1"}, true)
	svc, _ := newSnapshotScopedVpnService(t, u, cmd)
	seedVpnDnsSnapshot(t, svc)
	state.noresolv = "1"

	statuses, err := svc.GetVpnStatus()
	if err != nil {
		t.Fatalf("GetVpnStatus: %v", err)
	}
	if !slices.Equal(state.servers, []string{"10.8.0.1"}) || state.noresolv != "1" {
		t.Fatalf("a connected tunnel must keep the VPN resolvers,"+
			" got %v noresolv=%q", state.servers, state.noresolv)
	}
	if statuses[0].StatusDetail != "connected" {
		t.Errorf("StatusDetail must stay at its documented value, got %q", statuses[0].StatusDetail)
	}
	if !statuses[0].Connected {
		t.Error("expected Connected=true")
	}
}

// With no snapshot there is nothing to heal, so the self-heal must not touch
// dnsmasq at all.
func TestGetVpnStatus_NoSnapshotLeavesDnsmasqAlone(t *testing.T) {
	u := uci.NewMockUCI()
	_ = u.Set("network", "wg0", "disabled", "0")
	cmd, state := newDnsmasqRunner([]string{"10.8.0.1"}, false)
	svc, _ := newSnapshotScopedVpnService(t, u, cmd)
	state.noresolv = "1"

	if _, err := svc.GetVpnStatus(); err != nil {
		t.Fatalf("GetVpnStatus: %v", err)
	}
	if !slices.Equal(state.servers, []string{"10.8.0.1"}) || state.noresolv != "1" {
		t.Fatalf("dnsmasq must be untouched without a snapshot,"+
			" got %v noresolv=%q", state.servers, state.noresolv)
	}
}

// The wg0 firewall sections must be gone on every exit path, including the
// missing-default-route one. Leaving them committed meant the next enable
// silently reused stale sections after any number of reboots.
func TestToggleWireguard_DisableTearsDownFirewallWithoutDefaultRoute(t *testing.T) {
	u := uci.NewMockUCI()
	cmd := &MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		if name == "/sbin/ip" && len(args) >= 3 && args[0] == "route" &&
			args[1] == "show" && args[2] == "default" {
			return []byte(""), nil
		}
		return nil, nil
	}}
	svc, _ := newSnapshotScopedVpnService(t, u, cmd)
	// Pretend a previous enable left the sections in place.
	if err := svc.setupWireGuardFirewall(); err != nil {
		t.Fatalf("setupWireGuardFirewall: %v", err)
	}

	if err := svc.ToggleWireguard(false); err == nil {
		t.Fatal("expected an error when no default route can be restored")
	}
	if _, err := u.GetAll("firewall", "wg0_zone"); err == nil {
		t.Error("wg0 firewall zone must be torn down even when the route restore failed")
	}
	if _, err := u.GetAll("firewall", "wg0_fwd"); err == nil {
		t.Error("wg0 forwarding must be torn down even when the route restore failed")
	}
}

// The disable path bounces uplinks, so it needs the crash guard too. It must be
// kept when the default route is not confirmed and removed when it is.
func TestToggleWireguard_DisableWritesAndClearsCrashGuard(t *testing.T) {
	prev := wireGuardVerifyTimeout
	wireGuardVerifyTimeout = 300 * time.Millisecond
	t.Cleanup(func() { wireGuardVerifyTimeout = prev })

	u := uci.NewMockUCI()
	kernelDefault := false
	cmd := &MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		if name == "/sbin/ip" && len(args) >= 3 && args[0] == "route" &&
			args[1] == "show" && args[2] == "default" {
			if kernelDefault {
				return []byte("default via 10.0.1.1 dev phy1-sta0"), nil
			}
			return []byte(""), nil
		}
		if name == "/sbin/ubus" && len(args) >= 3 && args[0] == "call" && args[2] == "renew" {
			kernelDefault = true
			return []byte("{}"), nil
		}
		return nil, nil
	}}
	svc, guard := newSnapshotScopedVpnService(t, u, cmd)

	if err := svc.ToggleWireguard(false); err != nil {
		t.Fatalf("ToggleWireguard(false): %v", err)
	}
	if _, err := os.Stat(guard); !os.IsNotExist(err) {
		t.Errorf("crash guard must be cleared once a default route is confirmed, stat err = %v", err)
	}

	// Now the route cannot be restored: the marker must survive.
	u2 := uci.NewMockUCI()
	svc2, guard2 := newSnapshotScopedVpnService(t, u2, &MockCommandRunner{
		RunFunc: func(name string, args ...string) ([]byte, error) {
			if name == "/sbin/ip" && len(args) >= 3 && args[0] == "route" &&
				args[1] == "show" && args[2] == "default" {
				return []byte(""), nil
			}
			return nil, nil
		},
	})
	if err := svc2.ToggleWireguard(false); err == nil {
		t.Fatal("expected an error when no default route can be restored")
	}
	if _, err := os.Stat(guard2); err != nil {
		t.Errorf("crash guard must be kept when the default route is not confirmed: %v", err)
	}
}

// The Tailscale exit node must not be cleared before the tunnel is verified:
// a failed enable used to silently move the router's egress with nothing to put
// it back.
func TestEnableWireguard_ClearsTailscaleExitNodeOnlyAfterTunnelVerified(t *testing.T) {
	prev := wireGuardVerifyTimeout
	wireGuardVerifyTimeout = 300 * time.Millisecond
	t.Cleanup(func() { wireGuardVerifyTimeout = prev })

	u := uci.NewMockUCI()
	var exitNodeCleared bool
	cmd := &MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		switch name {
		case "tailscale":
			if slices.Contains(args, "--exit-node=") {
				exitNodeCleared = true
			}
			return nil, nil
		case "/etc/init.d/firewall", "/sbin/ubus", "/sbin/ifup", "/sbin/ifdown":
			return nil, nil
		case "/usr/bin/wg":
			return nil, fmt.Errorf("no such device")
		case "/sbin/ip":
			return []byte("3: wg0: state DOWN"), nil
		}
		return nil, nil
	}}
	svc, _ := newSnapshotScopedVpnService(t, u, cmd)

	if err := svc.ToggleWireguard(true); err == nil {
		t.Fatal("expected the enable to fail when wg0 never comes up")
	}
	if exitNodeCleared {
		t.Error("the Tailscale exit node must not be cleared when the tunnel failed to start")
	}

	// And it IS cleared once the tunnel is verified.
	u2 := uci.NewMockUCI()
	var clearedAfterVerify bool
	var verified bool
	cmd2 := &MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		switch name {
		case "tailscale":
			if slices.Contains(args, "--exit-node=") && verified {
				clearedAfterVerify = true
			}
			return nil, nil
		case "/etc/init.d/firewall", "/sbin/ubus", "/sbin/ifup", "/sbin/ifdown":
			return nil, nil
		case "/usr/bin/wg":
			verified = true
			return []byte("PRIV\tPUB\t51820\toff\n"), nil
		case "/sbin/ip":
			if len(args) >= 3 && args[0] == "link" && args[1] == "show" {
				return []byte("3: wg0: <POINTOPOINT,NOARP,UP,LOWER_UP> state UNKNOWN"), nil
			}
			return nil, nil
		}
		return nil, nil
	}}
	svc2, _ := newSnapshotScopedVpnService(t, u2, cmd2)
	if err := svc2.ToggleWireguard(true); err != nil {
		t.Fatalf("ToggleWireguard(true): %v", err)
	}
	if !clearedAfterVerify {
		t.Error("the Tailscale exit node must be cleared once the tunnel is verified")
	}
}
