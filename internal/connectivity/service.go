// Package connectivity reports kernel uplink and on-demand internet reachability.
package connectivity

import (
	"context"
	"sync"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
)

type Internet string

const (
	Online  Internet = "online"
	Offline Internet = "offline"
	Captive Internet = "captive"
	Unknown Internet = "unknown"
)

type Status struct {
	ActiveInterface *string  `json:"active_interface" nullable:"true"`
	Carrier         *bool    `json:"carrier" nullable:"true"`
	Internet        Internet `json:"internet" enum:"online,offline,captive,unknown"`
	// PortalURL is where a captive portal redirected the probe (http or https only).
	PortalURL *string `json:"portal_url" nullable:"true" doc:"Captive portal login URL; null unless internet is captive"`
}

type Probe func(context.Context) Result

type flight struct {
	done   chan struct{}
	result Result
}

// Service keeps only the current route generation's completed or pending probe.
type Service struct {
	daemon context.Context
	routes device.DefaultRouteReader
	ubus   device.Ubus
	probe  Probe
	now    func() time.Time

	observe  chan struct{} // serialize kernel observations, not their probes
	mu       sync.Mutex
	route    device.IPv4DefaultRoute
	selected bool
	current  *flight
	expires  time.Time
}

func New(daemon context.Context, routes device.DefaultRouteReader, ubus device.Ubus, probe Probe, now func() time.Time) *Service {
	return &Service{daemon: daemon, routes: routes, ubus: ubus, probe: probe, now: now, observe: make(chan struct{}, 1)}
}

func (s *Service) Status(ctx context.Context) Status {
	route, pending, initial := s.observeRoute(ctx)
	if pending == nil {
		return Status{Internet: initial}
	}
	carrier := s.carrier(ctx, route.Device)
	st := Status{ActiveInterface: &route.Device, Carrier: carrier, Internet: initial}
	select {
	case <-ctx.Done():
	case <-s.daemon.Done():
	case <-pending.done:
		st.Internet = pending.result.Internet
		if st.Internet == Captive && pending.result.Portal != "" {
			portal := pending.result.Portal
			st.PortalURL = &portal
		}
	}
	return st
}

func (s *Service) observeRoute(ctx context.Context) (device.IPv4DefaultRoute, *flight, Internet) {
	select {
	case s.observe <- struct{}{}:
		defer func() { <-s.observe }()
	case <-ctx.Done():
		return device.IPv4DefaultRoute{}, nil, Unknown
	case <-s.daemon.Done():
		return device.IPv4DefaultRoute{}, nil, Unknown
	}
	if ctx.Err() != nil || s.daemon.Err() != nil {
		return device.IPv4DefaultRoute{}, nil, Unknown
	}
	route, selected, err := s.routes.DefaultIPv4Route(ctx)
	if ctx.Err() != nil || s.daemon.Err() != nil {
		return route, nil, Unknown
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	selected = selected && err == nil
	if selected != s.selected || route != s.route {
		s.route, s.selected = route, selected
		s.current, s.expires = nil, time.Time{}
	}
	if err != nil {
		return route, nil, Unknown
	}
	if !selected {
		return route, nil, Offline
	}
	if s.current == nil || (!s.expires.IsZero() && !s.now().Before(s.expires)) {
		s.expires = time.Time{}
		s.current = &flight{done: make(chan struct{})}
		go s.runProbe(s.current)
	}
	return route, s.current, Unknown
}

func (s *Service) carrier(ctx context.Context, iface string) *bool {
	raw, err := s.ubus.Call(ctx, "network.device", "status", map[string]string{"name": iface})
	if err != nil {
		return nil
	}
	link, err := facts.ParseDeviceStatus(raw)
	if err != nil {
		return nil
	}
	return &link.Carrier
}

func (s *Service) runProbe(f *flight) {
	f.result = s.probe(s.daemon)
	s.mu.Lock()
	if s.current == f {
		s.expires = s.now().Add(2 * time.Second)
	}
	close(f.done)
	s.mu.Unlock()
}

// Invalidate drops the current probe generation after a DNS apply attempt.
func (s *Service) Invalidate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.current, s.expires = nil, time.Time{}
}
