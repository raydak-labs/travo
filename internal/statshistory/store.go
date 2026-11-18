package statshistory

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/atomicfile"
)

const fileVersion = 1

// file is the persisted shape: compact JSON, oldest point first.
type file struct {
	Version         int     `json:"version"`
	IntervalSeconds int64   `json:"interval_seconds"`
	Points          []Point `json:"points"`
}

// Store is the History plus its file. Only Flush and Reset write; both are rare.
type Store struct {
	*History
	path   string
	now    func() time.Time // monotonic source for the flush cadence
	write  func(path string, data []byte, perm os.FileMode) error
	remove func(path string) error
	flushd time.Time
}

// Open loads path. Absent is an empty history. A torn, foreign or unreadable file fails
// closed: collection runs in RAM, persistence is degraded and the file is never overwritten
// until the explicit Reset.
func Open(path string, floor time.Time, now func() time.Time) *Store {
	points, persistence := load(path)
	return &Store{History: NewHistory(floor, points, persistence), path: path, now: now,
		write: atomicfile.Write, remove: os.Remove, flushd: now()}
}

func load(path string) ([]Point, Persistence) {
	var f file
	err := atomicfile.ReadJSON(path, &f)
	if errors.Is(err, os.ErrNotExist) {
		return []Point{}, PersistenceOK
	}
	if err == nil {
		err = validate(f)
	}
	if err != nil {
		slog.Warn("stats history unreadable: persistence degraded until reset", "path", path, "err", err)
		return []Point{}, PersistenceDegraded
	}
	return f.Points, PersistenceOK
}

func validate(f file) error {
	if f.Version != fileVersion || f.IntervalSeconds != int64(Interval/time.Second) || f.Points == nil {
		return fmt.Errorf("stats history: unsupported version or interval")
	}
	for i, p := range f.Points {
		if i > 0 && p.At <= f.Points[i-1].At {
			return fmt.Errorf("stats history: points out of order")
		}
		if p.Uplinks == nil {
			return fmt.Errorf("stats history: point without uplinks")
		}
		for _, u := range p.Uplinks {
			if !slices.Contains(Uplinks, u.Name) {
				return fmt.Errorf("stats history: unknown uplink %q", u.Name)
			}
		}
	}
	return nil
}

// MaybeFlush writes when there is news and FlushEvery passed since the last write.
func (s *Store) MaybeFlush() {
	if s.now().Sub(s.flushd) >= FlushEvery {
		s.Flush()
	}
}

// Flush writes the ring if it changed. A write failure degrades persistence (visible in the
// API) and stops further writes: a full or failing flash is not retried every tick.
func (s *Store) Flush() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.flushd = s.now()
	if !s.dirty || s.persistence != PersistenceOK {
		return
	}
	b, err := json.Marshal(file{Version: fileVersion, IntervalSeconds: int64(Interval / time.Second), Points: s.points})
	if err == nil {
		err = s.write(s.path, append(b, '\n'), 0o600)
	}
	if err != nil {
		slog.Warn("stats history flush failed: persistence degraded until reset", "err", err)
		s.persistence = PersistenceDegraded
		return
	}
	s.dirty = false
}

// Reset is the explicit retry path for degraded persistence: it drops RAM and file.
func (s *Store) Reset() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	s.points, s.last, s.dirty = []Point{}, nil, false
	s.persistence = PersistenceOK
	return nil
}
