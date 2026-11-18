package api

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	errStreamCapacity = errors.New("dashboard stream capacity reached")
	errStreamClosed   = errors.New("dashboard stream service stopped")
)

type dashboardEvent struct {
	name string
	data any
}

type dashboardEventSource struct {
	name     string
	interval time.Duration
	read     func(context.Context) (any, error)
}

type dashboardProducerRun struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

type pendingDashboardEvent struct {
	event dashboardEvent
	set   bool
}

// dashboardSubscription has one replaceable slot per source. The wake channel
// is only a hint; values never queue behind a slow subscriber.
type dashboardSubscription struct {
	service *dashboardEventService
	wake    chan struct{}
	slots   []pendingDashboardEvent
	closed  bool
}

type dashboardEventService struct {
	daemon  context.Context
	limit   int
	sources []dashboardEventSource

	mu   sync.Mutex
	subs map[*dashboardSubscription]struct{}
	run  *dashboardProducerRun
	// latest holds the current run's last event per source; a late subscriber starts with it.
	latest []pendingDashboardEvent
}

func newDashboardEventService(daemon context.Context, limit int, sources []dashboardEventSource) *dashboardEventService {
	if daemon == nil {
		daemon = context.Background()
	}
	return &dashboardEventService{daemon: daemon, limit: limit, sources: sources, subs: make(map[*dashboardSubscription]struct{})}
}

func (s *dashboardEventService) subscribe() (*dashboardSubscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.daemon.Err() != nil {
		return nil, errStreamClosed
	}
	if len(s.subs) >= s.limit {
		return nil, errStreamCapacity
	}
	sub := &dashboardSubscription{service: s, wake: make(chan struct{}, 1), slots: make([]pendingDashboardEvent, len(s.sources))}
	s.subs[sub] = struct{}{}
	if (s.run == nil || s.run.ctx.Err() != nil) && len(s.sources) > 0 {
		s.startRun()
		return sub, nil
	}
	// Joining a running stream: replay the last value per source instead of waiting a tick.
	copy(sub.slots, s.latest)
	for _, slot := range sub.slots {
		if slot.set {
			sub.wake <- struct{}{}
			break
		}
	}
	return sub, nil
}

// startRun starts one producer per source; a stopped run's values are not replayed.
// The caller holds s.mu.
func (s *dashboardEventService) startRun() {
	previous := s.run
	run := &dashboardProducerRun{}
	run.ctx, run.cancel = context.WithCancel(s.daemon)
	s.run = run
	s.latest = make([]pendingDashboardEvent, len(s.sources))
	for i, source := range s.sources {
		run.wg.Add(1)
		go s.produce(run, previous, i, source)
	}
}

func (s *dashboardEventService) produce(run, previous *dashboardProducerRun, index int, source dashboardEventSource) {
	defer run.wg.Done()
	if previous != nil {
		// Wait outside the service lock so cancellation and publication stay non-blocking.
		previous.wg.Wait()
	}
	if run.ctx.Err() != nil {
		return
	}
	ticker := time.NewTicker(source.interval)
	defer ticker.Stop()
	for {
		body, err := source.read(run.ctx)
		if run.ctx.Err() != nil {
			return
		}
		event := dashboardEvent{name: source.name, data: body}
		if err != nil {
			event = dashboardEvent{name: "source_unavailable", data: sourceUnavailableBody{Source: source.name}}
		}
		s.publish(run, index, event)
		select {
		case <-run.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// publish only swaps values and sends a bounded wake hint. In particular, no
// network write or session lookup happens while the service lock is held.
func (s *dashboardEventService) publish(run *dashboardProducerRun, index int, event dashboardEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.run != run || run.ctx.Err() != nil {
		return
	}
	s.latest[index] = pendingDashboardEvent{event: event, set: true}
	for sub := range s.subs {
		sub.slots[index] = pendingDashboardEvent{event: event, set: true}
		select {
		case sub.wake <- struct{}{}:
		default:
		}
	}
}

func (s *dashboardEventService) unsubscribe(sub *dashboardSubscription) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sub.closed {
		return
	}
	sub.closed = true
	delete(s.subs, sub)
	if len(s.subs) == 0 && s.run != nil {
		// Keep the canceled run as the drain barrier for the next first subscriber.
		s.run.cancel()
	}
}

func (s *dashboardSubscription) close() { s.service.unsubscribe(s) }

// take returns one newest pending value. Slots not yet consumed remain pending
// and are drained before the subscriber waits for another wake hint.
func (s *dashboardSubscription) take() (dashboardEvent, bool) {
	s.service.mu.Lock()
	defer s.service.mu.Unlock()
	for i := range s.slots {
		if !s.slots[i].set {
			continue
		}
		event := s.slots[i].event
		s.slots[i] = pendingDashboardEvent{}
		return event, true
	}
	return dashboardEvent{}, false
}
