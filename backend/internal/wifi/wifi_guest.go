package wifi

import (
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/platform"
)

// The guest subnet is fixed, so enabling guest WiFi means claiming
// 192.168.2.0/24 outright. On a router whose LAN is already on that subnet the
// claim would create a second interface on one network — DHCP handing out
// addresses the LAN already routes — so the request is refused instead.
// ErrGuestSubnetOverlap is returned so the API can map it to a 4xx.
var ErrGuestSubnetOverlap = errors.New("refusing to create the guest network: its subnet " +
	"overlaps an existing network interface on this router")

// GetGuestWifi returns the guest WiFi configuration.
func (w *Service) GetGuestWifi() (*models.GuestWifiConfig, error) {
	opts, err := w.uci.GetAll("wireless", "guest")
	if err != nil {
		return &models.GuestWifiConfig{Enabled: false}, nil
	}
	return &models.GuestWifiConfig{
		Enabled:    opts["disabled"] != "1",
		SSID:       opts["ssid"],
		Encryption: opts["encryption"],
		Key:        opts["key"],
	}, nil
}

// SetGuestWifi creates or updates the guest WiFi network with full isolation.
func (w *Service) SetGuestWifi(cfg models.GuestWifiConfig, req LockoutRequest) (*WirelessApplyResult, error) {
	// Disabling guest WiFi takes the guest access point down; enabling it puts
	// one up.
	if !cfg.Enabled {
		if err := w.guardLockoutExcluding(req, "guest", ""); err != nil {
			return nil, err
		}
	}
	return w.mutateWireless([]string{"wireless", "network", "dhcp", "firewall"}, func() (*WirelessApplyResult, error) {
		if !cfg.Enabled {
			return w.teardownGuestWifi()
		}
		// The guest subnet is fixed, so the only question is whether this
		// router already has it. Refuse first: the network, dhcp and firewall
		// commits below cannot be undone by revertUCIConfig.
		if err := w.rejectGuestSubnetOverlap(); err != nil {
			return nil, err
		}
		// Pick the radio and police it before writing anything. The refusal has to
		// come first: the network and dhcp commits below cannot be undone by
		// revertUCIConfig, so a request answered as "refused" must not already
		// have created network.guest, dhcp.guest and the guest firewall zone.
		guestRadio, err := w.preferredGuestRadio()
		if err != nil {
			return nil, err
		}
		if err := w.rejectAPOnUplinkRadio(guestRadio, "guest access point enabled"); err != nil {
			return nil, err
		}
		// Network interface for guest subnet
		if err := w.ensureNamedSection("network", "guest", "interface"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("network", "guest", "proto", "static"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("network", "guest", "ipaddr", guestIPAddr); err != nil {
			return nil, err
		}
		if err := w.uci.Set("network", "guest", "netmask", guestNetmask); err != nil {
			return nil, err
		}
		if err := w.uci.Commit("network"); err != nil {
			return nil, err
		}
		// DHCP for guest network
		if err := w.ensureNamedSection("dhcp", "guest", "dhcp"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("dhcp", "guest", "interface", "guest"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("dhcp", "guest", "start", "100"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("dhcp", "guest", "limit", "50"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("dhcp", "guest", "leasetime", "2h"); err != nil {
			return nil, err
		}
		if err := w.uci.Commit("dhcp"); err != nil {
			return nil, err
		}
		// Wireless interface for guest AP
		if err := w.ensureNamedSection("wireless", "guest", "wifi-iface"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("wireless", "guest", "device", guestRadio); err != nil {
			return nil, err
		}
		if err := w.uci.Set("wireless", "guest", "mode", "ap"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("wireless", "guest", "network", "guest"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("wireless", "guest", "ssid", cfg.SSID); err != nil {
			return nil, err
		}
		if err := w.uci.Set("wireless", "guest", "encryption", cfg.Encryption); err != nil {
			return nil, err
		}
		if err := w.uci.Set("wireless", "guest", "key", cfg.Key); err != nil {
			return nil, err
		}
		if err := w.uci.Set("wireless", "guest", "isolate", "1"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("wireless", "guest", "disabled", "0"); err != nil {
			return nil, err
		}
		if err := w.ensureSectionRadioEnabled("guest"); err != nil {
			return nil, err
		}
		if err := w.uci.Commit("wireless"); err != nil {
			return nil, err
		}
		// Firewall zone for guest
		if err := w.ensureNamedSection("firewall", "guest_zone", "zone"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("firewall", "guest_zone", "name", "guest"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("firewall", "guest_zone", "network", "guest"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("firewall", "guest_zone", "input", "REJECT"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("firewall", "guest_zone", "output", "ACCEPT"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("firewall", "guest_zone", "forward", "REJECT"); err != nil {
			return nil, err
		}
		// Forwarding: guest -> wan
		if err := w.ensureNamedSection("firewall", "guest_fwd", "forwarding"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("firewall", "guest_fwd", "src", "guest"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("firewall", "guest_fwd", "dest", "wan"); err != nil {
			return nil, err
		}
		// Allow DNS from guest
		if err := w.ensureNamedSection("firewall", "guest_dns", "rule"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("firewall", "guest_dns", "name", "Allow-Guest-DNS"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("firewall", "guest_dns", "src", "guest"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("firewall", "guest_dns", "dest_port", "53"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("firewall", "guest_dns", "target", "ACCEPT"); err != nil {
			return nil, err
		}
		// Allow DHCP from guest
		if err := w.ensureNamedSection("firewall", "guest_dhcp", "rule"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("firewall", "guest_dhcp", "name", "Allow-Guest-DHCP"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("firewall", "guest_dhcp", "src", "guest"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("firewall", "guest_dhcp", "dest_port", "67-68"); err != nil {
			return nil, err
		}
		if err := w.uci.Set("firewall", "guest_dhcp", "target", "ACCEPT"); err != nil {
			return nil, err
		}
		if err := w.uci.Commit("firewall"); err != nil {
			return nil, err
		}
		return w.stageWirelessApply()
	})
}

// rejectGuestSubnetOverlap refuses the fixed guest subnet when an existing
// network interface already claims it. Two interfaces on one subnet is not a
// cosmetic problem: the guest DHCP scope hands out addresses the LAN already
// routes, and the guest firewall zone then applies to LAN traffic as well.
func (w *Service) rejectGuestSubnetOverlap() error {
	sections, err := w.uci.GetSections("network")
	if err != nil {
		return fmt.Errorf("reading network sections: %w", err)
	}
	guest := ipv4Net(guestIPAddr, guestNetmask)
	if guest == nil {
		return fmt.Errorf("guest subnet %s/%s is not a valid IPv4 network", guestIPAddr, guestNetmask)
	}
	names := make([]string, 0, len(sections))
	for name := range sections {
		names = append(names, name)
	}
	sort.Strings(names) // the error must name the same section every time
	for _, name := range names {
		if name == guestNetwork {
			continue // re-enabling the guest network itself is not an overlap
		}
		opts := sections[name]
		existing := ipv4Net(opts["ipaddr"], opts["netmask"])
		if existing == nil || !subnetsOverlap(guest, existing) {
			continue
		}
		return fmt.Errorf("%w: network.%s is %s/%s",
			ErrGuestSubnetOverlap, name, opts["ipaddr"], opts["netmask"])
	}
	return nil
}

// ipv4Net turns an address/netmask pair into its network, or nil when either is
// absent or not IPv4. Interfaces without both (DHCP clients, wireguard peers)
// are skipped: they have no static subnet to collide with.
func ipv4Net(ip, mask string) *net.IPNet {
	addr := net.ParseIP(strings.TrimSpace(ip)).To4()
	m := net.ParseIP(strings.TrimSpace(mask)).To4()
	if addr == nil || m == nil {
		return nil
	}
	netmask := net.IPMask(m)
	return &net.IPNet{IP: addr.Mask(netmask), Mask: netmask}
}

// subnetsOverlap reports whether two IPv4 networks share any address.
func subnetsOverlap(a, b *net.IPNet) bool {
	return a.Contains(b.IP) || b.Contains(a.IP)
}

// teardownGuestWifi disables the guest AP and removes the guest subnet it
// owned. Setting wireless.guest.disabled=1 alone leaves network.guest,
// dhcp.guest and the four firewall.guest_* sections in place, so the
// 192.168.2.0/24 interface, its DHCP scope and the guest zone stay up and
// continue to route and answer on a network the operator switched off.
func (w *Service) teardownGuestWifi() (*WirelessApplyResult, error) {
	abort := func(err error) (*WirelessApplyResult, error) {
		platform.RevertUCIConfig(w.uci, "wireless", "network", "dhcp", "firewall")
		return nil, err
	}
	if _, err := w.uci.GetAll("wireless", "guest"); err != nil {
		// Guest WiFi was never configured: nothing to tear down.
		return nil, nil
	}

	// 1. Take the AP down first so no client is left on a subnet that is about
	//    to disappear.
	if err := w.uci.Set("wireless", "guest", "disabled", "1"); err != nil {
		return abort(err)
	}
	if err := w.uci.Commit("wireless"); err != nil {
		return abort(err)
	}

	// 2. Remove the DHCP scope before the interface it is bound to.
	if err := w.deleteSectionIfPresent("dhcp", "guest"); err != nil {
		return abort(err)
	}
	if err := w.uci.Commit("dhcp"); err != nil {
		return abort(err)
	}

	// 3. Remove the guest network interface (192.168.2.0/24).
	if err := w.deleteSectionIfPresent("network", "guest"); err != nil {
		return abort(err)
	}
	if err := w.uci.Commit("network"); err != nil {
		return abort(err)
	}

	// 4. Remove the guest firewall zone and its rules/forwarding.
	for _, section := range []string{"guest_dhcp", "guest_dns", "guest_fwd", "guest_zone"} {
		if err := w.deleteSectionIfPresent("firewall", section); err != nil {
			return abort(err)
		}
	}
	if err := w.uci.Commit("firewall"); err != nil {
		return abort(err)
	}

	return w.stageWirelessApply()
}

// deleteSectionIfPresent removes config/section when it exists. A section that
// was never created is not an error: teardown must be idempotent.
func (w *Service) deleteSectionIfPresent(config, section string) error {
	if _, err := w.uci.GetAll(config, section); err != nil {
		return nil
	}
	if err := w.uci.DeleteSection(config, section); err != nil {
		return fmt.Errorf("removing %s.%s: %w", config, section, err)
	}
	return nil
}
