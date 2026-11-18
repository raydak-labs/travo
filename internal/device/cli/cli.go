package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/runner"
)

// Timeout tiers: every shell-out gets one.
const (
	TierRead  = 5 * time.Second
	TierWrite = 15 * time.Second
	TierApply = 60 * time.Second
)

// Device is the real-router adapter.
type Device struct {
	Run runner.Runner
	// RunInput carries stdin (passwd); ReadFile reads /etc/shadow. Nil ReadFile means os.ReadFile.
	RunInput runner.InputRunner
	ReadFile func(string) ([]byte, error)
	// AuthorizedKeysPath overrides /etc/dropbear/authorized_keys (tests).
	AuthorizedKeysPath string
	// SysNet overrides /sys/class/net (tests).
	SysNet string
}

func New() *Device { return &Device{Run: runner.Exec, RunInput: runner.RunInput} }

func hasDiagnostic(err error, diagnostic string) bool {
	stderr, ok := runner.Stderr(err)
	return ok && stderr == diagnostic
}

func (d *Device) run(ctx context.Context, tier time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, tier)
	defer cancel()
	out, code, err := d.Run(ctx, name, args...)
	if err != nil && name == "uci" && code == 1 {
		// Exec includes stderr in err. Quiet commands suppress the not-found diagnostic,
		// so their historical exit-1 mapping is the only evidence available to callers.
		quiet := len(args) > 0 && args[0] == "-q"
		if quiet || hasDiagnostic(err, "uci: Entry not found") {
			return out, fmt.Errorf("%w: %v", device.ErrNotFound, err)
		}
	}
	return out, err
}

// Show reads via `uci export`: `uci show` cannot tell a one-element list from an option.
// Staged (uncommitted) changes are included, like `uci show`.
func (d *Device) Show(ctx context.Context, config string) ([]device.Section, error) {
	out, err := d.run(ctx, TierRead, "uci", "export", config)
	if err != nil {
		return nil, err
	}
	return ParseExport(config, out)
}

func (d *Device) Set(ctx context.Context, config, section, option, value string) error {
	// the section must exist: uci would silently create nothing useful otherwise
	if _, err := d.run(ctx, TierRead, "uci", "-q", "get", config+"."+section); err != nil {
		return fmt.Errorf("%s.%s: %w", config, section, device.ErrNotFound)
	}
	_, err := d.run(ctx, TierWrite, "uci", "set", fmt.Sprintf("%s.%s.%s=%s", config, section, option, value))
	return err
}

func (d *Device) SetType(ctx context.Context, config, section, typ string) error {
	_, err := d.run(ctx, TierWrite, "uci", "set", fmt.Sprintf("%s.%s=%s", config, section, typ))
	return err
}

func (d *Device) AddToList(ctx context.Context, config, section, option, value string) error {
	_, err := d.run(ctx, TierWrite, "uci", "add_list", fmt.Sprintf("%s.%s.%s=%s", config, section, option, value))
	return err
}

func (d *Device) DeleteOption(ctx context.Context, config, section, option string) error {
	_, err := d.run(ctx, TierWrite, "uci", "-q", "delete", fmt.Sprintf("%s.%s.%s", config, section, option))
	return err
}

func (d *Device) DeleteSection(ctx context.Context, config, section string) error {
	_, err := d.run(ctx, TierWrite, "uci", "delete", fmt.Sprintf("%s.%s", config, section))
	return err
}

func (d *Device) Commit(ctx context.Context, config string) error {
	_, err := d.run(ctx, TierWrite, "uci", "commit", config)
	return err
}

func (d *Device) Revert(ctx context.Context, config string) error {
	_, err := d.run(ctx, TierWrite, "uci", "revert", config)
	return err
}

func (d *Device) Pending(ctx context.Context, config string) (bool, error) {
	out, err := d.run(ctx, TierRead, "uci", "changes", config)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func (d *Device) Call(ctx context.Context, object, method string, args any) (json.RawMessage, error) {
	payload := []byte("{}")
	if args != nil {
		var err error
		if payload, err = json.Marshal(args); err != nil {
			return nil, err
		}
	}
	out, err := d.run(ctx, TierRead, "ubus", "call", object, method, string(payload))
	if err != nil {
		if hasDiagnostic(err, "Command failed: Not found") {
			return nil, fmt.Errorf("%w: %v", device.ErrNotFound, err)
		}
		return nil, err
	}
	if !json.Valid([]byte(out)) {
		return nil, fmt.Errorf("ubus %s.%s: reply is not JSON", object, method)
	}
	return json.RawMessage(out), nil
}

// Reload runs procd's reload_config; it restarts only services whose config changed.
func (d *Device) Reload(ctx context.Context) error {
	_, err := d.run(ctx, TierApply, "reload_config")
	return err
}

// HasDefaultRoute reads the kernel table (netifd status can lie, see device-facts §1).
func (d *Device) HasDefaultRoute(ctx context.Context) (bool, error) {
	out, err := d.run(ctx, TierRead, "ip", "-4", "route", "show", "default")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

var _ device.Device = (*Device)(nil)
