// Package crontab edits Travo's blocks in root's BusyBox crontab (`/etc/crontabs/root`;
// `/etc/cron.d` does not exist on OpenWrt, device-facts §8). Each feature owns one block
// between marker comments; lines outside every Travo block are kept byte for byte.
// Scheduled commands call the daemon through the hook socket (internal/hook, ADR 0024),
// never uci in shell.
package crontab

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// HookCommand is the command a scheduled job runs for a daemon hook action. Its output goes
// to syslog: crond would otherwise drop a failure silently.
func HookCommand(action string) string {
	return "/usr/bin/travo-greenfield --hook " + action + " 2>&1 | logger -t travo-greenfield-hook"
}

var (
	ownerName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)
	// ErrInvalid means a line or owner that the block format cannot hold.
	ErrInvalid = errors.New("crontab: invalid line or owner")
)

func begin(owner string) string { return "# travo-greenfield:" + owner + " begin" }
func end(owner string) string   { return "# travo-greenfield:" + owner + " end" }

// Lines returns the owner's job lines, in order; nil when it has no block.
func Lines(content, owner string) []string {
	var out []string
	in := false
	for _, l := range strings.Split(content, "\n") {
		switch {
		case l == begin(owner):
			in = true
		case l == end(owner):
			in = false
		case in && l != "":
			out = append(out, l)
		}
	}
	return out
}

// Replace drops the owner's block and, when lines is not empty, appends a new one. Every
// other line stays as it was. An unterminated block of the owner is removed to its end.
func Replace(content, owner string, lines []string) (string, error) {
	if !ownerName.MatchString(owner) {
		return "", fmt.Errorf("%w: owner %q", ErrInvalid, owner)
	}
	for _, l := range lines {
		if err := validLine(l); err != nil {
			return "", err
		}
	}
	kept := []string{}
	in := false
	for _, l := range strings.Split(strings.TrimSuffix(content, "\n"), "\n") {
		switch {
		case l == begin(owner):
			in = true
		case l == end(owner) && in:
			in = false
		case !in:
			kept = append(kept, l)
		}
	}
	if len(kept) == 1 && kept[0] == "" {
		kept = kept[:0]
	}
	if len(lines) > 0 {
		kept = append(kept, begin(owner))
		kept = append(kept, lines...)
		kept = append(kept, end(owner))
	}
	if len(kept) == 0 {
		return "", nil
	}
	return strings.Join(kept, "\n") + "\n", nil
}

// validLine accepts five time fields and a command on one line.
func validLine(l string) error {
	if strings.ContainsAny(l, "\n\r\x00") || len(strings.Fields(l)) < 6 || strings.HasPrefix(l, "#") {
		return fmt.Errorf("%w: %q", ErrInvalid, l)
	}
	return nil
}

// Clock is a daily time of day, minutes precision.
type Clock struct {
	Hour   int
	Minute int
}

// ParseClock reads "HH:MM" (24 h).
func ParseClock(s string) (Clock, error) {
	h, m, ok := strings.Cut(s, ":")
	if !ok || len(h) != 2 || len(m) != 2 {
		return Clock{}, fmt.Errorf("%w: time %q is not HH:MM", ErrInvalid, s)
	}
	hour, err1 := strconv.Atoi(h)
	minute, err2 := strconv.Atoi(m)
	if err1 != nil || err2 != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return Clock{}, fmt.Errorf("%w: time %q is not HH:MM", ErrInvalid, s)
	}
	return Clock{Hour: hour, Minute: minute}, nil
}

func (c Clock) String() string { return fmt.Sprintf("%02d:%02d", c.Hour, c.Minute) }

// Daily is the job line that runs command every day at c (router local time).
func Daily(c Clock, command string) string {
	return fmt.Sprintf("%d %d * * * %s", c.Minute, c.Hour, command)
}

// ParseDaily reads a line Daily wrote back; ok is false for any other shape.
func ParseDaily(line string) (c Clock, command string, ok bool) {
	f := strings.SplitN(line, " ", 6)
	if len(f) != 6 || f[2] != "*" || f[3] != "*" || f[4] != "*" {
		return Clock{}, "", false
	}
	minute, err1 := strconv.Atoi(f[0])
	hour, err2 := strconv.Atoi(f[1])
	if err1 != nil || err2 != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return Clock{}, "", false
	}
	return Clock{Hour: hour, Minute: minute}, f[5], true
}
