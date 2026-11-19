// Package reboot schedules a guarded router restart and settles its durable record on the next boot.
package reboot

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/atomicfile"
	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/factoryreset"
	"github.com/blackdark-org/travo-greenfield/internal/firmware"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/updateapply"
)

// GuardName is the crash guard retained after a restart is scheduled.
const GuardName = "reboot-in-progress"

// Grace is the age after which a same-boot guard means the reboot never happened.
const Grace = 2 * time.Minute

var (
	ErrPending    = errors.New("reboot already requested")
	ErrTxnPending = errors.New("transaction awaiting confirmation")
	ErrUpdateBusy = errors.New("update in progress")
	// ErrTerminalBusy: a firmware flash or factory reset holds its guard; a restart
	// would interrupt it.
	ErrTerminalBusy = errors.New("firmware upgrade or factory reset in progress")
	ErrUnavailable  = errors.New("reboot unavailable")
	ErrFailed       = errors.New("reboot could not be scheduled")
)

// Router is the narrow device port needed to schedule and verify a restart.
type Router interface {
	device.Rebooter
	device.BootIDReader
}

// Accepted identifies the boot that accepted the request without exposing the raw kernel id.
type Accepted struct{ BootID string }

// Service serialises restart requests and owns their durable guard and record.
type Service struct {
	Router     Router
	Guards     guard.Dir
	RecordPath string
	Now        func() time.Time
	Stopped    func() bool
	mu         sync.Mutex
}

type record struct {
	BootID    string    `json:"boot_id"`
	Requested time.Time `json:"requested"`
}

func New(r Router, g guard.Dir, recordPath string, now func() time.Time, stopped func() bool) *Service {
	return &Service{Router: r, Guards: g, RecordPath: recordPath, Now: now, Stopped: stopped}
}

// OpaqueID exposes only the first 32 hex characters of the raw id's SHA-256 digest.
func OpaqueID(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])[:32]
}

// BootID returns an opaque id for the current boot.
func (s *Service) BootID(ctx context.Context) (string, error) {
	raw, err := s.Router.BootID(ctx)
	if err != nil {
		return "", err
	}
	if raw == "" {
		return "", errors.New("empty kernel boot id")
	}
	return OpaqueID(raw), nil
}

// Reboot writes durable intent before scheduling the terminal operation.
func (s *Service) Reboot(ctx context.Context) (Accepted, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Stopped != nil && s.Stopped() {
		return Accepted{}, ErrUnavailable
	}
	raw, err := s.Router.BootID(ctx)
	if err != nil || raw == "" {
		return Accepted{}, errors.Join(ErrUnavailable, err)
	}
	if err := s.checkBeforeAcquire(raw); err != nil {
		return Accepted{}, err
	}
	if err := s.Guards.Acquire(GuardName); err != nil {
		if errors.Is(err, guard.ErrActive) {
			return Accepted{}, ErrPending
		}
		return Accepted{}, errors.Join(ErrFailed, err)
	}
	if err := s.checkExclusions(); err != nil {
		return Accepted{}, errors.Join(err, s.Guards.Release(GuardName))
	}
	if err := s.writeRecord(raw); err != nil {
		return Accepted{}, errors.Join(ErrFailed, err, s.Guards.Release(GuardName))
	}
	if err := s.Router.Reboot(ctx); err != nil {
		return Accepted{}, errors.Join(ErrFailed, err, s.Guards.Release(GuardName), s.removeRecord())
	}
	return Accepted{BootID: OpaqueID(raw)}, nil
}

func (s *Service) checkBeforeAcquire(raw string) error {
	stale, err := s.Guards.Stale()
	if err != nil {
		return errors.Join(ErrFailed, err)
	}
	if err := exclusionError(stale); err != nil {
		return err
	}
	for _, g := range stale {
		if g.Name == GuardName {
			return s.prepareRetry(g, raw)
		}
	}
	return nil
}

// exclusionError runs before and again after the reboot guard is written. Firmware upgrade
// and factory reset write their guard first and then scan too, so of two racing requests
// at least one sees the other's guard.
func exclusionError(stale []guard.Info) error {
	for _, g := range stale {
		if g.Name == firmware.GuardName || g.Name == factoryreset.GuardName {
			return ErrTerminalBusy
		}
	}
	for _, g := range stale {
		if g.Name == updateapply.GuardName {
			return ErrUpdateBusy
		}
	}
	for _, g := range stale {
		if g.Name == txn.GuardName {
			return ErrTxnPending
		}
	}
	return nil
}

func (s *Service) prepareRetry(g guard.Info, raw string) error {
	var previous record
	err := atomicfile.ReadJSON(s.RecordPath, &previous)
	if err == nil {
		if previous.BootID == "" {
			return ErrPending
		}
		if previous.BootID != raw {
			return s.releaseForRetry()
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return ErrPending
	}
	if g.Started.IsZero() || s.Now().Sub(g.Started) <= Grace {
		return ErrPending
	}
	return s.releaseForRetry()
}

func (s *Service) releaseForRetry() error {
	if err := s.releaseRequest(); err != nil {
		return errors.Join(ErrFailed, err)
	}
	return nil
}

func (s *Service) releaseRequest() error {
	return errors.Join(s.Guards.Release(GuardName), s.removeRecord())
}

func (s *Service) checkExclusions() error {
	stale, err := s.Guards.Stale()
	if err != nil {
		return errors.Join(ErrFailed, err)
	}
	return exclusionError(stale)
}

func (s *Service) writeRecord(raw string) error {
	if err := os.MkdirAll(filepath.Dir(s.RecordPath), 0o700); err != nil {
		return err
	}
	return atomicfile.WriteJSON(s.RecordPath, record{BootID: raw, Requested: s.Now().UTC()}, 0o600)
}

func (s *Service) removeRecord() error {
	if err := os.Remove(s.RecordPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Settle clears durable intent only after a different kernel boot proves the restart happened.
func (s *Service) Settle(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	stale, err := s.Guards.Stale()
	if err != nil {
		return err
	}
	present := false
	for _, g := range stale {
		present = present || g.Name == GuardName
	}
	if !present {
		return s.removeRecord()
	}
	raw, err := s.Router.BootID(ctx)
	if err != nil {
		return err
	}
	if raw == "" {
		return errors.New("empty kernel boot id")
	}
	var previous record
	if err := atomicfile.ReadJSON(s.RecordPath, &previous); err != nil {
		slog.Warn("reboot guard retained: unknown boot", "error", err)
		return nil
	}
	if previous.BootID == "" {
		slog.Warn("reboot guard retained: unknown boot; record has an empty boot id")
		return nil
	}
	if previous.BootID == raw {
		return nil
	}
	if err := s.releaseRequest(); err != nil {
		return err
	}
	slog.Info("reboot guard settled after a new boot")
	return nil
}
