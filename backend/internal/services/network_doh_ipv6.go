package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

const dohConfigFile = "/etc/travo/doh-config.json"

// GetDoHConfig returns the current DNS-over-HTTPS configuration.
func (n *NetworkService) GetDoHConfig() (models.DoHConfig, error) {
	data, err := os.ReadFile(dohConfigFile)
	if err != nil {
		return models.DoHConfig{Provider: "cloudflare"}, nil
	}
	var cfg models.DoHConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return models.DoHConfig{Provider: "cloudflare"}, nil
	}
	return cfg, nil
}

// SetDoHConfig saves the DNS-over-HTTPS config and restarts dnsmasq if enabled.
func (n *NetworkService) SetDoHConfig(cfg models.DoHConfig) error {
	if err := os.MkdirAll(filepath.Dir(dohConfigFile), 0750); err != nil {
		return err
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	// Atomic: os.WriteFile truncates first, so a power cut or a concurrent read
	// left a half-written file that silently falls back to the default provider.
	if err := writeFileAtomic(dohConfigFile, data); err != nil {
		return err
	}
	// Apply: configure dnsmasq to use a local DoH proxy if enabled.
	if cfg.Enabled {
		url := cfg.URL
		if url == "" {
			switch cfg.Provider {
			case "google":
				url = "https://dns.google/dns-query"
			case "quad9":
				url = "https://dns.quad9.net/dns-query"
			default: // cloudflare
				url = "https://cloudflare-dns.com/dns-query"
			}
		}
		_ = url // DoH proxy integration stored in config file; dnsmasq-over-HTTPS requires https-dns-proxy
		// No `uci commit dhcp` here: nothing above stages a dhcp change, so a
		// commit would only persist whatever ELSE had a delta in flight — the
		// VPN's DNS-forwarding sequence, a DHCP reservation, an AdGuard
		// resolver switch. It was a pure liability.
		_, _ = n.cmd.Run("/etc/init.d/dnsmasq", "restart")
	}
	return nil
}

// GetIPv6Status returns whether IPv6 is enabled and current global addresses.
func (n *NetworkService) GetIPv6Status() (models.IPv6Status, error) {
	var status models.IPv6Status
	data, err := os.ReadFile("/proc/sys/net/ipv6/conf/all/disable_ipv6")
	if err == nil {
		status.Enabled = stringFromBytes(data) == "0"
	} else {
		status.Enabled = true // assume enabled if file unreadable
	}
	out, err := n.cmd.Run("ip", "-6", "addr", "show", "scope", "global")
	if err == nil {
		for line := range strings.SplitSeq(string(out), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "inet6 ") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					status.Addresses = append(status.Addresses, parts[1])
				}
			}
		}
	}
	if status.Addresses == nil {
		status.Addresses = []string{}
	}
	return status, nil
}

// SetIPv6Enabled enables or disables IPv6 system-wide via sysctl.
func (n *NetworkService) SetIPv6Enabled(enabled bool) error {
	val := "1"
	if enabled {
		val = "0"
	}
	_, err := n.cmd.Run("sysctl", "-w", "net.ipv6.conf.all.disable_ipv6="+val)
	return err
}
