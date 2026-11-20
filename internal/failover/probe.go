package failover

import (
	"errors"
	"fmt"
	"log/slog"
)

// ProbeCheck names the apply-probe check that failed (#277): the transaction answer carries
// it so the UI can say why failover did not verify, and the WARN log names it.
type ProbeCheck string

const (
	// CheckConfig: a managed mwan3 section does not read back as planned.
	CheckConfig ProbeCheck = "config"
	// CheckService: mwan3 is not running.
	CheckService ProbeCheck = "service"
	// CheckTracking: an uplink is not "online and tracking is active" in `mwan3 interfaces`.
	CheckTracking ProbeCheck = "tracking"
	// CheckHook: mwan3_hook is not on PREROUTING and OUTPUT (or mangle is unreadable).
	CheckHook ProbeCheck = "hook"
	// CheckFirstRule: the first mwan3_rules jump does not go to the Travo policy.
	CheckFirstRule ProbeCheck = "first_rule"
	// CheckPolicyMark: the Travo policy does not mark for the first uplink.
	CheckPolicyMark ProbeCheck = "policy_mark"
	// CheckOutbound: no internet through the policy.
	CheckOutbound ProbeCheck = "outbound"
)

// ProbeError is the last failed check of the apply probe.
type ProbeError struct {
	Check ProbeCheck
	Err   error
}

func (e *ProbeError) Error() string { return fmt.Sprintf("failover probe %s: %v", e.Check, e.Err) }
func (e *ProbeError) Unwrap() error { return e.Err }

func probeErr(check ProbeCheck, format string, args ...any) error {
	return &ProbeError{Check: check, Err: fmt.Errorf(format, args...)}
}

// logProbeFailure names the last failed check before the transaction rolls back (#277).
func logProbeFailure(last error) {
	var pe *ProbeError
	if errors.As(last, &pe) {
		slog.Warn("failover: apply probe failed; rolling back", "check", pe.Check, "err", pe.Err)
		return
	}
	slog.Warn("failover: apply probe failed; rolling back", "err", last)
}
