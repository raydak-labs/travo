package services

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

// MockStorageProvider returns realistic mock storage stats.
type MockStorageProvider struct{}

// GetRootStorage returns mock storage stats.
func (m *MockStorageProvider) GetRootStorage() (int64, int64, int64, error) {
	// 256MB total, 96MB used, 160MB free — realistic for a travel router
	return 268435456, 100663296, 167772160, nil
}

// MockHTTPProber returns preset responses for testing.
type MockHTTPProber struct {
	StatusCode  int
	Body        string
	RedirectURL string
	Err         error
}

// Do returns the preset mock response.
func (m *MockHTTPProber) Do(_ string) (int, string, string, error) {
	return m.StatusCode, m.Body, m.RedirectURL, m.Err
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

// NoopEventWatcher satisfies EventWatcher but never emits anything.
// Used in mock mode and tests.
type NoopEventWatcher struct {
	ch       chan models.NetworkStatus
	stopCh   chan struct{}
	started  atomic.Bool
	stopOnce sync.Once
}

func NewNoopEventWatcher() *NoopEventWatcher {
	return &NoopEventWatcher{
		ch:     make(chan models.NetworkStatus),
		stopCh: make(chan struct{}),
	}
}

// Start blocks until Stop is called. A second call returns immediately instead
// of spawning a second loop.
func (w *NoopEventWatcher) Start() {
	if !w.started.CompareAndSwap(false, true) {
		return
	}
	<-w.stopCh
}

// Stop is safe to call multiple times.
func (w *NoopEventWatcher) Stop() {
	w.stopOnce.Do(func() { close(w.stopCh) })
}

func (w *NoopEventWatcher) Ch() <-chan models.NetworkStatus {
	return w.ch
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

// NoopWifiReloader does nothing (for tests).
type NoopWifiReloader struct{}

// Reload is a no-op.
func (r *NoopWifiReloader) Reload() error { return nil }
