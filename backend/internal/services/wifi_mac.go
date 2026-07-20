package services

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

// MAC address management (clone, randomize, policies).

// GetMACAddresses returns the MAC address info for WiFi interfaces.
func (w *WifiService) GetMACAddresses() ([]models.MACConfig, error) {
	var configs []models.MACConfig

	staSection, err := w.findSTASection()
	if err != nil {
		return configs, nil // No STA section; return empty (not an error)
	}
	staOpts, err := w.uci.GetAll("wireless", staSection)
	if err != nil {
		return configs, nil
	}
	currentMAC := ""
	// Try reading from sysfs (ifname pattern: phy<N>-sta<N>)
	if ifname, _, sysErr := w.findSTADevice(); sysErr == nil && ifname != "" {
		if data, readErr := os.ReadFile("/sys/class/net/" + ifname + "/address"); readErr == nil {
			currentMAC = strings.TrimSpace(string(data))
		}
	}
	customMAC := staOpts["macaddr"]
	isApplied := customMAC != "" && strings.EqualFold(currentMAC, customMAC)
	configs = append(configs, models.MACConfig{
		Interface:  "sta",
		CurrentMAC: currentMAC,
		CustomMAC:  customMAC,
		IsApplied:  isApplied,
	})

	return configs, nil
}

// SetMACAddress sets a custom MAC address on the STA WiFi interface.
// It writes the macaddr UCI option (persists across reboots via wifi up) and
// also applies it immediately via "ip link set" so the change takes effect
// without requiring a full wifi restart.
func (w *WifiService) SetMACAddress(mac string) (*WirelessApplyResult, error) {
	staSection, err := w.findSTASection()
	if err != nil {
		return nil, fmt.Errorf("STA interface not found")
	}
	if mac == "" {
		// Reset: clear the macaddr option
		if err := w.uci.Set("wireless", staSection, "macaddr", ""); err != nil {
			return nil, fmt.Errorf("clearing MAC: %w", err)
		}
	} else {
		if err := w.uci.Set("wireless", staSection, "macaddr", mac); err != nil {
			return nil, fmt.Errorf("setting MAC: %w", err)
		}
	}
	if err := w.uci.Commit("wireless"); err != nil {
		return nil, fmt.Errorf("committing wireless: %w", err)
	}

	// Apply the MAC immediately at runtime via ip link so it takes effect
	// without waiting for wifi up. mac80211.sh also applies it on wifi up.
	w.applyMACImmediate(mac)

	return w.stageWirelessApply()
}

// applyMACImmediate applies (or restores) the MAC address on the live STA
// interface right now using ip link, without requiring a wifi restart.
// Errors are ignored because the UCI/wifi-up path is the authoritative one.
func (w *WifiService) applyMACImmediate(mac string) {
	if w.cmd == nil {
		return
	}
	ifname, _, err := w.findSTADevice()
	if err != nil || ifname == "" {
		return
	}
	if mac == "" {
		// Restore hardware MAC from the phy's permanent address list.
		hwMAC := w.readPhyHardwareMAC(ifname)
		if hwMAC == "" {
			return // can't restore without knowing the permanent MAC
		}
		mac = hwMAC
	}
	_, _ = w.cmd.Run("ip", "link", "set", ifname, "down")
	_, _ = w.cmd.Run("ip", "link", "set", ifname, "address", mac)
	_, _ = w.cmd.Run("ip", "link", "set", ifname, "up")
}

// readPhyHardwareMAC reads the permanent/hardware MAC address for a wireless
// interface from its parent phy's sysfs address list.
func (w *WifiService) readPhyHardwareMAC(ifname string) string {
	// Resolve the phy name from the interface: /sys/class/net/<ifname>/phy80211/name
	phyNameBytes, err := os.ReadFile("/sys/class/net/" + ifname + "/phy80211/name")
	if err != nil {
		return ""
	}
	phyName := strings.TrimSpace(string(phyNameBytes))
	// Read the first address from the phy (permanent hardware MAC).
	addrsBytes, err := os.ReadFile("/sys/class/ieee80211/" + phyName + "/addresses")
	if err != nil {
		return ""
	}
	lines := strings.Fields(string(addrsBytes))
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}
	return ""
}

// RandomizeMAC generates a random locally-administered unicast MAC address
// and applies it to the STA WiFi interface. It returns the new MAC.
func (w *WifiService) RandomizeMAC() (string, *WirelessApplyResult, error) {
	mac, err := generateRandomMAC()
	if err != nil {
		return "", nil, fmt.Errorf("generating random MAC: %w", err)
	}
	apply, err := w.SetMACAddress(mac)
	if err != nil {
		return "", nil, err
	}
	return mac, apply, nil
}

// generateRandomMAC creates a random locally-administered unicast MAC address.
// Locally-administered: bit 1 of first octet set. Unicast: bit 0 of first octet cleared.
func generateRandomMAC() (string, error) {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	// Set locally-administered bit (bit 1) and clear unicast/multicast bit (bit 0)
	buf[0] = (buf[0] | 0x02) & 0xFE
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", buf[0], buf[1], buf[2], buf[3], buf[4], buf[5]), nil
}

const macPoliciesPath = "/etc/travo/mac-policies.json"

// GetMACPolicies returns the saved per-network MAC policies.
func (w *WifiService) GetMACPolicies() (models.MACPolicies, error) {
	data, err := os.ReadFile(macPoliciesPath)
	if err != nil {
		return models.MACPolicies{Policies: []models.MACPolicy{}}, nil
	}
	var p models.MACPolicies
	if err := json.Unmarshal(data, &p); err != nil {
		return models.MACPolicies{Policies: []models.MACPolicy{}}, nil
	}
	return p, nil
}

// SetMACPolicies saves the per-network MAC policies.
func (w *WifiService) SetMACPolicies(policies models.MACPolicies) error {
	data, err := json.Marshal(policies)
	if err != nil {
		return err
	}
	if err := os.MkdirAll("/etc/travo", 0o755); err != nil {
		return err
	}
	return os.WriteFile(macPoliciesPath, data, 0o644)
}
