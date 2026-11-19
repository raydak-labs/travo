package datausage

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/atomicfile"
	"github.com/blackdark-org/travo-greenfield/internal/statshistory"
)

// View is the Network page's data usage card.
type View struct {
	Persistence     statshistory.Persistence `json:"persistence" enum:"ok,degraded" doc:"The stats history's state (ADR 0026)"`
	Budget          Budget                   `json:"budget"`
	BudgetState     State                    `json:"budget_state" enum:"off,ok,warning,exceeded,unreadable"`
	BudgetUsedBytes uint64                   `json:"budget_used_bytes" doc:"Traffic of the budget's uplinks in the billing period"`
	Today           Period                   `json:"today"`
	BillingPeriod   Period                   `json:"billing_period"`
	Days            []statshistory.Day       `json:"days" nullable:"false" doc:"Day totals, oldest first, up to 93 days"`
	RetentionHours  int64                    `json:"retention_hours" doc:"How far back the uptime log reaches"`
	Uptime          UptimeLog                `json:"uptime"`
}

// History is the read side of the stats history store this package needs.
type History interface {
	statshistory.Reader
	Days() ([]statshistory.Day, statshistory.Persistence)
	// Location is the zone the days were counted in; periods use the same.
	Location() *time.Location
}

// Service reads the history and owns the budget file.
type Service struct {
	History History
	Path    string           // budget file, e.g. /etc/travo-greenfield/data-budget.json
	Now     func() time.Time // router wall clock

	mu        sync.Mutex
	writeJSON func(string, any, os.FileMode) error
	remove    func(string) error
}

func (s *Service) load() (Budget, error) {
	var b Budget
	err := atomicfile.ReadJSON(s.Path, &b)
	if errors.Is(err, os.ErrNotExist) {
		return DefaultBudget(), nil
	}
	if err == nil {
		err = b.Validate()
	}
	if err != nil {
		return DefaultBudget(), ErrUnreadable // shown as budget_state unreadable, not logged per read
	}
	return b, nil
}

// View reads the budget and the history once. An unreadable budget still shows the usage.
func (s *Service) View(context.Context) (View, error) {
	s.mu.Lock()
	b, berr := s.load()
	s.mu.Unlock()
	now := s.Now()
	days, persistence := s.History.Days()
	points, _ := s.History.Points(now.Add(-statshistory.Retention))
	v := View{
		Persistence: persistence, Budget: b, Days: days,
		RetentionHours: int64(statshistory.Retention / time.Hour),
		Uptime:         DeriveUptime(points),
	}
	loc := s.History.Location()
	start, end := Today(now, loc)
	v.Today = Sum(days, start, end)
	v.BillingPeriod, v.BudgetUsedBytes, v.BudgetState = evaluate(b, berr, days, now, loc)
	return v, nil
}

// evaluate sums the billing period and compares the budget's share with its limit.
func evaluate(b Budget, berr error, days []statshistory.Day, now time.Time, loc *time.Location) (Period, uint64, State) {
	start, end := BillingPeriod(now, b.StartDay, loc)
	period := Sum(days, start, end)
	used := Used(period, b.Uplinks)
	if berr != nil {
		return period, used, StateUnreadable
	}
	return period, used, Evaluate(b, used)
}

// SetBudget validates and writes the budget file (a rare, user-triggered write).
func (s *Service) SetBudget(ctx context.Context, b Budget) (View, error) {
	if err := b.Validate(); err != nil {
		return View{}, err
	}
	s.mu.Lock()
	if _, err := s.load(); err != nil {
		s.mu.Unlock()
		slog.Warn("data budget unreadable: kept unchanged until reset", "path", s.Path)
		return View{}, err
	}
	write := s.writeJSON
	if write == nil {
		write = atomicfile.WriteJSON
	}
	err := write(s.Path, b, 0o600)
	s.mu.Unlock()
	if err != nil {
		slog.Warn("data budget write", "err", err)
		return View{}, ErrUnavailable
	}
	return s.View(ctx)
}

// ResetBudget is the explicit path out of an unreadable file: it removes it (budget off).
func (s *Service) ResetBudget(ctx context.Context) (View, error) {
	s.mu.Lock()
	remove := s.remove
	if remove == nil {
		remove = os.Remove
	}
	err := remove(s.Path)
	s.mu.Unlock()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		slog.Warn("data budget reset", "err", err)
		return View{}, ErrUnavailable
	}
	return s.View(ctx)
}

// BudgetState is the alert source: derived on read, never written.
func (s *Service) BudgetState(context.Context) (State, error) {
	s.mu.Lock()
	b, berr := s.load()
	s.mu.Unlock()
	days, _ := s.History.Days()
	_, _, state := evaluate(b, berr, days, s.Now(), s.History.Location())
	return state, nil
}
