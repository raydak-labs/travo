package services

import (
	"testing"
)

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
	return "", nil
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
	svc := NewUSBTetheringServiceWithRunner(runner)
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
}

func TestUnconfigure_DeletesAndCommits(t *testing.T) {
	runner := newMockUSBTetherRunner()
	svc := NewUSBTetheringServiceWithRunner(runner)
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
}
