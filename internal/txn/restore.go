package txn

import (
	"errors"
	"os"
	"strings"
	"time"
)

func stateErr(s State) error {
	switch s {
	case RollbackFailed:
		return errors.New("rollback failed: guard kept, operator action needed")
	case Restoring:
		return errors.New("rollback still running: guard kept")
	}
	return nil
}

// rollbackNow runs an immediate restore through the same script (one restore path).
func (m *Manager) rollbackNow(id string, meta Meta, cause error) (Status, error) {
	st := m.restoreSync(id, meta)
	return st, errors.Join(cause, stateErr(st.State))
}

// restoreSync runs the watchdog with window 0. If the detached watchdog already claimed,
// the script exits at once and we wait for its result instead.
func (m *Manager) restoreSync(id string, meta Meta) Status {
	if err := m.Spawner.Run(m.path(id, "watchdog.sh"), m.watchdogArgs(id, 0, 0)...); err != nil {
		if st := m.outcome(id, meta); st.State == RolledBack {
			return m.finish(id, st)
		}
		return Status{Meta: meta, State: RollbackFailed}
	}
	return m.waitOutcome(id, meta)
}

// waitOutcome polls for the script's result for up to Settle.
func (m *Manager) waitOutcome(id string, meta Meta) Status {
	deadline := m.Clock.Now().Add(m.Settle)
	for {
		st := m.outcome(id, meta)
		if st.State == RolledBack || st.State == RollbackFailed {
			return m.finish(id, st)
		}
		if !m.Clock.Now().Before(deadline) {
			return Status{Meta: meta, State: Restoring}
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// finish cleans up a restored transaction. "Rolled back" only when the script said so.
// The guard is released only when no other transaction is left on disk.
func (m *Manager) finish(id string, st Status) Status {
	if st.State == RolledBack {
		_ = os.RemoveAll(m.path(id))
		m.releaseIfIdle()
	}
	return st
}

func (m *Manager) releaseIfIdle() {
	entries, err := os.ReadDir(m.Dir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return
	}
	for _, e := range entries {
		if e.IsDir() && idRe.MatchString(e.Name()) {
			return
		}
	}
	_ = m.Guards.Release(GuardName)
}

// outcome reads the on-disk state without side effects.
func (m *Manager) outcome(id string, meta Meta) Status {
	res, _ := os.ReadFile(m.path(id, "result"))
	switch strings.TrimSpace(string(res)) {
	case "restored":
		return Status{Meta: meta, State: RolledBack}
	case "failed":
		return Status{Meta: meta, State: RollbackFailed}
	}
	claim, _ := os.ReadFile(m.path(id, "claim"))
	switch strings.TrimSpace(string(claim)) {
	case "restore":
		return Status{Meta: meta, State: Restoring}
	case "confirm":
		return Status{Meta: meta, State: Confirmed}
	}
	return Status{Meta: meta, State: Pending}
}
