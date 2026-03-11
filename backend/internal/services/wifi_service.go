package services

import (
	"fmt"
	"os/exec"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

// WifiReloader applies wireless configuration changes (e.g. "wifi reload").
type WifiReloader interface {
	Reload() error
}

// ShellWifiReloader runs "wifi reload" via exec.
type ShellWifiReloader struct{}

// Reload executes "wifi reload" to apply UCI wireless changes.
func (r *ShellWifiReloader) Reload() error {
	return exec.Command("wifi", "reload").Run()
}

// NoopWifiReloader does nothing (for tests).
type NoopWifiReloader struct{}

// Reload is a no-op.
func (r *NoopWifiReloader) Reload() error { return nil }

// WifiService provides WiFi scanning, connection, and configuration.
type WifiService struct {
	uci      uci.UCI
	ubus     ubus.Ubus
	reloader WifiReloader
}

// NewWifiService creates a new WifiService.
func NewWifiService(u uci.UCI, ub ubus.Ubus) *WifiService {
	return &WifiService{uci: u, ubus: ub, reloader: &ShellWifiReloader{}}
}

// NewWifiServiceWithReloader creates a WifiService with a custom reloader (for tests).
func NewWifiServiceWithReloader(u uci.UCI, ub ubus.Ubus, r WifiReloader) *WifiService {
	return &WifiService{uci: u, ubus: ub, reloader: r}
}

// findSTADevice discovers the station (client) WiFi interface name by querying network.wireless status.
// It looks for an interface with mode "sta" and returns its ifname (e.g., "phy0-sta0") and section name (e.g., "wifinet2").
func (w *WifiService) findSTADevice() (ifname string, section string, err error) {
	resp, err := w.ubus.Call("network.wireless", "status", nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to get wireless status: %w", err)
	}

	for _, radioData := range resp {
		radioMap, ok := radioData.(map[string]interface{})
		if !ok {
			continue
		}
		ifaces, ok := radioMap["interfaces"].([]interface{})
		if !ok {
			continue
		}
		for _, iface := range ifaces {
			ifaceMap, ok := iface.(map[string]interface{})
			if !ok {
				continue
			}
			config, ok := ifaceMap["config"].(map[string]interface{})
			if !ok {
				continue
			}
			mode, _ := config["mode"].(string)
			if mode == "sta" {
				ifn, _ := ifaceMap["ifname"].(string)
				sec, _ := ifaceMap["section"].(string)
				if ifn != "" {
					return ifn, sec, nil
				}
			}
		}
	}
	return "", "", fmt.Errorf("no STA interface found")
}

// Scan returns available WiFi networks.
func (w *WifiService) Scan() ([]models.WifiScanResult, error) {
	ifname, _, err := w.findSTADevice()
	if err != nil {
		return []models.WifiScanResult{}, nil
	}

	resp, err := w.ubus.Call("iwinfo", "scan", map[string]interface{}{"device": ifname})
	if err != nil {
		return nil, err
	}

	results, ok := resp["results"].([]interface{})
	if !ok {
		return []models.WifiScanResult{}, nil
	}

	var scanResults []models.WifiScanResult
	for _, r := range results {
		rm, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		ssid, _ := rm["ssid"].(string)
		bssid, _ := rm["bssid"].(string)
		ch, _ := rm["channel"].(float64)
		sig, _ := rm["signal"].(float64)
		qual, _ := rm["quality"].(float64)
		band, _ := rm["band"].(string)

		enc := "none"
		if encMap, ok := rm["encryption"].(map[string]interface{}); ok {
			if enabled, ok := encMap["enabled"].(bool); ok && enabled {
				if wpa, ok := encMap["wpa"].([]interface{}); ok {
					if auth, ok := encMap["authentication"].([]interface{}); ok {
						authStr := ""
						if len(auth) > 0 {
							authStr, _ = auth[0].(string)
						}
						wpaVer := 0
						if len(wpa) > 0 {
							wpaVer = int(wpa[len(wpa)-1].(float64))
						}
						if authStr == "sae" {
							enc = "sae"
						} else if authStr == "psk" && wpaVer == 2 {
							enc = "psk2"
						} else if authStr == "psk" {
							enc = "psk"
						} else {
							enc = "psk2"
						}
					} else {
						enc = "psk2"
					}
				} else {
					enc = "wep"
				}
			}
		}

		// Detect band from frequency or channel
		if band == "" {
			freq, _ := rm["frequency"].(float64)
			if freq > 0 {
				if freq < 3000 {
					band = "2.4GHz"
				} else if freq < 6000 {
					band = "5GHz"
				} else {
					band = "6GHz"
				}
			} else if ch > 0 {
				if ch <= 14 {
					band = "2.4GHz"
				} else if ch <= 177 {
					band = "5GHz"
				} else {
					band = "6GHz"
				}
			}
		}

		scanResults = append(scanResults, models.WifiScanResult{
			SSID: ssid, BSSID: bssid,
			Channel: int(ch), SignalDBM: int(sig),
			SignalPercent: int(qual), Encryption: enc, Band: band,
		})
	}
	return scanResults, nil
}

// Connect connects to a WiFi network.
func (w *WifiService) Connect(config models.WifiConfig) error {
	_, section, err := w.findSTADevice()
	if err != nil {
		section = "sta0" // fallback
	}
	_ = w.uci.Set("wireless", section, "ssid", config.SSID)
	_ = w.uci.Set("wireless", section, "key", config.Password)
	if config.Encryption != "" {
		_ = w.uci.Set("wireless", section, "encryption", config.Encryption)
	}
	_ = w.uci.Set("wireless", section, "disabled", "0")
	if err := w.uci.Commit("wireless"); err != nil {
		return err
	}
	return w.reloader.Reload()
}

// Disconnect disconnects from the current WiFi network.
func (w *WifiService) Disconnect() error {
	_, section, err := w.findSTADevice()
	if err != nil {
		section = "sta0"
	}
	_ = w.uci.Set("wireless", section, "disabled", "1")
	if err := w.uci.Commit("wireless"); err != nil {
		return err
	}
	return w.reloader.Reload()
}

// GetConnection returns the current WiFi connection info.
func (w *WifiService) GetConnection() (models.WifiConnection, error) {
	ifname, _, err := w.findSTADevice()
	if err != nil {
		return models.WifiConnection{}, nil
	}

	resp, err := w.ubus.Call("iwinfo", "info", map[string]interface{}{"device": ifname})
	if err != nil {
		return models.WifiConnection{}, err
	}

	ssid, _ := resp["ssid"].(string)
	bssid, _ := resp["bssid"].(string)
	mode, _ := resp["mode"].(string)
	ch, _ := resp["channel"].(float64)
	sig, _ := resp["signal"].(float64)
	qual, _ := resp["quality"].(float64)
	enc, _ := resp["encryption"].(string)
	band, _ := resp["band"].(string)

	conn := models.WifiConnection{
		SSID: ssid, BSSID: bssid,
		Mode: mode, Channel: int(ch),
		SignalDBM: int(sig), SignalPercent: int(qual),
		Encryption: enc, Band: band,
		Connected: ssid != "",
	}

	// Get IP from wwan interface
	if conn.Connected {
		if wwanData, err := w.ubus.Call("network.interface.wwan", "status", nil); err == nil {
			if addrs, ok := wwanData["ipv4-address"].([]interface{}); ok && len(addrs) > 0 {
				if a, ok := addrs[0].(map[string]interface{}); ok {
					conn.IPAddress, _ = a["address"].(string)
				}
			}
		}
	}

	return conn, nil
}

// SetMode sets the WiFi operating mode (e.g., "ap", "sta", "repeater").
func (w *WifiService) SetMode(mode string) error {
	_ = w.uci.Set("wireless", "default_radio0", "mode", mode)
	if err := w.uci.Commit("wireless"); err != nil {
		return err
	}
	return w.reloader.Reload()
}

// GetSavedNetworks returns saved WiFi networks.
func (w *WifiService) GetSavedNetworks() ([]models.SavedNetwork, error) {
	resp, err := w.ubus.Call("network.wireless", "status", nil)
	if err != nil {
		return []models.SavedNetwork{}, nil
	}

	var networks []models.SavedNetwork
	for _, radioData := range resp {
		radioMap, ok := radioData.(map[string]interface{})
		if !ok {
			continue
		}
		ifaces, ok := radioMap["interfaces"].([]interface{})
		if !ok {
			continue
		}
		for _, iface := range ifaces {
			ifaceMap, ok := iface.(map[string]interface{})
			if !ok {
				continue
			}
			config, ok := ifaceMap["config"].(map[string]interface{})
			if !ok {
				continue
			}
			mode, _ := config["mode"].(string)
			if mode != "sta" {
				continue
			}
			ssid, _ := config["ssid"].(string)
			encryption, _ := config["encryption"].(string)

			section, _ := ifaceMap["section"].(string)
			disabled := false
			if section != "" {
				if d, err := w.uci.Get("wireless", section, "disabled"); err == nil {
					disabled = d == "1"
				}
			}

			networks = append(networks, models.SavedNetwork{
				SSID:        ssid,
				Section:     section,
				Encryption:  encryption,
				Mode:        mode,
				AutoConnect: !disabled,
				Priority:    1,
			})
		}
	}
	return networks, nil
}

// DeleteNetwork removes a saved WiFi network by its UCI section name.
func (w *WifiService) DeleteNetwork(section string) error {
	if section == "" {
		return fmt.Errorf("section name is required")
	}
	if err := w.uci.DeleteSection("wireless", section); err != nil {
		return fmt.Errorf("failed to delete network: %w", err)
	}
	if err := w.uci.Commit("wireless"); err != nil {
		return err
	}
	return w.reloader.Reload()
}
