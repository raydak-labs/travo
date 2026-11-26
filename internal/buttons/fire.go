package buttons

import (
	"context"
	"errors"
	"log/slog"
	"net/netip"

	"github.com/blackdark-org/travo-greenfield/internal/lockout"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/wifi"
)

// Outcome of one slide; stable codes for the UI and the hook log line.
type Outcome string

const (
	Applied        Outcome = "applied"
	Unchanged      Outcome = "unchanged"
	NoAction       Outcome = "no_action"
	RolledBack     Outcome = "rolled_back"
	SkippedGuard   Outcome = "skipped_guard"
	SkippedBusy    Outcome = "skipped_busy"
	SkippedLockout Outcome = "skipped_lockout"
	Failed         Outcome = "failed"
)

func (o Outcome) ok() bool { return o == Applied || o == Unchanged || o == NoAction }

// Run is the last slide of a switch. Memory only: no flash write per slide.
type Run struct {
	At       int64   `json:"at" doc:"Router wall clock, unix seconds"`
	Position string  `json:"position" enum:"pressed,released" doc:"Hotplug ACTION; pressed turns the action on"`
	Outcome  Outcome `json:"outcome" enum:"applied,unchanged,no_action,rolled_back,skipped_guard,skipped_busy,skipped_lockout,failed"`
}

func (s *Service) record(node string, r Run) {
	s.runsMu.Lock()
	defer s.runsMu.Unlock()
	if s.runs == nil {
		s.runs = map[string]Run{}
	}
	s.runs[node] = r
	slog.Info("button", "node", node, "position", r.Position, "outcome", r.Outcome)
}

// fire runs the switch's action through the feature's own guarded transaction with an
// unknown caller, so the fail-closed lockout rule applies (invariants §2).
func (s *Service) fire(ctx context.Context, node string, on bool) Outcome {
	secs, err := s.sections(ctx)
	if err != nil {
		return Failed
	}
	action, ok := Configured(secs)[node]
	if !ok || action == None {
		return NoAction
	}
	if held, err := s.Guards.Stale(); err != nil || len(held) > 0 {
		return SkippedGuard
	}
	// The scan above is a fast path; Begin repeats it in one step with its own guard.
	ctx = txn.RefuseAnyGuard(ctx)
	switch action {
	case VPN:
		if s.Tunnel == nil {
			return Failed
		}
		return s.settle(s.Tunnel.Set(ctx, on))
	case GuestWiFi:
		return s.guest(ctx, on)
	}
	return Failed
}

func (s *Service) guest(ctx context.Context, on bool) Outcome {
	if s.Guest == nil {
		return Failed
	}
	g, err := s.Guest.Guest(ctx)
	if err != nil || !g.Configured {
		return Failed
	}
	if g.Enabled == on {
		return Unchanged
	}
	// An empty key keeps the stored one; Acknowledge stays false: never override lockout.
	return s.settle(s.Guest.SetGuest(ctx, wifi.GuestRequest{SSID: g.SSID, Radio: g.Radio, Enabled: on, Caller: netip.Addr{}}))
}

// settle confirms a proven transaction (ADR 0027) and maps errors to outcomes.
func (s *Service) settle(st txn.Status, err error) Outcome {
	switch {
	case errors.Is(err, lockout.ErrLockoutRisk):
		return SkippedLockout
	case errors.Is(err, txn.ErrGuardHeld):
		return SkippedGuard
	case errors.Is(err, wifi.ErrTransactionPending), errors.Is(err, txn.ErrActive):
		return SkippedBusy
	case errors.Is(err, txn.ErrNoChanges):
		return Unchanged
	case st.ID != "" && st.State != txn.Pending:
		return RolledBack
	case err != nil:
		return Failed
	}
	if s.Txns == nil {
		return Failed
	}
	done, err := s.Txns.Confirm(st.ID)
	if err != nil || done.State != txn.Confirmed {
		return RolledBack
	}
	return Applied
}
