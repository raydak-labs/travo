// Package guard implements the crash-guard markers of ADR 0003: a file written
// before a dangerous live-state change and removed only after it succeeded, so
// an interrupted change is discoverable instead of silently half-applied.
package guard

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

// DefaultDir holds every crash guard on the device (ADR 0003 §2). State files
// stay in /etc/travo; only abort markers live here.
const DefaultDir = "/etc/trafo"

var dir atomic.Value // string

// Dir returns the directory guards resolve to when none is given.
func Dir() string {
	if d, ok := dir.Load().(string); ok && d != "" {
		return d
	}
	return DefaultDir
}

// SetDir redirects the default directory (mock mode, tests) and returns a
// function that restores the previous value.
func SetDir(d string) (restore func()) {
	prev := Dir()
	dir.Store(d)
	return func() { dir.Store(prev) }
}

// Guard is one crash-guard marker file.
type Guard struct{ path string }

// In returns the guard for feature inside dir; an empty dir means Dir().
func In(d, feature string) Guard {
	if d == "" {
		d = Dir()
	}
	return Guard{path: filepath.Join(d, feature+"-in-progress")}
}

// New returns the guard for feature in the default directory.
func New(feature string) Guard { return In("", feature) }

// Path returns the marker's file path.
func (g Guard) Path() string { return g.path }

// Present reports whether the marker exists.
func (g Guard) Present() bool {
	_, err := os.Stat(g.path)
	return err == nil
}

// Read returns the marker's content.
func (g Guard) Read() ([]byte, error) { return os.ReadFile(g.path) }

// Write creates or replaces the marker atomically (temp file + rename), so a
// concurrent reader never sees a half-written payload.
func (g Guard) Write(content string) error {
	d := filepath.Dir(g.path)
	if err := os.MkdirAll(d, 0o750); err != nil {
		return fmt.Errorf("crash guard %s: %w", g.path, err)
	}
	tmp, err := os.CreateTemp(d, filepath.Base(g.path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("crash guard %s: %w", g.path, err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	_, err = tmp.WriteString(content)
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(tmp.Name(), 0o600)
	}
	if err == nil {
		err = os.Rename(tmp.Name(), g.path)
	}
	if err != nil {
		return fmt.Errorf("crash guard %s: %w", g.path, err)
	}
	return nil
}

// Mark writes the marker with the current time as its payload.
func (g Guard) Mark() error { return g.Write(time.Now().Format(time.RFC3339Nano)) }

// Create writes the marker but refuses when one already exists: a guard left
// by an interrupted run must be cleared by a human or a redeploy, never retried
// over (ADR 0003 §1.2).
func (g Guard) Create(content string) error {
	if data, err := g.Read(); err == nil {
		return fmt.Errorf("crash guard %s exists from an interrupted run (%s); remove it or redeploy before retrying",
			g.path, strings.TrimSpace(string(data)))
	}
	return g.Write(content)
}

// Clear removes the marker; a missing marker is not an error.
func (g Guard) Clear() error {
	if err := os.Remove(g.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("crash guard %s: %w", g.path, err)
	}
	return nil
}
