package device

import (
	"context"
	"net/netip"
	"strings"
)

// Diagnostic tools the router runs for the operator (internal/diagnostics).
const (
	DiagPing       = "ping"
	DiagTraceroute = "traceroute"
	DiagLookup     = "lookup"
)

// DiagnosticRun is one finished probe: stdout then stderr, OK when the tool exited 0,
// TimedOut when its timeout tier killed it.
type DiagnosticRun struct {
	Output   string
	OK       bool
	TimedOut bool
}

// Diagnoser runs one read-only network probe from the router. The target goes to the tool
// as one argv element, never through a shell; adapters refuse a target that fails
// ValidDiagnosticTarget before running anything.
type Diagnoser interface {
	Diagnose(ctx context.Context, tool, target string) (DiagnosticRun, error)
}

// ValidDiagnosticTarget accepts an IP literal without zone, or an RFC 1123 host name of at
// most 253 characters whose labels are 1 to 63 letters, digits and inner hyphens and whose
// last label is not all digits. No leading '-' can reach a tool as an option.
func ValidDiagnosticTarget(s string) bool {
	if s == "" || len(s) > 253 {
		return false
	}
	if ip, err := netip.ParseAddr(s); err == nil {
		return ip.Zone() == ""
	}
	labels := strings.Split(s, ".")
	for _, l := range labels {
		if !validLabel(l) {
			return false
		}
	}
	return strings.Trim(labels[len(labels)-1], "0123456789") != ""
}

func validLabel(l string) bool {
	if l == "" || len(l) > 63 || l[0] == '-' || l[len(l)-1] == '-' {
		return false
	}
	for _, c := range l {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
			return false
		}
	}
	return true
}
