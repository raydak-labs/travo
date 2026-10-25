package uplink

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/platform"
)

const splitTunnelPath = "/etc/travo/split-tunnel.json"

// GetSplitTunnel returns the current WireGuard split tunnel configuration.
func (v *VpnService) GetSplitTunnel() (models.SplitTunnelConfig, error) {
	data, err := os.ReadFile(splitTunnelPath)
	if err != nil {
		return models.SplitTunnelConfig{Mode: "all"}, nil
	}
	var cfg models.SplitTunnelConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return models.SplitTunnelConfig{Mode: "all"}, nil
	}
	return cfg, nil
}

// SetSplitTunnel saves the split tunnel config and updates WireGuard allowed IPs in UCI.
// Mode "all" routes everything through the VPN (0.0.0.0/0, ::/0); mode "custom"
// routes only the given CIDR ranges and requires at least one, so an empty list
// never silently becomes a full tunnel. Staged UCI deltas are reverted when any
// write fails.
//
// It holds the UCI locks for every config the VPN flow touches (vpnFlowConfigs).
func (v *VpnService) SetSplitTunnel(cfg models.SplitTunnelConfig) error {
	return platform.MutateUCI(v.uci, vpnFlowConfigs, func() error {
		return v.setSplitTunnelLocked(cfg)
	})
}

// setSplitTunnelLocked is SetSplitTunnel for a caller that holds the locks.
func (v *VpnService) setSplitTunnelLocked(cfg models.SplitTunnelConfig) error {
	allowedParts, err := splitTunnelAllowedIPs(cfg)
	if err != nil {
		return err
	}

	sections, err := v.uci.GetSections("network")
	if err != nil {
		return err
	}
	// Snapshot the current values so a partial write can be reverted.
	var snapshots []splitTunnelPeerSnapshot
	for name, opts := range sections {
		if !strings.HasPrefix(opts[".type"], "wireguard_") {
			continue
		}
		var allowed []string
		for _, cidr := range strings.Split(opts["allowed_ips"], ",") {
			if trimmed := strings.TrimSpace(cidr); trimmed != "" {
				allowed = append(allowed, trimmed)
			}
		}
		snapshots = append(snapshots, splitTunnelPeerSnapshot{name: name, allowed: allowed})
	}
	slices.SortFunc(snapshots, func(a, b splitTunnelPeerSnapshot) int { return strings.Compare(a.name, b.name) })

	for _, snap := range snapshots {
		if err := v.setPeerAllowedIPs(snap.name, allowedParts); err != nil {
			if rbErr := v.restorePeerAllowedIPs(snapshots); rbErr != nil {
				return fmt.Errorf("updating split tunnel: %w (revert incomplete: %w)", err, rbErr)
			}
			return fmt.Errorf("updating split tunnel: %w", err)
		}
	}
	if err := v.uci.Commit("network"); err != nil {
		if rbErr := v.restorePeerAllowedIPs(snapshots); rbErr != nil {
			return fmt.Errorf("committing split tunnel: %w (revert incomplete: %w)", err, rbErr)
		}
		return fmt.Errorf("committing split tunnel: %w", err)
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(splitTunnelPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(splitTunnelPath, data, 0o644)
}

// splitTunnelAllowedIPs validates the split tunnel config and returns the
// allowed_ips values to write to every WireGuard peer.
func splitTunnelAllowedIPs(cfg models.SplitTunnelConfig) ([]string, error) {
	switch cfg.Mode {
	case "all", "":
		return []string{"0.0.0.0/0", "::/0"}, nil
	case "custom":
		var routes []string
		for _, route := range cfg.Routes {
			if trimmed := strings.TrimSpace(route); trimmed != "" {
				routes = append(routes, trimmed)
			}
		}
		if len(routes) == 0 {
			return nil, errors.New("custom split tunnel requires at least one route")
		}
		return routes, nil
	default:
		return nil, fmt.Errorf("unknown split tunnel mode %q (expected \"all\" or \"custom\")", cfg.Mode)
	}
}

// setPeerAllowedIPs replaces a peer's allowed_ips with the given CIDR list.
func (v *VpnService) setPeerAllowedIPs(peer string, cidrs []string) error {
	_ = v.uci.DeleteOption("network", peer, "allowed_ips")
	for _, cidr := range cidrs {
		if err := v.uci.AddList("network", peer, "allowed_ips", cidr); err != nil {
			return fmt.Errorf("set allowed_ips %s: %w", cidr, err)
		}
	}
	return v.uci.Set("network", peer, "route_allowed_ips", "1")
}

// splitTunnelPeerSnapshot holds a peer's allowed_ips before a split tunnel change.
type splitTunnelPeerSnapshot struct {
	name    string
	allowed []string
}

// restorePeerAllowedIPs reverts peers to their pre-change allowed_ips values.
func (v *VpnService) restorePeerAllowedIPs(snapshots []splitTunnelPeerSnapshot) error {
	var errs []error
	for _, snap := range snapshots {
		if len(snap.allowed) == 0 {
			if err := v.uci.DeleteOption("network", snap.name, "allowed_ips"); err != nil {
				errs = append(errs, fmt.Errorf("clearing allowed_ips on %s: %w", snap.name, err))
			}
			continue
		}
		if err := v.setPeerAllowedIPs(snap.name, snap.allowed); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	if err := v.uci.Commit("network"); err != nil {
		return fmt.Errorf("committing reverted split tunnel: %w", err)
	}
	return nil
}
