// Package throughput samples cumulative kernel counters and publishes interface rates.
package throughput

import (
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/lifecycle"
)

const (
	Interval   = 2 * time.Second
	retention  = 10 * time.Minute
	maxSamples = 300
)

type Rate struct {
	Name             string   `json:"name"`
	RXBytesPerSecond *float64 `json:"rx_bytes_per_second" nullable:"true"`
	TXBytesPerSecond *float64 `json:"tx_bytes_per_second" nullable:"true"`
}

type sample struct {
	slot     int64
	at       time.Time
	counters device.InterfaceCounters
}

// Sampler has one worker writer and may be queried by concurrent API callers.
type Sampler struct {
	mu        sync.RWMutex
	history   map[string][]sample
	rates     []Rate
	available bool
}

func New() *Sampler {
	return &Sampler{history: make(map[string][]sample)}
}

func (s *Sampler) Run(ctx context.Context, source device.InterfaceCounterReader, now func() time.Time) {
	start := now()
	var lastSlot int64 = -1
	lifecycle.Every(Interval, func(ctx context.Context) {
		if ctx.Err() != nil {
			return
		}
		at := now()
		slot := int64(at.Sub(start) / Interval)
		if slot <= lastSlot {
			return
		}
		lastSlot = slot
		snapshot, err := source.ReadInterfaceCounters(ctx)
		s.record(slot, at, snapshot, err)
	})(ctx)
}

// Latest returns the latest successful snapshot's rates if the latest attempt succeeded.
func (s *Sampler) Latest() ([]Rate, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return slices.Clone(s.rates), s.available
}

func (s *Sampler) record(slot int64, at time.Time, snapshot map[string]device.InterfaceCounters, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune(at)
	s.available = err == nil
	if err != nil {
		return
	}

	rates := make([]Rate, 0, len(snapshot))
	for name, counters := range snapshot {
		current := sample{slot: slot, at: at, counters: counters}
		history := s.history[name]
		rate := Rate{Name: name}
		if len(history) > 0 {
			previous := history[len(history)-1]
			if seconds := at.Sub(previous.at).Seconds(); previous.slot == slot-1 && seconds > 0 {
				rx := bytesPerSecond(previous.counters.RXBytes, counters.RXBytes, seconds)
				tx := bytesPerSecond(previous.counters.TXBytes, counters.TXBytes, seconds)
				rate.RXBytesPerSecond, rate.TXBytesPerSecond = &rx, &tx
			}
		}
		if len(history) == maxSamples {
			copy(history, history[1:])
			history = history[:maxSamples-1]
		}
		s.history[name] = append(history, current)
		rates = append(rates, rate)
	}
	slices.SortFunc(rates, func(a, b Rate) int { return strings.Compare(a.Name, b.Name) })
	s.rates = rates
}

func (s *Sampler) prune(at time.Time) {
	for name, history := range s.history {
		first := 0
		for first < len(history) && at.Sub(history[first].at) > retention {
			first++
		}
		if first == len(history) {
			delete(s.history, name)
		} else if first > 0 {
			s.history[name] = history[:copy(history, history[first:])]
		}
	}
}

func bytesPerSecond(previous, current uint64, seconds float64) float64 {
	if current < previous {
		return 0
	}
	return float64(current-previous) / seconds
}
