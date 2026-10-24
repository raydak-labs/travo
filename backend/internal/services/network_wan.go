package services

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

// GetWanConfig returns the WAN configuration.
func (n *NetworkService) GetWanConfig() (models.WanConfig, error) {
	opts, err := n.uci.GetAll("network", "wan")
	if err != nil {
		return models.WanConfig{}, err
	}
	var dnsServers []string
	if dns, ok := opts["dns"]; ok && dns != "" {
		dnsServers = strings.Split(dns, " ")
	}
	mtu := 1500
	if m, ok := opts["mtu"]; ok && m != "" {
		if v, err := strconv.Atoi(m); err == nil {
			mtu = v
		}
	}
	return models.WanConfig{
		Type: opts["proto"], InterfaceName: opts["ifname"],
		IPAddress: opts["ip4addr"], Netmask: opts["netmask"],
		Gateway: opts["gateway"], DNSServers: dnsServers, MTU: mtu,
	}, nil
}

// wanStaticOptions are the options that only mean something for a static WAN.
// They are deleted when the WAN is switched to another mode: leaving them behind
// is how a static→dhcp switch produced a WAN that kept a stale ipaddr/netmask/
// gateway, and — combined with a leftover peerdns=0 + dns from SetDNSConfig —
// kept resolving through a DNS server the DHCP lease never handed out.
var wanStaticOptions = []string{"ip4addr", "netmask", "gateway"}

// errPPPoEUnsupported is returned for a save that would create a PPPoE WAN
// without credentials. Writing proto=pppoe alone produces a WAN that can never
// authenticate: the handler validates "type", not a username/password pair the
// model does not have.
var errPPPoEUnsupported = errors.New(
	"PPPoE needs a username and password that this API cannot store: " +
		"configure PPPoE in LuCI (Network -> Interfaces -> WAN), then save other WAN fields here")

// SetWanConfig updates the WAN configuration.
//
// Everything the request declares is written. MTU and the DNS servers used to be
// validated by the handler, published in the OpenAPI body and then dropped on
// the floor, so the documented "GET, change a field, PUT it back" client pattern
// answered 200 having changed nothing at all.
func (n *NetworkService) SetWanConfig(config models.WanConfig) error {
	return mutateUCI(n.uci, []string{"network"}, func() error {
		if config.MTU != 0 && (config.MTU < 68 || config.MTU > 9000) {
			return fmt.Errorf("MTU must be between 68 and 9000, got %d", config.MTU)
		}
		current := map[string]string{}
		if opts, err := n.uci.GetAll("network", "wan"); err == nil {
			current = opts
		}
		if config.Type == "pppoe" {
			// Editing other fields of an already-configured PPPoE WAN is fine;
			// switching INTO one is not.
			if current["proto"] != "pppoe" || current["username"] == "" || current["password"] == "" {
				return errPPPoEUnsupported
			}
		}
		if config.Type != "" {
			if err := n.uciSet("network", "wan", "proto", config.Type); err != nil {
				return err
			}
			if config.Type != current["proto"] && config.Type != "static" {
				for _, option := range wanStaticOptions {
					// Only options that are actually set: `uci delete` on a
					// missing entry is itself an error, and that must not fail
					// the whole save.
					if _, present := current[option]; !present {
						continue
					}
					if err := n.uci.DeleteOption("network", "wan", option); err != nil {
						return fmt.Errorf("clear %s after switching WAN to %s: %w", option, config.Type, err)
					}
				}
			}
		}
		if config.IPAddress != "" {
			if err := n.uciSet("network", "wan", "ip4addr", config.IPAddress); err != nil {
				return err
			}
		}
		if config.Netmask != "" {
			if err := n.uciSet("network", "wan", "netmask", config.Netmask); err != nil {
				return err
			}
		}
		if config.Gateway != "" {
			if err := n.uciSet("network", "wan", "gateway", config.Gateway); err != nil {
				return err
			}
		}
		if config.MTU > 0 {
			if err := n.uciSet("network", "wan", "mtu", strconv.Itoa(config.MTU)); err != nil {
				return err
			}
		}
		// A nil dns_servers means "not part of this save"; an explicit empty
		// list means "go back to the DHCP-provided resolvers".
		if config.DNSServers != nil {
			peerdns := "1"
			if len(config.DNSServers) > 0 {
				peerdns = "0"
			}
			if err := n.uciSet("network", "wan", "peerdns", peerdns); err != nil {
				return err
			}
			if err := n.uciSet("network", "wan", "dns", strings.Join(config.DNSServers, " ")); err != nil {
				return err
			}
		}
		return n.uciCommit("network")
	})
}

// DetectWanType auto-detects the WAN connection type and returns
// both the detected type and the currently configured type.
func (n *NetworkService) DetectWanType() (models.WanDetectResult, error) {
	// Read current UCI configuration
	currentType := "dhcp"
	opts, err := n.uci.GetAll("network", "wan")
	if err == nil {
		if proto, ok := opts["proto"]; ok && proto != "" {
			currentType = proto
		}
	}

	// Detect running services to determine actual WAN type
	var detectedType string

	// Check if pppd is running → PPPoE
	if _, err := n.cmd.Run("pgrep", "-x", "pppd"); err == nil {
		detectedType = "pppoe"
	} else if _, err := n.cmd.Run("pgrep", "-x", "udhcpc"); err == nil {
		// Check if udhcpc (DHCP client) is running → DHCP
		detectedType = "dhcp"
	} else {
		// Neither running — fall back to current config
		detectedType = currentType
	}

	return models.WanDetectResult{
		DetectedType: detectedType,
		CurrentType:  currentType,
	}, nil
}
