package services

import "testing"

func TestListServices(t *testing.T) {
	sm := NewServiceManager()
	services, err := sm.ListServices()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(services) == 0 {
		t.Error("expected services")
	}
}

func TestInstallService(t *testing.T) {
	sm := NewServiceManager()
	err := sm.Install("adguardhome")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	info, _ := sm.GetServiceStatus("adguardhome")
	if info.State != "stopped" {
		t.Errorf("expected state 'stopped', got %q", info.State)
	}
}

func TestStartService(t *testing.T) {
	sm := NewServiceManager()
	_ = sm.Install("adguardhome")
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
	sm := NewServiceManager()
	_ = sm.Install("adguardhome")
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
	sm := NewServiceManager()
	info, err := sm.GetServiceStatus("wireguard")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.ID != "wireguard" {
		t.Errorf("expected id 'wireguard', got %q", info.ID)
	}
}

func TestGetServiceStatusNotFound(t *testing.T) {
	sm := NewServiceManager()
	_, err := sm.GetServiceStatus("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent service")
	}
}
