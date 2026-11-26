// Package txn is the only way to change live state (ADR 0005):
// guard -> snapshot -> detached watchdog -> commit -> reload -> probe -> arm -> user confirm.
// The watchdog restores the snapshot unless the user confirms in time; the backend never confirms.
package txn

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/atomicfile"
	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/uciwrite"
)

//go:embed watchdog.sh
var WatchdogScript []byte

var (
	ErrActive        = errors.New("another transaction is awaiting confirmation")
	ErrNotFound      = errors.New("no such transaction")
	ErrClosed        = errors.New("transaction already finished")
	ErrNoChanges     = errors.New("transaction staged nothing")
	ErrStopped       = errors.New("daemon is stopping; no commits")
	ErrProbeFailed   = errors.New("post-apply probe failed")
	ErrForeignStaged = errors.New("foreign staged changes")
	ErrBadWindow     = errors.New("confirm window too short")
)

// State of a transaction as the UI sees it.
type State string

const (
	Pending    State = "pending" // applied, watchdog armed, waiting for the user
	Confirmed  State = "confirmed"
	RolledBack State = "rolled_back"
	// Restoring: a restore was claimed but has not reported yet. Guard kept.
	Restoring State = "restoring"
	// RollbackFailed: restore ran but could not finish. Guard kept for the operator.
	RollbackFailed State = "rollback_failed"
)

// GuardName is the crash guard held while a transaction needs confirmation.
const GuardName = "txn-in-progress"

// MinWindow is the floor for the confirm window (AP bring-up ~10 s, device-facts §2).
const MinWindow = 15 * time.Second

// armMargin separates the apply budget from the arm limit.
const armMargin = 10 * time.Second

var idRe = regexp.MustCompile(`^[0-9a-f]{16}$`)

// Spawner runs the watchdog script. Start detaches it from the daemon (own session,
// survives daemon death) and returns its pid; Run waits for it (immediate restore).
type Spawner interface {
	Start(script string, args ...string) (pid int, err error)
	Run(script string, args ...string) error
}

// Probe checks the device after apply. An error means "not proven up": roll back.
type Probe func(ctx context.Context) error

// Clock is injected (time is an edge effect).
type Clock interface{ Now() time.Time }

// Manager runs transactions. One may be pending at a time: a second change while the
// first is unconfirmed would make "rollback" restore the wrong state.
type Manager struct {
	Device device.Device
	Locks  *uciwrite.Locks
	Guards guard.Dir
	// Dir holds snapshots on flash (e.g. /etc/travo-greenfield/txn): a reboot mid-window
	// (ath11k crashes can cause one) must still restore. Writes are rare, per transaction.
	Dir       string
	ConfigDir string   // /etc/config
	Reload    []string // argv the watchdog runs after restore, e.g. ["reload_config"]
	UCI       string   // uci binary for the watchdog
	Spawner   Spawner
	Clock     Clock
	NewID     func() string
	Stopped   func() bool
	// Window is how long the user has to confirm, counted from "armed" (after probe).
	Window time.Duration
	// ArmLimit bounds commit+reload+probe; past it the watchdog restores (daemon died mid-apply).
	// Must exceed ApplyBudget, or a slow commit could land after the restore.
	ArmLimit time.Duration
	// ApplyBudget is the shared deadline for the live apply phase.
	ApplyBudget time.Duration
	// Settle bounds how long Rollback/Recover wait for an already-claimed restore to report.
	Settle time.Duration

	mu                      sync.Mutex
	begun                   atomic.Uint64 // transactions that acquired the guard; see ShowSettled
	testHookAfterFirstCheck func()        // runs between ShowSettled's first guard check and its read
}

// Meta is persisted beside the snapshot so status survives a daemon restart.
type Meta struct {
	ID      string    `json:"id"`
	Configs []string  `json:"configs"`
	Started time.Time `json:"started"`
	Summary string    `json:"summary"`
	// Window in seconds, counted from when the change was armed (after probe).
	WindowSeconds int `json:"window_seconds"`
}

// Status is the read-only view of a transaction.
type Status struct {
	Meta
	State State `json:"state"`
}

func (m *Manager) path(id string, parts ...string) string {
	return filepath.Join(append([]string{m.Dir, id}, parts...)...)
}

func (m *Manager) stopped() bool { return m.Stopped != nil && m.Stopped() }

// Begin runs stage under the config locks, then commits, reloads and probes under a
// watchdog. On success the transaction is Pending until Confirm or the window expires.
// stage receives a ctx that marks the locks held: helpers must not lock again.
func (m *Manager) Begin(ctx context.Context, summary string, configs []string, stage func(ctx context.Context) error, probe Probe) (Status, error) {
	return m.begin(ctx, summary, configs, false, m.Window, stage, probe)
}

// BeginWireGuard restores the live wg0 device as part of rolling back a network change.
func (m *Manager) BeginWireGuard(ctx context.Context, summary string, stage func(context.Context) error, probe Probe) (Status, error) {
	return m.begin(ctx, summary, []string{"network"}, true, m.Window, stage, probe)
}

// BeginWireGuardWith is BeginWireGuard with more configs in the same snapshot and lock set
// (e.g. firewall for the VPN zone), so one confirm or restore covers them all.
func (m *Manager) BeginWireGuardWith(ctx context.Context, summary string, extra []string, stage func(context.Context) error, probe Probe) (Status, error) {
	return m.begin(ctx, summary, append([]string{"network"}, extra...), true, m.Window, stage, probe)
}

func (m *Manager) begin(ctx context.Context, summary string, configs []string, restoreWireGuard bool, window time.Duration, stage func(context.Context) error, probe Probe) (Status, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Window < MinWindow || window < m.Window || m.ApplyBudget <= 0 || m.ArmLimit < m.ApplyBudget+armMargin {
		return Status{}, ErrBadWindow
	}
	cs, err := uciwrite.Normalize(configs)
	if err != nil {
		return Status{}, err
	}
	if m.stopped() {
		return Status{}, ErrStopped
	}
	m.reapFinished()
	if err := m.Guards.AcquireExclusive(GuardName, Conflicts(ctx)); err != nil {
		switch {
		case errors.Is(err, ErrGuarded), errors.Is(err, ErrGuardHeld):
			return Status{}, err
		case errors.Is(err, guard.ErrActive):
			return Status{}, ErrActive
		}
		return Status{}, fmt.Errorf("txn guard: %w", err) // never degrade to unguarded
	}
	// After the guard, before its release: a reader whose first guard check missed this
	// transaction sees either the guard or the new count at its second check.
	m.begun.Add(1)
	lctx, unlock, err := m.Locks.Lock(ctx, cs)
	if err != nil {
		return Status{}, m.release(err)
	}
	defer unlock()
	if err := m.refuseForeign(lctx, cs); err != nil {
		return Status{}, m.release(err)
	}

	id := m.NewID()
	if !idRe.MatchString(id) {
		return Status{}, m.release(fmt.Errorf("txn: bad id %q", id))
	}
	abort := func(err error) (Status, error) { // nothing committed yet
		return Status{}, m.release(errors.Join(err, m.revertAll(context.WithoutCancel(lctx), cs), os.RemoveAll(m.path(id))))
	}
	meta, err := m.snapshot(id, cs, summary, window)
	if err != nil {
		return abort(fmt.Errorf("txn: snapshot: %w", err))
	}
	if restoreWireGuard {
		if err := m.recordWireGuardBaseline(lctx, id); err != nil {
			return abort(fmt.Errorf("txn: record wg0 baseline: %w", err))
		}
	} else {
		m.recordSettle(lctx, id, cs)
	}
	if err := stage(lctx); err != nil {
		return abort(fmt.Errorf("txn: stage: %w", err))
	}
	if changed, err := m.anyPending(lctx, cs); err != nil {
		return abort(err)
	} else if !changed {
		return abort(ErrNoChanges)
	}
	if m.stopped() {
		return abort(ErrStopped)
	}

	applyCtx, cancel := context.WithTimeout(lctx, m.ApplyBudget)
	defer cancel()
	if err := applyCtx.Err(); err != nil {
		return abort(err)
	}
	// Arm the watchdog BEFORE the first commit: from here on, any crash ends in a restore.
	pid, err := m.Spawner.Start(m.path(id, "watchdog.sh"), m.watchdogArgs(id, window, m.ArmLimit)...)
	if err != nil {
		return abort(fmt.Errorf("txn: watchdog: %w", err))
	}
	if err := atomicfile.Write(m.path(id, "pid"), []byte(strconv.Itoa(pid)+"\n"), 0o600); err != nil {
		return m.rollbackNow(id, meta, fmt.Errorf("txn: record watchdog pid: %w", err))
	}
	return m.applyLive(applyCtx, meta, probe)
}

func (m *Manager) applyLive(ctx context.Context, meta Meta, probe Probe) (Status, error) {
	id := meta.ID
	for _, c := range meta.Configs {
		if err := m.stillOurs(id); err != nil {
			return m.settledAfterRace(id, meta, errors.Join(ctx.Err(), err))
		}
		if m.stopped() {
			return m.rollbackNow(id, meta, errors.Join(ctx.Err(), ErrStopped))
		}
		if err := runBeforeDeadline(ctx, func() error { return m.Device.Commit(ctx, c) }); err != nil {
			return m.rollbackNow(id, meta, fmt.Errorf("txn: commit %s: %w", c, err))
		}
	}
	if err := m.stillOurs(id); err != nil {
		return m.settledAfterRace(id, meta, errors.Join(ctx.Err(), err))
	}
	if m.stopped() {
		return m.rollbackNow(id, meta, errors.Join(ctx.Err(), ErrStopped))
	}
	if err := runBeforeDeadline(ctx, func() error { return m.Device.Reload(ctx) }); err != nil {
		return m.rollbackNow(id, meta, fmt.Errorf("txn: reload: %w", err))
	}
	if probe != nil {
		if err := runBeforeDeadline(ctx, func() error { return probe(ctx) }); err != nil {
			return m.rollbackNow(id, meta, fmt.Errorf("%w: %w", ErrProbeFailed, err))
		}
	}
	if err := m.stillOurs(id); err != nil {
		return m.settledAfterRace(id, meta, errors.Join(ctx.Err(), err))
	}
	if err := ctx.Err(); err != nil {
		return m.rollbackNow(id, meta, err)
	}
	// Start the user's window only now: they get all of it.
	if err := atomicfile.Write(m.path(id, "armed"), []byte("1\n"), 0o600); err != nil {
		return m.rollbackNow(id, meta, fmt.Errorf("txn: arm: %w", err))
	}
	return Status{Meta: meta, State: Pending}, nil
}

// runBeforeDeadline observes cancellation even if a live operation returns nil.
func runBeforeDeadline(ctx context.Context, run func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	err := run()
	return errors.Join(err, ctx.Err())
}

func (m *Manager) watchdogArgs(id string, window, armLimit time.Duration) []string {
	args := []string{m.path(id), m.ConfigDir, strconv.Itoa(int(window / time.Second)), strconv.Itoa(int(armLimit / time.Second)), m.UCI}
	return append(args, m.Reload...)
}

// stillOurs fails when the watchdog already claimed (ArmLimit exceeded mid-apply).
func (m *Manager) stillOurs(id string) error {
	if _, err := os.Stat(m.path(id, "claim")); err == nil {
		return errors.New("txn: apply exceeded the arm limit; watchdog is restoring")
	}
	return nil
}

func (m *Manager) settledAfterRace(id string, meta Meta, cause error) (Status, error) {
	st := m.waitOutcome(id, meta)
	return st, errors.Join(cause, stateErr(st.State))
}

func (m *Manager) refuseForeign(ctx context.Context, cs []string) error {
	for _, c := range cs { // never commit someone else's staged delta
		if p, err := m.Device.Pending(ctx, c); err != nil {
			return fmt.Errorf("txn: %s: %w", c, err)
		} else if p {
			return fmt.Errorf("txn: %s: %w", c, ErrForeignStaged)
		}
	}
	return nil
}

func (m *Manager) anyPending(ctx context.Context, cs []string) (bool, error) {
	for _, c := range cs {
		p, err := m.Device.Pending(ctx, c)
		if err != nil || p {
			return p, err
		}
	}
	return false, nil
}

func (m *Manager) snapshot(id string, cs []string, summary string, window time.Duration) (Meta, error) {
	if err := os.MkdirAll(m.path(id, "snap"), 0o700); err != nil {
		return Meta{}, err
	}
	for _, c := range cs {
		b, err := os.ReadFile(filepath.Join(m.ConfigDir, c))
		if errors.Is(err, os.ErrNotExist) {
			continue // absent snapshot = config did not exist; restore deletes it
		}
		if err != nil {
			return Meta{}, err
		}
		if err := atomicfile.Write(m.path(id, "snap", c), b, 0o600); err != nil {
			return Meta{}, err
		}
	}
	if err := atomicfile.Write(m.path(id, "configs"), []byte(strings.Join(cs, "\n")+"\n"), 0o600); err != nil {
		return Meta{}, err
	}
	if err := atomicfile.Write(m.path(id, "watchdog.sh"), WatchdogScript, 0o700); err != nil {
		return Meta{}, err
	}
	meta := Meta{ID: id, Configs: cs, Started: m.Clock.Now().UTC(), Summary: summary, WindowSeconds: int(window / time.Second)}
	return meta, atomicfile.WriteJSON(m.path(id, "meta.json"), meta, 0o600)
}

func (m *Manager) revertAll(ctx context.Context, cs []string) error {
	var errs []error
	for _, c := range cs {
		errs = append(errs, m.Device.Revert(ctx, c))
	}
	return errors.Join(errs...)
}

func (m *Manager) release(err error) error {
	if rerr := m.Guards.Release(GuardName); rerr != nil {
		return errors.Join(err, rerr)
	}
	return err
}

// Confirm is called only from the user's explicit confirm request.
func (m *Manager) Confirm(id string) (Status, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	meta, err := m.load(id)
	if err != nil {
		return Status{}, err
	}
	if _, err := os.Stat(m.path(id, "armed")); err != nil {
		return m.outcome(id, meta), ErrClosed // not yet armed, or apply failed
	}
	f, err := os.OpenFile(m.path(id, "claim"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return m.finish(id, m.outcome(id, meta)), ErrClosed // watchdog won the race
	}
	if err != nil {
		return Status{}, err
	}
	_, werr := f.WriteString("confirm\n")
	if err := errors.Join(werr, f.Sync(), f.Close()); err != nil {
		return Status{}, err
	}
	_ = os.RemoveAll(m.path(id))
	m.releaseIfIdle()
	return Status{Meta: meta, State: Confirmed}, nil
}

// Rollback is the user's "undo now".
func (m *Manager) Rollback(id string) (Status, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	meta, err := m.load(id)
	if err != nil {
		return Status{}, err
	}
	st := m.restoreSync(id, meta)
	return st, stateErr(st.State)
}

// Get reports state; a finished restore is cleaned up so the next change is not blocked.
func (m *Manager) Get(id string) (Status, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	meta, err := m.load(id)
	if err != nil {
		return Status{}, err
	}
	return m.finish(id, m.outcome(id, meta)), nil
}

func (m *Manager) load(id string) (Meta, error) {
	if !idRe.MatchString(id) {
		return Meta{}, ErrNotFound
	}
	var meta Meta
	if err := atomicfile.ReadJSON(m.path(id, "meta.json"), &meta); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Meta{}, ErrNotFound
		}
		return Meta{}, err
	}
	return meta, nil
}
