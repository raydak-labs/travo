// Package syssettings reads and writes the router's hostname, timezone and NTP client
// (UCI `system.@system[0]` and `system.ntp`). The change cannot cut connectivity, so it
// takes the direct apply path (lock, stage, commit, reload_config), no watchdog.
package syssettings

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

const (
	config     = "system"
	ntpSection = "ntp"
)

// NTP is the sysntpd client. Servers keep their configured order.
type NTP struct {
	Enabled bool     `json:"enabled"`
	Servers []string `json:"servers" nullable:"false"`
}

// Settings is what the API reads and writes. Timezone is derived from Zonename on write.
type Settings struct {
	Hostname string `json:"hostname"`
	Zonename string `json:"zonename"`
	Timezone string `json:"timezone"`
	NTP      NTP    `json:"ntp"`
}

// FieldError names the request field that failed validation.
type FieldError struct {
	Field  string
	Reason string
}

func (e *FieldError) Error() string { return e.Field + ": " + e.Reason }

// ErrNoSystemSection means the config has no `system` section to edit.
var ErrNoSystemSection = errors.New("syssettings: no system section")

// Applier is txn.Manager.Apply: lock, refuse foreign staged changes, stage, commit, reload.
type Applier interface {
	Apply(ctx context.Context, configs []string, stage func(ctx context.Context) error) error
}

// Service is the read/write port the API uses.
type Service struct {
	UCI   device.UCI
	Apply Applier
	// OnChange runs after Set wrote a change, with the settings read back (e.g. the stats
	// history moves its day boundaries to the new timezone). Nil: nothing to notify.
	OnChange func(Settings)
}

// Get reads the current settings.
func (s *Service) Get(ctx context.Context) (Settings, error) {
	secs, err := s.UCI.Show(ctx, config)
	if err != nil {
		return Settings{}, err
	}
	_, cur, err := Parse(secs)
	return cur, err
}

// Set validates want, then stages only the options that differ. Nothing differs: no write.
func (s *Service) Set(ctx context.Context, want Settings) (Settings, error) {
	cur, err := s.Get(ctx)
	if err != nil {
		return Settings{}, err
	}
	tz, err := Validate(want, cur)
	if err != nil {
		return Settings{}, err
	}
	want.Timezone = tz
	if equal(cur, want) {
		return cur, nil
	}
	err = s.Apply.Apply(ctx, []string{config}, func(lctx context.Context) error {
		secs, err := s.UCI.Show(lctx, config)
		if err != nil {
			return err
		}
		return Stage(lctx, s.UCI, secs, want)
	})
	if err != nil {
		return Settings{}, err
	}
	got, err := s.Get(ctx)
	if err == nil && s.OnChange != nil {
		s.OnChange(got)
	}
	return got, err
}

// Parse finds the first `system` section (anonymous on stock images) and the `ntp` timeserver.
func Parse(secs []device.Section) (string, Settings, error) {
	sys := -1
	for i, sec := range secs {
		if sec.Type == "system" {
			sys = i
			break
		}
	}
	if sys < 0 {
		return "", Settings{}, ErrNoSystemSection
	}
	o := secs[sys].Options
	out := Settings{Hostname: o["hostname"], Zonename: o["zonename"], Timezone: o["timezone"], NTP: NTP{Servers: []string{}}}
	if ntp, ok := findNTP(secs); ok {
		out.NTP.Enabled = ntp.Options["enabled"] != "0"
		out.NTP.Servers = ntpServers(ntp)
	}
	return secs[sys].Name, out, nil
}

func findNTP(secs []device.Section) (device.Section, bool) {
	for _, sec := range secs {
		if sec.Name == ntpSection && sec.Type == "timeserver" {
			return sec, true
		}
	}
	return device.Section{}, false
}

// ntpServers reads the `server` list; a one-element list read via `uci show` is an option.
func ntpServers(sec device.Section) []string {
	if l := sec.Lists["server"]; len(l) > 0 {
		return slices.Clone(l)
	}
	if v := sec.Options["server"]; v != "" {
		return []string{v}
	}
	return []string{}
}

func equal(a, b Settings) bool {
	return a.Hostname == b.Hostname && a.Zonename == b.Zonename && a.Timezone == b.Timezone &&
		a.NTP.Enabled == b.NTP.Enabled && slices.Equal(a.NTP.Servers, b.NTP.Servers)
}

// Stage writes the differing options of want into the staged `system` delta.
func Stage(ctx context.Context, u device.UCI, secs []device.Section, want Settings) error {
	name, cur, err := Parse(secs)
	if err != nil {
		return err
	}
	for _, kv := range [][3]string{
		{"hostname", cur.Hostname, want.Hostname},
		{"zonename", cur.Zonename, want.Zonename},
		{"timezone", cur.Timezone, want.Timezone},
	} {
		if kv[1] != kv[2] {
			if err := u.Set(ctx, config, name, kv[0], kv[2]); err != nil {
				return err
			}
		}
	}
	return stageNTP(ctx, u, secs, cur.NTP, want.NTP)
}

func stageNTP(ctx context.Context, u device.UCI, secs []device.Section, cur, want NTP) error {
	if _, ok := findNTP(secs); !ok {
		if err := u.SetType(ctx, config, ntpSection, "timeserver"); err != nil {
			return err
		}
		cur = NTP{Enabled: true, Servers: []string{}} // a fresh section reads as enabled
	}
	if cur.Enabled != want.Enabled {
		if err := u.Set(ctx, config, ntpSection, "enabled", boolOption(want.Enabled)); err != nil {
			return err
		}
	}
	if slices.Equal(cur.Servers, want.Servers) {
		return nil
	}
	if err := u.DeleteOption(ctx, config, ntpSection, "server"); err != nil && !errors.Is(err, device.ErrNotFound) {
		return err
	}
	for _, srv := range want.Servers {
		if err := u.AddToList(ctx, config, ntpSection, "server", srv); err != nil {
			return err
		}
	}
	return nil
}

func boolOption(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// Validate checks every field against the current settings and returns the POSIX timezone
// to store. An unchanged zonename keeps the stored timezone, so a GET body PUT back is a
// no-op even when LuCI's table wrote a slightly different string or a zone we do not list.
func Validate(s, cur Settings) (string, error) {
	if !validHostname(s.Hostname) {
		return "", &FieldError{"hostname", "1-63 letters, digits or '-', not starting or ending with '-'"}
	}
	tz, err := timezoneFor(s, cur)
	if err != nil {
		return "", err
	}
	if len(s.NTP.Servers) > maxServer {
		return "", &FieldError{"ntp.servers", fmt.Sprintf("at most %d servers", maxServer)}
	}
	if s.NTP.Enabled && len(s.NTP.Servers) == 0 {
		return "", &FieldError{"ntp.servers", "at least one server while NTP is enabled"}
	}
	seen := map[string]bool{}
	for _, srv := range s.NTP.Servers {
		if !validServer(srv) {
			return "", &FieldError{"ntp.servers", fmt.Sprintf("%q is not a host name or IP address", srv)}
		}
		if seen[strings.ToLower(srv)] {
			return "", &FieldError{"ntp.servers", fmt.Sprintf("%q is listed twice", srv)}
		}
		seen[strings.ToLower(srv)] = true
	}
	return tz, nil
}

func timezoneFor(s, cur Settings) (string, error) {
	if s.Zonename == cur.Zonename && cur.Zonename != "" {
		if s.Timezone != "" && s.Timezone != cur.Timezone {
			return "", &FieldError{"timezone", "must be empty or match the zonename"}
		}
		return cur.Timezone, nil
	}
	tz, ok := POSIXTimezone(s.Zonename)
	if !ok {
		return "", &FieldError{"zonename", "unknown time zone"}
	}
	if s.Timezone != "" && s.Timezone != tz {
		return "", &FieldError{"timezone", "must be empty or match the zonename"}
	}
	return tz, nil
}

// validHostname is one RFC 1123 label: the router's own name, not an FQDN.
func validHostname(h string) bool {
	return validLabel(h)
}

func validLabel(l string) bool {
	if len(l) == 0 || len(l) > 63 || l[0] == '-' || l[len(l)-1] == '-' {
		return false
	}
	for _, c := range l {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
			return false
		}
	}
	return true
}

func validServer(s string) bool {
	if _, err := netip.ParseAddr(s); err == nil {
		return !strings.Contains(s, "%") // no zone ids
	}
	if len(s) > 253 || strings.HasSuffix(s, ".") {
		return false
	}
	labels := strings.Split(s, ".")
	for _, l := range labels {
		if !validLabel(l) {
			return false
		}
	}
	// An all-numeric dotted name would be a malformed IPv4 address, not a host name.
	last := labels[len(labels)-1]
	return strings.ContainsFunc(last, func(r rune) bool { return r < '0' || r > '9' })
}
