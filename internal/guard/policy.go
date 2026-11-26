package guard

import "fmt"

// terminal guards end in a reboot, flash, wipe or daemon replacement: nothing may start.
var terminal = map[string]bool{
	"firmware-upgrade-in-progress": true,
	"factory-reset-in-progress":    true,
	"reboot-in-progress":           true,
	"update-in-progress":           true,
}

// packageOps guards are held across apk/opkg installs and removes.
var packageOps = map[string]bool{
	"adguard-install-in-progress":   true,
	"catalog-package-in-progress":   true,
	"ddns-install-in-progress":      true,
	"failover-install-in-progress":  true,
	"tailscale-install-in-progress": true,
	"usbtether-install-in-progress": true,
}

// IsTerminal reports a guard of a reboot, firmware upgrade, factory reset or update.
func IsTerminal(name string) bool { return terminal[name] }

// IsPackageOp reports a guard held across a package install or remove.
func IsPackageOp(name string) bool { return packageOps[name] }

// RefuseAny is the conflict policy of operations that run only while no other guard is held.
func RefuseAny(held []Info) error {
	if len(held) > 0 {
		return fmt.Errorf("%w (%s)", ErrHeld, held[0].Name)
	}
	return nil
}

// RefuseTerminal is the conflict policy of package operations: never while the router is
// about to reboot, flash, wipe or replace the daemon.
func RefuseTerminal(held []Info) error {
	for _, g := range held {
		if IsTerminal(g.Name) {
			return fmt.Errorf("%w (%s)", ErrTerminal, g.Name)
		}
	}
	return nil
}
