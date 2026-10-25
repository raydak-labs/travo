package uplink

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/guard"
	"github.com/openwrt-travel-gui/backend/internal/platform"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

func TestGetVpnStatus(t *testing.T) {
	u := uci.NewMockUCI()
	svc := NewVpnService(VpnDeps{UCI: u})

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
	svc := NewVpnService(VpnDeps{UCI: u})

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
	cmd := &platform.MockCommandRunner{RunFunc: mockRunWireGuardEnableOK}
	svc := NewVpnService(VpnDeps{UCI: u, Cmd: cmd})

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
	cmd := &platform.MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
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
	svc := NewVpnService(VpnDeps{UCI: u, Cmd: cmd})

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
	cmd := &platform.MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
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
	svc := NewVpnService(VpnDeps{UCI: u, Cmd: cmd})

	err := svc.ToggleWireguard(true)
	if err == nil {
		t.Fatal("expected error when wg0 does not come up")
	}
}

func TestToggleWireguard_FailsPreflightWhenMissingPrivateKey(t *testing.T) {
	u := uci.NewMockUCI()
	_ = u.Set("network", "wg0", "private_key", "")
	called := 0
	cmd := &platform.MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		called++
		// tailscale is best-effort and may still be invoked before preflight validation.
		if name == "tailscale" || strings.HasSuffix(name, "/tailscale") {
			return nil, nil
		}
		return nil, fmt.Errorf("unexpected command during preflight failure: %s %v", name, args)
	}}
	svc := NewVpnService(VpnDeps{UCI: u, Cmd: cmd})

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
	cmd := &platform.MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		if name == "tailscale" || strings.HasSuffix(name, "/tailscale") {
			return nil, nil
		}
		return nil, fmt.Errorf("unexpected command during preflight failure: %s %v", name, args)
	}}
	svc := NewVpnService(VpnDeps{UCI: u, Cmd: cmd})

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
	cmd := &platform.MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		if name == "tailscale" || strings.HasSuffix(name, "/tailscale") {
			return nil, nil
		}
		return nil, fmt.Errorf("unexpected command during preflight failure: %s %v", name, args)
	}}
	svc := NewVpnService(VpnDeps{UCI: u, Cmd: cmd})

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
	cmd := &platform.MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		if name == "tailscale" || strings.HasSuffix(name, "/tailscale") {
			return nil, nil
		}
		return nil, fmt.Errorf("unexpected command during preflight failure: %s %v", name, args)
	}}
	svc := NewVpnService(VpnDeps{UCI: u, Cmd: cmd})

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
	cmd := &platform.MockCommandRunner{RunFunc: mockRunWireGuardEnableOK}
	svc := NewVpnService(VpnDeps{UCI: u, Cmd: cmd})

	if err := svc.ToggleWireguard(true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	proto, _ := u.Get("network", "wg0", "proto")
	if proto != "wireguard" {
		t.Errorf("expected proto=wireguard, got %q", proto)
	}
}

func TestWgRuntimeState_Disabled(t *testing.T) {
	u := uci.NewMockUCI()
	cmd := &platform.MockCommandRunner{Err: fmt.Errorf("exit status 1")}
	svc := NewVpnService(VpnDeps{UCI: u, Cmd: cmd})
	state := svc.wgRuntimeState(false)
	if state != "disabled" {
		t.Errorf("expected 'disabled', got %q", state)
	}
}

func TestWgRuntimeState_EnabledNotUp(t *testing.T) {
	u := uci.NewMockUCI()
	cmd := &platform.MockCommandRunner{Err: fmt.Errorf("exit status 1")}
	svc := NewVpnService(VpnDeps{UCI: u, Cmd: cmd})
	state := svc.wgRuntimeState(true)
	if state != "enabled_not_up" {
		t.Errorf("expected 'enabled_not_up', got %q", state)
	}
}

func TestWgRuntimeState_Connected(t *testing.T) {
	u := uci.NewMockUCI()
	dump := "PRIV\tPUB\t51820\toff\n" +
		"peerpub\t(none)\tvpn.example.com:51820\t0.0.0.0/0\t1740000000\t100\t200\t0\n"
	cmd := &platform.MockCommandRunner{Output: []byte(dump)}
	svc := NewVpnService(VpnDeps{UCI: u, Cmd: cmd})
	state := svc.wgRuntimeState(true)
	if state != "connected" {
		t.Errorf("expected 'connected', got %q", state)
	}
}

func TestGetWireGuardStatus_Success(t *testing.T) {
	u := uci.NewMockUCI()
	dump := "PRIVATE_KEY\tPUBLIC_KEY_IFACE\t51820\toff\n" +
		"PEER_PUB_KEY\t(none)\t1.2.3.4:51820\t0.0.0.0/0\t1710000000\t123456789\t987654321\toff\n"
	cmd := &platform.MockCommandRunner{Output: []byte(dump)}
	svc := NewVpnService(VpnDeps{UCI: u, Cmd: cmd})

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
	cmd := &platform.MockCommandRunner{Err: fmt.Errorf("exit status 1")}
	svc := NewVpnService(VpnDeps{UCI: u, Cmd: cmd})

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

// applyAndVerifyWireGuard must stop retrying ifup once wg0 is already up —
// blind fixed-count retries add ~1s of settle sleeps to every enable.
func TestApplyAndVerifyWireGuard_StopsWhenInterfaceIsUp(t *testing.T) {
	var ifupCalls, reloadCalls int
	cmd := &platform.MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
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
	svc := NewVpnService(VpnDeps{UCI: uci.NewMockUCI(), Cmd: cmd})

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
	cmd := &platform.MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
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
	svc := NewVpnService(VpnDeps{UCI: uci.NewMockUCI(), Cmd: cmd})

	if err := svc.applyAndVerifyWireGuard(); err == nil {
		t.Fatal("expected error when wg0 never comes up")
	}
	if ifupCalls < 2 {
		t.Errorf("expected retries when wg0 stays down, got %d ifup calls", ifupCalls)
	}
}

func TestToggleWireguard_EnableVerifiesTunnelBeforeFirewallAndDNS(t *testing.T) {
	prev := wireGuardVerifyTimeout
	wireGuardVerifyTimeout = 300 * time.Millisecond
	t.Cleanup(func() { wireGuardVerifyTimeout = prev })

	u := uci.NewMockUCI()
	cmd := &platform.MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
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
	svc := NewVpnService(VpnDeps{UCI: u, Cmd: cmd})

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
	cmd := &platform.MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
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
	svc := NewVpnService(VpnDeps{UCI: u, Cmd: &platform.MockCommandRunner{RunFunc: mockRunWireGuardEnableOK}})
	// A regular file cannot be a parent directory, so the guard cannot be created.
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatalf("write blocker file: %v", err)
	}
	svc.crashGuard = guard.In(blocker, "vpn")

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
	svc, guard := newGuardedVpnService(t, u, &platform.MockCommandRunner{RunFunc: mockRunWireGuardEnableOK})

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

func TestToggleWireguard_DisableKeepsUserKillSwitch(t *testing.T) {
	u := uci.NewMockUCI()
	cmd := &platform.MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		if name == "/sbin/ip" && len(args) >= 3 && args[0] == "route" && args[1] == "show" && args[2] == "default" {
			return []byte("default via 10.0.1.1 dev phy1-sta0"), nil
		}
		return nil, nil
	}}
	svc := NewVpnService(VpnDeps{UCI: u, Cmd: cmd})

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
	cmd := &platform.MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		if name == "/sbin/ip" && len(args) >= 3 && args[0] == "route" && args[1] == "show" && args[2] == "default" {
			return []byte("default via 10.0.1.1 dev phy1-sta0"), nil
		}
		return nil, nil
	}}
	svc := NewVpnService(VpnDeps{UCI: u, Cmd: cmd})

	if err := svc.ToggleWireguard(false); err != nil {
		t.Fatalf("ToggleWireguard(false): %v", err)
	}
	if _, err := u.GetAll("firewall", "vpn_killswitch"); err == nil {
		t.Error("a VPN-toggle owned kill switch must be removed when the tunnel is disabled")
	}
}

// A tunnel that stays down leaves dnsmasq at noresolv=1 pointing at resolvers
// reachable only through it. Polling GET /vpn/status must restore the layer
// below instead of waiting for an operator toggle — but only once the state has
// persisted past the debounce window.
func TestGetVpnStatus_SelfHealsDNSSnapshotWhenTunnelIsDown(t *testing.T) {
	compressVpnDNSHealWindow(t, 3, 0)
	u := uci.NewMockUCI()
	_ = u.Set("network", "wg0", "disabled", "0")
	cmd, state := newDnsmasqRunner([]string{"127.0.0.1#5353"}, false)
	svc, _ := newSnapshotScopedVpnService(t, u, cmd)

	// The record an earlier enable left behind, and dnsmasq pointed at the
	// tunnel's resolvers because the tunnel never came back.
	seedVpnLayerStack(t, svc, "127.0.0.1#5353")
	state.servers = []string{"10.8.0.1"}
	state.noresolv = "1"

	for range vpnDNSHealStreakRequired {
		if _, err := svc.GetVpnStatus(); err != nil {
			t.Fatalf("GetVpnStatus: %v", err)
		}
	}
	if !slices.Equal(state.servers, []string{"127.0.0.1#5353"}) {
		t.Fatalf("expected dnsmasq restored to the pre-VPN list, got %v", state.servers)
	}
	if state.noresolv != "0" {
		t.Fatalf("expected noresolv cleared to 0, got %q", state.noresolv)
	}
	// The heal must not destroy the only record of the pre-any-layer state: it
	// is what a later explicit disable still restores from.
	st := readVpnLayerStack(t, svc)
	if !slices.Equal(st.Servers, []string{"127.0.0.1#5353"}) {
		t.Errorf("the record must keep the pre-any-layer base, got %v", st.Servers)
	}
	if len(st.Layers) != 0 {
		t.Errorf("the healed layer must be off the stack, got %+v", st.Layers)
	}
}

// One failed `wg show` on a healthy tunnel reports enabled_not_up. Healing on it
// rewrote dnsmasq back to the pre-VPN resolvers and dropped the record, so LAN
// DNS silently stopped using the tunnel until the operator toggled it off and
// on. A transient failure must be ignored.
func TestGetVpnStatus_TransientWgFailureDoesNotHeal(t *testing.T) {
	compressVpnDNSHealWindow(t, 3, 0)
	u := uci.NewMockUCI()
	_ = u.Set("network", "wg0", "disabled", "0")
	cmd, state := newDnsmasqRunner([]string{"10.8.0.1"}, true)
	svc, _ := newSnapshotScopedVpnService(t, u, cmd)
	seedVpnLayerStack(t, svc, "127.0.0.1#5353")
	state.noresolv = "1"

	// A single flaky reading on an otherwise healthy tunnel.
	state.setWgUp(false)
	if _, err := svc.GetVpnStatus(); err != nil {
		t.Fatalf("GetVpnStatus: %v", err)
	}
	if !slices.Equal(state.servers, []string{"10.8.0.1"}) || state.noresolv != "1" {
		t.Fatalf("a transient wg failure must not heal,"+
			" got %v noresolv=%q", state.servers, state.noresolv)
	}
	if st := readVpnLayerStack(t, svc); len(st.Layers) != 1 {
		t.Fatalf("the VPN layer must still be stacked, got %+v", st.Layers)
	}

	// The tunnel is healthy again, which resets the streak.
	state.setWgUp(true)
	if _, err := svc.GetVpnStatus(); err != nil {
		t.Fatalf("GetVpnStatus: %v", err)
	}

	// Two down readings, a healthy one, then two more: the streak resets on
	// the healthy reading, so the heal still must not fire.
	for range 2 {
		state.setWgUp(false)
		if _, err := svc.GetVpnStatus(); err != nil {
			t.Fatalf("GetVpnStatus: %v", err)
		}
	}
	state.setWgUp(true)
	if _, err := svc.GetVpnStatus(); err != nil {
		t.Fatalf("GetVpnStatus: %v", err)
	}
	for range 2 {
		state.setWgUp(false)
		if _, err := svc.GetVpnStatus(); err != nil {
			t.Fatalf("GetVpnStatus: %v", err)
		}
	}
	if !slices.Equal(state.servers, []string{"10.8.0.1"}) || state.noresolv != "1" {
		t.Fatalf("an interrupted streak must not heal, got %v noresolv=%q",
			state.servers, state.noresolv)
	}

	// Only an unbroken streak heals.
	state.setWgUp(true)
	if _, err := svc.GetVpnStatus(); err != nil {
		t.Fatalf("GetVpnStatus: %v", err)
	}
	for range 3 {
		state.setWgUp(false)
		if _, err := svc.GetVpnStatus(); err != nil {
			t.Fatalf("GetVpnStatus: %v", err)
		}
	}
	if !slices.Equal(state.servers, []string{"127.0.0.1#5353"}) {
		t.Fatalf("an unbroken streak must heal, got %v", state.servers)
	}
}

// The grace period is what separates "the tunnel is down" from "wg failed while
// the tunnel is up". A streak that arrives faster than the grace window must not
// heal.
func TestGetVpnStatus_HealWaitsForTheGracePeriod(t *testing.T) {
	compressVpnDNSHealWindow(t, 1, time.Hour)
	u := uci.NewMockUCI()
	_ = u.Set("network", "wg0", "disabled", "0")
	cmd, state := newDnsmasqRunner([]string{"10.8.0.1"}, false)
	svc, _ := newSnapshotScopedVpnService(t, u, cmd)
	seedVpnLayerStack(t, svc, "127.0.0.1#5353")
	state.noresolv = "1"

	// Four readings in a row, still inside the grace window.
	for range 4 {
		if _, err := svc.GetVpnStatus(); err != nil {
			t.Fatalf("GetVpnStatus: %v", err)
		}
	}
	if !slices.Equal(state.servers, []string{"10.8.0.1"}) {
		t.Fatalf("the heal must wait for the grace period, got %v", state.servers)
	}
	if st := readVpnLayerStack(t, svc); len(st.Layers) != 1 {
		t.Fatalf("the VPN layer must still be stacked, got %+v", st.Layers)
	}
}

// A tunnel that has not finished coming up is not a failure. `configured` (no
// peers yet) and `up_no_handshake` (peers, tunnel up, no handshake yet) are
// what a reboot into an unreachable upstream looks like for the first few
// minutes, and burning the restore on them left dnsmasq pointing at the router
// itself with nothing to put VPN DNS back.
func TestGetVpnStatus_NeverHealsWhileTheTunnelIsComingUp(t *testing.T) {
	for _, tc := range []struct {
		name  string
		dump  string
		peers string
	}{
		{name: "up_no_handshake", dump: wgDumpNoHandshake, peers: "up_no_handshake"},
		{name: "configured", dump: wgDumpNoPeers, peers: "configured"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			compressVpnDNSHealWindow(t, 1, 0)
			u := uci.NewMockUCI()
			_ = u.Set("network", "wg0", "disabled", "0")
			cmd, state := newDnsmasqRunner([]string{"10.8.0.1"}, true)
			// The tunnel is up but not finished coming up.
			state.setWgDump(tc.dump)
			svc, _ := newSnapshotScopedVpnService(t, u, cmd)
			seedVpnLayerStack(t, svc, "127.0.0.1#5353")
			state.noresolv = "1"

			for range 5 {
				statuses, err := svc.GetVpnStatus()
				if err != nil {
					t.Fatalf("GetVpnStatus: %v", err)
				}
				if statuses[0].StatusDetail != tc.peers {
					t.Fatalf("expected StatusDetail %q, got %q", tc.peers, statuses[0].StatusDetail)
				}
			}
			if !slices.Equal(state.servers, []string{"10.8.0.1"}) || state.noresolv != "1" {
				t.Fatalf("a tunnel that is still coming up must not heal,"+
					" got %v noresolv=%q", state.servers, state.noresolv)
			}
			if st := readVpnLayerStack(t, svc); len(st.Layers) != 1 {
				t.Fatalf("the VPN layer must still be stacked, got %+v", st.Layers)
			}
		})
	}
}

// A healthy tunnel must not be healed: the VPN resolvers are the point.
func TestGetVpnStatus_KeepsVPNResolversWhenTunnelIsConnected(t *testing.T) {
	u := uci.NewMockUCI()
	_ = u.Set("network", "wg0", "disabled", "0")
	cmd, state := newDnsmasqRunner([]string{"10.8.0.1"}, true)
	svc, _ := newSnapshotScopedVpnService(t, u, cmd)
	seedVpnLayerStack(t, svc, "127.0.0.1#5353")
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

// With no record there is nothing to heal, so the self-heal must not touch
// dnsmasq at all.
func TestGetVpnStatus_NoSnapshotLeavesDnsmasqAlone(t *testing.T) {
	compressVpnDNSHealWindow(t, 1, 0)
	u := uci.NewMockUCI()
	_ = u.Set("network", "wg0", "disabled", "0")
	cmd, state := newDnsmasqRunner([]string{"10.8.0.1"}, false)
	svc, _ := newSnapshotScopedVpnService(t, u, cmd)
	state.noresolv = "1"

	for range 3 {
		if _, err := svc.GetVpnStatus(); err != nil {
			t.Fatalf("GetVpnStatus: %v", err)
		}
	}
	if !slices.Equal(state.servers, []string{"10.8.0.1"}) || state.noresolv != "1" {
		t.Fatalf("dnsmasq must be untouched without a record,"+
			" got %v noresolv=%q", state.servers, state.noresolv)
	}
}

// A tunnel disabled in UCI is terminal, not a reading that could be transient:
// the layer must come off the stack immediately.
func TestGetVpnStatus_HealsImmediatelyWhenTheTunnelIsDisabled(t *testing.T) {
	compressVpnDNSHealWindow(t, 5, time.Hour)
	u := uci.NewMockUCI()
	_ = u.Set("network", "wg0", "disabled", "1")
	cmd, state := newDnsmasqRunner([]string{"10.8.0.1"}, false)
	svc, _ := newSnapshotScopedVpnService(t, u, cmd)
	seedVpnLayerStack(t, svc, "127.0.0.1#5353")
	state.noresolv = "1"

	if _, err := svc.GetVpnStatus(); err != nil {
		t.Fatalf("GetVpnStatus: %v", err)
	}
	if !slices.Equal(state.servers, []string{"127.0.0.1#5353"}) || state.noresolv != "0" {
		t.Fatalf("a disabled tunnel must heal on the first read, got %v noresolv=%q",
			state.servers, state.noresolv)
	}
}

// The wg0 firewall sections must be gone on every exit path, including the
// missing-default-route one. Leaving them committed meant the next enable
// silently reused stale sections after any number of reboots.
func TestToggleWireguard_DisableTearsDownFirewallWithoutDefaultRoute(t *testing.T) {
	u := uci.NewMockUCI()
	cmd := &platform.MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
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
	cmd := &platform.MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
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
	svc2, guard2 := newSnapshotScopedVpnService(t, u2, &platform.MockCommandRunner{
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
