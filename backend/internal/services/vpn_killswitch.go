package services

import (
	"fmt"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/platform"
)

// GetKillSwitch checks whether the VPN kill switch firewall rule exists.
func (v *VpnService) GetKillSwitch() (models.KillSwitchStatus, error) {
	opts, err := v.uci.GetAll("firewall", vpnKillSwitchSection)
	if err != nil {
		return models.KillSwitchStatus{Enabled: false}, nil
	}
	return models.KillSwitchStatus{
		Enabled: opts["src"] == "lan" && opts["dest"] == "wan" && opts["target"] == "REJECT",
	}, nil
}

// vpnKillSwitchSection is the firewall rule implementing the VPN kill switch.
const vpnKillSwitchSection = "vpn_killswitch"

// vpnKillSwitchOwnerOption marks which flow created the kill switch rule.
// "user" means the user configured it as a standalone policy, so a VPN toggle
// must not delete it; "vpn_toggle" means the VPN toggle owns it.
const (
	vpnKillSwitchOwnerOption = "travo_owner"
	vpnKillSwitchOwnerUser   = "user"
	vpnKillSwitchOwnerToggle = "vpn_toggle"
)

// SetKillSwitch enables or disables the VPN kill switch firewall rule.
// Every write error is propagated: a partial rule would be committed with the
// default target (ACCEPT), i.e. a "kill switch" that allows all traffic.
func (v *VpnService) SetKillSwitch(enabled bool) error {
	return platform.MutateUCI(v.uci, vpnFirewallConfigs, func() error {
		if enabled {
			// Create the firewall rule that blocks LAN→WAN when VPN is down.
			if _, err := v.uci.GetAll("firewall", vpnKillSwitchSection); err != nil {
				if addErr := v.uci.AddSection("firewall", vpnKillSwitchSection, "rule"); addErr != nil {
					return fmt.Errorf("creating vpn kill switch rule: %w", addErr)
				}
			}
			settings := [][2]string{
				{"name", "VPN Kill Switch"},
				{"src", "lan"},
				{"dest", "wan"},
				{"target", "REJECT"},
				{vpnKillSwitchOwnerOption, vpnKillSwitchOwnerUser},
			}
			for _, kv := range settings {
				if err := v.uci.Set("firewall", vpnKillSwitchSection, kv[0], kv[1]); err != nil {
					return fmt.Errorf("set vpn kill switch %s: %w", kv[0], err)
				}
			}
		} else {
			// Remove the firewall rule; a missing section is not an error.
			if _, err := v.uci.GetAll("firewall", vpnKillSwitchSection); err == nil {
				if delErr := v.uci.DeleteSection("firewall", vpnKillSwitchSection); delErr != nil {
					return fmt.Errorf("removing vpn kill switch rule: %w", delErr)
				}
			}
		}
		if err := v.uci.Commit("firewall"); err != nil {
			return err
		}
		v.reloadFirewall()
		return nil
	})
}

// removeVPNOwnedKillSwitch deletes the kill switch rule only when the VPN toggle
// created it. A kill switch the user configured as a standalone policy is left
// untouched.
//
// It does NOT take a config lock: every caller is already inside the VPN
// transaction that holds `firewall`, and the config locks are not reentrant.
func (v *VpnService) removeVPNOwnedKillSwitch() error {
	{
		opts, err := v.uci.GetAll("firewall", vpnKillSwitchSection)
		if err != nil {
			// Nothing to remove.
			return nil
		}
		if opts[vpnKillSwitchOwnerOption] != vpnKillSwitchOwnerToggle {
			return nil
		}
		if err := v.uci.DeleteSection("firewall", vpnKillSwitchSection); err != nil {
			return fmt.Errorf("removing vpn kill switch rule: %w", err)
		}
		if err := v.uci.Commit("firewall"); err != nil {
			return fmt.Errorf("committing firewall: %w", err)
		}
		v.reloadFirewall()
		return nil
	}
}
