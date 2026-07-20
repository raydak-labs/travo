package services

import (
	"testing"
)

func TestInstallSpeedtestCLI_UsesPackageManager(t *testing.T) {
	pkg := NewMockPackageManager()
	svc := NewSpeedtestServiceWith(pkg)

	if err := svc.InstallSpeedtestCLI(); err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if pkg.UpdateCalls != 1 {
		t.Errorf("expected index update before install, got %d", pkg.UpdateCalls)
	}
	if !pkg.IsInstalled("speedtest") {
		t.Error("expected speedtest package installed via package manager")
	}
}

func TestInstallSpeedtestCLI_AlreadyInstalled(t *testing.T) {
	pkg := NewMockPackageManager()
	_, _ = pkg.Install("speedtest")
	svc := NewSpeedtestServiceWith(pkg)

	if err := svc.InstallSpeedtestCLI(); err == nil {
		t.Error("expected error when already installed")
	}
}

func TestUninstallSpeedtestCLI_UsesPackageManager(t *testing.T) {
	pkg := NewMockPackageManager()
	_, _ = pkg.Install("speedtest")
	svc := NewSpeedtestServiceWith(pkg)

	if err := svc.UninstallSpeedtestCLI(); err != nil {
		t.Fatalf("uninstall failed: %v", err)
	}
	if pkg.IsInstalled("speedtest") {
		t.Error("expected speedtest package removed")
	}
}

func TestUninstallSpeedtestCLI_NotInstalled(t *testing.T) {
	svc := NewSpeedtestServiceWith(NewMockPackageManager())
	if err := svc.UninstallSpeedtestCLI(); err == nil {
		t.Error("expected error when not installed")
	}
}
