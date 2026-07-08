package services

import (
	"sync"
	"time"
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
type StatsHistoryService struct {
	mu       sync.RWMutex
	points   []StatsHistoryPoint
	maxLen   int
	interval time.Duration
	checker  AlertChecker
	stopCh   chan struct{}
	stopOnce sync.Once
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

// Start begins periodic collection in the background. Call Stop to shut it down.
func (s *StatsHistoryService) Start() {
	go s.collectLoop()
}

// Stop stops the collection goroutine. Safe to call multiple times.
func (s *StatsHistoryService) Stop() {
	s.stopOnce.Do(func() { close(s.stopCh) })
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
	defer s.mu.Unlock()

	if len(s.points) >= s.maxLen {
		// Shift ring buffer
		copy(s.points, s.points[1:])
		s.points[len(s.points)-1] = point
	} else {
		s.points = append(s.points, point)
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
