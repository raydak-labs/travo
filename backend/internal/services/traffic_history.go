package services

import (
	"sync"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

// TrafficHistoryDefaultPoints is the per-interface ring capacity: 300 samples
// at the 2s WebSocket broadcast tick is the last ~10 minutes of traffic, which
// is what the dashboard chart paints on page load.
//
// The cap is deliberately by COUNT, not by age: a broadcast loop that stalls
// (or a clock that jumps backwards) then still cannot grow the buffer without
// bound. With the four interfaces readNetworkStats monitors (br-lan, wwan0,
// wg0, eth0) the worst case is 4 x 300 = 1200 small structs, roughly 40 KB --
// negligible on a ~128 MB router.
const TrafficHistoryDefaultPoints = 300

// TrafficHistoryPoint is one cumulative RX/TX sample for a single interface.
type TrafficHistoryPoint struct {
	Time    int64  `json:"t"` // unix timestamp, seconds
	IfName  string `json:"ifname"`
	RxBytes int64  `json:"rx_bytes"`
	TxBytes int64  `json:"tx_bytes"`
}

// TrafficHistoryService keeps a bounded in-memory ring of per-interface traffic
// samples so a newly loaded dashboard has something to draw before its first
// WebSocket push arrives. Samples are appended by the WebSocket broadcast tick
// whether or not any client is connected.
type TrafficHistoryService struct {
	mu     sync.RWMutex
	points []TrafficHistoryPoint // oldest first, insertion ordered
	counts map[string]int        // per-interface retained count
	maxLen int
}

// NewTrafficHistoryService creates an in-memory history keeping maxPoints
// samples per interface.
func NewTrafficHistoryService(maxPoints int) *TrafficHistoryService {
	return &TrafficHistoryService{
		points: make([]TrafficHistoryPoint, 0, maxPoints),
		counts: make(map[string]int),
		maxLen: maxPoints,
	}
}

// Append records one sample per reported interface, stamped with the current
// time, dropping that interface's oldest point once it is at capacity.
func (s *TrafficHistoryService) Append(stats []models.NetworkInterfaceStats) {
	s.appendAt(stats, time.Now().Unix())
}

// appendAt is Append with an injectable timestamp.
func (s *TrafficHistoryService) appendAt(stats []models.NetworkInterfaceStats, now int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, n := range stats {
		s.points = append(s.points, TrafficHistoryPoint{
			Time:    now,
			IfName:  n.Interface,
			RxBytes: n.RxBytes,
			TxBytes: n.TxBytes,
		})
		s.counts[n.Interface]++
		s.evictOldest(n.Interface)
	}
}

// evictOldest drops the oldest retained point of one interface once it is over
// capacity. Points are kept oldest-first, so the first match is the one to go.
// Caller must hold the lock.
func (s *TrafficHistoryService) evictOldest(iface string) {
	if s.counts[iface] <= s.maxLen {
		return
	}
	for i, p := range s.points {
		if p.IfName != iface {
			continue
		}
		s.points = append(s.points[:i], s.points[i+1:]...)
		s.counts[iface]--
		return
	}
}

// History returns the retained points, oldest first.
func (s *TrafficHistoryService) History() []TrafficHistoryPoint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]TrafficHistoryPoint, len(s.points))
	copy(out, s.points)
	return out
}

// RetainedSeconds is the wall-clock span the retained points actually cover.
// It is 0 until two points exist, and is reported by the API so a client can
// tell a short history from a full one.
func (s *TrafficHistoryService) RetainedSeconds() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.points) < 2 {
		return 0
	}
	return s.points[len(s.points)-1].Time - s.points[0].Time
}
