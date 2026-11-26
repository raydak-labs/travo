// Package factoryreset wipes the overlay (`firstboot -y`) and restarts the router.
// Invariants §1 and §5: the guard is written FIRST, a failed guard write aborts, and the
// guard is never removed by Travo; the wipe itself removes it with the overlay.
package factoryreset

import (
	"context"
	"errors"
	"sync"

	"github.com/blackdark-org/travo-greenfield/internal/guard"
)

// GuardName stays until the overlay is wiped; a leftover one means the reset never ran.
const GuardName = "factory-reset-in-progress"

// Phrase is what the operator types to confirm.
const Phrase = "factory reset"

var (
	ErrConfirm     = errors.New("factory reset: confirmation phrase does not match")
	ErrPending     = errors.New("factory reset: already requested")
	ErrBusy        = errors.New("factory reset: another guarded operation is in progress")
	ErrUnavailable = errors.New("factory reset: unavailable")
	ErrFailed      = errors.New("factory reset: could not be scheduled")
)

// Resetter is the device port: schedule `firstboot -y && reboot` detached and return
// before it runs, so the HTTP reply can flush. A terminal operation: no timeout tier.
type Resetter interface {
	FactoryReset(context.Context) error
}

// Guards is the subset of guard.Dir the service needs.
type Guards interface {
	AcquireExclusive(name string, conflicts func(held []guard.Info) error) error
}

// Service serialises reset requests.
type Service struct {
	Device  Resetter
	Guards  Guards
	Stopped func() bool
	mu      sync.Mutex
}

// Reset checks the phrase and every guard, writes its own guard, then schedules the wipe.
func (s *Service) Reset(ctx context.Context, phrase string) error {
	if phrase != Phrase {
		return ErrConfirm
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Stopped != nil && s.Stopped() {
		return ErrUnavailable
	}
	// The scan and our guard are one step: a refusal writes nothing, and once our guard is
	// written no other guarded operation can start. From here on it is never removed.
	err := s.Guards.AcquireExclusive(GuardName, refuseOthers)
	switch {
	case errors.Is(err, ErrBusy):
		return ErrBusy
	case errors.Is(err, guard.ErrActive):
		return ErrPending
	case err != nil:
		return errors.Join(ErrFailed, err) // never degrade to unguarded
	}
	if err := s.Device.FactoryReset(ctx); err != nil {
		return errors.Join(ErrFailed, err)
	}
	return nil
}

func refuseOthers(held []guard.Info) error {
	if len(held) > 0 {
		return ErrBusy
	}
	return nil
}
