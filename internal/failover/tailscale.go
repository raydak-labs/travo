package failover

import (
	"context"
	"errors"
	"fmt"

	"github.com/blackdark-org/travo-greenfield/internal/pkgmgr"
)

// TailscalePackage is the OpenWrt package and init script of Tailscale (tailscale.Package).
const TailscalePackage = "tailscale"

// tailscaleOff refuses failover while tailscaled runs or starts at boot: in its nftables
// mode (the package default) it writes native rules into ip mangle, and mwan3 cannot
// manage its chains next to them (ADR 0018). A stopped but boot-enabled tailscaled would
// come back after a reboot. A failed check is an error, never "absent". Callers hold Exclusive.
func (s *Service) tailscaleOff(ctx context.Context) error {
	if _, err := s.Packages.Version(ctx, TailscalePackage); errors.Is(err, pkgmgr.ErrNotInstalled) {
		return nil
	} else if err != nil {
		return fmt.Errorf("query %s: %w", TailscalePackage, err)
	}
	running, err := s.Services.ServiceRunning(ctx, TailscalePackage)
	if err != nil {
		return fmt.Errorf("%s running: %w", TailscalePackage, err)
	}
	if running {
		return ErrTailscale
	}
	enabled, err := s.Autostart.ServiceEnabled(ctx, TailscalePackage)
	if err != nil {
		return fmt.Errorf("%s enabled: %w", TailscalePackage, err)
	}
	if enabled {
		return ErrTailscale
	}
	return nil
}

// Enabled is the saved choice; tailscale.Service refuses to install or start Tailscale
// while it is true, reading it under Exclusive.
func (s *Service) Enabled() (bool, error) {
	set, err := s.LoadSettings()
	return set.Enabled, err
}
