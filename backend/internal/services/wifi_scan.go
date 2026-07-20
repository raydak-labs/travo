package services

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

// Scanning: iwinfo parsing, band mapping, STA scan section, signal info, radio switching.

// ensureSTASectionForScan creates a STA wifi-iface (sta0) if none exists (for Connect, not for Scan).
// Uses network=wwan so the STA gets DHCP and is used as WAN when connected; ensures wwan exists.
// Returns the STA section name and commits wireless if created.
func (w *WifiService) ensureSTASectionForScan() (string, error) {
	if section, err := w.findSTASection(); err == nil {
		return section, nil
	}
	if err := w.ensureWwanNetwork(); err != nil {
		return "", fmt.Errorf("ensuring wwan network: %w", err)
	}
	sections, err := w.uci.GetSections("wireless")
	if err != nil {
		return "", fmt.Errorf("wireless sections: %w", err)
	}
	var firstRadio string
	for name, opts := range sections {
		if opts["type"] != "" {
			firstRadio = name
			break
		}
	}
	if firstRadio == "" {
		return "", fmt.Errorf("no radio found in wireless config")
	}
	if err := w.uci.AddSection("wireless", "sta0", "wifi-iface"); err != nil {
		return "", fmt.Errorf("adding sta0: %w", err)
	}
	_ = w.uci.Set("wireless", "sta0", "device", firstRadio)
	_ = w.uci.Set("wireless", "sta0", "mode", "sta")
	_ = w.uci.Set("wireless", "sta0", "network", "wwan")
	_ = w.uci.Set("wireless", "sta0", "disabled", "0")
	if err := w.uci.Commit("wireless"); err != nil {
		return "", fmt.Errorf("committing sta0: %w", err)
	}
	return "sta0", nil
}

// parseIwinfoEncryption parses the structured encryption map returned by iwinfo
// (both from "scan" and "info" responses) into a normalized encryption string
// (e.g. "psk2", "sae", "wep", "none").
func parseIwinfoEncryption(encField interface{}) string {
	encMap, ok := encField.(map[string]interface{})
	if !ok {
		// Fallback: plain string (e.g. UCI value stored directly).
		if s, ok := encField.(string); ok && s != "" {
			return s
		}
		return "none"
	}
	enabled, _ := encMap["enabled"].(bool)
	if !enabled {
		return "none"
	}
	if wpa, ok := encMap["wpa"].([]interface{}); ok {
		authStr := ""
		if auth, ok := encMap["authentication"].([]interface{}); ok && len(auth) > 0 {
			authStr, _ = auth[0].(string)
		}
		wpaVer := 0
		if len(wpa) > 0 {
			wpaVer = int(wpa[len(wpa)-1].(float64))
		}
		switch {
		case authStr == "sae":
			return "sae"
		case authStr == "psk" && wpaVer == 2:
			return "psk2"
		case authStr == "psk":
			return "psk"
		default:
			return "psk2"
		}
	}
	return "wep"
}

// parseScanResultItem builds a WifiScanResult from one iwinfo scan result map.
// bandOverride (e.g. "2.4GHz", "5GHz") is used when the result has no band; empty means derive from channel/frequency.
func parseScanResultItem(rm map[string]interface{}, bandOverride string) models.WifiScanResult {
	ssid, _ := rm["ssid"].(string)
	bssid, _ := rm["bssid"].(string)
	ch, _ := rm["channel"].(float64)
	sig, _ := rm["signal"].(float64)
	qual, _ := rm["quality"].(float64)
	band, _ := rm["band"].(string)

	enc := parseIwinfoEncryption(rm["encryption"])

	if band == "" && bandOverride != "" {
		band = bandOverride
	}
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

	return models.WifiScanResult{
		SSID: ssid, BSSID: bssid,
		Channel: int(ch), SignalDBM: int(sig),
		SignalPercent: int(qual), Encryption: enc, Band: band,
	}
}

// radioBandToDisplay maps UCI band (2g, 5g) to display band string.
func radioBandToDisplay(uciBand string) string {
	switch strings.ToLower(uciBand) {
	case "5g":
		return "5GHz"
	case "2g":
		return "2.4GHz"
	case "6g":
		return "6GHz"
	default:
		return ""
	}
}

// requestBandToUCI maps frontend/API band string (2.4ghz, 5ghz, 6ghz) to UCI band (2g, 5g, 6g).
func requestBandToUCI(band string) string {
	switch strings.ToLower(strings.TrimSpace(band)) {
	case "2.4ghz", "2.4g":
		return "2g"
	case "5ghz", "5g":
		return "5g"
	case "6ghz", "6g":
		return "6g"
	default:
		return ""
	}
}

// getRadioForBand returns the UCI radio section name (e.g. "radio0") that has the given band.
// band is the request band (2.4ghz, 5ghz, 6ghz) or UCI band (2g, 5g, 6g).
func (w *WifiService) getRadioForBand(band string) (string, error) {
	uciBand := requestBandToUCI(band)
	if uciBand == "" {
		uciBand = strings.ToLower(strings.TrimSpace(band))
		if uciBand != "2g" && uciBand != "5g" && uciBand != "6g" {
			return "", fmt.Errorf("unsupported band: %q", band)
		}
	}
	radios, err := w.getWifiRadioNames()
	if err != nil || len(radios) == 0 {
		return "", fmt.Errorf("no radios found")
	}
	for _, name := range radios {
		opts, _ := w.uci.GetAll("wireless", name)
		if opts["band"] == uciBand {
			return name, nil
		}
	}
	return "", fmt.Errorf("no radio found for band %s", uciBand)
}

// Scan returns available WiFi networks by scanning each radio (LuCI-style). No STA required.
func (w *WifiService) Scan() ([]models.WifiScanResult, error) {
	radios, err := w.getWifiRadioNames()
	if err != nil || len(radios) == 0 {
		return []models.WifiScanResult{}, nil
	}

	var all []models.WifiScanResult
	seen := make(map[string]bool)

	for _, radioName := range radios {
		opts, _ := w.uci.GetAll("wireless", radioName)
		bandOverride := radioBandToDisplay(opts["band"])

		resp, err := w.ubus.Call("iwinfo", "scan", map[string]interface{}{"device": radioName})
		if err != nil {
			continue
		}
		results, ok := resp["results"].([]interface{})
		if !ok {
			continue
		}
		for _, r := range results {
			rm, ok := r.(map[string]interface{})
			if !ok {
				continue
			}
			item := parseScanResultItem(rm, bandOverride)
			key := item.BSSID + ":" + strconv.Itoa(item.Channel)
			if seen[key] {
				continue
			}
			seen[key] = true
			all = append(all, item)
		}
	}

	sort.Slice(all, func(i, j int) bool {
		if all[i].SignalDBM != all[j].SignalDBM {
			return all[i].SignalDBM > all[j].SignalDBM
		}
		if all[i].Channel != all[j].Channel {
			return all[i].Channel < all[j].Channel
		}
		return all[i].SSID < all[j].SSID
	})
	return all, nil
}

// GetSTASignalInfo returns the active STA connection signal, SSID, and radio device.
// Returns (ssid="", signalDBM=0, radio="") when no STA is connected.
func (w *WifiService) GetSTASignalInfo() (ssid string, signalDBM int, radio string, err error) {
	ifname, _, err := w.findSTADevice()
	if err != nil || ifname == "" {
		return "", 0, "", nil // not connected
	}
	resp, err := w.ubus.Call("iwinfo", "info", map[string]interface{}{"device": ifname})
	if err != nil {
		return "", 0, "", err
	}
	ssid, _ = resp["ssid"].(string)
	if sig, ok := resp["signal"].(float64); ok {
		signalDBM = int(sig)
	}
	// Determine current radio from UCI
	section, _ := w.findSTASection()
	if section != "" {
		opts, _ := w.uci.GetAll("wireless", section)
		radio = opts["device"]
	}
	return ssid, signalDBM, radio, nil
}

// ScanRadioForSSID scans the given radio and returns the best signal for the target SSID.
// Returns (0, false, nil) when the SSID is not found on that radio.
func (w *WifiService) ScanRadioForSSID(radioName, ssid string) (int, bool, error) {
	resp, err := w.ubus.Call("iwinfo", "scan", map[string]interface{}{"device": radioName})
	if err != nil {
		return 0, false, err
	}
	results, _ := resp["results"].([]interface{})
	bestSignal := -999
	found := false
	for _, r := range results {
		rm, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		if s, _ := rm["ssid"].(string); s != ssid {
			continue
		}
		found = true
		if sig, ok := rm["signal"].(float64); ok && int(sig) > bestSignal {
			bestSignal = int(sig)
		}
	}
	if !found {
		return 0, false, nil
	}
	return bestSignal, true, nil
}

// SwitchSTAToRadio moves the active STA section to a different radio device and
// applies the change. This is for automated background switches; it uses
// applyWireless (ApplyAndConfirm) rather than the staged browser-confirm flow.
func (w *WifiService) SwitchSTAToRadio(targetRadio string) error {
	section, err := w.findSTASection()
	if err != nil {
		return fmt.Errorf("no STA section to switch: %w", err)
	}
	if err := w.uci.Set("wireless", section, "device", targetRadio); err != nil {
		return fmt.Errorf("uci set device: %w", err)
	}
	// Reconcile AP layout: the target radio may host an AP that must be disabled
	// (and the previous radio's AP re-enabled) to avoid ath11k/IPQ6018 crash.
	if err := w.reconcileRepeaterAPRadioLayout(); err != nil {
		return fmt.Errorf("reconciling AP radio layout: %w", err)
	}
	if err := w.uci.Commit("wireless"); err != nil {
		return fmt.Errorf("uci commit: %w", err)
	}
	return w.applyWireless()
}
