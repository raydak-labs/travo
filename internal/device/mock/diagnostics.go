package mock

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

var (
	//go:embed testdata/diag-ping-ok.txt
	diagPingOK string
	//go:embed testdata/diag-ping-bad-host.txt
	diagPingBadHost string
	//go:embed testdata/diag-traceroute-ok.txt
	diagTracerouteOK string
	//go:embed testdata/diag-lookup-ok.txt
	diagLookupOK string
	//go:embed testdata/diag-lookup-nxdomain.txt
	diagLookupNXDomain string
)

// Diagnose answers with the verbatim AXT1800 captures: success for any valid target, the
// ping and lookup failure captures for a `.invalid` name. The mock never sends a packet.
func (d *Device) Diagnose(ctx context.Context, tool, target string) (device.DiagnosticRun, error) {
	if err := ctx.Err(); err != nil {
		return device.DiagnosticRun{}, err
	}
	if !device.ValidDiagnosticTarget(target) {
		return device.DiagnosticRun{}, fmt.Errorf("diagnostic target %q", target)
	}
	failed := strings.HasSuffix(target, ".invalid")
	var capture string
	switch {
	case tool == device.DiagPing && failed:
		capture = diagPingBadHost
	case tool == device.DiagPing:
		capture = diagPingOK
	case tool == device.DiagTraceroute: // only a success was captured
		capture = diagTracerouteOK
	case tool == device.DiagLookup && failed:
		capture = diagLookupNXDomain
	case tool == device.DiagLookup:
		capture = diagLookupOK
	default:
		return device.DiagnosticRun{}, fmt.Errorf("diagnostic tool %q", tool)
	}
	out, exit, _ := strings.Cut(capture, "exit=")
	return device.DiagnosticRun{Output: out, OK: strings.TrimSpace(exit) == "0"}, nil
}

var _ device.Diagnoser = (*Device)(nil)
