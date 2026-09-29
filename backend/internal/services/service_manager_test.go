package services

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// newTestServiceManagerWithGuard builds a ServiceManager whose crash-guard
// files land in a temp dir instead of /etc/travo.
func newTestServiceManagerWithGuard(t *testing.T, pkg PackageManager, probe SystemProbe) *ServiceManager {
	t.Helper()
	sm := NewServiceManagerWith(pkg, probe)
	sm.SetGuardDir(t.TempDir())
	return sm
}

func newTestServiceManager(t *testing.T) (*ServiceManager, *MockPackageManager, *MockSystemProbe) {
	pkg := NewMockPackageManager()
	probe := NewMockSystemProbe()
	sm := newTestServiceManagerWithGuard(t, pkg, probe)
	return sm, pkg, probe
}

func TestListServices(t *testing.T) {
	sm, _, _ := newTestServiceManager(t)
	services, err := sm.ListServices()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(services) != 8 {
		t.Errorf("expected 8 services, got %d", len(services))
	}
	for _, s := range services {
		if s.State != "not_installed" {
			t.Errorf("expected all not_installed, got %q for %s", s.State, s.ID)
		}
	}
}

func TestInstallService(t *testing.T) {
	sm, pkg, probe := newTestServiceManager(t)
	// Install sets the package as installed and adds init.d script
	err := sm.Install("adguardhome")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pkg.IsInstalled("adguardhome") {
		t.Error("expected mock package to be installed")
	}
	// Simulate that init.d script now exists (apk would create it)
	probe.scripts["adguardhome"] = true
	info, _ := sm.GetServiceStatus("adguardhome")
	if info.State != "stopped" {
		t.Errorf("expected state 'stopped', got %q", info.State)
	}
}

func TestStartService(t *testing.T) {
	sm, _, probe := newTestServiceManager(t)
	_ = sm.Install("adguardhome")
	probe.scripts["adguardhome"] = true
	err := sm.Start("adguardhome")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	info, _ := sm.GetServiceStatus("adguardhome")
	if info.State != "running" {
		t.Errorf("expected state 'running', got %q", info.State)
	}
}

func TestStopService(t *testing.T) {
	sm, _, probe := newTestServiceManager(t)
	_ = sm.Install("adguardhome")
	probe.scripts["adguardhome"] = true
	_ = sm.Start("adguardhome")
	err := sm.Stop("adguardhome")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	info, _ := sm.GetServiceStatus("adguardhome")
	if info.State != "stopped" {
		t.Errorf("expected state 'stopped', got %q", info.State)
	}
}

func TestGetServiceStatus(t *testing.T) {
	sm, _, _ := newTestServiceManager(t)
	info, err := sm.GetServiceStatus("wireguard")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.ID != "wireguard" {
		t.Errorf("expected id 'wireguard', got %q", info.ID)
	}
	if info.State != "not_installed" {
		t.Errorf("expected 'not_installed', got %q", info.State)
	}
}

func TestGetServiceStatusNotFound(t *testing.T) {
	sm, _, _ := newTestServiceManager(t)
	_, err := sm.GetServiceStatus("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent service")
	}
}

func TestWireguardInstalledState(t *testing.T) {
	sm, pkg, _ := newTestServiceManager(t)
	pkg.installed["wireguard-tools"] = true
	info, _ := sm.GetServiceStatus("wireguard")
	// WireGuard has no init.d, so installed state is "installed"
	if info.State != "installed" {
		t.Errorf("expected 'installed', got %q", info.State)
	}
}

func TestRemoveServiceStopsFirst(t *testing.T) {
	sm, pkg, probe := newTestServiceManager(t)
	_ = sm.Install("adguardhome")
	probe.scripts["adguardhome"] = true
	_ = sm.Start("adguardhome")
	if !probe.running["adguardhome"] {
		t.Fatal("expected running before remove")
	}
	err := sm.Remove("adguardhome")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.IsInstalled("adguardhome") {
		t.Error("expected package removed")
	}
	if probe.running["adguardhome"] {
		t.Error("expected stopped after remove")
	}
}

func TestInstallWithLog(t *testing.T) {
	sm, pkg, _ := newTestServiceManager(t)
	var lines []string
	logFn := func(line string) { lines = append(lines, line) }

	err := sm.InstallWithLog("adguardhome", logFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pkg.IsInstalled("adguardhome") {
		t.Error("expected package installed")
	}
	if len(lines) < 2 {
		t.Errorf("expected at least 2 log lines, got %d", len(lines))
	}
}

func TestInstallWithLogNotFound(t *testing.T) {
	sm, _, _ := newTestServiceManager(t)
	err := sm.InstallWithLog("nonexistent", func(string) {})
	if err == nil {
		t.Error("expected error for nonexistent service")
	}
}

func TestRemoveWithLog(t *testing.T) {
	sm, pkg, probe := newTestServiceManager(t)
	_ = sm.Install("adguardhome")
	probe.scripts["adguardhome"] = true
	_ = sm.Start("adguardhome")

	var lines []string
	logFn := func(line string) { lines = append(lines, line) }

	err := sm.RemoveWithLog("adguardhome", logFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.IsInstalled("adguardhome") {
		t.Error("expected package removed")
	}
	if len(lines) < 2 {
		t.Errorf("expected at least 2 log lines, got %d", len(lines))
	}
}

func TestRemoveWithLogStopsRunning(t *testing.T) {
	sm, _, probe := newTestServiceManager(t)
	_ = sm.Install("adguardhome")
	probe.scripts["adguardhome"] = true
	_ = sm.Start("adguardhome")

	var lines []string
	logFn := func(line string) { lines = append(lines, line) }

	_ = sm.RemoveWithLog("adguardhome", logFn)
	if probe.running["adguardhome"] {
		t.Error("expected service stopped before removal")
	}
	// Should contain a "Stopping" line
	found := slices.Contains(lines, "Stopping adguardhome...")
	if !found {
		t.Error("expected 'Stopping adguardhome...' in log output")
	}
}

func TestCacheUpdatesOnInstall(t *testing.T) {
	sm, _, probe := newTestServiceManager(t)
	sm.RefreshCache()

	// Before install, cache shows not_installed
	info, _ := sm.GetServiceStatus("adguardhome")
	if info.State != "not_installed" {
		t.Errorf("expected 'not_installed', got %q", info.State)
	}

	// Install updates cache
	_ = sm.Install("adguardhome")
	probe.scripts["adguardhome"] = true
	sm.RefreshCache() // simulate init script appearing after install
	info, _ = sm.GetServiceStatus("adguardhome")
	if info.State != "stopped" {
		t.Errorf("expected 'stopped' after install, got %q", info.State)
	}
}

func TestCacheUpdatesOnStartStop(t *testing.T) {
	sm, _, probe := newTestServiceManager(t)
	_ = sm.Install("adguardhome")
	probe.scripts["adguardhome"] = true
	sm.RefreshCache()

	_ = sm.Start("adguardhome")
	info, _ := sm.GetServiceStatus("adguardhome")
	if info.State != "running" {
		t.Errorf("expected 'running' after start, got %q", info.State)
	}

	_ = sm.Stop("adguardhome")
	info, _ = sm.GetServiceStatus("adguardhome")
	if info.State != "stopped" {
		t.Errorf("expected 'stopped' after stop, got %q", info.State)
	}
}

func TestCacheUpdatesOnRemove(t *testing.T) {
	sm, _, probe := newTestServiceManager(t)
	_ = sm.Install("adguardhome")
	probe.scripts["adguardhome"] = true
	sm.RefreshCache()

	_ = sm.Remove("adguardhome")
	info, _ := sm.GetServiceStatus("adguardhome")
	if info.State != "not_installed" {
		t.Errorf("expected 'not_installed' after remove, got %q", info.State)
	}
}

func TestListServicesUsesCache(t *testing.T) {
	sm, pkg, probe := newTestServiceManager(t)
	pkg.installed["wireguard-tools"] = true
	sm.RefreshCache()

	// Cache should now reflect wireguard as installed
	services, _ := sm.ListServices()
	for _, s := range services {
		if s.ID == "wireguard" && s.State != "installed" {
			t.Errorf("expected wireguard 'installed' from cache, got %q", s.State)
		}
	}

	// Externally change state but don't refresh — cache should still show old state
	delete(pkg.installed, "wireguard-tools")
	services, _ = sm.ListServices()
	for _, s := range services {
		if s.ID == "wireguard" && s.State != "installed" {
			t.Errorf("expected wireguard still 'installed' from stale cache, got %q", s.State)
		}
	}

	// After refresh, should show not_installed
	sm.RefreshCache()
	info, _ := sm.GetServiceStatus("wireguard")
	if info.State != "not_installed" {
		t.Errorf("expected 'not_installed' after refresh, got %q", info.State)
	}
	_ = probe // suppress unused
}

func TestSetAutoStart(t *testing.T) {
	pkg := NewMockPackageManager()
	probe := NewMockSystemProbe()
	probe.scripts["adguardhome"] = true
	sm := newTestServiceManagerWithGuard(t, pkg, probe)

	// Install adguardhome first
	pkg.installed["adguardhome"] = true
	sm.RefreshCache()

	// Enable auto-start
	err := sm.SetAutoStart("adguardhome", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, _ := sm.GetServiceStatus("adguardhome")
	if !info.AutoStart {
		t.Error("expected auto_start to be true")
	}

	// Disable auto-start
	err = sm.SetAutoStart("adguardhome", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, _ = sm.GetServiceStatus("adguardhome")
	if info.AutoStart {
		t.Error("expected auto_start to be false")
	}
}

func TestSetAutoStart_NotInstalled(t *testing.T) {
	pkg := NewMockPackageManager()
	probe := NewMockSystemProbe()
	sm := newTestServiceManagerWithGuard(t, pkg, probe)

	err := sm.SetAutoStart("adguardhome", true)
	if err == nil {
		t.Error("expected error for not installed service")
	}
}

func TestInstall_RunsIndexUpdateFirst(t *testing.T) {
	pkg := NewMockPackageManager()
	sm := newTestServiceManagerWithGuard(t, pkg, NewMockSystemProbe())

	if err := sm.Install("tailscale"); err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if pkg.UpdateCalls != 1 {
		t.Errorf("expected 1 index update before install, got %d", pkg.UpdateCalls)
	}
	if !pkg.IsInstalled("tailscale") {
		t.Error("expected tailscale installed")
	}
}

func TestInstallWithLog_RunsIndexUpdateFirst(t *testing.T) {
	pkg := NewMockPackageManager()
	sm := newTestServiceManagerWithGuard(t, pkg, NewMockSystemProbe())

	var lines []string
	if err := sm.InstallWithLog("tailscale", func(s string) { lines = append(lines, s) }); err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if pkg.UpdateCalls != 1 {
		t.Errorf("expected 1 index update before streamed install, got %d", pkg.UpdateCalls)
	}
	if len(lines) == 0 {
		t.Error("expected log lines")
	}
}

func TestInstall_ProceedsWhenIndexUpdateFails(t *testing.T) {
	pkg := NewMockPackageManager()
	pkg.UpdateErr = fmt.Errorf("no network")
	sm := newTestServiceManagerWithGuard(t, pkg, NewMockSystemProbe())

	if err := sm.Install("tailscale"); err != nil {
		t.Fatalf("install must proceed on best-effort update failure, got: %v", err)
	}
	if !pkg.IsInstalled("tailscale") {
		t.Error("expected tailscale installed despite failed index update")
	}
}

// blockingPkgManager stalls every package operation until released, so a test
// can observe what reads look like while an install is in flight.
type blockingPkgManager struct {
	*MockPackageManager
	release chan struct{}
	entered chan struct{}
	once    sync.Once
}

func newBlockingPkgManager() *blockingPkgManager {
	return &blockingPkgManager{
		MockPackageManager: NewMockPackageManager(),
		release:            make(chan struct{}),
		entered:            make(chan struct{}, 1),
	}
}

func (b *blockingPkgManager) Install(pkg string) (string, error) {
	b.once.Do(func() { b.entered <- struct{}{} })
	<-b.release
	return b.MockPackageManager.Install(pkg)
}

// The install path must not hold the cache write lock: previously
// ListServices/GetServiceStatus blocked for the whole (up to 30 min) install.
func TestInstall_DoesNotBlockReads(t *testing.T) {
	pkg := newBlockingPkgManager()
	sm := newTestServiceManagerWithGuard(t, pkg, NewMockSystemProbe())

	installDone := make(chan error, 1)
	go func() { installDone <- sm.Install("wireguard") }()

	select {
	case <-pkg.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("install never started")
	}

	readsDone := make(chan struct{})
	go func() {
		if _, err := sm.ListServices(); err != nil {
			t.Errorf("ListServices: %v", err)
		}
		if _, err := sm.GetServiceStatus("tailscale"); err != nil {
			t.Errorf("GetServiceStatus: %v", err)
		}
		close(readsDone)
	}()

	select {
	case <-readsDone:
	case <-time.After(5 * time.Second):
		t.Fatal("reads blocked while an install was in flight")
	}

	close(pkg.release)
	if err := <-installDone; err != nil {
		t.Fatalf("install failed: %v", err)
	}
}

// Package installs mutate live state, so a crash guard must exist while they
// run and be removed only after full success (ADR 0003).
func TestInstall_WritesAndClearsCrashGuard(t *testing.T) {
	guardDir := t.TempDir()
	pkg := NewMockPackageManager()
	sm := NewServiceManagerWith(pkg, NewMockSystemProbe())
	sm.SetGuardDir(guardDir)
	guard := filepath.Join(guardDir, "pkg-install-in-progress")

	if err := sm.Install("tailscale"); err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if _, err := os.Stat(guard); !os.IsNotExist(err) {
		t.Errorf("guard file must be removed after a successful install, stat err = %v", err)
	}

	// A failing install must leave the marker behind.
	sm2 := NewServiceManagerWith(&failingPkgManager{}, NewMockSystemProbe())
	sm2.SetGuardDir(guardDir)
	if err := sm2.Install("tailscale"); err == nil {
		t.Fatal("expected install failure")
	}
	if _, err := os.Stat(guard); err != nil {
		t.Errorf("guard file must remain after a failed install: %v", err)
	}
}

type failingPkgManager struct{ MockPackageManager }

func (f *failingPkgManager) Install(string) (string, error) {
	return "boom", fmt.Errorf("apk add failed")
}

// The post-install hook error must reach the caller: an installed but
// unconfigured service used to be reported as a success.
func TestInstall_PropagatesPostInstallHookError(t *testing.T) {
	sm, _, _ := newTestServiceManager(t)
	sm.SetPostInstallHook("adguardhome", func() error { return fmt.Errorf("adguard config failed") })

	err := sm.Install("adguardhome")
	if err == nil {
		t.Fatal("expected the post-install hook failure to be reported")
	}
	if !strings.Contains(err.Error(), "post-install") {
		t.Errorf("expected a post-install error, got %v", err)
	}
}

func TestInstallWithLog_PropagatesPostInstallHookError(t *testing.T) {
	sm, _, _ := newTestServiceManager(t)
	sm.SetPostInstallHook("adguardhome", func() error { return fmt.Errorf("adguard config failed") })

	var lines []string
	err := sm.InstallWithLog("adguardhome", func(s string) { lines = append(lines, s) })
	if err == nil {
		t.Fatal("expected the post-install hook failure to be reported")
	}
	if !strings.Contains(err.Error(), "post-install") {
		t.Errorf("expected a post-install error, got %v", err)
	}
}

// safePkgManager is a concurrency-safe PackageManager that adds a small delay
// to every install, so reader goroutines and the cache-publishing
// refreshOne() genuinely overlap in time. (MockPackageManager's own map is not
// synchronized and would itself trip the race detector.)
type safePkgManager struct {
	mu        sync.Mutex
	installed map[string]bool
	delay     time.Duration
}

func newSafePkgManager(delay time.Duration) *safePkgManager {
	return &safePkgManager{installed: make(map[string]bool), delay: delay}
}

func (s *safePkgManager) Update() (string, error) { return "ok", nil }

func (s *safePkgManager) Install(pkg string) (string, error) {
	time.Sleep(s.delay)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.installed[pkg] = true
	return "ok", nil
}

func (s *safePkgManager) Remove(pkg string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.installed, pkg)
	return "ok", nil
}

func (s *safePkgManager) IsInstalled(pkg string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.installed[pkg]
}

func (s *safePkgManager) InstallStream(pkg string, logFn func(string)) error {
	_, err := s.Install(pkg)
	return err
}

func (s *safePkgManager) RemoveStream(pkg string, logFn func(string)) error {
	_, err := s.Remove(pkg)
	return err
}

// Publishing the refreshed cache entry must be a real write-locked update.
// refreshOne used to write sm.cache with no lock at all, so a concurrent
// ListServices/GetServiceStatus (which hold only the read lock) raced with it
// — a Go map read/write race that can abort the process. Run with -race.
func TestInstall_ConcurrentReadsDoNotRaceWithCachePublish(t *testing.T) {
	sm := newTestServiceManagerWithGuard(t, newSafePkgManager(time.Millisecond), NewMockSystemProbe())

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				if _, err := sm.ListServices(); err != nil {
					t.Errorf("ListServices: %v", err)
					return
				}
				if _, err := sm.GetServiceStatus("wireguard"); err != nil {
					t.Errorf("GetServiceStatus: %v", err)
					return
				}
			}
		}()
	}

	for i := 0; i < 20; i++ {
		if err := sm.Install("wireguard"); err != nil {
			t.Errorf("install %d: %v", i, err)
			break
		}
	}
	close(stop)
	wg.Wait()
}
