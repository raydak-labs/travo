package leds

import (
	"errors"
	"fmt"

	"github.com/blackdark-org/travo-greenfield/internal/crontab"
)

const (
	// Owner is the crontab block of the night schedule.
	Owner = "leds"
	// ActionOff and ActionOn are the hook actions the block's jobs call.
	ActionOff = "leds-off"
	ActionOn  = "leds-on"
	// DefaultOffAt and DefaultOnAt are shown while no schedule is set: the block is the
	// only record of the times (ADR 0024), so a disabled schedule has none.
	DefaultOffAt = "23:00"
	DefaultOnAt  = "07:00"
)

// ErrInvalid is a schedule the block cannot hold (400).
var ErrInvalid = errors.New("invalid LED schedule")

// Schedule turns the LEDs off at OffAt and back to default at OnAt, every day, router
// local time (crond applies /etc/TZ, device-facts §8).
type Schedule struct {
	Enabled bool   `json:"enabled"`
	OffAt   string `json:"off_at" pattern:"^[0-9]{2}:[0-9]{2}$" doc:"Daily time the LEDs go off, HH:MM router local time"`
	OnAt    string `json:"on_at" pattern:"^[0-9]{2}:[0-9]{2}$" doc:"Daily time the LEDs return to default, HH:MM router local time"`
}

// Lines renders the crontab block: none when disabled.
func (s Schedule) Lines() ([]string, error) {
	off, err1 := crontab.ParseClock(s.OffAt)
	on, err2 := crontab.ParseClock(s.OnAt)
	if err := errors.Join(err1, err2); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if !s.Enabled {
		return nil, nil
	}
	if off == on {
		return nil, fmt.Errorf("%w: off and on time are the same", ErrInvalid)
	}
	return []string{
		crontab.Daily(off, crontab.HookCommand(ActionOff)),
		crontab.Daily(on, crontab.HookCommand(ActionOn)),
	}, nil
}

// ParseSchedule reads the block back. Anything but one off and one on job of Lines' shape
// (an operator's edit) reads as disabled with the default times.
func ParseSchedule(lines []string) Schedule {
	off := Schedule{OffAt: DefaultOffAt, OnAt: DefaultOnAt}
	if len(lines) != 2 {
		return off
	}
	offAt, offCmd, ok1 := crontab.ParseDaily(lines[0])
	onAt, onCmd, ok2 := crontab.ParseDaily(lines[1])
	if !ok1 || !ok2 || offCmd != crontab.HookCommand(ActionOff) || onCmd != crontab.HookCommand(ActionOn) {
		return off
	}
	return Schedule{Enabled: true, OffAt: offAt.String(), OnAt: onAt.String()}
}
