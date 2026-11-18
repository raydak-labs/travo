// Package wifischedule turns access points on and off at set times. BusyBox crond owns
// the clock (it reads the router time zone); its lines call the daemon's hook socket
// (ADR 0024), which applies the toggle through the AP transaction (docs/adr/0027-wifi-schedule.md).
package wifischedule

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/crontab"
)

const maxRules = 16

var (
	// ErrInvalid is a rule set the API refuses with 400.
	ErrInvalid     = errors.New("invalid WiFi schedule")
	idPattern      = regexp.MustCompile(`^[0-9a-f]{8}$`)
	sectionPattern = regexp.MustCompile(`^[A-Za-z0-9_]{1,64}$`)
	timePattern    = regexp.MustCompile(`^([01][0-9]|2[0-3]):([0-5][0-9])$`)
)

// Action is what a trigger asks for.
type Action string

const (
	On  Action = "on"
	Off Action = "off"
)

// Rule turns one AP off and back on, on the listed weekdays (0 = Sunday, as in cron).
type Rule struct {
	ID                 string `json:"id" doc:"8 hex characters; empty on PUT assigns one"`
	Section            string `json:"section" doc:"UCI section of the access point"`
	Days               []int  `json:"days" nullable:"false" minItems:"1" maxItems:"7" doc:"Weekdays, 0 = Sunday; both times apply on each"`
	Off                string `json:"off" doc:"Router local time HH:MM to turn the AP off"`
	On                 string `json:"on" doc:"Router local time HH:MM to turn the AP back on"`
	Enabled            bool   `json:"enabled"`
	AcknowledgeLockout bool   `json:"acknowledge_lockout" doc:"Allow turning off the last non-guest AP (invariants §2)"`
}

// Rules is the stored and exchanged shape (GET and PUT are symmetric).
type Rules struct {
	Rules []Rule `json:"rules" nullable:"false" maxItems:"16"`
}

// Validate checks every field; ids must be unique (empty ids are assigned later).
func (r Rules) Validate() error {
	if len(r.Rules) > maxRules {
		return fmt.Errorf("%w: at most %d rules", ErrInvalid, maxRules)
	}
	seen := map[string]bool{}
	for _, rule := range r.Rules {
		if err := rule.validate(); err != nil {
			return err
		}
		if rule.ID != "" && seen[rule.ID] {
			return fmt.Errorf("%w: duplicate id %s", ErrInvalid, rule.ID)
		}
		seen[rule.ID] = true
	}
	return nil
}

func (r Rule) validate() error {
	switch {
	case r.ID != "" && !idPattern.MatchString(r.ID):
		return fmt.Errorf("%w: id", ErrInvalid)
	case !sectionPattern.MatchString(r.Section):
		return fmt.Errorf("%w: section", ErrInvalid)
	case !timePattern.MatchString(r.Off) || !timePattern.MatchString(r.On):
		return fmt.Errorf("%w: times must be HH:MM", ErrInvalid)
	case r.Off == r.On:
		return fmt.Errorf("%w: off and on times are equal", ErrInvalid)
	case len(r.Days) == 0 || len(r.Days) > 7:
		return fmt.Errorf("%w: days", ErrInvalid)
	}
	days := slices.Clone(r.Days)
	slices.Sort(days)
	if days[0] < 0 || days[len(days)-1] > 6 || len(slices.Compact(days)) != len(r.Days) {
		return fmt.Errorf("%w: days must be distinct values 0..6", ErrInvalid)
	}
	return nil
}

// Owner is this feature's block in root's crontab (internal/crontab).
const Owner = "wifi-schedule"

// ActionName is the hook action a rule's cron line calls (ADR 0024).
func ActionName(id string, a Action) string { return "wifi-schedule-" + id + "-" + string(a) }

// cronLine renders one action of a rule: the weekdays at the rule's time, calling the daemon.
func cronLine(r Rule, a Action) string {
	at := r.On
	if a == Off {
		at = r.Off
	}
	hour, _ := strconv.Atoi(at[:2])
	minute, _ := strconv.Atoi(at[3:])
	days := slices.Clone(r.Days)
	slices.Sort(days)
	list := make([]string, len(days))
	for i, d := range days {
		list[i] = strconv.Itoa(d)
	}
	return fmt.Sprintf("%d %d * * %s %s", minute, hour, strings.Join(list, ","), crontab.HookCommand(ActionName(r.ID, a)))
}

// Lines renders the crontab block for the enabled rules: off then on per rule.
func Lines(r Rules) []string {
	lines := []string{}
	for _, rule := range r.Rules {
		if rule.Enabled {
			lines = append(lines, cronLine(rule, Off), cronLine(rule, On))
		}
	}
	return lines
}
