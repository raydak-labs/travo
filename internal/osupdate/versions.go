// Package osupdate reports newer OpenWrt releases and upgradable packages. It only reports:
// Travo never flashes from the release index; LuCI attended sysupgrade does that and keeps
// installed packages (device-facts §10).
package osupdate

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Version is a stable OpenWrt release `YY.MM.patch`; release candidates are not stable.
type Version struct{ Year, Month, Patch int }

func (v Version) String() string { return fmt.Sprintf("%d.%02d.%d", v.Year, v.Month, v.Patch) }

// Series is `YY.MM`, the major release line.
func (v Version) Series() string { return fmt.Sprintf("%d.%02d", v.Year, v.Month) }

// Less orders versions.
func (v Version) Less(o Version) bool {
	if v.Year != o.Year {
		return v.Year < o.Year
	}
	if v.Month != o.Month {
		return v.Month < o.Month
	}
	return v.Patch < o.Patch
}

// ParseVersion accepts only stable `YY.MM.patch` (SNAPSHOT and -rcN are refused).
func ParseVersion(s string) (Version, bool) {
	p := strings.Split(s, ".")
	if len(p) != 3 {
		return Version{}, false
	}
	n := [3]int{}
	for i, part := range p {
		v, err := strconv.Atoi(part)
		if err != nil || v < 0 || part == "" || (len(part) > 1 && part[0] == '+') {
			return Version{}, false
		}
		n[i] = v
	}
	return Version{n[0], n[1], n[2]}, true
}

// Index is the release index `<firmware_url>.versions.json`.
type Index struct {
	Stable   string   `json:"stable_version"`
	Versions []string `json:"versions_list"`
}

// ParseIndex parses the release index; an empty versions list is an error.
func ParseIndex(b []byte) (Index, error) {
	var ix Index
	if err := json.Unmarshal(b, &ix); err != nil {
		return Index{}, fmt.Errorf("release index: %w", err)
	}
	if len(ix.Versions) == 0 {
		return Index{}, fmt.Errorf("release index: no versions")
	}
	return ix, nil
}

// Releases is what the check reports. Empty strings mean "none newer".
type Releases struct {
	Current string `json:"current" doc:"Running OpenWrt version"`
	// InSeries is the newest stable release of the running series newer than Current.
	InSeries string `json:"in_series" doc:"Newest stable release of the running series that is newer, e.g. 25.12.7; empty when current"`
	// NextSeries is the newest stable release of the next series after the running one.
	NextSeries string `json:"next_series" doc:"Newest stable release of the next major series, e.g. 26.06.1; empty when none is out"`
}

// Compare picks the updates for current from the index. A non-stable current (SNAPSHOT,
// rc) still reports the series' newest stable release when that is newer.
func Compare(current string, ix Index) Releases {
	out := Releases{Current: current}
	cur, ok := ParseVersion(current)
	if !ok {
		base, _, _ := strings.Cut(current, "-")
		if cur, ok = ParseVersion(base); !ok {
			return out
		}
		cur.Patch-- // 25.12.0-rc1 is older than 25.12.0
	}
	var inSeries, next *Version
	for _, s := range ix.Versions {
		v, ok := ParseVersion(s)
		if !ok || !cur.Less(v) {
			continue
		}
		switch {
		case v.Series() == cur.Series():
			if inSeries == nil || inSeries.Less(v) {
				inSeries = &v
			}
		case next == nil || v.Series() < next.Series() || (v.Series() == next.Series() && next.Less(v)):
			next = &v
		}
	}
	if inSeries != nil {
		out.InSeries = inSeries.String()
	}
	if next != nil {
		out.NextSeries = next.String()
	}
	return out
}
