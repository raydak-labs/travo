// Package statshistory keeps a bounded history of uplink traffic, client count and uptime.
// Samples live in a RAM ring; the ring reaches flash only in rare, bounded flushes
// (docs/adr/0026-stats-history.md, invariants §5: no per-sample flash writes).
package statshistory

import (
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	// Interval is the bucket width: one sample, one point.
	Interval = 10 * time.Minute
	// Retention is how far back points are kept; older points are dropped on record.
	Retention = 7 * 24 * time.Hour
	// FlushEvery bounds flash writes: at most one flush per period, plus one on shutdown.
	FlushEvery = 6 * time.Hour
	maxPoints  = int(Retention / Interval)
)

// Uplinks are the logical candidate uplinks (invariants §7), by name, never by index.
var Uplinks = []string{"wan", "wwan", "usbtether"}

// UplinkBytes is the traffic one uplink moved during a bucket.
type UplinkBytes struct {
	Name    string `json:"name" enum:"wan,wwan,usbtether"`
	RXBytes uint64 `json:"rx_bytes"`
	TXBytes uint64 `json:"tx_bytes"`
}

// Point is one bucket. Uplinks lists only uplinks with a provable delta (same device,
// no counter reset, no reboot); an uplink that was down or changed device is absent.
type Point struct {
	At            int64         `json:"at" doc:"Router wall clock at the end of the bucket, unix seconds"`
	UptimeSeconds int64         `json:"uptime_seconds"`
	Clients       *int          `json:"clients" nullable:"true" doc:"Distinct LAN and guest neighbour MACs; null when unreadable"`
	Uplinks       []UplinkBytes `json:"uplinks" nullable:"false"`
}

// Counter is a cumulative kernel counter of an uplink's layer-3 device.
type Counter struct {
	Device string
	RX, TX uint64
}

// Observation is one raw reading of the device.
type Observation struct {
	At      time.Time
	Uptime  time.Duration
	Clients *int
	Uplinks map[string]Counter // logical uplink name → its device counters
}

// Persistence is ok while flushes may run, degraded after a torn or unwritable file.
type Persistence string

const (
	PersistenceOK       Persistence = "ok"
	PersistenceDegraded Persistence = "degraded"
)

// Reader is the read side other pages use (clients, network).
type Reader interface {
	Points(since time.Time) ([]Point, Persistence)
}

// History is the RAM ring. One sampler writes; API callers read concurrently.
type History struct {
	mu          sync.Mutex
	floor       time.Time // clock-plausibility floor (build time)
	points      []Point
	last        *Observation
	persistence Persistence
	dirty       bool
}

// NewHistory starts with loaded points (oldest first) and their persistence state.
func NewHistory(floor time.Time, points []Point, persistence Persistence) *History {
	return &History{floor: floor, points: points, persistence: persistence}
}

// Record turns an observation into a point. A clock below the floor or behind the newest
// point records nothing and restarts the delta chain: wall time is not trustworthy then.
func (h *History) Record(o Observation) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if o.At.Before(h.floor) || (len(h.points) > 0 && o.At.Unix() <= h.points[len(h.points)-1].At) {
		h.last = nil
		return
	}
	h.points = append(h.points, pointFrom(h.last, o))
	h.last = &o
	h.points = prune(h.points, o.At)
	h.dirty = true
}

// Skip drops the delta chain after a failed reading, so the next point never spans two buckets.
func (h *History) Skip() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.last = nil
}

// Points returns the points at or after since, oldest first.
func (h *History) Points(since time.Time) ([]Point, Persistence) {
	h.mu.Lock()
	defer h.mu.Unlock()
	cut := since.Unix()
	i, _ := slices.BinarySearchFunc(h.points, cut, func(p Point, at int64) int {
		switch {
		case p.At < at:
			return -1
		case p.At > at:
			return 1
		}
		return 0
	})
	return slices.Clone(h.points[i:]), h.persistence
}

// pointFrom computes the bucket deltas against the previous observation.
func pointFrom(prev *Observation, cur Observation) Point {
	p := Point{At: cur.At.Unix(), UptimeSeconds: int64(cur.Uptime.Seconds()), Clients: cur.Clients, Uplinks: []UplinkBytes{}}
	if prev == nil || cur.Uptime < prev.Uptime {
		return p // first sample or a reboot: counters restarted
	}
	for name, c := range cur.Uplinks {
		old, ok := prev.Uplinks[name]
		if !ok || old.Device != c.Device || c.RX < old.RX || c.TX < old.TX {
			continue
		}
		p.Uplinks = append(p.Uplinks, UplinkBytes{Name: name, RXBytes: c.RX - old.RX, TXBytes: c.TX - old.TX})
	}
	slices.SortFunc(p.Uplinks, func(a, b UplinkBytes) int { return strings.Compare(a.Name, b.Name) })
	return p
}

// prune drops points older than Retention and caps the ring at maxPoints.
func prune(points []Point, now time.Time) []Point {
	cut := now.Add(-Retention).Unix()
	first := 0
	for first < len(points) && points[first].At < cut {
		first++
	}
	if n := len(points) - first; n > maxPoints {
		first += n - maxPoints
	}
	if first == 0 {
		return points
	}
	return points[:copy(points, points[first:])]
}
