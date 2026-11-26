package leds

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// Applier is txn.Manager.Apply: lock, refuse foreign staged changes, stage, commit, reload.
type Applier interface {
	Apply(ctx context.Context, configs []string, stage func(ctx context.Context) error) error
}

// Cron is the shared crontab writer (internal/crontab.Service, ADR 0024).
type Cron interface {
	Get(ctx context.Context, owner string) ([]string, error)
	Set(ctx context.Context, owner string, lines []string) error
}

// Service reads the LEDs and owns their led sections and schedule block.
type Service struct {
	UCI      device.UCI
	Sysfs    device.LEDReader
	Apply    Applier
	Services device.Services // runs /etc/init.d/led restart (not a procd service)
	Cron     Cron
	mu       sync.Mutex
}

// Get reads sysfs, the Travo led sections and the schedule block.
func (s *Service) Get(ctx context.Context) (Status, error) {
	sysfs, err := s.Sysfs.LEDs(ctx)
	if err != nil {
		return Status{}, fmt.Errorf("leds: sysfs: %w", err)
	}
	secs, err := s.UCI.Show(ctx, config)
	if err != nil {
		return Status{}, fmt.Errorf("leds: uci: %w", err)
	}
	lines, err := s.Cron.Get(ctx, Owner)
	if err != nil {
		return Status{}, fmt.Errorf("leds: crontab: %w", err)
	}
	entries := Entries(sysfs)
	held := map[string]bool{}
	for _, sec := range Owned(secs) {
		held[sec.Name] = true
	}
	all := len(entries) > 0
	for i := range entries {
		entries[i].HeldOff = held[SectionName(entries[i].Name)]
		all = all && entries[i].HeldOff
	}
	return Status{LEDs: entries, Off: all, Schedule: ParseSchedule(lines)}, nil
}

// SetOff holds every controllable LED off (true) or returns them all to their default state
// (false). An unchanged config writes nothing and does not restart the LEDs.
func (s *Service) SetOff(ctx context.Context, off bool) (Status, error) {
	return s.change(ctx, func(secs []device.Section, sysfs []device.LED) (Plan, error) {
		if off {
			return PlanOff(secs, sysfs, false), nil
		}
		return PlanDefault(secs, false), nil
	})
}

// SetLED holds one LED off or returns it to its default state.
func (s *Service) SetLED(ctx context.Context, name string, off bool) (Status, error) {
	return s.change(ctx, func(secs []device.Section, sysfs []device.LED) (Plan, error) {
		return PlanLED(secs, sysfs, name, off)
	})
}

// night is the schedule's off (true) and on (false): it adds and removes only its own sections.
func (s *Service) night(ctx context.Context, off bool) error {
	_, err := s.change(ctx, func(secs []device.Section, sysfs []device.LED) (Plan, error) {
		if off {
			return PlanOff(secs, sysfs, true), nil
		}
		return PlanDefault(secs, true), nil
	})
	return err
}

func (s *Service) change(ctx context.Context, plan func([]device.Section, []device.LED) (Plan, error)) (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sysfs, err := s.Sysfs.LEDs(ctx)
	if err != nil {
		return Status{}, fmt.Errorf("leds: sysfs: %w", err)
	}
	secs, err := s.UCI.Show(ctx, config)
	if err != nil {
		return Status{}, fmt.Errorf("leds: uci: %w", err)
	}
	p, err := plan(secs, sysfs)
	if err != nil {
		return Status{}, err
	}
	if !p.Empty() {
		if err := s.Apply.Apply(ctx, []string{config}, func(lctx context.Context) error { return stage(lctx, s.UCI, p) }); err != nil {
			return Status{}, err
		}
		// The led script replays /var/run/led.state (each LED's state before its section
		// applied) and then applies the remaining sections: no radio or network effect.
		if err := s.Services.ServiceAction(ctx, "led", "restart"); err != nil {
			return Status{}, fmt.Errorf("leds: led restart: %w", err)
		}
	}
	return s.Get(ctx)
}

// SetSchedule replaces the night schedule block; disabled removes it.
func (s *Service) SetSchedule(ctx context.Context, sch Schedule) (Status, error) {
	lines, err := sch.Lines()
	if err != nil {
		return Status{}, err
	}
	if err := s.Cron.Set(ctx, Owner, lines); err != nil {
		return Status{}, fmt.Errorf("leds: crontab: %w", err)
	}
	return s.Get(ctx)
}

// Actions are the hook actions the schedule's jobs call.
func (s *Service) Actions() map[string]func(context.Context) error {
	return map[string]func(context.Context) error{
		ActionOff: func(ctx context.Context) error { return s.night(ctx, true) },
		ActionOn:  func(ctx context.Context) error { return s.night(ctx, false) },
	}
}

func stage(ctx context.Context, u device.UCI, p Plan) error {
	var errs []error
	for _, name := range p.Delete {
		errs = append(errs, u.DeleteSection(ctx, config, name))
	}
	for _, sec := range p.Write {
		if err := u.SetType(ctx, config, sec.Name, sec.Type); err != nil {
			return err
		}
		for _, k := range []string{"name", "sysfs", "trigger", "default", NightOption} {
			if v, ok := sec.Options[k]; ok {
				errs = append(errs, u.Set(ctx, config, sec.Name, k, v))
			}
		}
	}
	return errors.Join(errs...)
}
