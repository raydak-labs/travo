package crontab

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

// File reads and atomically replaces root's crontab. An absent file reads as "".
type File interface {
	ReadCrontab(context.Context) (string, error)
	WriteCrontab(context.Context, string) error
}

// Applier is txn.Manager.Apply: lock, refuse foreign staged changes, stage, commit, reload.
type Applier interface {
	Apply(ctx context.Context, configs []string, stage func(ctx context.Context) error) error
}

// LogLevel is BusyBox crond's `-l`: 9 keeps per-job start lines out of the error log
// (device-facts §14, invariants §15). An operator's own setting is kept.
const LogLevel = "9"

// Service is the one writer of root's crontab in the daemon. Every feature that schedules
// a job goes through Set, so two features never overwrite each other's block.
type Service struct {
	File     File
	UCI      device.UCI
	Apply    Applier
	Services device.Services
	mu       sync.Mutex
}

// Get returns the owner's job lines.
func (s *Service) Get(ctx context.Context, owner string) ([]string, error) {
	content, err := s.File.ReadCrontab(ctx)
	if err != nil {
		return nil, err
	}
	return Lines(content, owner), nil
}

// Set replaces the owner's block with lines (none removes it). On a change it makes sure
// crond logs at LogLevel, writes the file and restarts cron, which also starts crond when
// the crontab was empty before (its init script skips an empty /etc/crontabs).
func (s *Service) Set(ctx context.Context, owner string, lines []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	old, err := s.File.ReadCrontab(ctx)
	if err != nil {
		return fmt.Errorf("crontab read: %w", err)
	}
	next, err := Replace(old, owner, lines)
	if err != nil {
		return err
	}
	if next == old {
		return nil
	}
	if len(lines) > 0 {
		if err := s.ensureLogLevel(ctx); err != nil {
			return fmt.Errorf("cron log level: %w", err)
		}
	}
	if err := s.File.WriteCrontab(ctx, next); err != nil {
		return fmt.Errorf("crontab write: %w", err)
	}
	if err := s.Services.ServiceAction(ctx, "cron", "restart"); err != nil {
		return fmt.Errorf("cron restart: %w", err)
	}
	return nil
}

// ensureLogLevel sets system.@system[0].cronloglevel only when it is unset.
func (s *Service) ensureLogLevel(ctx context.Context) error {
	secs, err := s.UCI.Show(ctx, "system")
	if err != nil {
		return err
	}
	name := ""
	for _, sec := range secs {
		if sec.Type == "system" {
			if sec.Options["cronloglevel"] != "" {
				return nil
			}
			name = sec.Name
			break
		}
	}
	if name == "" {
		return errors.New("no system section")
	}
	return s.Apply.Apply(ctx, []string{"system"}, func(ctx context.Context) error {
		return s.UCI.Set(ctx, "system", name, "cronloglevel", LogLevel)
	})
}
