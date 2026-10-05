package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/uci"
)

// fakeDevice is a CommandRunner that models the parts of a router the DNS
// layering actually depends on: the shelled-out `uci` writes to dnsmasq's
// resolver options, `uci commit dhcp`, the dnsmasq restart, and the tunnel and
// route probes a VPN toggle makes.
//
// The dnsmasq options live in a NAMED section (cfg01411c on the device), which
// is what `dhcp.@dnsmasq[0]` resolves to — an anonymous index by type, not a
// literal section name. Modelling it that way is what keeps a test from
// inventing a config layout the real system does not have.
type fakeDevice struct {
	mu        sync.Mutex
	section   string
	servers   []string
	noresolv  string
	rebind    string
	tunnelUp  bool
	wgToolOut string
	calls     []string
}

func newFakeDevice() *fakeDevice {
	return &fakeDevice{section: "cfg01411c", noresolv: "0"}
}

func (d *fakeDevice) Run(name string, args ...string) ([]byte, error) {
	key := name
	if len(args) > 0 {
		key += " " + strings.Join(args, " ")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, strings.TrimSpace(name+" "+strings.Join(args, " ")))

	switch {
	case key == "uci get dhcp.@dnsmasq[0].server":
		if len(d.servers) == 0 {
			return nil, fmt.Errorf("uci: entry not found")
		}
		return []byte(strings.Join(d.servers, " ")), nil
	case key == "uci get dhcp.@dnsmasq[0].noresolv":
		if d.noresolv == "" {
			return nil, fmt.Errorf("uci: entry not found")
		}
		return []byte(d.noresolv), nil
	case key == "uci get dhcp.@dnsmasq[0].rebind_protection":
		if d.rebind == "" {
			return []byte("0"), nil
		}
		return []byte(d.rebind), nil
	case key == "uci delete dhcp.@dnsmasq[0].server":
		d.servers = nil
		return nil, nil
	case strings.HasPrefix(key, "uci add_list dhcp.@dnsmasq[0].server="):
		d.servers = append(d.servers, strings.TrimPrefix(key, "uci add_list dhcp.@dnsmasq[0].server="))
		return nil, nil
	case strings.HasPrefix(key, "uci set dhcp.@dnsmasq[0].noresolv="):
		d.noresolv = strings.TrimPrefix(key, "uci set dhcp.@dnsmasq[0].noresolv=")
		return nil, nil
	case strings.HasPrefix(key, "uci set dhcp.@dnsmasq[0].rebind_protection="):
		d.rebind = strings.TrimPrefix(key, "uci set dhcp.@dnsmasq[0].rebind_protection=")
		return nil, nil
	case key == "uci commit dhcp":
		return nil, nil
	case key == "/etc/init.d/dnsmasq restart":
		return nil, nil
	case key == "/usr/bin/wg show wg0 dump":
		if !d.tunnelUp {
			return nil, fmt.Errorf("Unable to access interface: No such device")
		}
		return []byte(d.wgToolOut), nil
	case key == "/sbin/ip link show dev wg0":
		if !d.tunnelUp {
			return []byte("3: wg0: <NOARP> mtu 1420 state DOWN"), nil
		}
		return []byte("3: wg0: <POINTOPOINT,NOARP,UP,LOWER_UP> mtu 1420 state UNKNOWN"), nil
	case key == "/sbin/ip route show default":
		return []byte("default via 203.0.113.1 dev wan"), nil
	}
	// Everything else (ubus, ifup/ifdown, init.d/firewall, tailscale) is
	// irrelevant to DNS and succeeds silently on a router.
	return nil, nil
}

// resolvers is the dnsmasq state an operator would see: the forwarders and
// whether dnsmasq may fall back to resolv.conf.
// recordedCalls returns the uci/dnsmasq invocations the restore path issued.
func (d *fakeDevice) recordedCalls() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.calls)
}

func (d *fakeDevice) resolvers(t *testing.T) (servers []string, noresolv string) {
	t.Helper()
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.servers), d.noresolv
}

// newLayerTestVpn returns a VPN service whose tunnel verifies up, pointed at a
// resolver inside the tunnel, with its layer record and legacy snapshot in dir.
func newLayerTestVpn(t *testing.T, u uci.UCI, dev *fakeDevice, dir string) *VpnService {
	t.Helper()
	dev.tunnelUp = true
	dev.wgToolOut = wgDumpWithHandshake
	if err := u.Set("network", "wg0", "dns", "10.8.0.1"); err != nil {
		t.Fatalf("set wg0 dns: %v", err)
	}
	svc := NewVpnServiceWithRunner(u, dev)
	svc.guardFile = ""
	svc.dnsStackPath = filepath.Join(dir, dnsmasqLayerStackFileName)
	svc.legacyDnsSnapshotPath = filepath.Join(dir, "vpn-dns-snapshot.json")
	return svc
}

// newLayerTestCaptive returns a captive service whose bypass guard, DHCP
// nameserver file and layer record all live in dir.
func newLayerTestCaptive(t *testing.T, u uci.UCI, dev *fakeDevice, dir string) *CaptiveService {
	t.Helper()
	resolv := filepath.Join(dir, "resolv.conf.auto")
	if err := os.WriteFile(resolv, []byte("# dhcp\nnameserver 10.1.2.3\n"), 0o600); err != nil {
		t.Fatalf("write resolv.conf.auto: %v", err)
	}
	return &CaptiveService{
		prober:                 &MockHTTPProber{StatusCode: 204},
		uci:                    u,
		cmd:                    dev,
		guardFile:              filepath.Join(dir, "captive-dns-in-progress"),
		stopCh:                 make(chan struct{}),
		dnsStackPath:           filepath.Join(dir, dnsmasqLayerStackFileName),
		resolvConfAutoOverride: resolv,
	}
}

// TestCaptiveBypassNeverBecomesTheDnsmasqBaseState walks the sequence an
// operator actually performs, and asserts the one thing that matters at the end:
// LAN DNS is not left pointing into a tunnel that is not there.
//
// Captive bypass and the layer stack were two independent owners of dnsmasq's
// server/noresolv. A bypass took a snapshot outside the stack, so the record
// survived the bypass but no longer described it; the next VPN enable found a
// record, and the next VPN disable removed the last layer and restored a base
// the bypass had silently replaced. Once the record was gone entirely, the
// following enable adopted the VPN's own tunnel resolver as the "pre-any-layer"
// base — noresolv=1 included, so there was no fallback left either.
func TestCaptiveBypassNeverBecomesTheDnsmasqBaseState(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dev := newFakeDevice()
	// The operator's own split-DNS entry: the state that must survive the whole
	// sequence and be what the router returns to.
	dev.servers = []string{"/lan.example.com/192.168.1.10"}
	dev.noresolv = "0"
	u := uci.NewMockUCI()
	vpn := newLayerTestVpn(t, u, dev, dir)
	captive := newLayerTestCaptive(t, u, dev, dir)
	operatorServers := slices.Clone(dev.servers)

	if err := vpn.ToggleWireguard(true); err != nil {
		t.Fatalf("enable VPN: %v", err)
	}
	if servers, noresolv := dev.resolvers(t); !slices.Equal(servers, []string{"10.8.0.1"}) || noresolv != "1" {
		t.Fatalf("after enabling the VPN, dnsmasq = %v noresolv=%q; want the tunnel resolver with noresolv=1", servers, noresolv)
	}

	if err := captive.BypassDNS(); err != nil {
		t.Fatalf("captive bypass: %v", err)
	}
	if servers, noresolv := dev.resolvers(t); !slices.Equal(servers, []string{"10.1.2.3"}) || noresolv != "0" {
		t.Fatalf("after the bypass, dnsmasq = %v noresolv=%q; want the hotel resolver with noresolv=0", servers, noresolv)
	}

	if err := vpn.ToggleWireguard(false); err != nil {
		t.Fatalf("disable VPN under bypass: %v", err)
	}
	if servers, noresolv := dev.resolvers(t); !slices.Equal(servers, []string{"10.1.2.3"}) || noresolv != "0" {
		t.Fatalf("the VPN disable stole the bypass: dnsmasq = %v noresolv=%q", servers, noresolv)
	}

	if err := captive.RestoreDNS(); err != nil {
		t.Fatalf("captive restore: %v", err)
	}
	if servers, _ := dev.resolvers(t); !slices.Equal(servers, operatorServers) {
		t.Fatalf("after restoring the bypass, dnsmasq = %v; want the operator's %v", servers, operatorServers)
	}

	if err := vpn.ToggleWireguard(true); err != nil {
		t.Fatalf("re-enable VPN: %v", err)
	}
	if err := vpn.ToggleWireguard(false); err != nil {
		t.Fatalf("disable VPN again: %v", err)
	}

	servers, noresolv := dev.resolvers(t)
	if slices.Contains(servers, "10.8.0.1") {
		t.Errorf("LAN DNS forwards into the dead tunnel resolver %q: dnsmasq = %v noresolv=%q", "10.8.0.1", servers, noresolv)
	}
	if !slices.Equal(servers, operatorServers) {
		t.Errorf("dnsmasq servers = %v, want the operator's %v", servers, operatorServers)
	}
	if noresolv != "0" {
		t.Errorf("dnsmasq noresolv = %q, want 0 so resolution can fall back to resolv.conf", noresolv)
	}
}

// The bypass must restore the layer it was stacked on, not just "something that
// was there before": the VPN resolver has to come back on its own.
func TestCaptiveRestorePutsBackTheLayerBeneathTheBypass(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dev := newFakeDevice()
	dev.servers = []string{"/lan.example.com/192.168.1.10"}
	u := uci.NewMockUCI()
	vpn := newLayerTestVpn(t, u, dev, dir)
	captive := newLayerTestCaptive(t, u, dev, dir)

	if err := vpn.ToggleWireguard(true); err != nil {
		t.Fatalf("enable VPN: %v", err)
	}
	if err := captive.BypassDNS(); err != nil {
		t.Fatalf("captive bypass: %v", err)
	}
	if err := captive.RestoreDNS(); err != nil {
		t.Fatalf("captive restore: %v", err)
	}

	servers, noresolv := dev.resolvers(t)
	if !slices.Equal(servers, []string{"10.8.0.1"}) || noresolv != "1" {
		t.Errorf("dnsmasq = %v noresolv=%q; want the VPN layer back on top of the stack", servers, noresolv)
	}
	if _, err := os.Stat(filepath.Join(dir, dnsmasqLayerStackFileName)); err != nil {
		t.Errorf("the layer record must survive a captive bypass: %v", err)
	}
}

// The legacy wan.peerdns=0 path blocks portal resolution without dnsmasq
// having noresolv=1, so the bypass pushes no dnsmasq layer and never removes a
// resolver. The restore must therefore not touch the resolver list either: it
// holds no copy of it, and deleting it destroys a split-DNS entry the operator
// configured that nothing will ever put back.
func TestCaptiveRestoreKeepsOperatorResolversOnThePeerdnsBypassPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dev := newFakeDevice()
	operatorServers := []string{"/lan.example.com/192.168.1.10", "/vpn.example.com/10.8.0.1"}
	dev.servers = slices.Clone(operatorServers)
	dev.noresolv = "0"
	u := uci.NewMockUCI()
	if err := u.Set("network", "wan", "peerdns", "0"); err != nil {
		t.Fatalf("set wan peerdns: %v", err)
	}
	if err := u.Set("network", "wan", "dns", "9.9.9.9"); err != nil {
		t.Fatalf("set wan dns: %v", err)
	}
	captive := newLayerTestCaptive(t, u, dev, dir)

	if err := captive.BypassDNS(); err != nil {
		t.Fatalf("captive bypass: %v", err)
	}
	if !captive.IsDNSBypassed() {
		t.Fatalf("precondition: peerdns=0 with wan.dns set must trigger a bypass")
	}
	if servers, noresolv := dev.resolvers(t); !slices.Equal(servers, operatorServers) || noresolv != "0" {
		t.Fatalf("after the bypass, dnsmasq = %v noresolv=%q; want the operator's resolvers untouched", servers, noresolv)
	}

	if err := captive.RestoreDNS(); err != nil {
		t.Fatalf("captive restore: %v", err)
	}
	servers, noresolv := dev.resolvers(t)
	if !slices.Equal(servers, operatorServers) {
		t.Errorf("the restore destroyed resolvers it never recorded: dnsmasq = %v; want the operator's %v", servers, operatorServers)
	}
	if noresolv != "0" {
		t.Errorf("dnsmasq noresolv = %q, want 0", noresolv)
	}
	for _, call := range dev.recordedCalls() {
		if strings.HasPrefix(call, "uci delete dhcp.") {
			t.Errorf("the restore staged %q with no recorded resolver list to put back", call)
		}
	}
}

// A guard file written before the layer stack existed DOES carry the resolver
// list it is responsible for restoring. That restore must still work, and the
// staged delete it needs must be committed rather than left for the next
// unrelated `uci commit dhcp` to flush.
func TestPreStackGuardRestoresRecordedResolversAndCommits(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dev := newFakeDevice()
	dev.servers = []string{"10.1.2.3"}
	dev.noresolv = "0"
	u := uci.NewMockUCI()
	captive := newLayerTestCaptive(t, u, dev, dir)

	recorded := []string{"/corp.example.com/10.0.0.2", "192.0.2.53"}
	data, err := json.Marshal(dnsBackup{
		DnsmasqNoResolv: "0",
		DnsmasqServers:  recorded,
		Time:            time.Now().Unix(),
	})
	if err != nil {
		t.Fatalf("marshal guard: %v", err)
	}
	if err := os.WriteFile(captive.guardFile, data, 0o600); err != nil {
		t.Fatalf("write guard: %v", err)
	}

	if err := captive.RestoreDNS(); err != nil {
		t.Fatalf("restore from a pre-stack guard: %v", err)
	}
	if servers, _ := dev.resolvers(t); !slices.Equal(servers, recorded) {
		t.Errorf("dnsmasq = %v; want the recorded resolvers %v back", servers, recorded)
	}

	calls := dev.recordedCalls()
	deleteAt, commitAt := -1, -1
	for i, call := range calls {
		if deleteAt < 0 && strings.HasPrefix(call, "uci delete dhcp.") {
			deleteAt = i
		}
		if call == "uci commit dhcp" {
			commitAt = i
		}
	}
	if deleteAt < 0 || commitAt < 0 {
		t.Fatalf("restore must replace the resolver list and commit it; calls = %v", calls)
	}
	if commitAt < deleteAt {
		t.Errorf("the staged resolver delete was never committed; calls = %v", calls)
	}
}

// An unreadable layer record is not an empty one. When the record that
// describes who owns dnsmasq cannot be read, nothing may be written to
// dnsmasq and the restore has to fail loudly with the guard file intact.
func TestCaptiveRestoreFailsClosedOnAnUnreadableLayerRecord(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dev := newFakeDevice()
	operatorServers := []string{"/corp.example.com/10.0.0.2"}
	dev.servers = slices.Clone(operatorServers)
	u := uci.NewMockUCI()
	captive := newLayerTestCaptive(t, u, dev, dir)

	if err := os.WriteFile(captive.guardFile, []byte(`{"time":`), 0o600); err != nil {
		t.Fatalf("write guard: %v", err)
	}
	// A torn record: written, but not valid JSON.
	if err := os.WriteFile(filepath.Join(dir, dnsmasqLayerStackFileName), []byte(`{"layers":[{"name":"vpn"`), 0o600); err != nil {
		t.Fatalf("write layer record: %v", err)
	}

	if err := captive.RestoreDNS(); err == nil {
		t.Error("a torn layer record must make the restore fail loudly, not proceed")
	}
	if servers, _ := dev.resolvers(t); !slices.Equal(servers, operatorServers) {
		t.Errorf("dnsmasq = %v; an unreadable record must leave it untouched", servers)
	}
	if _, err := os.Stat(captive.guardFile); err != nil {
		t.Errorf("the guard file must survive a failed restore: %v", err)
	}
}

// The layer record is STATE, not a crash guard: a partial /etc/trafo loss, or a
// cleanup from an older package layout, can take it while the guard file
// survives. The guard records dnsmasq's noresolv flag but never the resolver
// list, so with the record gone nothing can say what the list used to be — and
// the hotel resolver the bypass pushed is still sitting in it. Restoring only
// noresolv there leaves dnsmasq forwarding ALL DNS to the hotel resolver while
// RestoreDNS returns nil, restarts dnsmasq, logs "DNS restored" and deletes the
// guard, so nothing ever retries.
//
// The restore must therefore refuse loudly and keep the guard: the bypass being
// still in force is a state the operator has to see, not one to paper over.
func TestCaptiveRestoreRefusesWhenTheLayerRecordIsLost(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dev := newFakeDevice()
	dev.servers = []string{"/corp.example.com/10.0.0.2"}
	dev.noresolv = "1"
	u := uci.NewMockUCI()
	captive := newLayerTestCaptive(t, u, dev, dir)

	if err := captive.BypassDNS(); err != nil {
		t.Fatalf("captive bypass: %v", err)
	}
	if servers, noresolv := dev.resolvers(t); !slices.Equal(servers, []string{"10.1.2.3"}) || noresolv != "0" {
		t.Fatalf("after the bypass, dnsmasq = %v noresolv=%q; want the hotel resolver with noresolv=0", servers, noresolv)
	}

	// The record is gone; the guard file is all that is left.
	if err := os.Remove(filepath.Join(dir, dnsmasqLayerStackFileName)); err != nil {
		t.Fatalf("delete the layer record: %v", err)
	}

	err := captive.RestoreDNS()
	if err == nil {
		servers, noresolv := dev.resolvers(t)
		t.Errorf("a lost layer record must not report success: dnsmasq = %v noresolv=%q still forwards to the hotel resolver",
			servers, noresolv)
	}
	if !strings.Contains(err.Error(), dnsmasqLayerStackFileName) {
		t.Errorf("the error must name the missing layer record, got: %v", err)
	}
	if !captive.IsDNSBypassed() {
		t.Error("the guard file must survive so a later attempt can retry")
	}
	if servers, noresolv := dev.resolvers(t); !slices.Equal(servers, []string{"10.1.2.3"}) || noresolv != "0" {
		t.Errorf("a refused restore must not half-apply: dnsmasq = %v noresolv=%q, want the bypassed state", servers, noresolv)
	}
}

// A bypass that never owned dnsmasq's resolver list (the legacy peerdns /
// AdGuard-encrypted paths) has no layer to pop and nothing to put back, so the
// no-record case must NOT refuse there: it would leave the guard and the
// "bypassed" marker in place forever for a bypass that is already gone.
func TestCaptiveRestoreSucceedsWithoutARecordWhenNoLayerWasEverTaken(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dev := newFakeDevice()
	operatorServers := []string{"/corp.example.com/10.0.0.2"}
	dev.servers = slices.Clone(operatorServers)
	dev.noresolv = "0"
	u := uci.NewMockUCI()
	if err := u.Set("network", "wan", "peerdns", "0"); err != nil {
		t.Fatalf("set wan peerdns: %v", err)
	}
	if err := u.Set("network", "wan", "dns", "9.9.9.9"); err != nil {
		t.Fatalf("set wan dns: %v", err)
	}
	captive := newLayerTestCaptive(t, u, dev, dir)

	if err := captive.BypassDNS(); err != nil {
		t.Fatalf("captive bypass: %v", err)
	}
	if err := os.Remove(filepath.Join(dir, dnsmasqLayerStackFileName)); err != nil && !os.IsNotExist(err) {
		t.Fatalf("delete the layer record: %v", err)
	}

	if err := captive.RestoreDNS(); err != nil {
		t.Fatalf("a bypass that never took a resolver layer must still restore cleanly: %v", err)
	}
	if captive.IsDNSBypassed() {
		t.Error("the guard file must be removed after a successful restore")
	}
	if servers, _ := dev.resolvers(t); !slices.Equal(servers, operatorServers) {
		t.Errorf("dnsmasq = %v; want the operator's %v", servers, operatorServers)
	}
}

// The read-path heal keeps the record so a later explicit disable still has a
// base. That base must not outlive its usefulness: with nothing stacked,
// dnsmasq belongs to the operator again, and a resolver they added in LuCI
// afterwards is the state a disable has to restore — not whatever was recorded
// before the heal.
func TestLayerBaseIsRereadAfterTheHealLeftAnEmptyRecord(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dev := newFakeDevice()
	dev.servers = []string{"192.0.2.53"}
	dev.noresolv = "0"
	u := uci.NewMockUCI()
	vpn := newLayerTestVpn(t, u, dev, dir)

	if err := vpn.ToggleWireguard(true); err != nil {
		t.Fatalf("enable VPN: %v", err)
	}
	// The heal: the tunnel is terminal, so the layer is popped and the record
	// deliberately survives.
	dev.tunnelUp = false
	vpn.maybeSelfHealVpnDNS("disabled")
	if _, err := os.Stat(filepath.Join(dir, dnsmasqLayerStackFileName)); err != nil {
		t.Fatalf("precondition: the heal must keep the record: %v", err)
	}

	// The operator edits dnsmasq in LuCI while no layer is stacked.
	dev.servers = []string{"192.0.2.53", "/corp.example.com/10.0.0.2"}

	dev.tunnelUp = true
	if err := vpn.ToggleWireguard(true); err != nil {
		t.Fatalf("re-enable VPN: %v", err)
	}
	if err := vpn.ToggleWireguard(false); err != nil {
		t.Fatalf("disable VPN: %v", err)
	}
	if servers, _ := dev.resolvers(t); !slices.Equal(servers, []string{"192.0.2.53", "/corp.example.com/10.0.0.2"}) {
		t.Errorf("dnsmasq = %v; want the operator's post-heal state, not the base recorded before it", servers)
	}
}

// The record is the only restore target for the resolvers, so once it has
// absorbed a pre-stack snapshot, every pre-stack snapshot has to go: two files
// claiming the same restore target is the collision the stack removes. Only the
// saving feature's own file used to be deleted, leaving the other's behind.
func TestBothPreStackSnapshotsAreRemovedWithTheRecord(t *testing.T) {
	dir := t.TempDir()
	vpnSnap := filepath.Join(dir, "vpn-dns-snapshot.json")
	agSnap := filepath.Join(dir, "adguard-dns-snapshot.json")
	for _, p := range []string{vpnSnap, agSnap} {
		if err := os.WriteFile(p, []byte(`{"noresolv":"0","servers":["192.0.2.53"]}`), 0o600); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
	saved := slices.Clone(legacyDnsmasqSnapshotPaths)
	legacyDnsmasqSnapshotPaths = []string{vpnSnap, agSnap}
	defer func() { legacyDnsmasqSnapshotPaths = saved }()

	dev := newFakeDevice()
	u := uci.NewMockUCI()
	vpn := newLayerTestVpn(t, u, dev, dir)
	if err := vpn.ToggleWireguard(true); err != nil {
		t.Fatalf("enable VPN: %v", err)
	}

	for _, p := range []string{vpnSnap, agSnap} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s must be removed once the shared record owns the base state", filepath.Base(p))
		}
	}
}

// A `wg show` that never works is a broken PROBE, not a broken tunnel. The heal
// is debounced so one failed poll cannot take the DNS layer away, but a
// persistently broken probe used to satisfy the debounce and pop the VPN
// resolvers of a tunnel that was carrying them perfectly well.
func TestBrokenWgProbeDoesNotHealAwayALiveTunnel(t *testing.T) {
	dir := t.TempDir()
	dev := newFakeDevice()
	u := uci.NewMockUCI()
	vpn := newLayerTestVpn(t, u, dev, dir)
	if err := vpn.ToggleWireguard(true); err != nil {
		t.Fatalf("enable VPN: %v", err)
	}

	// The tunnel stays up; `wg show` stops returning anything usable on every
	// poll, so the link state is the only evidence left.
	dev.wgToolOut = ""

	var heals int
	for range vpnDNSHealStreakRequired + 1 {
		if vpn.maybeSelfHealVpnDNS(vpn.wgRuntimeState(true)) {
			heals++
		}
	}
	if heals != 0 {
		t.Errorf("the heal fired %d time(s) while the tunnel was up and only the probe was broken", heals)
	}
	if servers, noresolv := dev.resolvers(t); !slices.Equal(servers, []string{"10.8.0.1"}) || noresolv != "1" {
		t.Errorf("dnsmasq = %v noresolv=%q; the VPN resolver layer must survive a broken probe", servers, noresolv)
	}
}
