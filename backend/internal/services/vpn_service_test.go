package services

import (
	"fmt"
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
	cmd := &MockCommandRunner{Err: fmt.Errorf("command not found")}
	svc := NewVpnServiceWithRunner(u, cmd)

	_, err := svc.GetWireGuardStatus()
	if err == nil {
		t.Error("expected error when wg command fails")
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
