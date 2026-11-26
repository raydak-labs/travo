package auth

import (
	"errors"
	"strings"
)

const (
	MinPasswordBytes = 8
	MaxPasswordBytes = 128
)

// ValidatePassword is the policy for NEW passwords. Login does not apply it: an older
// password set elsewhere (LuCI, ssh) must still work. Errors never contain the password.
func ValidatePassword(p string) error {
	switch {
	case len(p) < MinPasswordBytes:
		return errors.New("password must be at least 8 bytes")
	case len(p) > MaxPasswordBytes:
		return errors.New("password must be at most 128 bytes")
	case strings.ContainsAny(p, "\x00\n\r"): // passwd reads lines from stdin
		return errors.New("password must not contain NUL or line breaks")
	}
	return nil
}
