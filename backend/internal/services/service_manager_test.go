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
	if len(services) != 3 {
		t.Errorf("expected 3 services, got %d", len(services))
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
