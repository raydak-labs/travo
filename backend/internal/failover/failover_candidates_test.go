package failover

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/network"
	"github.com/openwrt-travel-gui/backend/internal/platform"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

func TestReadTrackerStatesMapsNamespacedSections(t *testing.T) {
	t.Parallel()

	mockUCI := uci.NewMockUCI()
	cmd := &platform.MockCommandRunner{RunFunc: func(name string, _ ...string) ([]byte, error) {
		if name == "mwan3" {
			return []byte("interface travo_if_wan is online\ninterface hotel is offline\n"), nil
		}
		return nil, nil
	}}
	tmpDir := t.TempDir()
	mockUbus := ubus.NewMockUbus()
	networkSvc := network.New(network.Deps{UCI: mockUCI, Ubus: mockUbus, Cmd: &platform.MockCommandRunner{}})
	configPath := filepath.Join(tmpDir, "failover.json")
	svc := New(Deps{UCI: mockUCI, Ubus: mockUbus, Network: networkSvc, Cmd: cmd, Applier: &platform.NoopUCIApplyConfirm{}, ConfigPath: configPath})
	svc.initScript = filepath.Join(tmpDir, "mwan3")
	if err := os.WriteFile(svc.initScript, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write init script: %v", err)
	}

	states := svc.readTrackerStates()
	if states["wan"] != models.FailoverTrackingStateOnline {
		t.Errorf("expected the namespaced section to report as network interface wan, got %v", states)
	}
	if states["hotel"] != models.FailoverTrackingStateOffline {
		t.Errorf("a hand-written mwan3 section must be reported under its own name, got %v", states)
	}
}
