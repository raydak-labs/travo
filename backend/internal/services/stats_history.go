package services

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/store"
)

const (
	statsHistoryBucket = "stats_history"
	statsHistoryKey    = "points"
	// flushEvery batches flash writes: at the default 30s sample interval one
	// write lands every ~10 minutes instead of per sample (NAND wear).
	flushEvery = 20
)

// StatsHistoryPoint is a timestamped snapshot of system stats.
type StatsHistoryPoint struct {
	Time    int64   `json:"time"` // unix timestamp
	CPU     float64 `json:"cpu"`
	Memory  float64 `json:"memory"`
	RxBytes int64   `json:"rx_bytes"`
	TxBytes int64   `json:"tx_bytes"`
}

// StatsHistoryService collects periodic system stats and keeps a ring buffer.
// With a store attached the buffer survives restarts: it is restored on
// construction and flushed every flushEvery collects plus on Stop.
type StatsHistoryService struct {
	mu         sync.RWMutex
	points     []StatsHistoryPoint
	maxLen     int
	interval   time.Duration
	checker    AlertChecker
	db         *store.Store // nil = in-memory only
	sinceFlush int
	stopCh     chan struct{}
	stopOnce   sync.Once
}

// NewStatsHistoryService creates a history service that samples every interval.
func NewStatsHistoryService(checker AlertChecker, interval time.Duration, maxPoints int) *StatsHistoryService {
	return &StatsHistoryService{
		points:   make([]StatsHistoryPoint, 0, maxPoints),
		maxLen:   maxPoints,
		interval: interval,
		checker:  checker,
		stopCh:   make(chan struct{}),
	}
}

// NewStatsHistoryServiceWithStore creates a history service whose ring buffer
// persists in db, restoring any previously flushed points (newest maxPoints).
func NewStatsHistoryServiceWithStore(checker AlertChecker, interval time.Duration, maxPoints int, db *store.Store) *StatsHistoryService {
	s := NewStatsHistoryService(checker, interval, maxPoints)
	s.db = db
	if data, err := db.Get(statsHistoryBucket, statsHistoryKey); err == nil && data != nil {
		var restored []StatsHistoryPoint
		if err := json.Unmarshal(data, &restored); err == nil {
			if len(restored) > maxPoints {
				restored = restored[len(restored)-maxPoints:]
			}
			s.points = append(s.points, restored...)
		}
	}
	return s
}

// Start begins periodic collection in the background. Call Stop to shut it down.
func (s *StatsHistoryService) Start() {
	go s.collectLoop()
}

// Stop stops the collection goroutine and flushes pending points. Safe to
// call multiple times.
func (s *StatsHistoryService) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
		s.Flush()
	})
}

// Flush persists the current ring buffer to the store (no-op without one).
func (s *StatsHistoryService) Flush() {
	if s.db == nil {
		return
	}
	s.mu.Lock()
	data, err := json.Marshal(s.points)
	s.sinceFlush = 0
	s.mu.Unlock()
	if err != nil {
		return
	}
	_ = s.db.Put(statsHistoryBucket, statsHistoryKey, data)
}

func (s *StatsHistoryService) collectLoop() {
	// Collect immediately on start
	s.collect()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.collect()
		case <-s.stopCh:
			return
		}
	}
}

func (s *StatsHistoryService) collect() {
	stats, err := s.checker.GetSystemStats()
	if err != nil {
		return
	}

	var rxTotal, txTotal int64
	for _, n := range stats.Network {
		rxTotal += n.RxBytes
		txTotal += n.TxBytes
	}

	point := StatsHistoryPoint{
		Time:    time.Now().Unix(),
		CPU:     stats.CPU.UsagePercent,
		Memory:  stats.Memory.UsagePercent,
		RxBytes: rxTotal,
		TxBytes: txTotal,
	}

	s.mu.Lock()
	if len(s.points) >= s.maxLen {
		// Shift ring buffer
		copy(s.points, s.points[1:])
		s.points[len(s.points)-1] = point
	} else {
		s.points = append(s.points, point)
	}
	s.sinceFlush++
	needFlush := s.db != nil && s.sinceFlush >= flushEvery
	s.mu.Unlock()

	if needFlush {
		s.Flush()
	}
}

// GetHistory returns all collected data points (oldest first).
func (s *StatsHistoryService) GetHistory() []StatsHistoryPoint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]StatsHistoryPoint, len(s.points))
	copy(out, s.points)
	return out
}

// GetHistorySince returns points newer than the given unix timestamp.
func (s *StatsHistoryService) GetHistorySince(since int64) []StatsHistoryPoint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []StatsHistoryPoint
	for _, p := range s.points {
		if p.Time > since {
			out = append(out, p)
		}
	}
	if out == nil {
		return []StatsHistoryPoint{}
	}
	return out
}
