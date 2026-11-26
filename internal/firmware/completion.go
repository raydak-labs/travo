package firmware

import (
	"context"
	"errors"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
)

// Keys of the firmware guard's detail (ADR 0030).
const (
	DetailBootID   = "boot_id"
	DetailRevision = "revision"
	DetailSHA256   = "sha256"
)

// Outcome is what Complete decided about a held firmware guard.
type Outcome string

const (
	OutcomeNone        Outcome = "none"        // no firmware guard
	OutcomeCompleted   Outcome = "completed"   // new boot, new revision: guard released
	OutcomeInterrupted Outcome = "interrupted" // same boot or same revision: guard kept
	OutcomeUnknown     Outcome = "unknown"     // something unreadable: guard kept (fail closed)
)

// guardDetail records what Complete compares after the reboot. A value that cannot be read
// stays empty, and an empty value later keeps the guard: the upgrade itself is not refused.
func (s *Service) guardDetail(ctx context.Context, sha string) map[string]string {
	d := map[string]string{DetailSHA256: sha}
	if id, err := s.Device.BootID(ctx); err == nil {
		d[DetailBootID] = id
	}
	if rev, err := s.revision(ctx); err == nil {
		d[DetailRevision] = rev
	}
	return d
}

func (s *Service) revision(ctx context.Context) (string, error) {
	raw, err := s.Device.Call(ctx, "system", "board", nil)
	if err != nil {
		return "", err
	}
	b, err := facts.ParseBoard(raw)
	if err != nil {
		return "", err
	}
	if b.Revision == "" {
		return "", errors.New("firmware: board reports no revision")
	}
	return b.Revision, nil
}

// Complete runs once at daemon start. A firmware guard whose recorded boot and pre-flash
// revision both differ from the running ones belongs to a flash that finished: it is
// released and Completed reports the time. Anything else keeps the guard as before.
func (s *Service) Complete(ctx context.Context) (Outcome, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	held, err := s.Guards.Stale()
	if err != nil {
		return OutcomeUnknown, err
	}
	var g *guard.Info
	for i := range held {
		if held[i].Name == GuardName {
			g = &held[i]
		}
	}
	if g == nil {
		return OutcomeNone, nil
	}
	out := Decide(g.Detail, s.currentBoot(ctx), s.currentRevision(ctx))
	if out != OutcomeCompleted {
		return out, nil
	}
	if err := s.Guards.Release(GuardName); err != nil {
		return OutcomeUnknown, err
	}
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	t := now().UTC()
	s.completed = &t
	return out, nil
}

func (s *Service) currentBoot(ctx context.Context) string {
	id, err := s.Device.BootID(ctx)
	if err != nil {
		return ""
	}
	return id
}

func (s *Service) currentRevision(ctx context.Context) string {
	rev, err := s.revision(ctx)
	if err != nil {
		return ""
	}
	return rev
}

// Decide is the pure rule: completed only when every value is known, the boot changed
// and the running revision is not the one the flash started from.
func Decide(detail map[string]string, bootID, revision string) Outcome {
	wasBoot, wasRev := detail[DetailBootID], detail[DetailRevision]
	switch {
	case wasBoot == "" || wasRev == "" || bootID == "" || revision == "":
		return OutcomeUnknown
	case wasBoot == bootID || wasRev == revision:
		return OutcomeInterrupted
	}
	return OutcomeCompleted
}

// Completed is when this daemon run released the guard of a finished upgrade; nil
// otherwise. It lives in memory: the alert shows until the next daemon restart.
func (s *Service) Completed() *time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.completed == nil {
		return nil
	}
	t := *s.completed
	return &t
}
