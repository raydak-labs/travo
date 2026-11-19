package failover

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/atomicfile"
	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/pkgmgr"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

const (
	// Package is the OpenWrt package and its init script.
	Package = "mwan3"
	// InstallGuard covers the package install and the immediate stop of its stock config.
	InstallGuard = "failover-install-in-progress"
	// ServiceGuard covers starting, stopping, enabling or disabling mwan3.
	ServiceGuard = "failover-service-in-progress"
	// ProbeWait bounds the apply probe: mwan3 start, hotplug of every uplink and a real
	// outbound probe (connectivity), each a few seconds on the AXT1800.
	ProbeWait = 45 * time.Second
)

var (
	ErrUnavailable = errors.New("failover is unavailable")
	// ErrOffline: the apply proof is a real outbound request, so it needs one before.
	ErrOffline = errors.New("enable failover while the router is online")
	ErrBusy    = errors.New("a failover install or service change did not finish")
	ErrNotUp   = errors.New("failover did not verify after apply")
)

// Transactions is the slice of txn.Manager the apply needs.
type Transactions interface {
	Begin(ctx context.Context, summary string, configs []string, stage func(context.Context) error, probe txn.Probe) (txn.Status, error)
}

// Service owns the failover JSON and drives mwan3 through the ports.
type Service struct {
	UCI      device.UCI
	Reorder  device.UCIReorderer
	Ubus     device.Ubus
	Services device.Services
	// Runtime reads mwan3's tracking and mangle rules for the apply probe.
	Runtime  device.MWAN3Reader
	Packages pkgmgr.PackageManager
	Guards   guard.Dir
	Txns     Transactions
	// Online is the shared real outbound probe (connectivity.Online).
	Online func(context.Context) bool
	// Path is failover.json in the state dir.
	Path string
	// Ordered: the UCI adapter reports `uci export` order (cli yes, mock no).
	Ordered   bool
	ProbeWait time.Duration

	mu          sync.Mutex
	bootEnabled bool
}

// UplinkState is netifd's view of one candidate (liveness, invariants §9).
type UplinkState struct {
	Name string `json:"name" enum:"wan,wwan,usbtether"`
	Up   bool   `json:"up"`
}

// Status is a read-only view; GET never mutates.
type Status struct {
	Settings
	Installed bool `json:"installed"`
	Running   bool `json:"running" doc:"mwan3 is running"`
	// Active: the committed mwan3 config carries Travo's stamped rules.
	Active   bool          `json:"active"`
	Blocked  bool          `json:"blocked" doc:"An install or service change kept its guard; enabling again retries it"`
	States   []UplinkState `json:"states" nullable:"false"`
	Warnings []Warning     `json:"warnings" nullable:"false"`
}

// DefaultSettings: off, wan before wwan.
var DefaultSettings = Settings{Uplinks: []string{"wan", "wwan"}}

// LoadSettings reads failover.json; absence is the default, a torn file is an error.
func (s *Service) LoadSettings() (Settings, error) {
	var st Settings
	err := atomicfile.ReadJSON(s.Path, &st)
	if errors.Is(err, fs.ErrNotExist) {
		return Settings{Enabled: DefaultSettings.Enabled, Uplinks: slices.Clone(DefaultSettings.Uplinks)}, nil
	}
	if err != nil {
		return Settings{}, err
	}
	if st.Uplinks == nil {
		st.Uplinks = []string{}
	}
	return st, st.Validate()
}

func (s *Service) saveSettings(st Settings) error {
	return atomicfile.WriteJSON(s.Path, st, 0o600)
}

func (s *Service) ready() bool {
	return s.UCI != nil && s.Reorder != nil && s.Ubus != nil && s.Services != nil && s.Runtime != nil && s.Packages != nil &&
		s.Txns != nil && s.Online != nil && s.Guards.Path != "" && s.Path != ""
}

// Status reads settings, package, service, config and netifd.
func (s *Service) Status(ctx context.Context) (Status, error) {
	if !s.ready() {
		return Status{}, ErrUnavailable
	}
	set, err := s.LoadSettings()
	if err != nil {
		return Status{}, errors.Join(ErrUnavailable, err)
	}
	st := Status{Settings: set, States: []UplinkState{}, Warnings: []Warning{}}
	if st.Blocked, err = s.blocked(); err != nil {
		return Status{}, errors.Join(ErrUnavailable, err)
	}
	for _, name := range Candidates {
		st.States = append(st.States, UplinkState{Name: name, Up: s.interfaceUp(ctx, name)})
	}
	if _, err := s.Packages.Version(ctx, Package); errors.Is(err, pkgmgr.ErrNotInstalled) {
		return st, nil
	} else if err != nil {
		return Status{}, errors.Join(ErrUnavailable, err)
	}
	st.Installed = true
	if st.Running, err = s.Services.ServiceRunning(ctx, Package); err != nil {
		return Status{}, errors.Join(ErrUnavailable, err)
	}
	current, err := s.UCI.Show(ctx, Config)
	if err != nil && !errors.Is(err, device.ErrNotFound) {
		return Status{}, errors.Join(ErrUnavailable, err)
	}
	st.Active = Active(current)
	if set.Validate() == nil && len(set.Uplinks) >= 2 {
		if p, err := PlanApply(current, Settings{Enabled: true, Uplinks: set.Uplinks}); err == nil {
			st.Warnings = append(st.Warnings, p.Warnings...)
		}
	}
	return st, nil
}

// Active: the config carries Travo's stamped v4 rule and policy.
func Active(current []device.Section) bool {
	has := func(name string) bool {
		i := slices.IndexFunc(current, func(c device.Section) bool { return c.Name == name })
		return i >= 0 && Stamped(current[i])
	}
	return has(RuleV4) && has(Policy)
}

func (s *Service) interfaceUp(ctx context.Context, name string) bool {
	raw, err := s.Ubus.Call(ctx, "network.interface."+name, "status", nil)
	if err != nil {
		return false
	}
	st, err := facts.ParseInterfaceStatus(raw)
	return err == nil && st.Up
}

func (s *Service) blocked() (bool, error) {
	stale, err := s.Guards.Stale()
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(stale, func(g guard.Info) bool { return g.Name == InstallGuard || g.Name == ServiceGuard }), nil
}

// Enable installs mwan3 if needed and applies the settings in one mwan3 transaction the
// user confirms. The request is the explicit retry: it clears kept failover guards by name.
func (s *Service) Enable(ctx context.Context, uplinks []string) (txn.Status, error) {
	if !s.ready() {
		return txn.Status{}, ErrUnavailable
	}
	set := Settings{Enabled: true, Uplinks: uplinks}
	if err := set.Validate(); err != nil {
		return txn.Status{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, g := range []string{InstallGuard, ServiceGuard} {
		if err := s.Guards.Clear(g); err != nil {
			return txn.Status{}, err
		}
	}
	if !s.Online(ctx) {
		return txn.Status{}, ErrOffline
	}
	if err := s.install(ctx); err != nil {
		return txn.Status{}, err
	}
	current, err := s.UCI.Show(ctx, Config)
	if err != nil {
		return txn.Status{}, errors.Join(ErrUnavailable, err)
	}
	plan, err := PlanApply(current, set)
	if err != nil {
		return txn.Status{}, err
	}
	for _, w := range plan.Warnings {
		slog.Warn("failover: mwan3 section taken over", "section", w.Section, "reason", w.Reason)
	}
	st, err := s.Txns.Begin(ctx, "Enable failover: "+fmt.Sprint(uplinks), []string{Config},
		func(c context.Context) error {
			fresh, err := s.UCI.Show(c, Config)
			if err != nil {
				return err
			}
			p, err := PlanApply(fresh, set)
			if err != nil {
				return err
			}
			plan = p
			return Stage(c, s.UCI, s.Reorder, fresh, p, s.Ordered)
		},
		func(c context.Context) error { return s.prove(c, &plan) })
	if err == nil && st.State == txn.Pending {
		s.bootEnabled = false // enable at boot only after the user confirms (reconciler)
		return st, s.saveSettings(set)
	}
	return st, err
}

// install adds mwan3 and stops it at once: its stock config routes 0/0 into a policy
// with no healthy member (device-facts §9). Kept guard on failure.
func (s *Service) install(ctx context.Context) error {
	if _, err := s.Packages.Version(ctx, Package); err == nil {
		return nil
	} else if !errors.Is(err, pkgmgr.ErrNotInstalled) {
		return fmt.Errorf("query %s: %w", Package, err) // a failed query is not "absent"
	}
	release, err := pkgmgr.Exclusive(ctx)
	if err != nil {
		return err
	}
	defer release()
	if err := s.Guards.AcquireExclusive(InstallGuard, guard.RefuseTerminal); err != nil {
		return fmt.Errorf("failover guard: %w", err)
	}
	if err := s.Packages.Update(ctx); err != nil {
		slog.Warn("failover: package list refresh failed; installing from cached lists", "err", err)
	}
	if err := s.Packages.Install(ctx, Package); err != nil {
		return fmt.Errorf("install %s: %w (guard %s kept)", Package, err, InstallGuard)
	}
	for _, a := range []string{"stop", "disable"} {
		if err := s.Services.ServiceAction(ctx, Package, a); err != nil {
			return fmt.Errorf("%s %s: %w (guard %s kept)", Package, a, err, InstallGuard)
		}
	}
	return s.Guards.Release(InstallGuard)
}

// prove starts mwan3 on the committed config, then waits until every managed section reads
// back as planned, mwan3 runs, tracks every uplink online, routes by travo_failover
// (CheckRuntime) and a real outbound probe is online.
func (s *Service) prove(ctx context.Context, plan *Plan) error {
	if err := s.Services.ServiceAction(ctx, Package, "restart"); err != nil {
		return err
	}
	wait := s.ProbeWait
	if wait <= 0 {
		wait = ProbeWait
	}
	ctx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	var last error
	for {
		if last = s.verified(ctx, plan); last == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return errors.Join(ErrNotUp, last, ctx.Err())
		case <-tick.C:
		}
	}
}

func (s *Service) verified(ctx context.Context, plan *Plan) error {
	current, err := s.UCI.Show(ctx, Config)
	if err != nil {
		return err
	}
	if err := Verify(current, *plan, s.Ordered); err != nil {
		return err
	}
	if running, err := s.Services.ServiceRunning(ctx, Package); err != nil || !running {
		return errors.Join(errors.New("mwan3 is not running"), err)
	}
	if err := s.runtimeVerified(ctx, plan); err != nil {
		return err
	}
	if !s.Online(ctx) {
		return errors.New("no internet through the failover policy")
	}
	return nil
}

func (s *Service) runtimeVerified(ctx context.Context, plan *Plan) error {
	var uplinks []string
	for _, w := range plan.Write {
		if w.Type == "member" {
			uplinks = append(uplinks, w.Options["interface"])
		}
	}
	interfaces, err := s.Runtime.MWAN3Interfaces(ctx)
	if err != nil {
		return fmt.Errorf("mwan3 interfaces: %w", err)
	}
	mangle, err := s.Runtime.MangleRules(ctx)
	if err != nil {
		// iptables-nft refuses a mangle table holding a foreign native nft rule; mwan3
		// cannot manage its policies then (ADR 0018).
		return fmt.Errorf("mwan3 mangle rules unreadable: %w", err)
	}
	return CheckRuntime(interfaces, mangle, uplinks)
}

// Disable records the choice and stops mwan3 (guarded). The config stays: Travo's rules
// do nothing while mwan3 is stopped, and the next enable reuses them.
func (s *Service) Disable(ctx context.Context) error {
	if !s.ready() {
		return ErrUnavailable
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	set, err := s.LoadSettings()
	if err != nil {
		set = Settings{Uplinks: slices.Clone(DefaultSettings.Uplinks)}
	}
	set.Enabled = false
	if err := s.saveSettings(set); err != nil {
		return err
	}
	if err := s.Guards.Clear(ServiceGuard); err != nil {
		return err
	}
	if _, err := s.Packages.Version(ctx, Package); errors.Is(err, pkgmgr.ErrNotInstalled) {
		return nil
	}
	return s.serviceActions(ctx, "stop", "disable")
}

func (s *Service) serviceActions(ctx context.Context, actions ...string) error {
	if err := s.Guards.Acquire(ServiceGuard); err != nil {
		if errors.Is(err, guard.ErrActive) {
			return ErrBusy
		}
		return fmt.Errorf("failover guard: %w", err)
	}
	for _, a := range actions {
		if err := s.Services.ServiceAction(ctx, Package, a); err != nil {
			return fmt.Errorf("%s %s: %w (guard %s kept)", Package, a, err, ServiceGuard)
		}
	}
	return s.Guards.Release(ServiceGuard)
}
