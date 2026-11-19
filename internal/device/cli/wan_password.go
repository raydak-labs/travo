package cli

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

var printableSecret = regexp.MustCompile(`^[ -~]{1,128}$`)

var (
	errStageWANPassword  = errors.New("uci: failed to stage wan password")
	errStageDDNSPassword = errors.New("uci: failed to stage ddns password")
)

// SetWANPassword stages network.wan.password through `uci batch` on stdin, like WiFi keys:
// the secret stays out of argv, and neither its value nor uci's output reaches an error.
func (d *Device) SetWANPassword(ctx context.Context, password string) error {
	return d.stageSecret(ctx, "network.wan.password", password, errStageWANPassword)
}

// SetDDNSPassword stages ddns.travo.password the same way (#245).
func (d *Device) SetDDNSPassword(ctx context.Context, password string) error {
	return d.stageSecret(ctx, "ddns.travo.password", password, errStageDDNSPassword)
}

func (d *Device) stageSecret(ctx context.Context, option, value string, failed error) error {
	if d.RunInput == nil || !printableSecret.MatchString(value) {
		return failed
	}
	ctx, cancel := context.WithTimeout(ctx, TierWrite)
	defer cancel()
	quoted := "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
	out, code, err := d.RunInput(ctx, "set "+option+"="+quoted+"\nget "+option+"\n", "uci", "batch")
	if err != nil || code != 0 || out != value+"\n" {
		return failed
	}
	return nil
}

var (
	_ device.WANPasswordWriter  = (*Device)(nil)
	_ device.DDNSPasswordWriter = (*Device)(nil)
)
