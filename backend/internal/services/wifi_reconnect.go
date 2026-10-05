package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/execx"
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
//
// BOTH guards are worthless unless the script can create them, and the script
// runs from cron — potentially before the backend has ever run — so nothing
// else guarantees /etc/trafo exists. Without `mkdir -p` the writes fail
// silently (there is no `set -e`), the guard check cannot see a leftover
// guard, and the one sanctioned `wifi up` in the whole tree runs unguarded
// every minute, forever, with a fail counter that never increments. The
// directory is therefore created first and the script exits non-zero when that
// fails. Same shape as the toggle helper (wifi_toggle_script.go).
const reconnectScriptContent = "#!/bin/sh\n# Auto-reconnect to saved WiFi networks\n# Managed by openwrt-travel-gui — do not edit manually\n\n" +
	"GUARD_DIR=\"" + crashGuardDir + "\"\n" +
	"GUARD=\"$GUARD_DIR/autoreconnect-crash-guard\"\n" +
	"FAILCOUNT_FILE=\"$GUARD_DIR/autoreconnect-failcount\"\n" +
	"MAX_FAIL=5\n\n" +
	"# The guard directory must exist before anything below can write a guard\n" +
	"# or the retry counter. If it cannot be created, exit non-zero: running\n" +
	"# `wifi up` from here would be the one unguarded wifi invocation in the tree.\n" +
	"mkdir -p \"$GUARD_DIR\" || exit 1\n\n" +
	"if [ -f \"$GUARD\" ]; then\n    exit 0\nfi\n\n" +
	"FAILCOUNT=0\n" +
	"if [ -f \"$FAILCOUNT_FILE\" ]; then\n    FAILCOUNT=$(cat \"$FAILCOUNT_FILE\" 2>/dev/null || echo 0)\nfi\n" +
	"if [ \"$FAILCOUNT\" -ge \"$MAX_FAIL\" ] 2>/dev/null; then\n    exit 0\nfi\n\n" +
	"IP=$(ubus call network.interface.wwan status 2>/dev/null | jsonfilter -e '@[\"ipv4-address\"][0].address' 2>/dev/null)\n" +
	"if [ -n \"$IP\" ]; then\n    rm -f \"$FAILCOUNT_FILE\"\n    exit 0\nfi\n\n" +
	"# Connection dropped — write crash guard, bring up WiFi, update counters on exit\n" +
	"echo wifi-reconnect > \"$GUARD\" || exit 1\n" +
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
	if err := writeGeneratedScript(w.reconnectScript, reconnectScriptContent, 0o750); err != nil {
		return fmt.Errorf("writing reconnect script: %w", err)
	}

	// Add cron entry (every minute). Under crontabMu: this is a read-modify-write
	// of /etc/crontabs/root, the same file the WiFi/LED schedules rewrite, and
	// an unlocked rewrite here drops their entries (and vice versa).
	crontabMu.Lock()
	defer crontabMu.Unlock()
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
	_ = writeGeneratedScript(w.reconnectScript, reconnectScriptContent, 0o750)
}

func (w *WifiService) disableAutoReconnect() error {
	// Remove cron entry, under the same lock enableAutoReconnect takes: both
	// rewrite /etc/crontabs/root in place.
	crontabMu.Lock()
	defer crontabMu.Unlock()
	cronCmd := fmt.Sprintf(`(crontab -l 2>/dev/null | grep -v '%s') | crontab -`, w.reconnectScript)
	_, _ = w.cmd.Run("sh", "-c", cronCmd)

	// Remove script file
	_ = os.Remove(w.reconnectScript)
	return nil
}

const wifiSchedulePath = "/etc/travo/wifi-schedule.json"

// OpenWrt has no /etc/cron.d. Its busybox crond is built with
// `-c /etc/crontabs` and reads exactly one file per user, so a schedule has to
// live in /etc/crontabs/root alongside the stock entries and be tagged so it
// can be found and removed again. Same approach as the LED schedule
// (SystemService.SetLEDSchedule, ledCronTag).
//
// The previous implementation wrote /etc/cron.d/openwrt-gui-wifi-schedule, which
// is wrong twice over: the directory does not exist on OpenWrt, so the write
// failed with ENOENT and the endpoint answered 500 — while wifi-schedule.json
// had ALREADY been written, so the UI then reported a schedule that was enabled
// and would never fire. Even had the directory existed, crond would never have
// read it. Verified on the device: PUT /wifi/schedule returned 500
// "open /etc/cron.d/openwrt-gui-wifi-schedule: no such file or directory" and
// GET /wifi/schedule reported enabled=true with no cron entry anywhere.
const (
	wifiCrontabPath = "/etc/crontabs/root"
	wifiCronTag     = "# openwrt-travel-gui-wifi-schedule"
)

// crontabPath returns the crontab in use, honouring the test override.
func (w *WifiService) crontabPath() string {
	if w.crontabFile != "" {
		return w.crontabFile
	}
	return wifiCrontabPath
}

// wifiScheduleStatePath returns the persisted schedule JSON in use, honouring
// the test override.
func (w *WifiService) wifiScheduleStatePath() string {
	if w.scheduleFile != "" {
		return w.scheduleFile
	}
	return wifiSchedulePath
}

// GetWiFiSchedule returns the current cron-based WiFi on/off schedule.
func (w *WifiService) GetWiFiSchedule() (models.WiFiSchedule, error) {
	data, err := os.ReadFile(w.wifiScheduleStatePath())
	if err != nil {
		return models.WiFiSchedule{Enabled: false}, nil
	}
	var s models.WiFiSchedule
	if err := json.Unmarshal(data, &s); err != nil {
		return models.WiFiSchedule{Enabled: false}, nil
	}
	return s, nil
}

// crontabMu serialises read-modify-write of /etc/crontabs/root.
//
// Every writer of that one file takes it: the WiFi schedule, the LED schedule,
// AND the auto-reconnect cron entry. The auto-reconnect path rewrites the file
// through `crontab -l | grep -v ... | crontab -`, which is the same
// read-modify-write as the others — two concurrent rewrites each read the same
// base and each wrote their own version, so one silently dropped the other's
// entries. That is not hypothetical at boot: main.go reconciles auto-reconnect
// at 5s and the schedules at 6s.
//
// Keyed on the FILE, not on a UCI config: this is a plain crontab, with no
// uci delta involved.
var crontabMu sync.Mutex

// SetWiFiSchedule saves the WiFi schedule and updates the crontab.
func (w *WifiService) SetWiFiSchedule(schedule models.WiFiSchedule) error {
	// The times are formatted straight into a crontab line, so they must be
	// strict HH:MM: a newline would inject an extra attacker-controlled cron
	// entry that runs as root.
	//
	// Validate BEFORE persisting. Writing first left the rejected value in
	// wifi-schedule.json (so GetWiFiSchedule reported it back to the UI and the
	// startup reconcile goroutine re-read it) while the previously written cron
	// file kept toggling WiFi — a request that returned 400 had in fact changed
	// persisted state.
	if schedule.Enabled && schedule.OnTime != "" && schedule.OffTime != "" {
		if err := ValidateHHMM(schedule.OnTime); err != nil {
			return fmt.Errorf("on_time: %w", err)
		}
		if err := ValidateHHMM(schedule.OffTime); err != nil {
			return fmt.Errorf("off_time: %w", err)
		}
	}

	// The crontab is the SOURCE OF TRUTH for "is the schedule actually
	// scheduled", so write it FIRST and persist the JSON only once it is in
	// place. The other order is how a 500 left the UI showing
	// enabled=true with nothing scheduled: the JSON was written, then the
	// /etc/cron.d write failed with ENOENT.
	if schedule.Enabled && schedule.OnTime != "" && schedule.OffTime != "" {
		if err := w.writeWiFiScheduleCronLines(schedule); err != nil {
			return err
		}
	} else if err := w.removeWiFiScheduleCronLines(); err != nil {
		return err
	}

	data, err := json.Marshal(schedule)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(w.wifiScheduleStatePath()), 0o755); err != nil {
		return err
	}
	return os.WriteFile(w.wifiScheduleStatePath(), data, 0o644)
}

// wifiScheduleOffWindow reports whether the persisted WiFi schedule says WiFi
// must currently be OFF, i.e. now falls inside the arc from off_time to the next
// on_time — which wraps past midnight when off_time is later than on_time.
//
// Startup AP repair (ap_health.go) consults this: the generated toggle helper
// expresses "off" as wireless.@wifi-device[*].disabled=1 with the AP ifaces
// still disabled=0, so without this check a backend restart inside the off
// window would re-enable the radios and quietly cancel the operator's schedule.
//
// An unreadable, disabled, incomplete or malformed schedule reports false, i.e.
// the caller keeps its previous behaviour: a repair must not be switched off by
// a config file that cannot be read.
func (w *WifiService) wifiScheduleOffWindow() bool {
	schedule, err := w.GetWiFiSchedule()
	if err != nil || !schedule.Enabled || schedule.OnTime == "" || schedule.OffTime == "" {
		return false
	}
	return scheduleWindowIsOff(time.Now(), schedule.OnTime, schedule.OffTime)
}

// The off window is the arc from off_time to the NEXT on_time, so it is the
// same-day interval [off, on) when off is earlier, and wraps past midnight when
// it is later. Equal times express no off window at all, so they report false
// (leave the radios alone rather than guess).
func scheduleWindowIsOff(now time.Time, onTime, offTime string) bool {
	on, onErr := time.Parse("15:04", strings.TrimSpace(onTime))
	off, offErr := time.Parse("15:04", strings.TrimSpace(offTime))
	if onErr != nil || offErr != nil {
		return false
	}
	nowMin := now.Hour()*60 + now.Minute()
	onMin := on.Hour()*60 + on.Minute()
	offMin := off.Hour()*60 + off.Minute()
	if offMin < onMin {
		return nowMin >= offMin && nowMin < onMin
	}
	if offMin > onMin {
		return nowMin >= offMin || nowMin < onMin
	}
	return false
}

// writeWiFiScheduleCronLines replaces our tagged crontab lines with entries for
// the given schedule, preserving every line it does not own.
func (w *WifiService) writeWiFiScheduleCronLines(schedule models.WiFiSchedule) error {
	// cron format: MM HH * * * command  (busybox crond, no user field)
	onParts := strings.SplitN(schedule.OnTime, ":", 2)
	offParts := strings.SplitN(schedule.OffTime, ":", 2)

	// The cron entries must not run `wifi up` / `wifi down`; they call the
	// generated toggle helper, which writes UCI and applies via rpcd.
	if err := w.writeWirelessToggleScript(); err != nil {
		return err
	}

	crontabMu.Lock()
	defer crontabMu.Unlock()

	existing, _ := os.ReadFile(w.crontabPath())
	var lines []string
	for line := range strings.SplitSeq(string(existing), "\n") {
		if line == "" || strings.Contains(line, wifiCronTag) {
			continue
		}
		lines = append(lines, line)
	}
	// Cron must name the path the helper was actually installed at, not the
	// bare const, or the two could disagree.
	helper := w.toggleScriptPathOrDefault()
	lines = append(lines,
		fmt.Sprintf("%s %s * * * %s up %s", onParts[1], onParts[0], helper, wifiCronTag),
		fmt.Sprintf("%s %s * * * %s down %s", offParts[1], offParts[0], helper, wifiCronTag),
		"")
	if err := writeFileAtomic(w.crontabPath(), []byte(strings.Join(lines, "\n")), 0o600); err != nil {
		return fmt.Errorf("writing crontab: %w", err)
	}
	// cron re-reads the crontab on mtime change, but an explicit restart makes
	// the new entries live immediately instead of up to a minute later.
	_ = execx.Run(execx.Quick, "/etc/init.d/cron", "restart")
	return nil
}

// removeWiFiScheduleCronLines strips our tagged lines from the crontab,
// preserving everything else (the stock LED and auto-reconnect entries).
func (w *WifiService) removeWiFiScheduleCronLines() error {
	crontabMu.Lock()
	defer crontabMu.Unlock()

	existing, err := os.ReadFile(w.crontabPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var lines []string
	for line := range strings.SplitSeq(string(existing), "\n") {
		if line == "" || strings.Contains(line, wifiCronTag) {
			continue
		}
		lines = append(lines, line)
	}
	lines = append(lines, "")
	// Atomic, like writeWiFiScheduleCronLines: busybox crond re-reads
	// /etc/crontabs/root while it runs, and a truncating os.WriteFile can hand it
	// a half-written file — which it then parses as the real crontab, dropping
	// the auto-reconnect and LED entries it did not manage to read.
	if err := writeFileAtomic(w.crontabPath(), []byte(strings.Join(lines, "\n")), 0o600); err != nil {
		return fmt.Errorf("writing crontab: %w", err)
	}
	_ = execx.Run(execx.Quick, "/etc/init.d/cron", "restart")
	return nil
}
