// Package sqm manages essential UCI `sqm.@queue[0]` settings for `sqm-scripts` (#302),
// leaving unknown options and extra queue sections untouched for LuCI.
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
	Package          = "sqm-scripts"
	Config           = "sqm"
	DefaultSection   = "eth1"
	DefaultInterface = "eth1"
	MaxBandwidth     = 10_000_000 // 10 Gbit/s in kbit/s
	MaxOverhead      = 256
)

var (
	Qdiscs     = []string{"cake", "fq_codel"}
	Scripts    = []string{"piece_of_cake.qos", "layer_cake.qos", "simple.qos", "simplest.qos", "simplest_tbf.qos"}
	Linklayers = []string{"none", "ethernet", "atm"}

	ErrUnavailable = errors.New("SQM settings are unavailable")
)

type FieldError struct {
	Field  string
	Reason string
}

func (e *FieldError) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Reason) }

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

type Applier interface {
	Apply(ctx context.Context, configs []string, stage func(ctx context.Context) error) error
}

type Service struct {
	UCI      device.UCI
	Packages pkgmgr.PackageManager
	Apply    Applier

	mu sync.Mutex
}

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

func Parse(secs []device.Section) (string, Settings) {
	out := Settings{
		Interface: DefaultInterface,
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
	// Stock config omits overhead; don't add overhead '0'.
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
