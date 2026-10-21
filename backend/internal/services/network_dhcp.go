package services

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

// GetDHCPConfig returns the DHCP configuration for the LAN.
func (n *NetworkService) GetDHCPConfig() (models.DHCPConfig, error) {
	opts, err := n.uci.GetAll("dhcp", "lan")
	if err != nil {
		return models.DHCPConfig{}, err
	}
	start := 100
	if s, ok := opts["start"]; ok && s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			start = v
		}
	}
	limit := 150
	if l, ok := opts["limit"]; ok && l != "" {
		if v, err := strconv.Atoi(l); err == nil {
			limit = v
		}
	}
	leaseTime := "12h"
	if lt, ok := opts["leasetime"]; ok && lt != "" {
		leaseTime = lt
	}
	return models.DHCPConfig{Start: start, Limit: limit, LeaseTime: leaseTime}, nil
}

// SetDHCPConfig updates the DHCP configuration for the LAN.
// SetDHCPConfig updates the LAN DHCP pool. Locked for the same reason as
// SetDNSConfig: `dhcp` is in the VPN transaction's revert set, and the VPN's
// DNS-forwarding helpers delete and re-add this same dhcp.@dnsmasq[0] section.
func (n *NetworkService) SetDHCPConfig(config models.DHCPConfig) error {
	return mutateUCI(n.uci, []string{"dhcp"}, func() error {
		if err := n.uciSet("dhcp", "lan", "start", strconv.Itoa(config.Start)); err != nil {
			return err
		}
		if err := n.uciSet("dhcp", "lan", "limit", strconv.Itoa(config.Limit)); err != nil {
			return err
		}
		if err := n.uciSet("dhcp", "lan", "leasetime", config.LeaseTime); err != nil {
			return err
		}
		return n.uciCommit("dhcp")
	})
}

// GetDHCPLeases reads active DHCP leases from /tmp/dhcp.leases.
func (n *NetworkService) GetDHCPLeases() []models.DHCPLease {
	data, err := os.ReadFile("/tmp/dhcp.leases")
	if err != nil {
		return []models.DHCPLease{}
	}
	return parseDHCPLeases(string(data))
}

// uciSectionExists reports whether a UCI section is already present.
//
// It exists because real `uci set config.section=stype` CREATES OR UPDATES:
// AddSection is not a duplicate check. Callers that relied on its error passed
// under a stricter mock and silently UPSERTED on the device.
func (n *NetworkService) uciSectionExists(config, section string) (bool, error) {
	sections, err := n.uci.GetSections(config)
	if err != nil {
		return false, fmt.Errorf("reading %s sections: %w", config, err)
	}
	_, ok := sections[section]
	return ok, nil
}

// requireNamedDHCPSection rejects anything that is not a named section of the
// expected type before it is deleted.
//
// uci.validSectionName deliberately admits anonymous references (@dnsmasq[0]),
// because stock /etc/config files use them, so
// DELETE /api/v1/network/dns/entries/@dnsmasq[0] passed validation and
// `uci delete dhcp.@dnsmasq[0]` removed the whole dnsmasq section and committed
// it: DHCP pool, lease time, DNS servers, DHCPv6 and RA all gone, with nothing
// but a 200 to show for it. Both add paths only ever create named sections, so
// no legitimate caller can produce an anonymous reference.
func (n *NetworkService) requireNamedDHCPSection(section, wantType string) error {
	if strings.HasPrefix(section, "@") {
		return fmt.Errorf("refusing to delete anonymous dhcp section %s: "+
			"it is part of the file, not one entry", section)
	}
	sections, err := n.uci.GetSections("dhcp")
	if err != nil {
		return fmt.Errorf("reading dhcp sections: %w", err)
	}
	opts, ok := sections[section]
	if !ok {
		return fmt.Errorf("dhcp section not found: %s", section)
	}
	if opts[".type"] != wantType {
		return fmt.Errorf("dhcp section %s is a %q section, not %q", section, opts[".type"], wantType)
	}
	return nil
}

// sanitizeSectionName converts a hostname to a valid UCI section name.
func sanitizeSectionName(name string) string {
	var sb strings.Builder
	for _, c := range strings.ToLower(name) {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' {
			sb.WriteRune(c)
		} else {
			sb.WriteRune('_')
		}
	}
	return sb.String()
}

// GetDHCPReservations returns all static DHCP reservations (host sections in dhcp config).
func (n *NetworkService) GetDHCPReservations() ([]models.DHCPReservation, error) {
	sections, err := n.uci.GetSections("dhcp")
	if err != nil {
		return nil, fmt.Errorf("reading dhcp sections: %w", err)
	}
	var reservations []models.DHCPReservation
	for section, opts := range sections {
		if opts[".type"] != "host" {
			continue
		}
		reservations = append(reservations, models.DHCPReservation{
			Name:    opts["name"],
			MAC:     opts["mac"],
			IP:      opts["ip"],
			Section: section,
		})
	}
	if reservations == nil {
		return []models.DHCPReservation{}, nil
	}
	return reservations, nil
}

// AddDHCPReservation adds a static DHCP reservation as a named UCI section in dhcp config.
func (n *NetworkService) AddDHCPReservation(reservation models.DHCPReservation) error {
	return mutateUCI(n.uci, []string{"dhcp"}, func() error {
		section := "host_" + sanitizeSectionName(reservation.Name)
		// Explicit duplicate check, inside the lock: a same-named reservation
		// would otherwise be overwritten in place, silently changing which MAC
		// gets which address.
		exists, err := n.uciSectionExists("dhcp", section)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("DHCP reservation %q already exists", reservation.Name)
		}
		if err := n.uci.AddSection("dhcp", section, "host"); err != nil {
			return fmt.Errorf("adding DHCP reservation section: %w", err)
		}
		if err := n.uci.Set("dhcp", section, "name", reservation.Name); err != nil {
			return fmt.Errorf("setting DHCP reservation name: %w", err)
		}
		if err := n.uci.Set("dhcp", section, "mac", reservation.MAC); err != nil {
			return fmt.Errorf("setting DHCP reservation MAC: %w", err)
		}
		if err := n.uci.Set("dhcp", section, "ip", reservation.IP); err != nil {
			return fmt.Errorf("setting DHCP reservation IP: %w", err)
		}
		return n.uci.Commit("dhcp")
	})
}

// DeleteDHCPReservation removes a static DHCP reservation by its UCI section name.
func (n *NetworkService) DeleteDHCPReservation(section string) error {
	return mutateUCI(n.uci, []string{"dhcp"}, func() error {
		if err := n.requireNamedDHCPSection(section, "host"); err != nil {
			return err
		}
		if err := n.uci.DeleteSection("dhcp", section); err != nil {
			return fmt.Errorf("deleting DHCP reservation: %w", err)
		}
		return n.uci.Commit("dhcp")
	})
}
