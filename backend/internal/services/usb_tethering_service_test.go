package services

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const mockFirewallShow = `firewall.@zone[0]=zone
firewall.@zone[0].name='lan'
firewall.@zone[0].input='ACCEPT'
firewall.@zone[0].output='ACCEPT'
firewall.@zone[0].network='lan'
firewall.@zone[1]=zone
firewall.@zone[1].name='wan'
firewall.@zone[1].input='REJECT'
firewall.@zone[1].output='ACCEPT'
firewall.@zone[1].forward='REJECT'
firewall.@zone[1].masq='1'
firewall.@zone[1].network='wan'
firewall.@zone[1].network='wwan'
`

// mockUSBTetherRunner simulates OS calls.
type mockUSBTetherRunner struct {
	usbIfaces  map[string]bool // iface name -> is USB
	ifaceUp    map[string]bool
	ifaceIP    map[string]string
	uciOutputs map[string]struct {
		out string
		err error
	}
	commands []string // recorded commands
}

func newMockUSBTetherRunner() *mockUSBTetherRunner {
	return &mockUSBTetherRunner{
		usbIfaces: make(map[string]bool),
		ifaceUp:   make(map[string]bool),
		ifaceIP:   make(map[string]string),
		uciOutputs: make(map[string]struct {
			out string
			err error
		}),
	}
}

func (m *mockUSBTetherRunner) ReadSymlink(path string) (string, error) {
	// Simulate USB path for usb-backed interfaces.
	for name := range m.usbIfaces {
		if path == "/sys/class/net/"+name+"/device" && m.usbIfaces[name] {
			return "/sys/bus/usb/devices/1-1/usb0", nil
		}
	}
	return path, nil
}

func (m *mockUSBTetherRunner) ReadFile(path string) (string, error) {
	return "", nil
}

func (m *mockUSBTetherRunner) DirExists(path string) bool {
	// The device directory exists for known USB ifaces.
	for name := range m.usbIfaces {
		if path == "/sys/class/net/"+name+"/device" {
			return true
		}
	}
	return false
}

func (m *mockUSBTetherRunner) GetIfaceIP(iface string) string {
	return m.ifaceIP[iface]
}

func (m *mockUSBTetherRunner) IsIfaceUp(iface string) bool {
	return m.ifaceUp[iface]
}

func (m *mockUSBTetherRunner) RunCommand(name string, args ...string) (string, error) {
	key := name
	for _, a := range args {
		key += " " + a
	}
	m.commands = append(m.commands, key)
	if r, ok := m.uciOutputs[key]; ok {
		return r.out, r.err
	}
	if key == "uci show firewall" {
		return mockFirewallShow, nil
	}
	return "", nil
}

// newTestUSBTetherService returns a service whose crash guard lives in a temp dir.
func newTestUSBTetherService(t *testing.T, r *mockUSBTetherRunner) *USBTetheringService {
	t.Helper()
	svc := NewUSBTetheringServiceWithRunner(r)
	svc.guardFile = filepath.Join(t.TempDir(), "usbtether-in-progress")
	return svc
}

func TestGetUSBTetherStatus_NoDevice(t *testing.T) {
	runner := newMockUSBTetherRunner()
	svc := NewUSBTetheringServiceWithRunner(runner)
	status := svc.GetStatus()
	if status.Detected {
		t.Error("expected Detected=false when no USB iface")
	}
}

func TestGetUSBTetherStatus_AndroidDetected(t *testing.T) {
	runner := newMockUSBTetherRunner()
	runner.usbIfaces["usb0"] = true
	runner.ifaceUp["usb0"] = true
	runner.ifaceIP["usb0"] = "192.168.42.129"
	svc := NewUSBTetheringServiceWithRunner(runner)
	status := svc.GetStatus()
	if !status.Detected {
		t.Error("expected Detected=true for USB-backed usb0")
	}
	if status.Interface != "usb0" {
		t.Errorf("expected Interface=usb0, got %q", status.Interface)
	}
	if !status.IsUp {
		t.Error("expected IsUp=true")
	}
	if status.IPAddress != "192.168.42.129" {
		t.Errorf("expected IP 192.168.42.129, got %q", status.IPAddress)
	}
	if status.DeviceType != "android" {
		t.Errorf("expected DeviceType=android, got %q", status.DeviceType)
	}
}

func TestConfigure_SetsUCIAndBringsUpInterface(t *testing.T) {
	runner := newMockUSBTetherRunner()
	svc := newTestUSBTetherService(t, runner)
	err := svc.Configure("usb0")
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}

	// Verify key commands were called.
	seen := make(map[string]bool)
	for _, cmd := range runner.commands {
		seen[cmd] = true
	}
	if !seen["uci set network.usbtether=interface"] {
		t.Error("expected uci set network.usbtether=interface")
	}
	if !seen["uci set network.usbtether.device=usb0"] {
		t.Error("expected uci set network.usbtether.device=usb0")
	}
	if !seen["uci commit network"] {
		t.Error("expected uci commit network")
	}
	if !seen["uci commit firewall"] {
		t.Error("expected uci commit firewall")
	}
	if !seen["uci add_list firewall.@zone[1].network=usbtether"] {
		t.Errorf("expected usbtether added to the resolved wan zone, got %v", runner.commands)
	}
	if !seen["ifup usbtether"] {
		t.Error("expected ifup usbtether")
	}
	if _, err := os.Stat(svc.guardFile); !os.IsNotExist(err) {
		t.Errorf("expected crash guard removed after success, stat err = %v", err)
	}
}

func TestConfigure_ResolvesWanZoneByNameNotIndex(t *testing.T) {
	runner := newMockUSBTetherRunner()
	// wan is the third zone here, so a hard-coded @zone[1] would target "dmz".
	runner.uciOutputs["uci show firewall"] = struct {
		out string
		err error
	}{out: `firewall.@zone[0].name='lan'
firewall.@zone[0].input='ACCEPT'
firewall.@zone[0].network='lan'
firewall.@zone[1].name='dmz'
firewall.@zone[1].input='REJECT'
firewall.@zone[1].network='dmz'
firewall.@zone[2].name='wan'
firewall.@zone[2].input='REJECT'
firewall.@zone[2].network='wan'
`, err: nil}
	svc := newTestUSBTetherService(t, runner)

	if err := svc.Configure("usb0"); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	seen := map[string]bool{}
	for _, cmd := range runner.commands {
		seen[cmd] = true
	}
	if !seen["uci add_list firewall.@zone[2].network=usbtether"] {
		t.Errorf("expected add_list against the wan zone by name, got %v", runner.commands)
	}
	if seen["uci add_list firewall.@zone[1].network=usbtether"] {
		t.Error("must not assume firewall.@zone[1] is the wan zone")
	}
}

func TestConfigure_PropagatesAddListError(t *testing.T) {
	runner := newMockUSBTetherRunner()
	runner.uciOutputs["uci add_list firewall.@zone[1].network=usbtether"] = struct {
		out string
		err error
	}{out: "", err: errors.New("add_list failed")}
	svc := newTestUSBTetherService(t, runner)

	if err := svc.Configure("usb0"); err == nil {
		t.Fatal("expected Configure to fail when add_list fails")
	}
	// ADR 0003 §1.3: a failure keeps the guard — it is the only marker that the
	// device was left mid-flight.
	if _, err := os.Stat(svc.guardFile); err != nil {
		t.Errorf("crash guard must survive a failed Configure, stat err = %v", err)
	}
	seen := map[string]bool{}
	for _, cmd := range runner.commands {
		seen[cmd] = true
	}
	if !seen["uci revert firewall"] || !seen["uci revert network"] {
		t.Errorf("a failed Configure must revert the staged uci delta, got %v", runner.commands)
	}
}

func TestConfigure_KeepsGuardWhenUciSetFails(t *testing.T) {
	runner := newMockUSBTetherRunner()
	runner.uciOutputs["uci set network.usbtether.device=usb0"] = struct {
		out string
		err error
	}{out: "", err: errors.New("set failed")}
	svc := newTestUSBTetherService(t, runner)

	if err := svc.Configure("usb0"); err == nil {
		t.Fatal("expected Configure to fail when a uci set fails")
	}
	if _, err := os.Stat(svc.guardFile); err != nil {
		t.Errorf("crash guard must survive a failed uci set, stat err = %v", err)
	}
}

func TestConfigure_KeepsGuardWhenWanZoneLookupFails(t *testing.T) {
	runner := newMockUSBTetherRunner()
	runner.uciOutputs["uci show firewall"] = struct {
		out string
		err error
	}{out: "", err: errors.New("uci show firewall failed")}
	svc := newTestUSBTetherService(t, runner)

	if err := svc.Configure("usb0"); err == nil {
		t.Fatal("expected Configure to fail when the wan zone cannot be resolved")
	}
	if _, err := os.Stat(svc.guardFile); err != nil {
		t.Errorf("crash guard must survive a failed wan-zone lookup, stat err = %v", err)
	}
}

func TestConfigure_KeepsGuardWhenNetworkCommitFails(t *testing.T) {
	runner := newMockUSBTetherRunner()
	runner.uciOutputs["uci commit network"] = struct {
		out string
		err error
	}{out: "", err: errors.New("commit failed")}
	svc := newTestUSBTetherService(t, runner)

	if err := svc.Configure("usb0"); err == nil {
		t.Fatal("expected Configure to fail when the network commit fails")
	}
	if _, err := os.Stat(svc.guardFile); err != nil {
		t.Errorf("crash guard must survive a failed network commit, stat err = %v", err)
	}
}

func TestConfigure_PropagatesFirewallCommitError(t *testing.T) {
	runner := newMockUSBTetherRunner()
	runner.uciOutputs["uci commit firewall"] = struct {
		out string
		err error
	}{out: "", err: errors.New("commit failed")}
	svc := newTestUSBTetherService(t, runner)

	if err := svc.Configure("usb0"); err == nil {
		t.Fatal("expected Configure to fail when the firewall commit fails")
	}
	// The worst case: network is already committed (a live DHCP interface) while
	// the firewall membership is not. The guard must survive.
	if _, err := os.Stat(svc.guardFile); err != nil {
		t.Errorf("crash guard must survive a failed firewall commit, stat err = %v", err)
	}
	seen := map[string]bool{}
	for _, cmd := range runner.commands {
		seen[cmd] = true
	}
	if !seen["uci revert firewall"] {
		t.Error("a failed firewall commit must discard the staged zone delta")
	}
}

func TestConfigure_FailsWhenWanZoneMissing(t *testing.T) {
	runner := newMockUSBTetherRunner()
	runner.uciOutputs["uci show firewall"] = struct {
		out string
		err error
	}{out: "firewall.@zone[0].name='lan'\nfirewall.@zone[0].input='ACCEPT'\n", err: nil}
	svc := newTestUSBTetherService(t, runner)

	if err := svc.Configure("usb0"); err == nil {
		t.Fatal("expected Configure to fail when no wan zone exists")
	}
	if _, err := os.Stat(svc.guardFile); err != nil {
		t.Errorf("crash guard must survive a missing wan zone, stat err = %v", err)
	}
}

func TestConfigure_SkipsDuplicateAddList(t *testing.T) {
	runner := newMockUSBTetherRunner()
	runner.uciOutputs["uci show firewall"] = struct {
		out string
		err error
	}{out: `firewall.@zone[1].name='wan'
firewall.@zone[1].input='REJECT'
firewall.@zone[1].network='wan'
firewall.@zone[1].network='usbtether'
`, err: nil}
	svc := newTestUSBTetherService(t, runner)

	if err := svc.Configure("usb0"); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	for _, cmd := range runner.commands {
		if cmd == "uci add_list firewall.@zone[1].network=usbtether" {
			t.Error("expected no duplicate add_list when usbtether is already in the wan zone")
		}
	}
}

func TestConfigure_RequiresInterfaceName(t *testing.T) {
	runner := newMockUSBTetherRunner()
	svc := newTestUSBTetherService(t, runner)
	if err := svc.Configure("  "); err == nil {
		t.Fatal("expected error for an empty interface name")
	}
}

func TestUnconfigure_DeletesAndCommits(t *testing.T) {
	runner := newMockUSBTetherRunner()
	svc := newTestUSBTetherService(t, runner)
	err := svc.Unconfigure()
	if err != nil {
		t.Fatalf("Unconfigure: %v", err)
	}
	seen := make(map[string]bool)
	for _, cmd := range runner.commands {
		seen[cmd] = true
	}
	if !seen["uci delete network.usbtether"] {
		t.Error("expected uci delete network.usbtether")
	}
	if !seen["uci commit network"] {
		t.Error("expected uci commit network")
	}
	if _, err := os.Stat(svc.guardFile); !os.IsNotExist(err) {
		t.Errorf("expected crash guard removed after success, stat err = %v", err)
	}
}

func TestUnconfigure_DelistsUsbtetherFromWanZone(t *testing.T) {
	runner := newMockUSBTetherRunner()
	runner.uciOutputs["uci show firewall"] = struct {
		out string
		err error
	}{out: `firewall.@zone[1].name='wan'
firewall.@zone[1].input='REJECT'
firewall.@zone[1].network='wan'
firewall.@zone[1].network='usbtether'
`, err: nil}
	svc := newTestUSBTetherService(t, runner)

	if err := svc.Unconfigure(); err != nil {
		t.Fatalf("Unconfigure: %v", err)
	}
	seen := map[string]bool{}
	for _, cmd := range runner.commands {
		seen[cmd] = true
	}
	if !seen["uci del_list firewall.@zone[1].network=usbtether"] {
		t.Errorf("expected del_list from the wan zone, got %v", runner.commands)
	}
	if !seen["uci commit firewall"] {
		t.Error("expected firewall commit after del_list")
	}
}

func TestUnconfigure_KeepsInterfaceWhenDelListFails(t *testing.T) {
	runner := newMockUSBTetherRunner()
	runner.uciOutputs["uci show firewall"] = struct {
		out string
		err error
	}{out: `firewall.@zone[1].name='wan'
firewall.@zone[1].input='REJECT'
firewall.@zone[1].network='usbtether'
`, err: nil}
	runner.uciOutputs["uci del_list firewall.@zone[1].network=usbtether"] = struct {
		out string
		err error
	}{out: "", err: errors.New("del_list failed")}
	svc := newTestUSBTetherService(t, runner)

	if err := svc.Unconfigure(); err == nil {
		t.Fatal("expected Unconfigure to fail when del_list fails")
	}
	for _, cmd := range runner.commands {
		if cmd == "uci delete network.usbtether" {
			t.Error("must not delete the network interface while the firewall still references it")
		}
	}
	if _, err := os.Stat(svc.guardFile); err != nil {
		t.Errorf("crash guard must survive a failed Unconfigure, stat err = %v", err)
	}
}

func TestUnconfigure_KeepsGuardWhenFirewallCommitFails(t *testing.T) {
	runner := newMockUSBTetherRunner()
	runner.uciOutputs["uci show firewall"] = struct {
		out string
		err error
	}{out: `firewall.@zone[1].name='wan'
firewall.@zone[1].input='REJECT'
firewall.@zone[1].network='usbtether'
`, err: nil}
	runner.uciOutputs["uci commit firewall"] = struct {
		out string
		err error
	}{out: "", err: errors.New("commit failed")}
	svc := newTestUSBTetherService(t, runner)

	if err := svc.Unconfigure(); err == nil {
		t.Fatal("expected Unconfigure to fail when the firewall commit fails")
	}
	if _, err := os.Stat(svc.guardFile); err != nil {
		t.Errorf("crash guard must survive a failed firewall commit, stat err = %v", err)
	}
}

func TestUnconfigure_KeepsGuardWhenNetworkCommitFails(t *testing.T) {
	runner := newMockUSBTetherRunner()
	runner.uciOutputs["uci commit network"] = struct {
		out string
		err error
	}{out: "", err: errors.New("commit failed")}
	svc := newTestUSBTetherService(t, runner)

	if err := svc.Unconfigure(); err == nil {
		t.Fatal("expected Unconfigure to fail when the network commit fails")
	}
	if _, err := os.Stat(svc.guardFile); err != nil {
		t.Errorf("crash guard must survive a failed network commit, stat err = %v", err)
	}
	seen := map[string]bool{}
	for _, cmd := range runner.commands {
		seen[cmd] = true
	}
	if !seen["uci revert network"] {
		t.Error("a failed network commit must discard the staged delete")
	}
}

func TestUnconfigure_KeepsGuardWhenWanZoneLookupFails(t *testing.T) {
	runner := newMockUSBTetherRunner()
	runner.uciOutputs["uci show firewall"] = struct {
		out string
		err error
	}{out: "", err: errors.New("uci show firewall failed")}
	svc := newTestUSBTetherService(t, runner)

	if err := svc.Unconfigure(); err == nil {
		t.Fatal("expected Unconfigure to fail when the wan zone cannot be resolved")
	}
	if _, err := os.Stat(svc.guardFile); err != nil {
		t.Errorf("crash guard must survive a failed wan-zone lookup, stat err = %v", err)
	}
}
