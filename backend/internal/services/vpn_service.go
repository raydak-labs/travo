package services

import (
	"fmt"
	"strings"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

// VpnService provides VPN status and configuration.
type VpnService struct {
	uci uci.UCI
}

// NewVpnService creates a new VpnService.
func NewVpnService(u uci.UCI) *VpnService {
	return &VpnService{uci: u}
}

// GetVpnStatus returns all VPN connection statuses.
func (v *VpnService) GetVpnStatus() ([]models.VpnStatus, error) {
	var statuses []models.VpnStatus

	// WireGuard — only include if configured
	disabled, err := v.uci.Get("network", "wg0", "disabled")
	if err == nil {
		// WireGuard is configured
		wgStatus := models.VpnStatus{Type: "wireguard"}
		wgStatus.Enabled = disabled != "1"
		wgStatus.Connected = wgStatus.Enabled
		if wgStatus.Enabled {
			if endpoint, err := v.uci.Get("network", "wg0_peer0", "endpoint"); err == nil {
				wgStatus.Endpoint = endpoint
			}
		}
		statuses = append(statuses, wgStatus)
	}

	// Tailscale
	statuses = append(statuses, models.VpnStatus{
		Type:    "tailscale",
		Enabled: false,
	})

	return statuses, nil
}

// GetWireguardConfig returns the WireGuard configuration.
func (v *VpnService) GetWireguardConfig() (models.WireguardConfig, error) {
	opts, err := v.uci.GetAll("network", "wg0")
	if err != nil {
		// WireGuard not configured - return empty config (not an error)
		return models.WireguardConfig{}, nil
	}

	config := models.WireguardConfig{
		PrivateKey: opts["private_key"],
		Address:    opts["addresses"],
	}
	if dns, ok := opts["dns"]; ok && dns != "" {
		config.DNS = strings.Split(dns, " ")
	}

	// Get peer
	peerOpts, err := v.uci.GetAll("network", "wg0_peer0")
	if err == nil {
		peer := models.WireguardPeer{
			PublicKey: peerOpts["public_key"],
			Endpoint:  peerOpts["endpoint"],
		}
		if ips, ok := peerOpts["allowed_ips"]; ok && ips != "" {
			peer.AllowedIPs = strings.Split(ips, ",")
		}
		config.Peers = []models.WireguardPeer{peer}
	}

	return config, nil
}

// SetWireguardConfig updates the WireGuard configuration.
func (v *VpnService) SetWireguardConfig(config models.WireguardConfig) error {
	if config.PrivateKey != "" {
		_ = v.uci.Set("network", "wg0", "private_key", config.PrivateKey)
	}
	if config.Address != "" {
		_ = v.uci.Set("network", "wg0", "addresses", config.Address)
	}
	if len(config.DNS) > 0 {
		_ = v.uci.Set("network", "wg0", "dns", strings.Join(config.DNS, " "))
	}
	return v.uci.Commit("network")
}

// ToggleWireguard enables or disables WireGuard.
func (v *VpnService) ToggleWireguard(enable bool) error {
	val := "1"
	if enable {
		val = "0"
	}
	_ = v.uci.Set("network", "wg0", "disabled", val)
	return v.uci.Commit("network")
}

// GetTailscaleStatus returns Tailscale status.
func (v *VpnService) GetTailscaleStatus() (models.TailscaleStatus, error) {
	return models.TailscaleStatus{
		Installed: false,
		Running:   false,
		LoggedIn:  false,
	}, nil
}

// ImportWireguardConfig parses a .conf file and applies it via UCI.
func (v *VpnService) ImportWireguardConfig(confContent string) error {
	parsed, err := ParseWireguardConfig(confContent)
	if err != nil {
		return err
	}

	// Apply interface settings
	_ = v.uci.Set("network", "wg0", "private_key", parsed.Interface.PrivateKey)
	if parsed.Interface.Address != "" {
		_ = v.uci.Set("network", "wg0", "addresses", parsed.Interface.Address)
	}
	if parsed.Interface.DNS != "" {
		_ = v.uci.Set("network", "wg0", "dns", parsed.Interface.DNS)
	}

	// Apply first peer (extend if needed)
	for i, peer := range parsed.Peers {
		section := fmt.Sprintf("wg0_peer%d", i)
		_ = v.uci.Set("network", section, "public_key", peer.PublicKey)
		if peer.Endpoint != "" {
			_ = v.uci.Set("network", section, "endpoint", peer.Endpoint)
		}
		if peer.AllowedIPs != "" {
			_ = v.uci.Set("network", section, "allowed_ips", peer.AllowedIPs)
		}
		if peer.PresharedKey != "" {
			_ = v.uci.Set("network", section, "preshared_key", peer.PresharedKey)
		}
	}

	return v.uci.Commit("network")
}

// ToggleTailscale enables or disables Tailscale.
func (v *VpnService) ToggleTailscale(_ bool) error {
	// Stub - tailscale not installed in mock
	return nil
}
