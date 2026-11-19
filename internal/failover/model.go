// Package failover drives multi-WAN failover through mwan3 (IPv4 only, invariants §9,
// ADR 0018). This file is pure: Travo's settings and the current /etc/config/mwan3 in,
// the sections to write, delete and reorder out.
package failover

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

const (
	// Config is the UCI config mwan3 reads.
	Config = "mwan3"
	// Prefix marks every generated section except interfaces (mwan3 keys those by the
	// netifd interface name).
	Prefix = "travo_"
	// FingerprintOption stamps a section Travo wrote (provenance, never content ownership).
	FingerprintOption = "travo_fp"
	Policy            = "travo_failover"
	RuleV4            = "travo_v4"
	// RuleV6 keeps IPv6 on the main table: no IPv6 policy until device-validated.
	RuleV6 = "travo_v6"
	// fingerprintSchema changes only when the owned option set changes; a build upgrade
	// alone must not turn every section into "operator-edited".
	fingerprintSchema = "travo-mwan3/2"
)

// Tracking is what mwan3track uses to decide an uplink is online (ADR 0018). Without
// track_ip mwan3 reports every interface "unknown and tracking is disabled" and its
// policies never get an online member (#277).
var (
	// TrackIPs: two independent anycast resolvers, by address (no DNS during an outage).
	TrackIPs = []string{"1.1.1.1", "8.8.8.8"}
	// Tracking: one reply from either target per round is enough (reliability 1, count 1),
	// a round every 5 s; 3 failed rounds take an uplink offline (~15 s), 6 good rounds
	// bring it back (the 30 s failback hold-down of invariants §9).
	Tracking = map[string]string{"reliability": "1", "count": "1", "interval": "5", "down": "3", "up": "6"}
)

// Candidates are the uplinks of invariants §7, in default priority.
var Candidates = []string{"wan", "wwan", "usbtether"}

var (
	ErrInvalid  = errors.New("failover needs two or three different uplinks from wan, wwan, usbtether")
	ErrConflict = errors.New("a foreign mwan3 section uses a name Travo writes")
)

// Settings are Travo's failover JSON, the source of truth for the generated sections.
type Settings struct {
	Enabled bool `json:"enabled"`
	// Uplinks in priority order: the first one that is up carries the traffic.
	Uplinks []string `json:"uplinks" nullable:"false" doc:"Uplinks in priority order (wan, wwan, usbtether)"`
}

// Validate: enabled settings name 2 or 3 different candidates. Disabled settings may keep
// any valid list (the order is remembered for the next enable).
func (s Settings) Validate() error {
	seen := map[string]bool{}
	for _, u := range s.Uplinks {
		if !slices.Contains(Candidates, u) || seen[u] {
			return ErrInvalid
		}
		seen[u] = true
	}
	if s.Enabled && len(s.Uplinks) < 2 {
		return ErrInvalid
	}
	return nil
}

// owned lists the options and lists Travo owns per section type. Anything else in a
// section (an adopted stock interface's `reliability`, say) is left alone.
var owned = map[string][]string{
	"interface": {"enabled", "family", "track_ip", "reliability", "count", "interval", "down", "up"},
	"member":    {"interface", "metric", "weight"},
	"policy":    {"use_member", "last_resort"},
	"rule":      {"dest_ip", "family", "use_policy", "proto", "dest_port", "sticky"},
}

// MemberName is the generated member of one uplink.
func MemberName(uplink string) string { return Prefix + "m_" + uplink }

// Desired is the stamped section set for enabled settings. Interfaces carry the tracking
// options: mwan3 marks an uplink online only through mwan3track (invariants §9).
func Desired(s Settings) []device.Section {
	var out []device.Section
	members := make([]string, 0, len(s.Uplinks))
	for i, u := range s.Uplinks {
		opts := maps.Clone(Tracking)
		opts["enabled"], opts["family"] = "1", "ipv4"
		out = append(out, section(u, "interface", opts, map[string][]string{"track_ip": slices.Clone(TrackIPs)}))
		out = append(out, section(MemberName(u), "member", map[string]string{
			"interface": u, "metric": fmt.Sprint(i + 1), "weight": "1",
		}, nil))
		members = append(members, MemberName(u))
	}
	// last_resort default: with every member down mwan3 uses the main table instead of
	// `unreachable` (the stock-install outage of device-facts §9).
	out = append(out, section(Policy, "policy", map[string]string{"last_resort": "default"}, map[string][]string{"use_member": members}))
	out = append(out, section(RuleV4, "rule", map[string]string{"dest_ip": "0.0.0.0/0", "family": "ipv4", "use_policy": Policy}, nil))
	out = append(out, section(RuleV6, "rule", map[string]string{"dest_ip": "::/0", "family": "ipv6", "use_policy": "default"}, nil))
	for i := range out {
		out[i].Options[FingerprintOption] = Fingerprint(out[i])
	}
	return out
}

func section(name, typ string, opts map[string]string, lists map[string][]string) device.Section {
	if lists == nil {
		lists = map[string][]string{}
	}
	return device.Section{Name: name, Type: typ, Options: opts, Lists: lists}
}

// Fingerprint hashes a section's name, type and owned values (not travo_fp itself).
func Fingerprint(s device.Section) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n%s\n%s\n", fingerprintSchema, s.Name, s.Type)
	for _, k := range owned[s.Type] {
		if v, ok := s.Options[k]; ok {
			fmt.Fprintf(&b, "o %s=%s\n", k, v)
		}
		if l := s.Lists[k]; len(l) > 0 {
			fmt.Fprintf(&b, "l %s=%s\n", k, strings.Join(l, "\x1f"))
		}
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:8])
}

// Stamped: the section carries a fingerprint and it still matches its owned values.
func Stamped(s device.Section) bool {
	fp, ok := s.Options[FingerprintOption]
	return ok && fp == Fingerprint(s)
}

// Warning is a loud notice about a section Travo did not write or someone edited.
type Warning struct {
	Section string `json:"section"`
	// adopted: an existing interface section (stock `wan`) now has Travo-owned options.
	// operator_edited: a Travo section was changed by hand; its owned options are overwritten.
	// stale_edited: a Travo section no longer needed was changed by hand; it is kept.
	Reason string `json:"reason" enum:"adopted,operator_edited,stale_edited"`
}

// Plan is what an apply stages.
type Plan struct {
	Write []device.Section
	// Delete names stale Travo sections whose fingerprint still matches.
	Delete []string
	// Disable names stamped interface sections to set `enabled 0` (never deleted).
	Disable  []string
	Warnings []Warning
}

// PlanApply compares the current config with the settings. A foreign section of another
// type on a name Travo writes is ErrConflict; nothing is staged then.
func PlanApply(current []device.Section, s Settings) (Plan, error) {
	if err := s.Validate(); err != nil {
		return Plan{}, err
	}
	want := Desired(s)
	byName := map[string]device.Section{}
	for _, c := range current {
		byName[c.Name] = c
	}
	p := Plan{Write: want}
	wanted := map[string]bool{}
	for _, w := range want {
		wanted[w.Name] = true
		c, ok := byName[w.Name]
		switch {
		case !ok:
		case c.Type != w.Type:
			return Plan{}, fmt.Errorf("%w: %s.%s is a %s", ErrConflict, Config, w.Name, c.Type)
		case w.Type == "interface" && c.Options[FingerprintOption] == "":
			p.Warnings = append(p.Warnings, Warning{Section: w.Name, Reason: "adopted"})
		case !Stamped(c):
			p.Warnings = append(p.Warnings, Warning{Section: w.Name, Reason: "operator_edited"})
		}
	}
	for _, c := range current {
		if wanted[c.Name] {
			continue
		}
		switch {
		case c.Type == "interface" && slices.Contains(Candidates, c.Name) && Stamped(c) && c.Options["enabled"] != "0":
			p.Disable = append(p.Disable, c.Name)
		case strings.HasPrefix(c.Name, Prefix) && Stamped(c):
			p.Delete = append(p.Delete, c.Name)
		case strings.HasPrefix(c.Name, Prefix):
			p.Warnings = append(p.Warnings, Warning{Section: c.Name, Reason: "stale_edited"})
		}
	}
	return p, nil
}

// Disabled is a stamped interface section turned off: same section, enabled 0, re-stamped.
func Disabled(c device.Section) device.Section {
	out := device.Section{Name: c.Name, Type: c.Type, Options: maps.Clone(c.Options), Lists: maps.Clone(c.Lists)}
	out.Options["enabled"] = "0"
	out.Options[FingerprintOption] = Fingerprint(out)
	return out
}

// Verify proves every written section reads back with its owned values and stamp. When
// ordered (the adapter keeps `uci export` order), the two Travo rules come first.
func Verify(current []device.Section, p Plan, ordered bool) error {
	for _, w := range p.Write {
		i := slices.IndexFunc(current, func(c device.Section) bool { return c.Name == w.Name })
		if i < 0 || current[i].Type != w.Type {
			return fmt.Errorf("mwan3.%s missing after apply", w.Name)
		}
		if current[i].Options[FingerprintOption] != w.Options[FingerprintOption] || !Stamped(current[i]) {
			return fmt.Errorf("mwan3.%s differs after apply", w.Name)
		}
	}
	if ordered && !rulesFirst(current) {
		return fmt.Errorf("mwan3 rules do not start with %s, %s", RuleV4, RuleV6)
	}
	return nil
}
