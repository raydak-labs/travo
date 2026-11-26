package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/runner"
)

// Timeout tiers per tool: ping sends 4 probes with a 2 s wait; traceroute is at most
// 20 hops x 1 probe x 2 s; nslookup asks the local dnsmasq once.
const (
	TierPing       = 15 * time.Second
	TierTraceroute = 45 * time.Second
	TierLookup     = 10 * time.Second
)

// diagnosticCommand is the argv for one probe (BusyBox tools on 25.12.6, testdata
// diagnostics/). lookup asks 127.0.0.1, i.e. dnsmasq and the DNS layer stack, the
// resolver LAN clients use.
func diagnosticCommand(tool, target string) (time.Duration, string, []string, error) {
	switch tool {
	case device.DiagPing:
		return TierPing, "ping", []string{"-c", "4", "-W", "2", target}, nil
	case device.DiagTraceroute:
		return TierTraceroute, "traceroute", []string{"-n", "-w", "2", "-q", "1", "-m", "20", target}, nil
	case device.DiagLookup:
		return TierLookup, "nslookup", []string{target, "127.0.0.1"}, nil
	}
	return 0, "", nil, fmt.Errorf("diagnostic tool %q", tool)
}

// Diagnose runs one probe. A non-zero exit is a finished run (OK=false), not an error;
// only a tool that could not start is an error.
func (d *Device) Diagnose(ctx context.Context, tool, target string) (device.DiagnosticRun, error) {
	if !device.ValidDiagnosticTarget(target) {
		return device.DiagnosticRun{}, fmt.Errorf("diagnostic target %q", target)
	}
	tier, name, args, err := diagnosticCommand(tool, target)
	if err != nil {
		return device.DiagnosticRun{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, tier)
	defer cancel()
	out, _, err := d.Run(ctx, name, args...)
	run := device.DiagnosticRun{Output: out, OK: err == nil}
	if err == nil {
		return run, nil
	}
	var ce *runner.CommandError
	if !errors.As(err, &ce) {
		return device.DiagnosticRun{}, err
	}
	if ce.Stderr != "" {
		run.Output = strings.TrimRight(out, "\n") + "\n" + ce.Stderr
		run.Output = strings.TrimLeft(run.Output, "\n")
	}
	run.TimedOut = errors.Is(ctx.Err(), context.DeadlineExceeded)
	return run, nil
}

var _ device.Diagnoser = (*Device)(nil)
