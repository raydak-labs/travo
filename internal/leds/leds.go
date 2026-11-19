// Package leds reads the router's LEDs from sysfs and turns them all off or back to their
// default state through UCI `system` led sections that Travo owns by name
// (`travo_led_<name>`), applied with `/etc/init.d/led restart`. The night schedule is one
// crontab block (internal/crontab, ADR 0024) whose jobs call the daemon's `leds-off` and
// `leds-on` hook actions.
package leds

import (
	"slices"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

const (
	config = "system"
	// SectionPrefix marks the led sections Travo owns; other led sections are never touched.
	SectionPrefix = "travo_led_"
)

// Entry is one controllable LED.
type Entry struct {
	Name    string `json:"name" doc:"Kernel LED name (/sys/class/leds)"`
	Trigger string `json:"trigger" doc:"Active kernel trigger, e.g. none, heartbeat, netdev"`
	On      bool   `json:"on" doc:"Lit now, or driven by a trigger other than none"`
}

// Status is what the API reads.
type Status struct {
	LEDs     []Entry  `json:"leds" nullable:"false"`
	Off      bool     `json:"off" doc:"Travo holds the LEDs off (its led sections are committed)"`
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

// OffSection is the led section that holds one LED dark: no trigger, brightness 0.
func OffSection(led string) device.Section {
	return device.Section{Name: SectionName(led), Type: "led", Options: map[string]string{
		"name": led, "sysfs": led, "trigger": "none", "default": "0",
	}}
}

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

// Empty is true when the config already matches.
func (p Plan) Empty() bool { return len(p.Write) == 0 && len(p.Delete) == 0 }

// PlanOff holds every controllable LED dark; Travo sections of LEDs that are gone are
// deleted. Sections that already match are left alone.
func PlanOff(secs []device.Section, sysfs []device.LED) Plan {
	var p Plan
	want := map[string]bool{}
	have := map[string]device.Section{}
	for _, s := range Owned(secs) {
		have[s.Name] = s
	}
	for _, e := range Entries(sysfs) {
		s := OffSection(e.Name)
		if want[s.Name] {
			continue // two LED names that map to one section name: the first wins
		}
		want[s.Name] = true
		if cur, ok := have[s.Name]; !ok || !sameOptions(cur, s) {
			p.Write = append(p.Write, s)
		}
	}
	for _, s := range Owned(secs) {
		if !want[s.Name] {
			p.Delete = append(p.Delete, s.Name)
		}
	}
	return p
}

// PlanDefault deletes every Travo led section, so the LEDs return to the firmware's state.
func PlanDefault(secs []device.Section) Plan {
	var p Plan
	for _, s := range Owned(secs) {
		p.Delete = append(p.Delete, s.Name)
	}
	return p
}

// sameOptions: cur carries every option of want (an operator's extra option is kept).
func sameOptions(cur, want device.Section) bool {
	for k, v := range want.Options {
		if cur.Options[k] != v {
			return false
		}
	}
	return true
}
