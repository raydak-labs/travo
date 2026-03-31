package services

import "testing"

func newTestServiceManager() (*ServiceManager, *MockPackageManager, *MockSystemProbe) {
	pkg := NewMockPackageManager()
	probe := NewMockSystemProbe()
	sm := NewServiceManagerWith(pkg, probe)
	return sm, pkg, probe
}

func TestListServices(t *testing.T) {
	sm, _, _ := newTestServiceManager()
	services, err := sm.ListServices()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(services) != 5 {
		t.Errorf("expected 5 services, got %d", len(services))
	}
	for _, s := range services {
		if s.State != "not_installed" {
			t.Errorf("expected all not_installed, got %q for %s", s.State, s.ID)
		}
	}
}

func TestInstallService(t *testing.T) {
	sm, pkg, probe := newTestServiceManager()
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
	sm, _, probe := newTestServiceManager()
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
	sm, _, probe := newTestServiceManager()
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
	sm, _, _ := newTestServiceManager()
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
	sm, _, _ := newTestServiceManager()
	_, err := sm.GetServiceStatus("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent service")
	}
}

func TestWireguardInstalledState(t *testing.T) {
	sm, pkg, _ := newTestServiceManager()
	pkg.installed["wireguard-tools"] = true
	info, _ := sm.GetServiceStatus("wireguard")
	// WireGuard has no init.d, so installed state is "installed"
	if info.State != "installed" {
		t.Errorf("expected 'installed', got %q", info.State)
	}
}

func TestRemoveServiceStopsFirst(t *testing.T) {
	sm, pkg, probe := newTestServiceManager()
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
	sm, pkg, _ := newTestServiceManager()
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
	sm, _, _ := newTestServiceManager()
	err := sm.InstallWithLog("nonexistent", func(string) {})
	if err == nil {
		t.Error("expected error for nonexistent service")
	}
}

func TestRemoveWithLog(t *testing.T) {
	sm, pkg, probe := newTestServiceManager()
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
	sm, _, probe := newTestServiceManager()
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
	found := false
	for _, l := range lines {
		if l == "Stopping adguardhome..." {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'Stopping adguardhome...' in log output")
	}
}

func TestCacheUpdatesOnInstall(t *testing.T) {
	sm, _, probe := newTestServiceManager()
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
	sm, _, probe := newTestServiceManager()
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
	sm, _, probe := newTestServiceManager()
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
	sm, pkg, probe := newTestServiceManager()
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
	sm := NewServiceManagerWith(pkg, probe)

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
	sm := NewServiceManagerWith(pkg, probe)

	err := sm.SetAutoStart("adguardhome", true)
	if err == nil {
		t.Error("expected error for not installed service")
	}
}
