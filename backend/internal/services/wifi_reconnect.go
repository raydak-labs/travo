package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

// Auto-reconnect cron script and WiFi on/off schedule.

// GetAutoReconnect returns whether auto-reconnect is enabled.
func (w *WifiService) GetAutoReconnect() (bool, error) {
	data, err := os.ReadFile(w.autoReconnectFile)
	if err != nil {
		return false, nil
	}
	var config struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return false, nil
	}
	return config.Enabled, nil
}

// SetAutoReconnect enables or disables auto-reconnect to saved WiFi networks.
// When enabled, it writes a reconnect script and adds a cron entry.
// When disabled, it removes the cron entry and script.
func (w *WifiService) SetAutoReconnect(enabled bool) error {
	dir := filepath.Dir(w.autoReconnectFile)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	config := struct {
		Enabled bool `json:"enabled"`
	}{Enabled: enabled}
	data, _ := json.Marshal(config)
	if err := os.WriteFile(w.autoReconnectFile, data, 0600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	if enabled {
		return w.enableAutoReconnect()
	}
	return w.disableAutoReconnect()
}

// reconnectScriptContent is the safe script body (wifi up, not wifi reload).
//
// Two layered guards:
//  1. Crash guard: written before `wifi up`; if the call causes a kernel crash
//     the file survives reboot and every subsequent cron tick becomes a no-op.
//  2. Failure-count guard: increments on every non-crash failure. Once it hits
//     MAX_FAIL the script stops retrying — this catches the case where the
//     saved wireless config is broken (e.g. after an rpcd rollback restored a
//     pre-incident bad config) and cron would otherwise replay the failure
//     forever. Counter is cleared on any successful reconnect or on redeploy.
const reconnectScriptContent = "#!/bin/sh\n# Auto-reconnect to saved WiFi networks\n# Managed by openwrt-travel-gui — do not edit manually\n\n" +
	"GUARD=\"/etc/travo/autoreconnect-crash-guard\"\n" +
	"FAILCOUNT_FILE=\"/etc/travo/autoreconnect-failcount\"\n" +
	"MAX_FAIL=5\n\n" +
	"if [ -f \"$GUARD\" ]; then\n    exit 0\nfi\n\n" +
	"FAILCOUNT=0\n" +
	"if [ -f \"$FAILCOUNT_FILE\" ]; then\n    FAILCOUNT=$(cat \"$FAILCOUNT_FILE\" 2>/dev/null || echo 0)\nfi\n" +
	"if [ \"$FAILCOUNT\" -ge \"$MAX_FAIL\" ] 2>/dev/null; then\n    exit 0\nfi\n\n" +
	"IP=$(ubus call network.interface.wwan status 2>/dev/null | jsonfilter -e '@[\"ipv4-address\"][0].address' 2>/dev/null)\n" +
	"if [ -n \"$IP\" ]; then\n    rm -f \"$FAILCOUNT_FILE\"\n    exit 0\nfi\n\n" +
	"# Connection dropped — write crash guard, bring up WiFi, update counters on exit\n" +
	"echo wifi-reconnect > \"$GUARD\"\n" +
	"if wifi up; then\n" +
	"    rm -f \"$GUARD\" \"$FAILCOUNT_FILE\"\n" +
	"else\n" +
	"    rm -f \"$GUARD\"\n" +
	"    echo $((FAILCOUNT + 1)) > \"$FAILCOUNT_FILE\"\n" +
	"fi\n"

func (w *WifiService) enableAutoReconnect() error {
	scriptDir := filepath.Dir(w.reconnectScript)
	if err := os.MkdirAll(scriptDir, 0750); err != nil {
		return fmt.Errorf("creating script directory: %w", err)
	}
	if err := os.WriteFile(w.reconnectScript, []byte(reconnectScriptContent), 0750); err != nil {
		return fmt.Errorf("writing reconnect script: %w", err)
	}

	// Add cron entry (every minute)
	cronCmd := fmt.Sprintf(`(crontab -l 2>/dev/null | grep -v '%s'; echo '* * * * * %s') | crontab -`,
		w.reconnectScript, w.reconnectScript)
	if _, err := w.cmd.Run("sh", "-c", cronCmd); err != nil {
		return fmt.Errorf("adding cron entry: %w", err)
	}
	return nil
}

// WriteReconnectScriptSafe writes the current safe reconnect script to disk if the
// script file already exists. Call this on startup so devices that had auto-reconnect
// enabled before a deploy get the safe "wifi up" script instead of the old "wifi reload".
func (w *WifiService) WriteReconnectScriptSafe() {
	if _, err := os.Stat(w.reconnectScript); err != nil {
		return // script not present, nothing to fix
	}
	_ = os.WriteFile(w.reconnectScript, []byte(reconnectScriptContent), 0750)
}

func (w *WifiService) disableAutoReconnect() error {
	// Remove cron entry
	cronCmd := fmt.Sprintf(`(crontab -l 2>/dev/null | grep -v '%s') | crontab -`, w.reconnectScript)
	_, _ = w.cmd.Run("sh", "-c", cronCmd)

	// Remove script file
	_ = os.Remove(w.reconnectScript)
	return nil
}

const wifiSchedulePath = "/etc/travo/wifi-schedule.json"
const wifiScheduleCronPath = "/etc/cron.d/openwrt-gui-wifi-schedule"

// GetWiFiSchedule returns the current cron-based WiFi on/off schedule.
func (w *WifiService) GetWiFiSchedule() (models.WiFiSchedule, error) {
	data, err := os.ReadFile(wifiSchedulePath)
	if err != nil {
		return models.WiFiSchedule{Enabled: false}, nil
	}
	var s models.WiFiSchedule
	if err := json.Unmarshal(data, &s); err != nil {
		return models.WiFiSchedule{Enabled: false}, nil
	}
	return s, nil
}

// SetWiFiSchedule saves the WiFi schedule and updates the cron file.
func (w *WifiService) SetWiFiSchedule(schedule models.WiFiSchedule) error {
	data, err := json.Marshal(schedule)
	if err != nil {
		return err
	}
	if err := os.MkdirAll("/etc/travo", 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(wifiSchedulePath, data, 0o644); err != nil {
		return err
	}

	if !schedule.Enabled || schedule.OnTime == "" || schedule.OffTime == "" {
		_ = os.Remove(wifiScheduleCronPath)
		return nil
	}

	onParts := strings.Split(schedule.OnTime, ":")
	offParts := strings.Split(schedule.OffTime, ":")
	if len(onParts) != 2 || len(offParts) != 2 {
		return fmt.Errorf("invalid time format, expected HH:MM")
	}

	// cron format: MM HH * * * user command
	cronContent := fmt.Sprintf("%s %s * * * root /sbin/wifi up\n%s %s * * * root /sbin/wifi down\n",
		onParts[1], onParts[0], offParts[1], offParts[0])
	return os.WriteFile(wifiScheduleCronPath, []byte(cronContent), 0o644)
}
