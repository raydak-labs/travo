// Package systemlog reads the router's system log (ubox `logread`, a RAM ring buffer) and
// turns its lines into entries. Read-only: nothing here writes or clears the log.
package systemlog

import (
	"context"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Levels in syslog order, most severe first; their index is the severity.
var Levels = []string{"emerg", "alert", "crit", "err", "warn", "notice", "info", "debug"}

const (
	// MaxLines caps one reply.
	MaxLines = 1000
	// DefaultLines is the reply size when the caller does not say.
	DefaultLines = 200
	// maxBytes keeps only the newest part of an oversized ring (log_size can be raised).
	maxBytes = 1 << 20
	// stamp is the ctime form logread prints, router-local time without a zone.
	stamp = "Mon Jan _2 15:04:05 2006"
)

// LogEntry is one log line. Time is router-local wall clock (RFC 3339 without zone), empty
// when the line did not parse; then Message carries the raw line.
type LogEntry struct {
	Time     string `json:"time" doc:"Router-local time, 2006-01-02T15:04:05; empty when the line did not parse"`
	Facility string `json:"facility"`
	Level    string `json:"level" doc:"emerg, alert, crit, err, warn, notice, info, debug; empty when unknown"`
	Program  string `json:"program"`
	PID      int    `json:"pid,omitempty"`
	Message  string `json:"message"`
}

var (
	head = regexp.MustCompile(`^(\w{3} \w{3} [ \d]\d \d\d:\d\d:\d\d \d{4}) ([a-z0-9]+)\.([a-z]+) (.*)$`)
	prog = regexp.MustCompile(`^([^\s:\[]+)(?:\[(\d+)\])?: ?(.*)$`)
)

// Parse reads one `logread` line: `Tue Nov 17 21:58:39 2026 daemon.notice prog[pid]: msg`.
// The program may be empty (`user.notice : msg`) or carry no pid (`kernel:`).
func Parse(line string) LogEntry {
	m := head.FindStringSubmatch(line)
	if m == nil {
		return LogEntry{Message: line}
	}
	t, err := time.Parse(stamp, m[1])
	if err != nil {
		return LogEntry{Message: line}
	}
	e := LogEntry{Time: t.Format("2006-01-02T15:04:05"), Facility: m[2], Level: level(m[3])}
	rest := m[4]
	if after, ok := strings.CutPrefix(rest, ": "); ok {
		e.Message = after
		return e
	}
	p := prog.FindStringSubmatch(rest)
	if p == nil {
		e.Message = rest
		return e
	}
	e.Program, e.Message = p[1], p[3]
	if p[2] != "" {
		e.PID = atoi(p[2])
	}
	return e
}

func level(s string) string {
	if slices.Contains(Levels, s) {
		return s
	}
	return ""
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
		if n > 1<<30 {
			return 0
		}
	}
	return n
}

// Query selects entries. Lines is clamped to 1..MaxLines (0 = DefaultLines); MinLevel ""
// keeps all; Contains is a case-insensitive substring of program or message.
type Query struct {
	Lines    int
	MinLevel string
	Contains string
}

// LogPage is the newest matching entries first; Truncated says older matches were cut.
type LogPage struct {
	Entries   []LogEntry `json:"entries" nullable:"false"`
	Truncated bool       `json:"truncated" doc:"Older matching lines exist beyond the limit"`
}

// Select filters parsed log text and returns the newest Lines matches, newest first.
func Select(text string, q Query) LogPage {
	if len(text) > maxBytes {
		text = text[len(text)-maxBytes:]
		if i := strings.IndexByte(text, '\n'); i >= 0 {
			text = text[i+1:] // drop the cut-off first line
		}
	}
	limit := q.Lines
	if limit <= 0 {
		limit = DefaultLines
	}
	limit = min(limit, MaxLines)
	maxSeverity := slices.Index(Levels, q.MinLevel)
	needle := strings.ToLower(q.Contains)
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	out := LogPage{Entries: []LogEntry{}}
	for i := len(lines) - 1; i >= 0; i-- {
		if lines[i] == "" {
			continue
		}
		e := Parse(lines[i])
		if !keep(e, maxSeverity, needle) {
			continue
		}
		if len(out.Entries) == limit {
			out.Truncated = true
			break
		}
		out.Entries = append(out.Entries, e)
	}
	return out
}

func keep(e LogEntry, maxSeverity int, needle string) bool {
	if maxSeverity >= 0 {
		sev := slices.Index(Levels, e.Level)
		if sev < 0 || sev > maxSeverity {
			return false
		}
	}
	if needle == "" {
		return true
	}
	return strings.Contains(strings.ToLower(e.Program), needle) || strings.Contains(strings.ToLower(e.Message), needle)
}

// Reader is the device port: the whole `logread` output.
type Reader interface {
	ReadSystemLog(context.Context) (string, error)
}

// Service answers queries from the device log.
type Service struct{ Log Reader }

// Query reads the log once and selects from it.
func (s *Service) Query(ctx context.Context, q Query) (LogPage, error) {
	text, err := s.Log.ReadSystemLog(ctx)
	if err != nil {
		return LogPage{}, err
	}
	return Select(text, q), nil
}
