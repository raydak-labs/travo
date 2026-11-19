package statshistory

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// DayRetention keeps day totals for three billing months, so the current and the previous
// period are always complete (docs/adr/0026). About 100 bytes per uplink and day.
const DayRetention = 93 * 24 * time.Hour

// DateLayout is a day's key. Days follow the router's timezone (system.@system[0].timezone,
// syssettings.Location); UTC until it is known. Stored days keep the zone they were
// recorded in: only the day the zone changes mixes both (docs/adr/0026).
const DateLayout = time.DateOnly

// Day is the traffic each uplink moved on one local calendar day: the sum of the day's points.
type Day struct {
	Date    string        `json:"date" doc:"Calendar day in the router's timezone, YYYY-MM-DD"`
	Uplinks []UplinkBytes `json:"uplinks" nullable:"false"`
}

// Days returns the day totals, oldest first.
func (h *History) Days() ([]Day, Persistence) {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]Day, len(h.days))
	for i, d := range h.days {
		out[i] = Day{Date: d.Date, Uplinks: slices.Clone(d.Uplinks)}
	}
	return out, h.persistence
}

// addToDays adds a point's bytes to the day in loc its bucket ends on. A bucket that
// crosses midnight counts for the later day. A date before the newest day (the zone moved
// west) counts for the newest day, so days stay in order.
func addToDays(days []Day, p Point, loc *time.Location) []Day {
	if len(p.Uplinks) == 0 {
		return days
	}
	date := time.Unix(p.At, 0).In(loc).Format(DateLayout)
	if len(days) == 0 || days[len(days)-1].Date < date {
		days = append(days, Day{Date: date, Uplinks: []UplinkBytes{}})
	}
	d := &days[len(days)-1]
	for _, u := range p.Uplinks {
		i := slices.IndexFunc(d.Uplinks, func(x UplinkBytes) bool { return x.Name == u.Name })
		if i < 0 {
			d.Uplinks = append(d.Uplinks, UplinkBytes{Name: u.Name})
			i = len(d.Uplinks) - 1
		}
		d.Uplinks[i].RXBytes += u.RXBytes
		d.Uplinks[i].TXBytes += u.TXBytes
	}
	slices.SortFunc(d.Uplinks, func(a, b UplinkBytes) int { return strings.Compare(a.Name, b.Name) })
	return days
}

// pruneDays drops days that ended before now-DayRetention in loc.
func pruneDays(days []Day, now time.Time, loc *time.Location) []Day {
	cut := now.Add(-DayRetention).In(loc).Format(DateLayout)
	first := 0
	for first < len(days) && days[first].Date < cut {
		first++
	}
	if first == 0 {
		return days
	}
	return days[:copy(days, days[first:])]
}

// daysFromPoints seeds day totals from a file written before they existed, in UTC: the
// zone is not known yet when the file loads.
func daysFromPoints(points []Point) []Day {
	days := []Day{}
	for _, p := range points {
		days = addToDays(days, p, time.UTC)
	}
	return days
}

func validateDays(days []Day) error {
	for i, d := range days {
		if _, err := time.Parse(DateLayout, d.Date); err != nil {
			return fmt.Errorf("stats history: bad day %q", d.Date)
		}
		if i > 0 && d.Date <= days[i-1].Date {
			return fmt.Errorf("stats history: days out of order")
		}
		if d.Uplinks == nil {
			return fmt.Errorf("stats history: day without uplinks")
		}
		if err := validateUplinks(d.Uplinks); err != nil {
			return err
		}
	}
	return nil
}

func validateUplinks(uplinks []UplinkBytes) error {
	for _, u := range uplinks {
		if !slices.Contains(Uplinks, u.Name) {
			return fmt.Errorf("stats history: unknown uplink %q", u.Name)
		}
	}
	return nil
}
