package main

import (
	"testing"

	"github.com/raydak-labs/travo/internal/adguard"
	"github.com/raydak-labs/travo/internal/catalog"
	"github.com/raydak-labs/travo/internal/ddns"
	"github.com/raydak-labs/travo/internal/factoryreset"
	"github.com/raydak-labs/travo/internal/failover"
	"github.com/raydak-labs/travo/internal/firmware"
	"github.com/raydak-labs/travo/internal/guard"
	"github.com/raydak-labs/travo/internal/reboot"
	"github.com/raydak-labs/travo/internal/tailscale"
	"github.com/raydak-labs/travo/internal/updateapply"
	"github.com/raydak-labs/travo/internal/usbtether"
)

// The conflict policies name guards by string; a renamed constant must not drop out.
func TestGuardClassesNameTheOwningPackagesGuards(t *testing.T) {
	for _, n := range []string{firmware.GuardName, factoryreset.GuardName, reboot.GuardName, updateapply.GuardName} {
		if !guard.IsTerminal(n) {
			t.Errorf("%s is not terminal", n)
		}
	}
	for _, n := range []string{adguard.InstallGuard, catalog.Guard, ddns.InstallGuard, failover.InstallGuard, tailscale.InstallGuard, usbtether.InstallGuard} {
		if !guard.IsPackageOp(n) {
			t.Errorf("%s is not a package-op guard", n)
		}
	}
}
