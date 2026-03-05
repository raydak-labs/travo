package services

import (
	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

// WifiService provides WiFi scanning, connection, and configuration.
type WifiService struct {
	uci  uci.UCI
	ubus ubus.Ubus
}

// NewWifiService creates a new WifiService.
func NewWifiService(u uci.UCI, ub ubus.Ubus) *WifiService {
	return &WifiService{uci: u, ubus: ub}
}

// Scan returns available WiFi networks.
func (w *WifiService) Scan() ([]models.WifiScanResult, error) {
	resp, err := w.ubus.Call("iwinfo", "scan", nil)
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
				enc = "psk2"
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
	_ = w.uci.Set("wireless", "sta0", "ssid", config.SSID)
	_ = w.uci.Set("wireless", "sta0", "key", config.Password)
	if config.Encryption != "" {
		_ = w.uci.Set("wireless", "sta0", "encryption", config.Encryption)
	}
	_ = w.uci.Set("wireless", "sta0", "disabled", "0")
	return w.uci.Commit("wireless")
}

// Disconnect disconnects from the current WiFi network.
func (w *WifiService) Disconnect() error {
	_ = w.uci.Set("wireless", "sta0", "disabled", "1")
	return w.uci.Commit("wireless")
}

// GetConnection returns the current WiFi connection info.
func (w *WifiService) GetConnection() (models.WifiConnection, error) {
	resp, err := w.ubus.Call("iwinfo", "info", nil)
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

	return models.WifiConnection{
		SSID: ssid, BSSID: bssid,
		Mode: mode, Channel: int(ch),
		SignalDBM: int(sig), SignalPercent: int(qual),
		Encryption: enc, Band: band,
		Connected: ssid != "", IPAddress: "192.168.1.100",
	}, nil
}

// SetMode sets the WiFi operating mode (e.g., "ap", "sta", "repeater").
func (w *WifiService) SetMode(mode string) error {
	_ = w.uci.Set("wireless", "default_radio0", "mode", mode)
	return w.uci.Commit("wireless")
}

// GetSavedNetworks returns saved WiFi networks.
func (w *WifiService) GetSavedNetworks() ([]models.SavedNetwork, error) {
	// Get sta0 config as a saved network
	opts, err := w.uci.GetAll("wireless", "sta0")
	if err != nil {
		return []models.SavedNetwork{}, nil
	}

	network := models.SavedNetwork{
		SSID:        opts["ssid"],
		Encryption:  opts["encryption"],
		Mode:        opts["mode"],
		AutoConnect: opts["disabled"] != "1",
		Priority:    1,
	}

	return []models.SavedNetwork{network}, nil
}
