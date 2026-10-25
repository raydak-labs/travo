package system

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

// buttonNameRe matches button labels safe to interpolate into a shell script.
var buttonNameRe = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// sshKeyRe matches a single-line OpenSSH public key ("<type> <base64> [comment]").
// Key types are limited to the algorithms dropbear accepts.
var sshKeyRe = regexp.MustCompile(`^(?:ssh-rsa|ssh-ed25519|ecdsa-sha2-nistp(?:256|384|521)|sk-ssh-ed25519@openssh\.com|sk-ecdsa-sha2-nistp256@openssh\.com) [A-Za-z0-9+/]+={0,3}(?: [^\r\n]*)?$`)

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
