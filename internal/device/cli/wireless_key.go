package cli

import (
	"context"
	"errors"
	"regexp"
	"strings"
)

var wirelessSection = regexp.MustCompile(`^(?:[A-Za-z0-9_]+|@wifi-iface\[-?[0-9]+\])$`)
var wirelessPassphrase = regexp.MustCompile(`^[ -~]{8,63}$`)
var errStageWirelessKey = errors.New("uci: failed to stage wireless key")

func (d *Device) SetWirelessKey(ctx context.Context, section, key string) error {
	if d.RunInput == nil || !wirelessSection.MatchString(section) || !wirelessPassphrase.MatchString(key) {
		return errStageWirelessKey
	}

	ctx, cancel := context.WithTimeout(ctx, TierWrite)
	defer cancel()
	option := "wireless." + section + ".key"
	quoted := "'" + strings.ReplaceAll(key, "'", "'\\''") + "'"
	// Batch can report a failed command with exit status zero. Verify the
	// staged value and keep both its stdout and diagnostics out of errors.
	out, code, err := d.RunInput(ctx, "set "+option+"="+quoted+"\nget "+option+"\n", "uci", "batch")
	if err != nil || code != 0 || out != key+"\n" {
		return errStageWirelessKey
	}
	return nil
}
