package services

import (
	"fmt"
	"regexp"
	"strings"
	"time"
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
	if !hhmmRe.MatchString(s) {
		return fmt.Errorf("invalid time %q: expected HH:MM (00:00-23:59)", s)
	}
	if _, err := time.Parse("15:04", s); err != nil {
		return fmt.Errorf("invalid time %q: %w", s, err)
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
