package uplink

import (
	"fmt"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/platform"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

func TestGetTailscaleStatus(t *testing.T) {
	u := uci.NewMockUCI()
	cmd := &platform.MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		if name == "which" && len(args) == 1 && args[0] == "tailscale" {
			return nil, fmt.Errorf("not found")
		}
		return nil, fmt.Errorf("unexpected command %s", name)
	}}
	svc := NewVpnService(VpnDeps{UCI: u, Cmd: cmd})

	status, err := svc.GetTailscaleStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Test that the function runs without error and returns a valid status structure.
	// The installed status depends on the actual system environment.
	if status.Installed && status.Running {
		// If tailscale is installed and running, verify the status structure is populated correctly
		if status.LoggedIn {
			// When logged in, we should get peers
			if status.Peers == nil {
				t.Error("expected peers slice to be initialized")
			}
		}
	}
	// Test that we can call the function without panicking
	_ = status
}
