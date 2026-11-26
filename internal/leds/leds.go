// Package leds reads the router's LEDs from sysfs and holds single LEDs, or all of them, off
// through UCI `system` led sections that Travo owns by name (`travo_led_<name>`), applied
// with `/etc/init.d/led restart`; deleting a section returns that LED to the firmware's
// state. The night schedule is one crontab block (internal/crontab, ADR 0024) whose jobs call
// the daemon's `leds-off` and `leds-on` hook actions. Night mode marks the sections it adds
// (NightOption), so the morning removes only those and an LED the user switched off stays off.
package leds

import (
	"errors"
	"slices"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

const (
	config = "system"
	// SectionPrefix marks the led sections Travo owns; other led sections are never touched.
	SectionPrefix = "travo_led_"
	// NightOption marks a section night mode added; the led init script ignores it.
	NightOption = "travo_night"
)

// ErrUnknownLED: the name is not a controllable LED of this router.
var ErrUnknownLED = errors.New("no such controllable LED")

// Entry is one controllable LED.
type Entry struct {
	Name    string `json:"name" doc:"Kernel LED name (/sys/class/leds)"`
	Trigger string `json:"trigger" doc:"Active kernel trigger, e.g. none, heartbeat, netdev"`
	On      bool   `json:"on" doc:"Lit now, or driven by a trigger other than none"`
	HeldOff bool   `json:"held_off" doc:"A Travo led section holds this LED off"`
}

// Status is what the API reads.
type Status struct {
	LEDs     []Entry  `json:"leds" nullable:"false"`
	Off      bool     `json:"off" doc:"Travo holds every controllable LED off"`
	Schedule Schedule `json:"schedule"`
}

// Controllable reports whether Travo may switch the LED: storage activity LEDs (`mmc*`)
// stay with the kernel.
func Controllable(name string) bool { return !strings.HasPrefix(name, "mmc") }

// ActiveTrigger is the bracketed word of a sysfs `trigger` file; "" when none is marked.
func ActiveTrigger(raw string) string {
	for f := range strings.FieldsSeq(raw) {
		if strings.HasPrefix(f, "[") && strings.HasSuffix(f, "]") {
			return strings.Trim(f, "[]")
		}
	}
	return ""
}

// Entries maps the sysfs listing to controllable LEDs, sorted by name.
func Entries(sysfs []device.LED) []Entry {
	out := []Entry{}
	for _, l := range sysfs {
		if !Controllable(l.Name) {
			continue
		}
		t := ActiveTrigger(l.Trigger)
		out = append(out, Entry{Name: l.Name, Trigger: t, On: l.Brightness > 0 || (t != "" && t != "none")})
	}
	slices.SortFunc(out, func(a, b Entry) int { return strings.Compare(a.Name, b.Name) })
	return out
}

// SectionName is the Travo-owned led section for an LED: characters UCI does not allow
// in a section name become `_`.
func SectionName(led string) string {
	var b strings.Builder
	b.WriteString(SectionPrefix)
	for _, r := range led {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

// OffSection is the led section that holds one LED dark: no trigger, brightness 0. A night
// section carries NightOption so the morning can tell it from the user's own choice.
func OffSection(led string, night bool) device.Section {
	opts := map[string]string{"name": led, "sysfs": led, "trigger": "none", "default": "0"}
	if night {
		opts[NightOption] = "1"
	}
	return device.Section{Name: SectionName(led), Type: "led", Options: opts}
}

func isNight(s device.Section) bool { return s.Options[NightOption] == "1" }

// Owned returns the committed Travo led sections, in config order.
func Owned(secs []device.Section) []device.Section {
	var out []device.Section
	for _, s := range secs {
		if s.Type == "led" && strings.HasPrefix(s.Name, SectionPrefix) {
			out = append(out, s)
		}
	}
	return out
}

// Plan is the staged change: sections to write and Travo sections to delete.
type Plan struct {
	Write  []device.Section
	Delete []string
}

// rewrite writes s; an existing section is deleted first (stage runs deletes before writes),
// so no old option such as a night mark survives.
func (p *Plan) rewrite(exists bool, s device.Section) {
	if exists {
		p.Delete = append(p.Delete, s.Name)
	}
	p.Write = append(p.Write, s)
}

// Empty is true when the config already matches.
func (p Plan) Empty() bool { return len(p.Write) == 0 && len(p.Delete) == 0 }

// PlanOff holds every controllable LED dark; Travo sections of LEDs that are gone are
// deleted. Sections that already match are left alone. Night mode (night=true) keeps the
// user's own sections and marks only the ones it adds.
func PlanOff(secs []device.Section, sysfs []device.LED, night bool) Plan {
	var p Plan
	want := map[string]bool{}
	have := map[string]device.Section{}
	for _, s := range Owned(secs) {
		have[s.Name] = s
	}
	for _, e := range Entries(sysfs) {
		s := OffSection(e.Name, night)
		if want[s.Name] {
			continue // two LED names that map to one section name: the first wins
		}
		want[s.Name] = true
		cur, ok := have[s.Name]
		if night && ok {
			continue // already dark, by the user or by an earlier night
		}
		if !ok || !sameOptions(cur, s) {
			p.rewrite(ok, s)
		}
	}
	for _, s := range Owned(secs) {
		if !want[s.Name] {
			p.Delete = append(p.Delete, s.Name)
		}
	}
	return p
}

// PlanDefault deletes Travo led sections, so those LEDs return to the firmware's state:
// all of them, or with nightOnly only the ones night mode added.
func PlanDefault(secs []device.Section, nightOnly bool) Plan {
	var p Plan
	for _, s := range Owned(secs) {
		if !nightOnly || isNight(s) {
			p.Delete = append(p.Delete, s.Name)
		}
	}
	return p
}

// PlanLED holds one LED off (a plain section, replacing a night mark) or returns it to the
// firmware's state (no section).
func PlanLED(secs []device.Section, sysfs []device.LED, name string, off bool) (Plan, error) {
	if !slices.ContainsFunc(Entries(sysfs), func(e Entry) bool { return e.Name == name }) {
		return Plan{}, ErrUnknownLED
	}
	want := OffSection(name, false)
	var cur *device.Section
	for _, s := range Owned(secs) {
		if s.Name == want.Name {
			cur = &s
		}
	}
	switch {
	case off && (cur == nil || !sameOptions(*cur, want)):
		var p Plan
		p.rewrite(cur != nil, want)
		return p, nil
	case !off && cur != nil:
		return Plan{Delete: []string{want.Name}}, nil
	}
	return Plan{}, nil
}

// sameOptions: cur carries every option of want (an operator's extra option is kept) and
// the same night mark.
func sameOptions(cur, want device.Section) bool {
	if isNight(cur) != isNight(want) {
		return false
	}
	for k, v := range want.Options {
		if cur.Options[k] != v {
			return false
		}
	}
	return true
}
