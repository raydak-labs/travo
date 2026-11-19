package cli

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

var wanPassword = regexp.MustCompile(`^[ -~]{1,128}$`)
var errStageWANPassword = errors.New("uci: failed to stage wan password")

// SetWANPassword stages network.wan.password through `uci batch` on stdin, like WiFi keys:
// the secret stays out of argv, and neither its value nor uci's output reaches an error.
func (d *Device) SetWANPassword(ctx context.Context, password string) error {
	if d.RunInput == nil || !wanPassword.MatchString(password) {
		return errStageWANPassword
	}
	ctx, cancel := context.WithTimeout(ctx, TierWrite)
	defer cancel()
	const option = "network.wan.password"
	quoted := "'" + strings.ReplaceAll(password, "'", "'\\''") + "'"
	out, code, err := d.RunInput(ctx, "set "+option+"="+quoted+"\nget "+option+"\n", "uci", "batch")
	if err != nil || code != 0 || out != password+"\n" {
		return errStageWANPassword
	}
	return nil
}

var _ device.WANPasswordWriter = (*Device)(nil)
