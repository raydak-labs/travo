package vpnstatus

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/vpnprofiles"
)

type Config interface {
	Show(ctx context.Context, config string) ([]device.Section, error)
}

type Selection interface {
	List() (vpnprofiles.List, error)
}

type Service struct {
	cfg     Config
	ubus    device.Ubus
	wg      device.WireGuardReader
	sel     Selection
	now     func() time.Time
	elapsed func() time.Duration

	mu              sync.Mutex
	lastGood        TunnelStatus
	lastGoodElapsed time.Duration
	hasGood         bool
}

// New keeps router wall time for observations and uses elapsed for monotonic cache expiry.
func New(cfg Config, ubus device.Ubus, wg device.WireGuardReader, sel Selection, now func() time.Time, elapsed func() time.Duration) *Service {
	return &Service{cfg: cfg, ubus: ubus, wg: wg, sel: sel, now: now, elapsed: elapsed}
}

// Status reads without changing device state; only its in-memory last-good cache mutates.
func (s *Service) Status(ctx context.Context) (TunnelStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	observation := s.observe(ctx)
	now := s.now().UTC()
	elapsed := s.elapsed()
	status, err := Classify(observation, now)
	if err != nil {
		return s.readFailure(elapsed)
	}
	s.lastGood = status
	s.lastGoodElapsed = elapsed
	s.hasGood = true
	return status, nil
}

func (s *Service) observe(ctx context.Context) Observation {
	var observation Observation
	profiles, err := s.sel.List()
	observation.Selected = profiles.SelectedID != nil
	observation.SelectionErr = err
	sections, err := s.cfg.Show(ctx, "network")
	observation.ConfigErr = err
	if err == nil {
		observation.Section = findSection(sections)
	}
	if observation.SelectionErr != nil || observation.ConfigErr != nil {
		return observation
	}
	on, err := tunnelOn(observation.Section)
	if err != nil || !on {
		return observation
	}
	s.observeUp(ctx, &observation)
	return observation
}

func (s *Service) observeUp(ctx context.Context, observation *Observation) {
	raw, err := s.ubus.Call(ctx, "network.interface.wg0", "status", nil)
	observation.NetifdErr = err
	if err != nil {
		return
	}
	var reply struct {
		Up *bool `json:"up"`
	}
	if err := json.Unmarshal(raw, &reply); err != nil {
		observation.NetifdErr = fmt.Errorf("parse netifd status: %w", err)
		return
	}
	observation.NetifdUp = reply.Up
	if reply.Up == nil || !*reply.Up {
		return
	}
	observation.Handshakes, observation.HandshakeErr = s.wg.LatestHandshakes(ctx, Interface)
}

func (s *Service) readFailure(elapsed time.Duration) (TunnelStatus, error) {
	if s.hasGood && elapsed-s.lastGoodElapsed <= MaxStale {
		stale := s.lastGood
		stale.Stale = true
		return stale, nil
	}
	return TunnelStatus{}, ErrUnavailable
}

func findSection(sections []device.Section) *device.Section {
	for _, section := range sections {
		if presentSection(&section) != nil {
			copy := section
			return &copy
		}
	}
	return nil
}
