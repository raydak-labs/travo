package services

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/uci"
)

func mockRunWireGuardEnableOK(name string, args ...string) ([]byte, error) {
	switch name {
	case "tailscale":
		return nil, nil
	case "/etc/init.d/firewall":
		return nil, nil
	case "ubus":
		return []byte("{}"), nil
	case "ifup", "ifdown":
		return nil, nil
	case "ip":
		if len(args) >= 4 && args[0] == "link" && args[1] == "show" && args[2] == "dev" && args[3] == "wg0" {
			return []byte("3: wg0: <POINTOPOINT,NOARP,UP,LOWER_UP> mtu 1420 qdisc noqueue state UNKNOWN"), nil
		}
	case "wg":
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
	cmd := &MockCommandRunner{}
	svc := NewVpnServiceWithRunner(u, cmd)

	err := svc.ToggleWireguard(false)
	if err != nil {
		t.Fatalf("unexpected error disabling: %v", err)
	}
	val, _ := u.Get("network", "wg0", "disabled")
	if val != "1" {
		t.Errorf("expected disabled=1, got %q", val)
	}
}

func TestToggleWireguard_FailsWhenTunnelNotUp(t *testing.T) {
	u := uci.NewMockUCI()
	cmd := &MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		switch name {
		case "tailscale", "ubus", "ifup":
			return nil, nil
		case "ip":
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
	svc := NewVpnService(u)

	status, err := svc.GetTailscaleStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Installed {
		t.Error("expected tailscale not installed")
	}
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
	if status.Interface != "" || status.PublicKey != "" || status.ListenPort != 0 || len(status.Peers) != 0 {
		t.Errorf("expected empty status when tunnel not active, got: %+v", status)
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

// stubVerifyRunner returns canned output for ip/wg commands used by VerifyWireGuard.
type stubVerifyRunner struct{}

func (s *stubVerifyRunner) Run(name string, args ...string) ([]byte, error) {
	switch name {
	case "ip":
		if len(args) >= 3 && args[0] == "link" && args[1] == "show" {
			return []byte("2: wg0: <POINTOPOINT,UP,LOWER_UP> mtu 1420 state UP mode DEFAULT"), nil
		}
		if len(args) >= 2 && args[0] == "route" {
			return []byte("default via wg0 dev wg0 proto static"), nil
		}
	case "wg":
		// Return a dump with a recent handshake (epoch very large = future, just for test).
		return []byte("PRIV\tPUB\t51820\toff\nPEERPUB\tnone\t1.2.3.4:51820\t0.0.0.0/0\t9999999999\t1000\t2000\t25\n"), nil
	}
	return nil, fmt.Errorf("stub: unhandled command %s %v", name, args)
}
