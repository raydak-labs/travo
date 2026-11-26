// Package diagnostics runs one ping, traceroute or DNS lookup from the router at a time.
// It is read-only for the router: no UCI, no guard. Targets are validated host names or
// IP literals and reach the tool as one argv element (device.Diagnoser).
package diagnostics

import (
	"context"
	"errors"
	"slices"
	"sync"
	"unicode/utf8"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// MaxOutput caps the output returned to the caller.
const MaxOutput = 16 << 10

// Tools are the accepted tool names.
var Tools = []string{device.DiagPing, device.DiagTraceroute, device.DiagLookup}

var (
	ErrInvalid     = errors.New("target must be a host name or IP address")
	ErrBusy        = errors.New("another diagnostic is running")
	ErrUnavailable = errors.New("diagnostics are unavailable")
)

// Diagnosis is one finished run. OK is the tool's exit status; Output is what it printed.
type Diagnosis struct {
	Tool      string `json:"tool" enum:"ping,traceroute,lookup"`
	Target    string `json:"target"`
	OK        bool   `json:"ok" doc:"The tool exited successfully (all replies, a route, an answer)"`
	TimedOut  bool   `json:"timed_out" doc:"The run hit its time limit; the output is partial"`
	Truncated bool   `json:"truncated" doc:"Output was cut at 16 KiB"`
	Output    string `json:"output"`
}

// Service serialises runs: one probe at a time across all callers.
type Service struct {
	Device device.Diagnoser
	mu     sync.Mutex
}

// Run validates and runs one probe; a second concurrent call gets ErrBusy.
func (s *Service) Run(ctx context.Context, tool, target string) (Diagnosis, error) {
	if !slices.Contains(Tools, tool) || !device.ValidDiagnosticTarget(target) {
		return Diagnosis{}, ErrInvalid
	}
	if !s.mu.TryLock() {
		return Diagnosis{}, ErrBusy
	}
	defer s.mu.Unlock()
	run, err := s.Device.Diagnose(ctx, tool, target)
	if err != nil {
		return Diagnosis{}, errors.Join(ErrUnavailable, err)
	}
	out, truncated := capOutput(run.Output)
	return Diagnosis{Tool: tool, Target: target, OK: run.OK, TimedOut: run.TimedOut, Truncated: truncated, Output: out}, nil
}

// capOutput keeps at most MaxOutput bytes without splitting a UTF-8 sequence.
func capOutput(s string) (string, bool) {
	if len(s) <= MaxOutput {
		return s, false
	}
	cut := MaxOutput
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut], true
}
