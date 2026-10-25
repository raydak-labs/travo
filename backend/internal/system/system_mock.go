package system

import (
	"fmt"
)

// MockStorageProvider returns realistic mock storage stats.
type MockStorageProvider struct{}

// GetRootStorage returns mock storage stats.
func (m *MockStorageProvider) GetRootStorage() (int64, int64, int64, error) {
	// 256MB total, 96MB used, 160MB free — realistic for a travel router
	return 268435456, 100663296, 167772160, nil
}

// MockPackageManager tracks install/remove state in memory.
type MockPackageManager struct {
	installed   map[string]bool
	UpdateCalls int
	UpdateErr   error
}

func NewMockPackageManager() *MockPackageManager {
	return &MockPackageManager{installed: make(map[string]bool)}
}

func (m *MockPackageManager) Update() (string, error) {
	m.UpdateCalls++
	if m.UpdateErr != nil {
		return "", m.UpdateErr
	}
	return "ok", nil
}

func (m *MockPackageManager) Install(pkg string) (string, error) {
	m.installed[pkg] = true
	return "ok", nil
}

func (m *MockPackageManager) Remove(pkg string) (string, error) {
	delete(m.installed, pkg)
	return "ok", nil
}

func (m *MockPackageManager) IsInstalled(pkg string) bool {
	return m.installed[pkg]
}

func (m *MockPackageManager) InstallStream(pkg string, logFn func(string)) error {
	logFn("Installing " + pkg + "...")
	m.installed[pkg] = true
	logFn("Package " + pkg + " installed successfully")
	return nil
}

func (m *MockPackageManager) RemoveStream(pkg string, logFn func(string)) error {
	logFn("Removing " + pkg + "...")
	delete(m.installed, pkg)
	logFn("Package " + pkg + " removed successfully")
	return nil
}

// MockSystemProbe tracks init.d state in memory.
type MockSystemProbe struct {
	scripts   map[string]bool
	running   map[string]bool
	autoStart map[string]bool
}

func NewMockSystemProbe() *MockSystemProbe {
	return &MockSystemProbe{
		scripts:   make(map[string]bool),
		running:   make(map[string]bool),
		autoStart: make(map[string]bool),
	}
}

func (m *MockSystemProbe) HasInitScript(name string) bool { return m.scripts[name] }

func (m *MockSystemProbe) IsRunning(initName string) bool { return m.running[initName] }

func (m *MockSystemProbe) Start(initName string) (string, error) {
	m.running[initName] = true
	return "ok", nil
}

func (m *MockSystemProbe) Stop(initName string) (string, error) {
	delete(m.running, initName)
	return "ok", nil
}

func (m *MockSystemProbe) IsAutoStart(initName string) bool { return m.autoStart[initName] }

func (m *MockSystemProbe) Enable(initName string) error {
	m.autoStart[initName] = true
	return nil
}

func (m *MockSystemProbe) Disable(initName string) error {
	delete(m.autoStart, initName)
	return nil
}

// NoopPackageManager for systems without a package manager.
type NoopPackageManager struct{}

func (n *NoopPackageManager) Update() (string, error) {
	return "", fmt.Errorf("no package manager available")
}

func (n *NoopPackageManager) Install(string) (string, error) {
	return "", fmt.Errorf("no package manager available")
}

func (n *NoopPackageManager) Remove(string) (string, error) {
	return "", fmt.Errorf("no package manager available")
}

func (n *NoopPackageManager) IsInstalled(string) bool { return false }

func (n *NoopPackageManager) InstallStream(string, func(string)) error {
	return fmt.Errorf("no package manager available")
}

func (n *NoopPackageManager) RemoveStream(string, func(string)) error {
	return fmt.Errorf("no package manager available")
}

// MockLogTail stands in for logread and dmesg on a machine that has neither.
func MockLogTail(cmd string) (string, error) {
	if cmd == "dmesg" {
		return `[    0.000000] Booting Linux on physical CPU 0x0
[    2.318204] ath11k c000000.wifi: qmi fw_version 0x290604a5
[   14.502117] br-lan: port 1(lan1) entered forwarding state`, nil
	}
	return `Sat Oct 24 09:17:50 2026 daemon.notice netifd: Interface 'lan' is now up
Sat Oct 24 09:17:52 2026 daemon.info dnsmasq[1234]: query from 192.168.8.100
Sat Oct 24 09:17:55 2026 daemon.warn travo[2210]: wan probe timed out
Sat Oct 24 09:18:01 2026 daemon.err hostapd: wlan1: deauthenticated 3c:22:fb:01:02:03`, nil
}
