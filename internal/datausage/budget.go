// Package datausage derives data usage per uplink, the monthly budget state and the uptime
// log from the stats history store (docs/adr/0026). It reads; the store samples. The only
// file it writes is the budget, and only on an explicit PUT or DELETE.
package datausage

import (
	"errors"
	"fmt"
	"slices"

	"github.com/blackdark-org/travo-greenfield/internal/statshistory"
)

// Limits of the budget settings.
const (
	MaxStartDay    = 28 // every month has it, so a period never skips a month
	MinLimitBytes  = 1 << 20
	MaxLimitBytes  = 1 << 50
	defaultWarnPct = 80
)

var (
	// ErrUnreadable is a budget file that is torn or invalid: it is never overwritten until
	// the explicit reset (invariants §5).
	ErrUnreadable = errors.New("data budget file unreadable")
	// ErrUnavailable is a failed write or a missing history.
	ErrUnavailable = errors.New("data usage unavailable")
)

// FieldError is a rejected budget field (400).
type FieldError struct{ Field, Reason string }

func (e *FieldError) Error() string { return e.Field + ": " + e.Reason }

// Budget is the monthly data allowance. It only warns: nothing is cut off (no ADR for that).
type Budget struct {
	Enabled     bool     `json:"enabled"`
	LimitBytes  uint64   `json:"limit_bytes" maximum:"1125899906842624" doc:"Allowance per billing period; at least 1 MiB while enabled"`
	WarnPercent int      `json:"warn_percent" minimum:"1" maximum:"100" doc:"Warn once this share of the limit is used"`
	StartDay    int      `json:"start_day" minimum:"1" maximum:"28" doc:"Day of the month (UTC) a billing period starts"`
	Uplinks     []string `json:"uplinks" nullable:"false" minItems:"1" maxItems:"3" enum:"wan,wwan,usbtether" doc:"Uplinks whose traffic counts against the limit"`
}

// DefaultBudget is the absent file: off, periods start on the 1st, every uplink counts.
func DefaultBudget() Budget {
	return Budget{WarnPercent: defaultWarnPct, StartDay: 1, Uplinks: slices.Clone(statshistory.Uplinks)}
}

// Validate checks every field.
func (b Budget) Validate() error {
	switch {
	case b.StartDay < 1 || b.StartDay > MaxStartDay:
		return &FieldError{"start_day", fmt.Sprintf("1 to %d", MaxStartDay)}
	case b.WarnPercent < 1 || b.WarnPercent > 100:
		return &FieldError{"warn_percent", "1 to 100"}
	case b.Enabled && (b.LimitBytes < MinLimitBytes || b.LimitBytes > MaxLimitBytes):
		return &FieldError{"limit_bytes", "1 MiB to 1 PiB while enabled"}
	case len(b.Uplinks) == 0:
		return &FieldError{"uplinks", "at least one uplink"}
	}
	seen := map[string]bool{}
	for _, u := range b.Uplinks {
		if !slices.Contains(statshistory.Uplinks, u) || seen[u] {
			return &FieldError{"uplinks", fmt.Sprintf("%q is unknown or listed twice", u)}
		}
		seen[u] = true
	}
	return nil
}

// State is where the period's usage stands against the budget.
type State string

const (
	StateOff        State = "off"
	StateOK         State = "ok"
	StateWarning    State = "warning"
	StateExceeded   State = "exceeded"
	StateUnreadable State = "unreadable"
)

// Evaluate compares the used bytes with the budget. Pure.
func Evaluate(b Budget, used uint64) State {
	switch {
	case !b.Enabled:
		return StateOff
	case used >= b.LimitBytes:
		return StateExceeded
	case used*100 >= b.LimitBytes*uint64(b.WarnPercent):
		return StateWarning
	}
	return StateOK
}
