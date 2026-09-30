package services

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

// hhmmRe matches a strict 24-hour "HH:MM" clock time.
var hhmmRe = regexp.MustCompile(`^([01]\d|2[0-3]):([0-5]\d)$`)

// buttonNameRe matches button labels safe to interpolate into a shell script.
var buttonNameRe = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// sshKeyRe matches a single-line OpenSSH public key ("<type> <base64> [comment]").
// Key types are limited to the algorithms dropbear accepts.
var sshKeyRe = regexp.MustCompile(`^(?:ssh-rsa|ssh-ed25519|ecdsa-sha2-nistp(?:256|384|521)|sk-ssh-ed25519@openssh\.com|sk-ecdsa-sha2-nistp256@openssh\.com) [A-Za-z0-9+/]+={0,3}(?: [^\r\n]*)?$`)

// ValidateHHMM reports whether s is a strict 24-hour "HH:MM" time.
//
// These values are formatted into crontab lines, so anything containing a
// newline, carriage return, whitespace or shell metacharacter must be
// rejected. Both the HTTP handler boundary and the service layer call this.
func ValidateHHMM(s string) error {
	if s == "" {
		return fmt.Errorf("time is required")
	}
	// hhmmRe is a full-string match, so it is strictly tighter than
	// time.Parse("15:04", ...) — a second parse here could never reject anything
	// the regex already accepted.
	if !hhmmRe.MatchString(s) {
		return fmt.Errorf("invalid time %q: expected HH:MM (00:00-23:59)", s)
	}
	return nil
}

// ValidateButtonName reports whether name is safe to write into the root
// hotplug shell script as a case label.
//
// Only labels actually discovered from /sys/firmware/devicetree/base/keys/*/label
// (see detectButtonNames) may be configured, and they must additionally match
// [A-Za-z0-9_-] so a crafted name cannot inject shell or a new case arm.
func ValidateButtonName(name string, discovered []string) error {
	if name == "" {
		return fmt.Errorf("button name is required")
	}
	if !buttonNameRe.MatchString(name) {
		return fmt.Errorf("invalid button name %q: only letters, digits, '-' and '_' are allowed", name)
	}
	if len(discovered) == 0 {
		// No button labels could be read from devicetree; the charset check
		// above is the only protection available.
		return nil
	}
	for _, d := range discovered {
		if d == name {
			return nil
		}
	}
	return fmt.Errorf("unknown button %q: not one of the detected buttons (%s)",
		name, strings.Join(discovered, ", "))
}

// ValidateAlertThresholds checks the alert thresholds are usable percentages.
//
// There was no validation at all here: PUT /system/alert-thresholds accepted
// storage_percent=500 and answered 200, which persists a threshold no usage
// level can ever reach — so the alert silently never fires and the UI shows a
// saved configuration that does nothing. A percentage is the whole contract
// here, so anything outside 0-100 is a mistake worth reporting.
func ValidateAlertThresholds(t models.AlertThresholds) error {
	for _, f := range []struct {
		name  string
		value float64
	}{
		{"storage_percent", t.StoragePercent},
		{"cpu_percent", t.CPUPercent},
		{"memory_percent", t.MemoryPercent},
	} {
		if math.IsNaN(f.value) || math.IsInf(f.value, 0) {
			return fmt.Errorf("%s must be a number", f.name)
		}
		if f.value < 0 || f.value > 100 {
			return fmt.Errorf("%s must be between 0 and 100, got %v", f.name, f.value)
		}
	}
	return nil
}

// ValidateBandSwitchConfig checks the automatic band switcher's parameters.
//
// Nothing was validated here either: preferred_band accepted "9g" (no such
// band, so the switcher would never match and quietly do nothing) and the
// signal thresholds accepted any integer.
//
// The thresholds are dBm, which is negative; the bounds below are the useful
// range for a STA association rather than an arbitrary sanity check, and they
// also keep the hysteresis meaningful: up_switch must be less than
// down_switch, otherwise the switcher oscillates between states every tick.
func ValidateBandSwitchConfig(c BandSwitchConfig) error {
	switch c.PreferredBand {
	case "", "2g", "5g":
	default:
		return fmt.Errorf("preferred_band must be \"2g\" or \"5g\", got %q", c.PreferredBand)
	}
	if c.CheckIntervalSec < 1 || c.CheckIntervalSec > 3600 {
		return fmt.Errorf("check_interval_sec must be between 1 and 3600, got %d", c.CheckIntervalSec)
	}
	for _, f := range []struct {
		name  string
		value int
	}{
		{"down_switch_threshold_dbm", c.DownSwitchThresholdDBm},
		{"up_switch_threshold_dbm", c.UpSwitchThresholdDBm},
		{"min_viable_signal_dbm", c.MinViableSignalDBm},
	} {
		// dBm runs from about -127 (noise floor) to 0 (theoretical max).
		if f.value < -127 || f.value > 0 {
			return fmt.Errorf("%s must be a dBm value between -127 and 0, got %d", f.name, f.value)
		}
	}
	for _, f := range []struct {
		name  string
		value int
	}{
		{"down_switch_delay_sec", c.DownSwitchDelaySec},
		{"up_switch_delay_sec", c.UpSwitchDelaySec},
	} {
		if f.value < 0 || f.value > 86400 {
			return fmt.Errorf("%s must be between 0 and 86400 seconds, got %d", f.name, f.value)
		}
	}
	// dBm is negative and a STRONGER signal is a LARGER number. Coming back up
	// must therefore require a better signal than going down did, or the two
	// thresholds coincide and the switcher oscillates on every check. The
	// defaults are -70 to drop and -60 to return.
	if c.UpSwitchThresholdDBm != 0 && c.DownSwitchThresholdDBm != 0 &&
		c.UpSwitchThresholdDBm <= c.DownSwitchThresholdDBm {
		return fmt.Errorf("up_switch_threshold_dbm (%d) must be greater than down_switch_threshold_dbm (%d) "+
			"(a stronger signal, i.e. less negative), otherwise the switcher oscillates on every check",
			c.UpSwitchThresholdDBm, c.DownSwitchThresholdDBm)
	}
	return nil
}
