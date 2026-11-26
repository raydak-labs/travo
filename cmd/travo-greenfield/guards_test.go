package main

import (
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/adguard"
	"github.com/blackdark-org/travo-greenfield/internal/catalog"
	"github.com/blackdark-org/travo-greenfield/internal/ddns"
	"github.com/blackdark-org/travo-greenfield/internal/factoryreset"
	"github.com/blackdark-org/travo-greenfield/internal/failover"
	"github.com/blackdark-org/travo-greenfield/internal/firmware"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/reboot"
	"github.com/blackdark-org/travo-greenfield/internal/tailscale"
	"github.com/blackdark-org/travo-greenfield/internal/updateapply"
	"github.com/blackdark-org/travo-greenfield/internal/usbtether"
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
