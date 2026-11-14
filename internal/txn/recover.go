package txn

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Alive reports whether a watchdog pid is still running it (production: ProcAlive).
type Alive func(pid int, script string) bool

// Recover runs at startup, before any Begin. For each transaction on disk:
//   - confirmed or restored: cleaned up
//   - restore failed: kept, guard kept
//   - watchdog alive: kept (the user can still confirm after a daemon restart)
//   - watchdog gone (reboot, kill) and something may be committed: restored now
//   - crashed before the watchdog started: nothing was committed; staging reverted, removed
//
// The guard is released only when nothing is left pending.
func (m *Manager) Recover(ctx context.Context, alive Alive) ([]Status, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entries, err := os.ReadDir(m.Dir)
	if errors.Is(err, os.ErrNotExist) {
		return []Status{}, m.Guards.Release(GuardName)
	}
	if err != nil {
		return nil, err
	}
	out := []Status{}
	pending := false
	for _, e := range entries {
		id := e.Name()
		if !e.IsDir() || !idRe.MatchString(id) {
			continue
		}
		if _, err := os.Stat(m.path(id, "pid")); errors.Is(err, os.ErrNotExist) {
			// Commit happens only after the pid is recorded: nothing reached /etc/config.
			if err := m.discardUnstarted(ctx, id); err != nil {
				pending = true
			}
			continue
		}
		meta, err := m.load(id)
		if err != nil {
			pending = true // unreadable but possibly committed: operator decides
			continue
		}
		st := m.outcome(id, meta)
		switch {
		case st.State == Confirmed:
			_ = os.RemoveAll(m.path(id))
		case st.State == RolledBack:
			out = append(out, m.finish(id, st))
		case st.State == RollbackFailed:
			pending = true
			out = append(out, st)
		case watchdogAlive(m.path(id), alive):
			pending = true
			out = append(out, st)
		default:
			st := m.restoreSync(id, meta)
			pending = pending || st.State != RolledBack
			out = append(out, st)
		}
	}
	if pending {
		return out, nil
	}
	return out, m.Guards.Release(GuardName)
}

func (m *Manager) discardUnstarted(ctx context.Context, id string) error {
	b, err := os.ReadFile(m.path(id, "configs"))
	if err == nil {
		for _, c := range strings.Fields(string(b)) {
			_ = m.Device.Revert(ctx, c)
		}
	}
	return os.RemoveAll(m.path(id))
}

// reapFinished clears restored transactions left by a detached watchdog, so a timeout
// restore does not block the next Begin until restart. Failed ones keep the guard.
func (m *Manager) reapFinished() {
	entries, err := os.ReadDir(m.Dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if meta, err := m.load(e.Name()); err == nil {
			m.finish(e.Name(), m.outcome(e.Name(), meta))
		}
	}
}

func watchdogAlive(dir string, alive Alive) bool {
	b, err := os.ReadFile(filepath.Join(dir, "pid"))
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	return err == nil && pid > 0 && alive(pid, filepath.Join(dir, "watchdog.sh"))
}
