package wifischedule

import (
	"context"
	"errors"
	"log/slog"
	"net/netip"
	"slices"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/hook"
	"github.com/blackdark-org/travo-greenfield/internal/lockout"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/wifi"
)

// Outcome of one scheduled toggle; stable codes for the UI.
type Outcome string

const (
	Applied        Outcome = "applied"
	Unchanged      Outcome = "unchanged"
	RolledBack     Outcome = "rolled_back"
	SkippedGuard   Outcome = "skipped_guard"
	SkippedBusy    Outcome = "skipped_busy"
	SkippedLockout Outcome = "skipped_lockout"
	Failed         Outcome = "failed"
)

// Run is the last toggle of a rule. Memory only: no flash write per toggle.
type Run struct {
	ID      string  `json:"id"`
	At      int64   `json:"at" doc:"Router wall clock, unix seconds"`
	Action  Action  `json:"action" enum:"on,off"`
	Outcome Outcome `json:"outcome" enum:"applied,unchanged,rolled_back,skipped_guard,skipped_busy,skipped_lockout,failed"`
}

// Status is the runtime view: whether crond runs and each rule's last toggle.
type Status struct {
	CronRunning *bool `json:"cron_running" nullable:"true" doc:"null when unreadable"`
	Runs        []Run `json:"runs" nullable:"false"`
}

func (s *Service) Status(ctx context.Context) Status {
	st := Status{Runs: []Run{}}
	if s.Services != nil {
		if running, err := s.Services.ServiceRunning(ctx, "cron"); err == nil {
			st.CronRunning = &running
		}
	}
	s.runsMu.Lock()
	defer s.runsMu.Unlock()
	for _, r := range s.runs {
		st.Runs = append(st.Runs, r)
	}
	slices.SortFunc(st.Runs, func(a, b Run) int { return int(b.At - a.At) })
	return st
}

func (s *Service) record(r Run) {
	s.runsMu.Lock()
	defer s.runsMu.Unlock()
	if s.runs == nil {
		s.runs = map[string]Run{}
	}
	s.runs[r.ID] = r
	slog.Info("wifi schedule", "rule", r.ID, "action", r.Action, "outcome", r.Outcome)
}

// action is the hook a rule's cron line calls. It records the outcome and reports every
// outcome but applied and unchanged as an error code, so the cron job's logger line shows it.
func (s *Service) action(id string, a Action) hook.Action {
	return func(ctx context.Context) error {
		outcome := s.fire(ctx, id, a)
		s.record(Run{ID: id, At: s.now().Unix(), Action: a, Outcome: outcome})
		if outcome == Applied || outcome == Unchanged {
			return nil
		}
		return &hook.CodedError{Code: string(outcome), Err: errors.New("wifi schedule " + string(a) + " not applied")}
	}
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// fire applies one toggle through the AP transaction with an unknown caller.
func (s *Service) fire(ctx context.Context, id string, action Action) Outcome {
	if held, err := s.Guards.Stale(); err != nil || len(held) > 0 {
		return SkippedGuard
	}
	s.mu.Lock()
	rules, err := s.load()
	s.mu.Unlock()
	if err != nil {
		return Failed
	}
	at := slices.IndexFunc(rules.Rules, func(r Rule) bool { return r.ID == id })
	if at < 0 || !rules.Rules[at].Enabled {
		return Unchanged // a stale cron line for a removed or paused rule
	}
	rule := rules.Rules[at]
	aps, err := s.APs.AccessPoints(ctx)
	if err != nil {
		return Failed
	}
	ap := slices.IndexFunc(aps, func(a wifi.APConfig) bool { return a.Section == rule.Section })
	if ap < 0 {
		return Failed
	}
	want := action == On
	if aps[ap].Enabled == want {
		return Unchanged
	}
	// The scan above is a fast path; Begin repeats it in one step with its own guard.
	st, err := s.APs.UpdateAP(txn.RefuseAnyGuard(ctx), wifi.APRequest{Section: rule.Section, SSID: aps[ap].SSID, Enabled: want,
		Acknowledge: rule.AcknowledgeLockout, Caller: netip.Addr{}})
	return s.settle(st, err)
}

// settle confirms a proven transaction (ADR 0027) and maps errors to outcomes.
func (s *Service) settle(st txn.Status, err error) Outcome {
	switch {
	case errors.Is(err, lockout.ErrLockoutRisk):
		return SkippedLockout
	case errors.Is(err, txn.ErrGuardHeld):
		return SkippedGuard
	case errors.Is(err, wifi.ErrTransactionPending):
		return SkippedBusy
	case errors.Is(err, txn.ErrNoChanges):
		return Unchanged
	case st.ID != "" && st.State != txn.Pending:
		return RolledBack
	case err != nil:
		return Failed
	}
	done, err := s.Txns.Confirm(st.ID)
	if err != nil || done.State != txn.Confirmed {
		return RolledBack
	}
	return Applied
}
