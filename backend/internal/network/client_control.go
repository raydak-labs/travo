package network

import (
	"fmt"
	"strings"

	"github.com/openwrt-travel-gui/backend/internal/platform"
)

// KickClient disconnects a WiFi client by deauthentication.
// It returns an error when no interface accepted the disassociate so the UI can
// report the failure instead of pretending the client is gone.
func (n *Service) KickClient(mac string) error {
	// Discover AP interfaces dynamically using iw dev
	iwDevOutput, err := n.cmd.Run("iw", "dev")
	if err != nil {
		// Fallback to common AP interfaces if iw fails
		for _, iface := range []string{"phy0-ap0", "phy1-ap0", "wlan0", "wlan1"} {
			if _, kickErr := n.cmd.Run("hostapd_cli", "-i", iface, "disassociate", mac); kickErr == nil {
				return nil
			}
		}
		return fmt.Errorf("kick %s: no wireless interface accepted the disassociate", mac)
	}

	// Parse iw dev output to find AP interfaces
	attempted := false
	for _, iface := range parseIwDev(string(iwDevOutput)) {
		attempted = true
		if _, kickErr := n.cmd.Run("hostapd_cli", "-i", iface, "disassociate", mac); kickErr == nil {
			// Successfully kicked from this interface
			return nil
		}
	}
	if !attempted {
		return fmt.Errorf("kick %s: no AP interface found to disassociate from", mac)
	}
	return fmt.Errorf("kick %s: no AP interface accepted the disassociate", mac)
}

// BlockClient adds a firewall rule to drop all traffic from a MAC address.
// Every failure after the first staged `uci` write reverts the staged firewall
// delta: the uci CLI keeps uncommitted changes in /tmp/.uci/firewall/changes,
// so an abandoned half-written rule would be committed by a later, unrelated
// `uci commit firewall`.
//
// This is a direct `uci commit firewall` + firewall reload rather than the rpcd
// apply/confirm flow, because a block has to be enforced on the running firewall
// immediately — a rollback window that silently expires on a policy change would
// unblock a client the operator believes is filtered. ADR 0004 §5 requires such
// a path to be listed there with a named crash guard AND a rollback; the
// rollback is the undoFirewallSection call below. The guard row is still
// missing (it needs ADR 0004 §5, ADR 0003 §2, scripts/deploy-local.sh and
// scripts/install.sh), and writing a guard here without those would be worse
// than no guard: nothing would clear it, and blocking would stay dead until a
// redeploy.
func (n *Service) BlockClient(mac string) error {
	return platform.MutateUCI(n.uci, []string{"firewall"}, func() error {
		section := "block_" + normalizeMACForSection(mac)
		macUpper := strings.ToUpper(mac)

		// Explicit duplicate check, inside the lock: real uci reuses the rule
		// section, so re-blocking an already-blocked client would report success
		// and restart the firewall for nothing.
		exists, err := n.uciSectionExists("firewall", section)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("client %s is already blocked", macUpper)
		}
		if err := n.uci.AddSection("firewall", section, "rule"); err != nil {
			return fmt.Errorf("add firewall block rule: %w", err)
		}
		if err := n.uciSet("firewall", section, "name", "Block-"+macUpper); err != nil {
			return err
		}
		if err := n.uciSet("firewall", section, "src", "lan"); err != nil {
			return err
		}
		if err := n.uciSet("firewall", section, "src_mac", macUpper); err != nil {
			return err
		}
		if err := n.uciSet("firewall", section, "target", "DROP"); err != nil {
			return err
		}
		if err := n.uciCommit("firewall"); err != nil {
			return err
		}
		if err := n.restartService("firewall"); err != nil {
			// The rule is committed but the running firewall never loaded it.
			// Take it back out so the file and the running firewall agree.
			if undoErr := n.undoFirewallSection(section, nil); undoErr != nil {
				return fmt.Errorf("restart firewall: %w (rollback of %s failed: %w)", err, section, undoErr)
			}
			return fmt.Errorf("restart firewall: %w", err)
		}
		return nil
	})
}

// UnblockClient removes the firewall block rule for a MAC address.
func (n *Service) UnblockClient(mac string) error {
	return platform.MutateUCI(n.uci, []string{"firewall"}, func() error {
		section := "block_" + normalizeMACForSection(mac)

		previous, _ := n.uci.GetAll("firewall", section)
		if err := n.uci.DeleteSection("firewall", section); err != nil {
			return fmt.Errorf("delete firewall block rule: %w", err)
		}
		// A failed commit leaves the deletion staged; mutateUCI reverts it.
		if err := n.uciCommit("firewall"); err != nil {
			return err
		}
		if err := n.restartService("firewall"); err != nil {
			// Same contract as BlockClient: the committed config must not be
			// left saying "unblocked" while the running firewall still drops it.
			if undoErr := n.undoFirewallSection(section, previous); undoErr != nil {
				return fmt.Errorf("restart firewall: %w (rollback of %s failed: %w)", err, section, undoErr)
			}
			return fmt.Errorf("restart firewall: %w", err)
		}
		return nil
	})
}

// undoFirewallSection restores a firewall section to a previous option set —
// nil means the section must be gone — and commits the result.
func (n *Service) undoFirewallSection(section string, previous map[string]string) error {
	// An unblock has usually already deleted the section by this point, and a
	// missing section IS the target state, so a failed delete is only an error
	// when the section is in fact still there.
	if _, err := n.uci.GetAll("firewall", section); err == nil {
		if err := n.uci.DeleteSection("firewall", section); err != nil {
			return err
		}
	}
	if previous != nil {
		if err := n.uci.AddSection("firewall", section, "rule"); err != nil {
			return err
		}
		for option, value := range previous {
			if strings.HasPrefix(option, ".") {
				continue
			}
			if err := n.uci.Set("firewall", section, option, value); err != nil {
				return err
			}
		}
	}
	return n.uciCommit("firewall")
}

// GetBlockedClients returns a list of blocked MAC addresses.
func (n *Service) GetBlockedClients() ([]string, error) {
	sections, err := n.uci.GetSections("firewall")
	if err != nil {
		// Never report "nothing is blocked" on a failed read: that silently
		// unblocks clients the operator believes are filtered.
		return nil, fmt.Errorf("reading firewall sections: %w", err)
	}
	var blocked []string
	for _, opts := range sections {
		if opts[".type"] == "rule" && strings.HasPrefix(opts["name"], "Block-") {
			if mac := opts["src_mac"]; mac != "" {
				blocked = append(blocked, mac)
			}
		}
	}
	if blocked == nil {
		return []string{}, nil
	}
	return blocked, nil
}
