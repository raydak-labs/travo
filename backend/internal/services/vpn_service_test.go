package services

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/uci"
)

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
	svc := NewVpnService(u)

	err := svc.ToggleWireguard(true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val, _ := u.Get("network", "wg0", "disabled")
	if val != "0" {
		t.Errorf("expected disabled=0, got %q", val)
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
