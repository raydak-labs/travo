package datausage

import (
	"cmp"
	"slices"
	"strings"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/statshistory"
)

// Period is a range of days in the router's timezone, end exclusive, with the traffic per
// uplink in it.
type Period struct {
	Start   string                     `json:"start" doc:"First day in the router's timezone, YYYY-MM-DD"`
	End     string                     `json:"end" doc:"Day after the last, YYYY-MM-DD"`
	Uplinks []statshistory.UplinkBytes `json:"uplinks" nullable:"false"`
}

// BillingPeriod is the period holding now that starts on startDay (1..28) of a month in loc.
func BillingPeriod(now time.Time, startDay int, loc *time.Location) (start, end time.Time) {
	now = now.In(loc)
	start = time.Date(now.Year(), now.Month(), startDay, 0, 0, 0, 0, loc)
	if now.Day() < startDay {
		start = start.AddDate(0, -1, 0)
	}
	return start, start.AddDate(0, 1, 0)
}

// Today is the day in loc holding now.
func Today(now time.Time, loc *time.Location) (start, end time.Time) {
	now = now.In(loc)
	start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	return start, start.AddDate(0, 0, 1)
}

// Sum totals the day totals in [start, end) per uplink, sorted by name.
func Sum(days []statshistory.Day, start, end time.Time) Period {
	p := Period{Start: start.Format(statshistory.DateLayout), End: end.Format(statshistory.DateLayout), Uplinks: []statshistory.UplinkBytes{}}
	for _, d := range days {
		if d.Date < p.Start || d.Date >= p.End {
			continue
		}
		for _, u := range d.Uplinks {
			i := slices.IndexFunc(p.Uplinks, func(x statshistory.UplinkBytes) bool { return x.Name == u.Name })
			if i < 0 {
				p.Uplinks = append(p.Uplinks, statshistory.UplinkBytes{Name: u.Name})
				i = len(p.Uplinks) - 1
			}
			p.Uplinks[i].RXBytes += u.RXBytes
			p.Uplinks[i].TXBytes += u.TXBytes
		}
	}
	slices.SortFunc(p.Uplinks, func(a, b statshistory.UplinkBytes) int { return strings.Compare(a.Name, b.Name) })
	return p
}

// Used is the traffic of the budget's uplinks in a period, both directions.
func Used(p Period, uplinks []string) uint64 {
	var n uint64
	for _, u := range p.Uplinks {
		if slices.Contains(uplinks, u.Name) {
			n += u.RXBytes + u.TXBytes
		}
	}
	return n
}

// EventKind is what changed at a sample.
type EventKind string

const (
	EventBoot EventKind = "boot"
	EventUp   EventKind = "up"
	EventDown EventKind = "down"
)

// Event is one change in the uptime log. Up and down carry the sample time, so they are
// accurate to one bucket; a boot carries the computed boot time.
type Event struct {
	At     int64     `json:"at" doc:"Unix seconds, router wall clock"`
	Kind   EventKind `json:"kind" enum:"boot,up,down"`
	Uplink string    `json:"uplink,omitempty" enum:"wan,wwan,usbtether" doc:"Set for up and down"`
}

// Availability is how many samples with a known up list saw an uplink up.
type Availability struct {
	Name      string `json:"name" enum:"wan,wwan,usbtether"`
	UpSamples int    `json:"up_samples"`
	Samples   int    `json:"samples"`
}

// UptimeLog is derived from the 10 min points; nothing extra is stored.
type UptimeLog struct {
	Events       []Event        `json:"events" nullable:"false" doc:"Oldest first"`
	Availability []Availability `json:"availability" nullable:"false"`
}

// DeriveUptime finds boots (uptime went down between two points) and uplink up/down changes
// between consecutive points whose up list is known. An outage that starts and ends between
// two samples is not seen. Pure.
func DeriveUptime(points []statshistory.Point) UptimeLog {
	log := UptimeLog{Events: []Event{}, Availability: []Availability{}}
	upCount := map[string]int{}
	samples := 0
	var prevUp []string
	for i, p := range points {
		if i > 0 && p.UptimeSeconds < points[i-1].UptimeSeconds {
			log.Events = append(log.Events, Event{At: p.At - p.UptimeSeconds, Kind: EventBoot})
		}
		if p.Up == nil {
			continue
		}
		samples++
		for _, name := range p.Up {
			upCount[name]++
		}
		if prevUp != nil {
			log.Events = append(log.Events, changes(p.At, prevUp, p.Up)...)
		}
		prevUp = p.Up
	}
	slices.SortStableFunc(log.Events, func(a, b Event) int { return cmp.Compare(a.At, b.At) })
	for _, name := range statshistory.Uplinks {
		if upCount[name] > 0 {
			log.Availability = append(log.Availability, Availability{Name: name, UpSamples: upCount[name], Samples: samples})
		}
	}
	return log
}

func changes(at int64, prev, cur []string) []Event {
	var out []Event
	for _, name := range statshistory.Uplinks {
		was, is := slices.Contains(prev, name), slices.Contains(cur, name)
		switch {
		case is && !was:
			out = append(out, Event{At: at, Kind: EventUp, Uplink: name})
		case was && !is:
			out = append(out, Event{At: at, Kind: EventDown, Uplink: name})
		}
	}
	return out
}
