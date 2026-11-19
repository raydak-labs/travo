// Package guard implements crash-guard files for live-state operations.
// A present guard means "an operation started and did not finish": features skip.
package guard

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/atomicfile"
)

var (
	// ErrActive means a guard of that name already exists (errors wrapping it: a conflicting one).
	ErrActive = errors.New("guard already active")
	// ErrHeld is RefuseAny's answer: another guarded operation is in progress.
	ErrHeld = errors.New("another guarded operation is in progress")
	// ErrTerminal is RefuseTerminal's answer; it wraps ErrActive so busy mappings apply.
	ErrTerminal = fmt.Errorf("%w: a firmware upgrade, factory reset, reboot or update is pending", ErrActive)
)

// mu makes scan-and-create one step for every Dir of this process: two operations that
// check each other's guards can never both pass. Creating by hard link keeps a same-name
// create exclusive across processes too.
var mu sync.Mutex

var validName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*-in-progress$`)

// Dir manages guards inside one directory that holds nothing else.
type Dir struct {
	Path string
	Now  func() time.Time
}

// Info describes a guard found on disk.
type Info struct {
	Name    string    `json:"name"`
	Started time.Time `json:"started"`
}

func (d Dir) file(name string) (string, error) {
	if !validName.MatchString(name) {
		return "", fmt.Errorf("invalid guard name %q", name)
	}
	return filepath.Join(d.Path, name), nil
}

// Acquire writes the guard first. A failed write aborts the caller's operation.
func (d Dir) Acquire(name string) error { return d.AcquireExclusive(name, nil) }

// AcquireExclusive scans the held guards and writes name in one step: ErrActive when name
// is held, conflicts' error when it refuses the others (nil conflicts refuses nothing).
// conflicts runs under the process-wide guard lock: it must be pure and never touch guards.
func (d Dir) AcquireExclusive(name string, conflicts func(held []Info) error) error {
	p, err := d.file(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(d.Path, 0o700); err != nil {
		return fmt.Errorf("guard dir: %w", err)
	}
	in := Info{Name: name, Started: d.Now().UTC()} // no injected callback runs under mu
	mu.Lock()
	defer mu.Unlock()
	held, err := d.Stale()
	if err != nil {
		return fmt.Errorf("guard scan: %w", err)
	}
	if slices.ContainsFunc(held, func(g Info) bool { return g.Name == name }) {
		return ErrActive
	}
	if conflicts != nil {
		if err := conflicts(held); err != nil {
			return err
		}
	}
	return d.create(p, in)
}

// Check runs conflicts against the held guards under the lock of AcquireExclusive, for
// operations that hold no guard of their own.
func (d Dir) Check(conflicts func(held []Info) error) error {
	mu.Lock()
	defer mu.Unlock()
	held, err := d.Stale()
	if err != nil {
		return fmt.Errorf("guard scan: %w", err)
	}
	return conflicts(held)
}

// create writes a complete temp file and links it to p: the link fails when p exists, so
// the guard appears whole and only once. The directory is synced before returning.
func (d Dir) create(p string, in Info) error {
	b, err := json.MarshalIndent(in, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(d.Path, "."+filepath.Base(p)+".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	_, err = tmp.Write(append(b, '\n'))
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if err := os.Link(name, p); errors.Is(err, fs.ErrExist) {
		return ErrActive
	} else if err != nil {
		return err
	}
	dir, err := os.Open(d.Path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

// Release removes the guard after the operation fully succeeded.
func (d Dir) Release(name string) error {
	p, err := d.file(name)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Clear is the explicit retry path; it removes one guard by exact name, never by glob.
func (d Dir) Clear(name string) error { return d.Release(name) }

// Stale lists guards present on disk, for the startup report.
func (d Dir) Stale() ([]Info, error) {
	entries, err := os.ReadDir(d.Path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := []Info{}
	for _, e := range entries {
		if !validName.MatchString(e.Name()) {
			continue
		}
		var in Info
		if err := atomicfile.ReadJSON(filepath.Join(d.Path, e.Name()), &in); err != nil {
			in = Info{Name: e.Name()} // present but unreadable still counts as active
		}
		out = append(out, in)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
