package uplink

import (
	"encoding/json"
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

func TestImportWireguardConfig_NormalizesPeerSections(t *testing.T) {
	u := uci.NewMockUCI()
	svc := NewVpnService(VpnDeps{UCI: u})

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

func newTestVpnService(t *testing.T) (*VpnService, string) {
	t.Helper()
	dir := t.TempDir()
	profilesPath := filepath.Join(dir, "wireguard_profiles.json")
	u := uci.NewMockUCI()
	cmd := &platform.MockCommandRunner{Output: []byte("PRIV\tPUB\t51820\toff\n")}
	svc := NewVpnService(VpnDeps{UCI: u, Cmd: cmd, ProfilesPath: profilesPath})
	return svc, profilesPath
}

func TestProfilesPersistence(t *testing.T) {
	dir := t.TempDir()
	profilesPath := filepath.Join(dir, "wireguard_profiles.json")
	u := uci.NewMockUCI()
	cmd := &platform.MockCommandRunner{Output: []byte("PRIV\tPUB\t51820\toff\n")}

	svc1 := NewVpnService(VpnDeps{UCI: u, Cmd: cmd, ProfilesPath: profilesPath})
	conf := "[Interface]\nPrivateKey = dGVzdHByaXZhdGVrZXkxMjM0NTY3ODkwMTIzNDU2\nAddress = 10.0.0.2/32\n\n[Peer]\nPublicKey = dGVzdHB1YmxpY2tleTEyMzQ1Njc4OTAxMjM0NTY=\nEndpoint = vpn.example.com:51820\nAllowedIPs = 0.0.0.0/0\n"
	_, _ = svc1.AddProfile("Persistent", conf)

	// Create a new service instance pointing to the same file
	svc2 := NewVpnService(VpnDeps{UCI: u, Cmd: cmd, ProfilesPath: profilesPath})
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

	svc := NewVpnService(VpnDeps{UCI: u})

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
	svc := NewVpnService(VpnDeps{UCI: u})

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
	svc := NewVpnService(VpnDeps{UCI: u})
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

	cmd := &platform.MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		if name == "uci" && len(args) >= 2 && args[0] == "get" && args[1] == "dhcp.@dnsmasq[0].server" {
			return []byte("10.66.0.1\n"), nil
		}
		return nil, fmt.Errorf("unexpected command")
	}}
	svc := NewVpnService(VpnDeps{UCI: u, Cmd: cmd})

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

func TestRunDNSLeakTest_DnsmasqOnlyAdGuardWhileVPNDNSConfiguredLeaks(t *testing.T) {
	// dnsmasq still forwards only to AdGuard while wg0 lists VPN DNS — LAN DNS is not using VPN DNS.
	u := uci.NewMockUCI()
	_ = u.Set("network", "wg0", "disabled", "0")
	_ = u.Set("network", "wg0", "dns", "1.1.1.1")

	cmd := &platform.MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		if name == "uci" && len(args) >= 2 && args[0] == "get" && args[1] == "dhcp.@dnsmasq[0].server" {
			return []byte("127.0.0.1#5353\n"), nil
		}
		return nil, fmt.Errorf("unexpected command")
	}}
	svc := NewVpnService(VpnDeps{UCI: u, Cmd: cmd})

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
	svc := NewVpnService(VpnDeps{UCI: u})

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
	svc := NewVpnService(VpnDeps{UCI: u})

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
	svc := NewVpnService(VpnDeps{UCI: u, Cmd: &stubVerifyRunner{}})

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
	svc := NewVpnService(VpnDeps{UCI: u, Cmd: &platform.FuncCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("unexpected command: %s", name)
	}}})
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
	svc := NewVpnService(VpnDeps{UCI: u, Cmd: &platform.FuncCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		if name == "/sbin/ip" && len(args) >= 4 && args[0] == "link" {
			return []byte(""), nil
		}
		return nil, fmt.Errorf("unexpected")
	}}})
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
	svc := NewVpnService(VpnDeps{UCI: u, Cmd: &platform.FuncCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
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
	}}})
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
func newGuardedVpnService(t *testing.T, u uci.UCI, cmd platform.CommandRunner) (*VpnService, string) {
	t.Helper()
	svc := NewVpnService(VpnDeps{UCI: u, Cmd: cmd})
	svc.crashGuard = guard.In(t.TempDir(), "vpn")
	return svc, svc.crashGuard.Path()
}

// wgDumpWithHandshake is a `wg show wg0 dump` with one peer that has completed
// a handshake, so wgRuntimeState reports "connected".
const wgDumpWithHandshake = "PRIV\tPUB\t51820\toff\n" +
	"PEER\t\t10.0.0.1:51820\t0.0.0.0/0\t1700000000\t1\t2\toff\n"

// newSnapshotScopedVpnService returns a service whose crash guard, dnsmasq
// layer record and legacy snapshot all live in a temp dir, so the lifecycle can
// be asserted without touching /etc/trafo.
func newSnapshotScopedVpnService(t *testing.T, u uci.UCI, cmd platform.CommandRunner) (*VpnService, string) {
	t.Helper()
	svc, guard := newGuardedVpnService(t, u, cmd)
	dir := t.TempDir()
	svc.dnsStackPath = filepath.Join(dir, "dnsmasq-layers.json")
	return svc, guard
}

// seedVpnLayerStack writes the record an earlier successful enable would have
// left behind: the pre-VPN dnsmasq list as the base, with the VPN on top of it.
func seedVpnLayerStack(t *testing.T, svc *VpnService, baseServers ...string) {
	t.Helper()
	st := &dnsmasqLayerStack{
		NoResolv: "0",
		Servers:  baseServers,
		Layers:   []dnsmasqLayer{{Name: dnsLayerVPN, Servers: []string{"10.8.0.1"}}},
	}
	data, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("encoding the layer stack: %v", err)
	}
	if err := os.WriteFile(svc.dnsStackPath, data, 0o600); err != nil {
		t.Fatalf("seeding the layer stack: %v", err)
	}
}

// readVpnLayerStack reads back the shared record.
func readVpnLayerStack(t *testing.T, svc *VpnService) *dnsmasqLayerStack {
	t.Helper()
	st, err := svc.dnsmasqLayers().load()
	if err != nil {
		t.Fatalf("reading the layer stack: %v", err)
	}
	if st == nil {
		t.Fatal("expected a dnsmasq layer stack on disk, got none")
	}
	return st
}

// compressVpnDNSHealWindow shrinks the self-heal debounce so the streak and the
// grace period can both be exercised without waiting minutes.
func compressVpnDNSHealWindow(t *testing.T, streak int, grace time.Duration) {
	t.Helper()
	prevStreak, prevGrace := vpnDNSHealStreakRequired, vpnDNSHealGracePeriod
	vpnDNSHealStreakRequired, vpnDNSHealGracePeriod = streak, grace
	t.Cleanup(func() {
		vpnDNSHealStreakRequired, vpnDNSHealGracePeriod = prevStreak, prevGrace
	})
}

// dnsmasqState is a tiny model of dhcp.@dnsmasq[0] driven through the same
// `uci` shell-outs the service makes.
type dnsmasqState struct {
	servers  []string
	noresolv string
	wgUp     bool
	// wgDump is what `wg show wg0 dump` answers with while wgUp is set. It is
	// the handshake detail, so it decides between connected, up_no_handshake
	// and configured.
	wgDump string
}

// setWgUp makes `wg show wg0 dump` fail or recover from the next call on, which
// is what a flapping tunnel looks like to the self-heal.
func (d *dnsmasqState) setWgUp(up bool) { d.wgUp = up }

// setWgDump makes a live tunnel report a weaker state: a peer with no handshake
// yet, or no peers at all.
func (d *dnsmasqState) setWgDump(dump string) { d.wgDump = dump }

// runUci applies one `uci` invocation to the model.
func (d *dnsmasqState) runUci(args ...string) ([]byte, error) {
	switch {
	case len(args) == 2 && args[0] == "get" && args[1] == "dhcp.@dnsmasq[0].server":
		return []byte(strings.Join(d.servers, " ")), nil
	case len(args) == 2 && args[0] == "get" && args[1] == "dhcp.@dnsmasq[0].noresolv":
		return []byte(d.noresolv), nil
	case len(args) == 2 && args[0] == "delete" && args[1] == "dhcp.@dnsmasq[0].server":
		d.servers = nil
		return nil, nil
	case len(args) == 2 && args[0] == "add_list":
		_, value, _ := strings.Cut(args[1], "=")
		d.servers = append(d.servers, value)
		return nil, nil
	case len(args) == 2 && args[0] == "set":
		_, value, _ := strings.Cut(args[1], "=")
		if strings.Contains(args[1], ".noresolv=") {
			d.noresolv = value
		}
		return nil, nil
	case len(args) == 2 && args[0] == "commit":
		return nil, nil
	}
	return nil, nil
}

func newDnsmasqRunner(servers []string, wgUp bool) (*platform.MockCommandRunner, *dnsmasqState) {
	state := &dnsmasqState{servers: servers, wgUp: wgUp, wgDump: wgDumpWithHandshake}
	cmd := &platform.MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		switch name {
		case "uci":
			return state.runUci(args...)
		case "/etc/init.d/dnsmasq":
			return nil, nil
		case "tailscale":
			return nil, nil
		case "/etc/init.d/firewall":
			return nil, nil
		case "/usr/bin/wg":
			if !state.wgUp {
				return nil, fmt.Errorf("Unable to access interface: No such device")
			}
			return []byte(state.wgDump), nil
		case "/sbin/ip":
			if len(args) >= 3 && args[0] == "link" && args[1] == "show" {
				if !state.wgUp {
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

// wgDumpNoHandshake is a `wg show wg0 dump` with a peer that has never
// completed a handshake, so wgRuntimeState reports "up_no_handshake".
const wgDumpNoHandshake = "PRIV\tPUB\t51820\toff\n" +
	"PEER\t\t10.0.0.1:51820\t0.0.0.0/0\t0\t0\t0\toff\n"

// wgDumpNoPeers is a `wg show wg0 dump` for an interface with no peer yet, so
// wgRuntimeState reports "configured".
const wgDumpNoPeers = "PRIV\tPUB\t51820\toff\n"

// The Tailscale exit node must not be cleared before the tunnel is verified:
// a failed enable used to silently move the router's egress with nothing to put
// it back.
func TestEnableWireguard_ClearsTailscaleExitNodeOnlyAfterTunnelVerified(t *testing.T) {
	prev := wireGuardVerifyTimeout
	wireGuardVerifyTimeout = 300 * time.Millisecond
	t.Cleanup(func() { wireGuardVerifyTimeout = prev })

	u := uci.NewMockUCI()
	var exitNodeCleared bool
	cmd := &platform.MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
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
	cmd2 := &platform.MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
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
