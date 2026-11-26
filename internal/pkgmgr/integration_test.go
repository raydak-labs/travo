//go:build integration

package pkgmgr

import (
	"context"
	"errors"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/runner"
)

// Runs inside an OpenWrt container: Detect must pick the right manager and read real output.
func TestRealManager(t *testing.T) {
	ctx := context.Background()
	pm, err := DetectSystem(runner.Exec)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("manager: %s", pm.Name())
	pkgs, err := pm.Installed(ctx)
	if err != nil || len(pkgs) < 20 {
		t.Fatalf("installed: n=%d err=%v", len(pkgs), err)
	}
	if v, err := pm.Version(ctx, "dnsmasq"); err != nil || v == "" {
		t.Fatalf("dnsmasq: %q %v", v, err)
	}
	if _, err := pm.Version(ctx, "travo-no-such-package"); !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("absent package: %v", err)
	}
}
