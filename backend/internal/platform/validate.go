package platform

import (
	"fmt"
	"regexp"
)

// hhmmRe matches a strict 24-hour "HH:MM" clock time.
var hhmmRe = regexp.MustCompile(`^([01]\d|2[0-3]):([0-5]\d)$`)

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
