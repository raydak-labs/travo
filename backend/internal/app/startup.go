package app

import (
	"log/slog"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/wifi"
)

// startWifiReconciliation schedules the startup repairs that keep the wireless
// configuration and its cron entries on the safe versions of themselves.
//
// Every task here is idempotent and runs on its own delay, so a slow boot does
// not serialise them. The wireless configuration is only ever WRITTEN here: the
// runtime apply stays with the operator (ADR 0002), so a repair that needs a
// reboot to take effect says so in the log.
func startWifiReconciliation(lifecycle *Lifecycle, wifiSvc *wifi.Service) {
	// Fix wireless UCI on startup (country/channel, missing SSID/key on existing APs,
	// enable radios when AP enabled). Do not auto-apply on startup: there is no
	// browser in the loop to confirm rpcd rollback safely, so we commit the repair
	// and require LuCI Save & Apply or reboot for runtime activation.
	lifecycle.Go(func(stop <-chan struct{}) {
		if !sleepOrStop(stop, 30*time.Second) {
			return
		}
		fixed, needApply, err := wifiSvc.EnsureAPRunning()
		if err != nil {
			slog.Warn("wifi: AP health check failed", "err", err)
			return
		}
		if fixed && needApply {
			slog.Info("wifi: AP health fixes committed; runtime apply skipped at startup, use LuCI Save & Apply or reboot")
		} else if fixed {
			slog.Info("wifi: AP health fixes committed (SSID/key only, no apply needed)")
		}
	})
	// Ensure auto-reconnect script is present and up-to-date when enabled.
	// Uses SetAutoReconnect to recreate a missing script (e.g. after a crash or
	// accidental deletion) and to upgrade any old "wifi reload" script to the safe
	// "wifi up" version. Safe to call idempotently: it rewrites the cron entry and
	// script atomically, which is the same state SetAutoReconnect(true) produces.
	lifecycle.Go(func(stop <-chan struct{}) {
		if !sleepOrStop(stop, 5*time.Second) {
			return
		}
		// Repair in BOTH directions, not just when the feature is on.
		// A device carrying a pre-rename cron entry or script keeps running
		// `wifi reload` / `wifi up` on a timer even though auto-reconnect is
		// disabled, and `wifi reload` is the documented ath11k crash trigger.
		// SetAutoReconnect rewrites the script and the cron entry to match the
		// configured state, so calling it unconditionally converges the device
		// onto the safe script in both cases.
		enabled, _ := wifiSvc.GetAutoReconnect()
		if err := wifiSvc.SetAutoReconnect(enabled); err != nil {
			slog.Warn("wifi: could not reconcile the auto-reconnect script", "err", err)
		}
	})
	// Same for the WiFi on/off schedule: an existing /etc/cron.d entry still
	// holds the pre-fix `/sbin/wifi up` / `/sbin/wifi down` lines until the
	// user re-saves the schedule. Re-writing it from the stored config points
	// the cron entries at the generated UCI toggle helper instead.
	lifecycle.Go(func(stop <-chan struct{}) {
		if !sleepOrStop(stop, 6*time.Second) {
			return
		}
		sched, err := wifiSvc.GetWiFiSchedule()
		if err != nil || !sched.Enabled || sched.OnTime == "" || sched.OffTime == "" {
			return
		}
		if err := wifiSvc.SetWiFiSchedule(sched); err != nil {
			slog.Warn("wifi: could not reconcile the schedule cron entry", "err", err)
		}
	})
	// Auto-discover radio hardware and persist config on first boot.
	lifecycle.Go(func(stop <-chan struct{}) {
		if !sleepOrStop(stop, 10*time.Second) {
			return
		}
		if discovered, err := wifiSvc.DiscoverAndPersistRadios(); err != nil {
			slog.Warn("radio discovery failed", "err", err)
		} else if discovered {
			slog.Info("radio discovery completed on first boot")
		}
	})
}
