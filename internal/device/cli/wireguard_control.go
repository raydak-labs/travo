package cli

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"regexp"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

var wireGuardPeerSection = regexp.MustCompile(`^wg0_peer[0-9]+$`)
var logicalInterface = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_-]*$`)

var errStageWireGuardKey = errors.New("uci: failed to stage WireGuard key")

func (d *Device) SetWireGuardKey(ctx context.Context, section, option, key string) error {
	raw, err := base64.StdEncoding.Strict().DecodeString(key)
	validTarget := section == "wg0" && option == "private_key" ||
		wireGuardPeerSection.MatchString(section) && option == "preshared_key"
	if err != nil || len(raw) != 32 || len(key) != 44 || !validTarget || d.RunInput == nil {
		return errStageWireGuardKey
	}

	ctx, cancel := context.WithTimeout(ctx, TierWrite)
	defer cancel()
	path := "network." + section + "." + option
	out, code, err := d.RunInput(ctx, "set "+path+"='"+key+"'\nget "+path+"\n", "uci", "batch")
	if err != nil || code != 0 || out != key+"\n" {
		return errStageWireGuardKey
	}
	return nil
}

func (d *Device) ReloadNetwork(ctx context.Context) error {
	_, err := d.run(ctx, 4*time.Second, "ubus", "-t", "3", "call", "network", "reload")
	return err
}

func (d *Device) SetInterfaceUp(ctx context.Context, logical string, up bool) error {
	if !logicalInterface.MatchString(logical) {
		return errors.New("invalid logical interface")
	}
	method := "down"
	if up {
		method = "up"
	}
	_, err := d.run(ctx, 4*time.Second, "ubus", "-t", "3", "call", "network.interface."+logical, method)
	return err
}

func (d *Device) WireGuardLinkPresent(ctx context.Context) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	_, err := os.Stat("/sys/class/net/wg0")
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

var _ device.WireGuardControl = (*Device)(nil)
var _ device.WireGuardKeyWriter = (*Device)(nil)
