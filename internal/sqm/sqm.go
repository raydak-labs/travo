// Package sqm reads and writes essential SQM queue settings (UCI `sqm.@queue[0]`,
// typically `sqm.eth1`) for the optional `sqm-scripts` package (#302).
// Unknown options and extra queue sections stay untouched for LuCI.
package sqm

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/raydak-labs/travo/internal/device"
	"github.com/raydak-labs/travo/internal/pkgmgr"
)

const (
	Package        = "sqm-scripts"
	Config         = "sqm"
	InitScript     = "sqm"
	DefaultSection = "eth1"
	MaxBandwidth   = 10_000_000 // 10 Gbit/s in kbit/s
	MaxOverhead    = 256
)

var (
	// Qdiscs are the queue disciplines Travo exposes.
	Qdiscs = []string{"cake", "fq_codel"}
	// Scripts are the queue setup scripts shipped by sqm-scripts (/usr/lib/sqm/*.qos).
	Scripts = []string{"piece_of_cake.qos", "layer_cake.qos", "simple.qos", "simplest.qos", "simplest_tbf.qos"}
	// Linklayers are the link layer adaptation modes Travo exposes.
	Linklayers = []string{"none", "ethernet", "atm"}

	ErrUnavailable = errors.New("SQM settings are unavailable")
)

// FieldError names the request field that failed validation.
type FieldError struct {
	Field  string
	Reason string
}

func (e *FieldError) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Reason) }

// Settings is the symmetric GET/PUT body for the primary SQM queue section.
type Settings struct {
	Enabled   bool   `json:"enabled" doc:"Enable SQM queue management"`
	Interface string `json:"interface" maxLength:"15" doc:"Kernel network device to shape (e.g. eth1, phy0-sta0)"`
	Download  int    `json:"download" minimum:"0" maximum:"10000000" doc:"Download bandwidth limit in kbit/s (0 disables ingress shaping)"`
	Upload    int    `json:"upload" minimum:"0" maximum:"10000000" doc:"Upload bandwidth limit in kbit/s (0 disables egress shaping)"`
	Qdisc     string `json:"qdisc" enum:"cake,fq_codel" doc:"Queue discipline"`
	Script    string `json:"script" enum:"piece_of_cake.qos,layer_cake.qos,simple.qos,simplest.qos,simplest_tbf.qos" doc:"SQM queue setup script"`
	Linklayer string `json:"linklayer" enum:"none,ethernet,atm" doc:"Link layer adaptation"`
	Overhead  int    `json:"overhead" minimum:"0" maximum:"256" doc:"Per-packet overhead in bytes (0..256)"`
}

// Applier is txn.Manager.Apply: lock, refuse foreign staged changes, stage, commit, reload.
type Applier interface {
	Apply(ctx context.Context, configs []string, stage func(ctx context.Context) error) error
}

// Service reads and writes the primary SQM queue through the Device and PackageManager ports.
type Service struct {
	UCI      device.UCI
	Packages pkgmgr.PackageManager
	Services device.Services
	Apply    Applier

	mu sync.Mutex
}

// Get reads the primary SQM queue settings. Missing sqm-scripts returns pkgmgr.ErrNotInstalled.
func (s *Service) Get(ctx context.Context) (Settings, error) {
	if err := s.checkInstalled(ctx); err != nil {
		return Settings{}, err
	}
	secs, err := s.show(ctx)
	if err != nil {
		return Settings{}, err
	}
	_, cur := Parse(secs)
	return cur, nil
}

// Set validates want, stages only differing options through txn.Apply, and reloads sqm.
func (s *Service) Set(ctx context.Context, want Settings) (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Apply == nil {
		return Settings{}, ErrUnavailable
	}
	if err := s.checkInstalled(ctx); err != nil {
		return Settings{}, err
	}
	if err := Validate(want); err != nil {
		return Settings{}, err
	}
	secs, err := s.show(ctx)
	if err != nil {
		return Settings{}, err
	}
	_, cur := Parse(secs)
	if cur == want {
		return cur, nil
	}
	err = s.Apply.Apply(ctx, []string{Config}, func(lctx context.Context) error {
		lsecs, err := s.show(lctx)
		if err != nil {
			return err
		}
		return Stage(lctx, s.UCI, lsecs, want)
	})
	if err != nil {
		return Settings{}, err
	}
	if s.Services != nil {
		if err := s.Services.ServiceAction(ctx, InitScript, "restart"); err != nil {
			return Settings{}, err
		}
	}
	return s.Get(ctx)
}

func (s *Service) checkInstalled(ctx context.Context) error {
	if s.Packages == nil {
		return ErrUnavailable
	}
	_, err := s.Packages.Version(ctx, Package)
	if errors.Is(err, pkgmgr.ErrNotInstalled) {
		return pkgmgr.ErrNotInstalled
	}
	if err != nil {
		return ErrUnavailable
	}
	return nil
}

func (s *Service) show(ctx context.Context) ([]device.Section, error) {
	secs, err := s.UCI.Show(ctx, Config)
	if errors.Is(err, device.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, ErrUnavailable
	}
	return secs, nil
}

// Parse extracts the first `queue` section from UCI `sqm` sections (or defaults if absent).
func Parse(secs []device.Section) (string, Settings) {
	out := Settings{
		Interface: DefaultSection,
		Qdisc:     "cake",
		Script:    "piece_of_cake.qos",
		Linklayer: "none",
	}
	idx := slices.IndexFunc(secs, func(sec device.Section) bool { return sec.Type == "queue" })
	if idx < 0 {
		return "", out
	}
	sec := secs[idx]
	o := sec.Options
	out.Enabled = o["enabled"] == "1"
	if v := o["interface"]; v != "" {
		out.Interface = v
	}
	if v, err := strconv.Atoi(o["download"]); err == nil && v >= 0 {
		out.Download = v
	}
	if v, err := strconv.Atoi(o["upload"]); err == nil && v >= 0 {
		out.Upload = v
	}
	if v := o["qdisc"]; v != "" {
		out.Qdisc = v
	}
	if v := o["script"]; v != "" {
		out.Script = v
	}
	if v := o["linklayer"]; v != "" {
		out.Linklayer = v
	}
	if v, err := strconv.Atoi(o["overhead"]); err == nil && v >= 0 {
		out.Overhead = v
	}
	return sec.Name, out
}

// Validate checks every field explicitly (invariants §4).
func Validate(in Settings) error {
	switch {
	case !validInterface(in.Interface):
		return &FieldError{"interface", "must be a valid Linux interface name (1-15 chars: letters, digits, '.', '_', '-')"}
	case in.Download < 0 || in.Download > MaxBandwidth:
		return &FieldError{"download", fmt.Sprintf("must be between 0 and %d kbit/s", MaxBandwidth)}
	case in.Upload < 0 || in.Upload > MaxBandwidth:
		return &FieldError{"upload", fmt.Sprintf("must be between 0 and %d kbit/s", MaxBandwidth)}
	case !slices.Contains(Qdiscs, in.Qdisc):
		return &FieldError{"qdisc", "must be one of: " + strings.Join(Qdiscs, ", ")}
	case !slices.Contains(Scripts, in.Script):
		return &FieldError{"script", "must be one of: " + strings.Join(Scripts, ", ")}
	case !slices.Contains(Linklayers, in.Linklayer):
		return &FieldError{"linklayer", "must be one of: " + strings.Join(Linklayers, ", ")}
	case in.Overhead < 0 || in.Overhead > MaxOverhead:
		return &FieldError{"overhead", fmt.Sprintf("must be between 0 and %d bytes", MaxOverhead)}
	}
	return nil
}

// Stage writes only the differing essential options onto the first `queue` section,
// preserving its section name and all unknown options.
func Stage(ctx context.Context, u device.UCI, secs []device.Section, want Settings) error {
	name, _ := Parse(secs)
	var curOpts map[string]string
	if name == "" {
		name = DefaultSection
		if err := u.SetType(ctx, Config, name, "queue"); err != nil {
			return err
		}
		curOpts = map[string]string{}
	} else {
		idx := slices.IndexFunc(secs, func(sec device.Section) bool { return sec.Name == name && sec.Type == "queue" })
		curOpts = secs[idx].Options
	}

	enabled := "0"
	if want.Enabled {
		enabled = "1"
	}
	pairs := [][2]string{
		{"enabled", enabled},
		{"interface", want.Interface},
		{"download", strconv.Itoa(want.Download)},
		{"upload", strconv.Itoa(want.Upload)},
		{"qdisc", want.Qdisc},
		{"script", want.Script},
		{"linklayer", want.Linklayer},
	}
	for _, kv := range pairs {
		if curOpts[kv[0]] != kv[1] {
			if err := u.Set(ctx, Config, name, kv[0], kv[1]); err != nil {
				return err
			}
		}
	}
	// Stock config omits `overhead` when 0; only write it when non-zero or already set to a different value.
	wantOverhead := strconv.Itoa(want.Overhead)
	if curOverhead, ok := curOpts["overhead"]; (ok && curOverhead != wantOverhead) || (!ok && want.Overhead > 0) {
		if err := u.Set(ctx, Config, name, "overhead", wantOverhead); err != nil {
			return err
		}
	}
	return nil
}

func validInterface(name string) bool {
	if len(name) == 0 || len(name) > 15 || name[0] == '-' || name == "." || name == ".." || strings.Contains(name, "..") {
		return false
	}
	for _, c := range name {
		ok := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-'
		if !ok {
			return false
		}
	}
	return true
}
