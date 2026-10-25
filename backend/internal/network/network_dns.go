package network

import (
	"fmt"
	"strings"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/platform"
)

// GetDNSConfig returns the custom DNS configuration.
func (n *Service) GetDNSConfig() (models.DNSConfig, error) {
	opts, err := n.uci.GetAll("network", "wan")
	if err != nil {
		return models.DNSConfig{}, err
	}
	config := models.DNSConfig{
		UseCustomDNS: opts["peerdns"] == "0",
	}
	if dns, ok := opts["dns"]; ok && dns != "" {
		config.Servers = strings.Split(dns, " ")
	}
	return config, nil
}

// SetDNSConfig updates the custom DNS configuration.
// SetDNSConfig sets the WAN DNS servers.
//
// Locked, not just committed: `network` is in the VPN transaction's revert set, so
// an unlocked writer here can have its staged delta discarded by a failing VPN
// toggle and still answer 200 — the user is told the WAN DNS changed and it did
// not. It was also the mirror image: its own `uci commit network` could commit the
// VPN's half-written peer, leaving a wg0 peer with new allowed_ips and no matching
// public_key.
func (n *Service) SetDNSConfig(config models.DNSConfig) error {
	return platform.MutateUCI(n.uci, []string{"network"}, func() error {
		if config.UseCustomDNS {
			if err := n.uciSet("network", "wan", "peerdns", "0"); err != nil {
				return err
			}
			dns := strings.Join(config.Servers, " ")
			if err := n.uciSet("network", "wan", "dns", dns); err != nil {
				return err
			}
		} else {
			if err := n.uciSet("network", "wan", "peerdns", "1"); err != nil {
				return err
			}
			if err := n.uciSet("network", "wan", "dns", ""); err != nil {
				return err
			}
		}
		return n.uciCommit("network")
	})
}

// GetDNSEntries returns all local DNS entries from dhcp config (domain sections).
func (n *Service) GetDNSEntries() ([]models.DNSEntry, error) {
	sections, err := n.uci.GetSections("dhcp")
	if err != nil {
		// A failed read is not "no entries": returning an empty list would make
		// the UI invite a duplicate entry for an entry that already exists.
		return nil, fmt.Errorf("reading dhcp sections: %w", err)
	}
	var entries []models.DNSEntry
	for section, opts := range sections {
		if opts[".type"] != "domain" {
			continue
		}
		entries = append(entries, models.DNSEntry{
			Name:    opts["name"],
			IP:      opts["ip"],
			Section: section,
		})
	}
	if entries == nil {
		return []models.DNSEntry{}, nil
	}
	return entries, nil
}

// AddDNSEntry adds a new local DNS entry as a named UCI section in dhcp config.
func (n *Service) AddDNSEntry(entry models.DNSEntry) error {
	// `dhcp` is locked, not just committed-and-hoped: the uci CLI keeps
	// uncommitted changes in the process-global /tmp/.uci/dhcp/changes file, so
	// an unlocked writer interleaves with AddDHCPReservation (which IS wrapped)
	// and destroys its staged delta. Verified on the device: concurrent adds to
	// these two endpoints failed with "uci: Invalid argument" and "uci set
	// dhcp.dns_...name", i.e. each request's own section was clobbered by the
	// other. This was a documented gap in the mutateUCI rollout; it is not a
	// theoretical one.
	return platform.MutateUCI(n.uci, []string{"dhcp"}, func() error {
		section := "dns_" + sanitizeSectionName(entry.Name)
		// Explicit duplicate check, inside the lock: real uci reuses the section
		// instead of failing, so a second entry with the same name would silently
		// REPLACE the first one's IP.
		exists, err := n.uciSectionExists("dhcp", section)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("DNS entry %q already exists", entry.Name)
		}
		if err := n.uci.AddSection("dhcp", section, "domain"); err != nil {
			return fmt.Errorf("adding DNS entry section: %w", err)
		}
		if err := n.uci.Set("dhcp", section, "name", entry.Name); err != nil {
			return fmt.Errorf("setting DNS entry name: %w", err)
		}
		if err := n.uci.Set("dhcp", section, "ip", entry.IP); err != nil {
			return fmt.Errorf("setting DNS entry IP: %w", err)
		}
		return n.uci.Commit("dhcp")
	})
}

// DeleteDNSEntry removes a local DNS entry by its UCI section name.
func (n *Service) DeleteDNSEntry(section string) error {
	// Same lock as AddDNSEntry, and for the same reason: a delete that reverts
	// or commits `dhcp` concurrently with an add silently discards the add.
	return platform.MutateUCI(n.uci, []string{"dhcp"}, func() error {
		if err := n.requireNamedDHCPSection(section, "domain"); err != nil {
			return err
		}
		if err := n.uci.DeleteSection("dhcp", section); err != nil {
			return fmt.Errorf("deleting DNS entry: %w", err)
		}
		return n.uci.Commit("dhcp")
	})
}
