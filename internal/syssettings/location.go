package syssettings

import (
	"bytes"
	"encoding/binary"
	"errors"
	"time"
)

// invalidZone names the only local time type of the blob below. Go falls back to it when
// the POSIX rule does not parse, so seeing it means the rule was rejected.
const invalidZone = "invalid"

var errBadTZ = errors.New("syssettings: unusable POSIX timezone")

// Location turns the POSIX TZ string OpenWrt stores in system.@system[0].timezone (e.g.
// "CET-1CEST,M3.5.0,M10.5.0/3") into a *time.Location, DST rules included. The router
// ships no zoneinfo (device-facts §8), so it wraps the rule in a minimal TZif v2 blob: no
// transitions, one placeholder type, the rule as the footer that Go applies to every time.
func Location(posixTZ string) (*time.Location, error) {
	switch posixTZ {
	case "", UTC, "UTC0":
		return time.UTC, nil
	}
	if bytes.ContainsAny([]byte(posixTZ), "\n\x00") {
		return nil, errBadTZ
	}
	loc, err := time.LoadLocationFromTZData(posixTZ, tzif(posixTZ))
	if err != nil {
		return nil, err
	}
	for _, at := range []time.Time{time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC), time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)} {
		if name, _ := at.In(loc).Zone(); name == invalidZone {
			return nil, errBadTZ
		}
	}
	return loc, nil
}

// tzif builds the blob: a v1 block and a v2 block, each with one local time type and no
// transitions, then the footer "\n<rule>\n".
func tzif(rule string) []byte {
	var b bytes.Buffer
	block := func() {
		b.WriteString("TZif2")
		b.Write(make([]byte, 15))
		// isutcnt, isstdcnt, leapcnt, timecnt, typecnt, charcnt
		for _, n := range []uint32{0, 0, 0, 0, 1, uint32(len(invalidZone) + 1)} {
			_ = binary.Write(&b, binary.BigEndian, n)
		}
		b.Write([]byte{0, 0, 0, 0, 0, 0}) // utoff 0, isdst 0, desigidx 0
		b.WriteString(invalidZone + "\x00")
	}
	block()
	block()
	b.WriteString("\n" + rule + "\n")
	return b.Bytes()
}
