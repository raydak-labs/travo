package adguard

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"slices"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/atomicfile"
	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

// TopologyGuard covers a topology switch from before its transaction until the outcome is
// settled on both sides (dhcp and AGH's YAML), and the automatic revert.
const TopologyGuard = "adguard-topology-in-progress"

// DefaultSettleWait bounds waiting for dnsmasq on its new port and for AGH to resolve.
const DefaultSettleWait = 20 * time.Second

var (
	ErrTopologyInvalid     = errors.New("mode must be default or expert")
	ErrTopologyUnavailable = errors.New("the AdGuard Home topology switch is unavailable")
	// ErrTopologyNotReady: expert needs AGH installed, running and on the loopback topology.
	ErrTopologyNotReady = errors.New("AdGuard Home must be installed, running and on the loopback topology")
	// ErrExpertTopology refuses a change that would break DNS while AGH owns port 53.
	ErrExpertTopology = errors.New("AdGuard Home answers DNS on port 53 (expert topology); switch back to the default topology first")
)

// TopologyTxns is the slice of txn.Manager the switch needs.
type TopologyTxns interface {
	Begin(ctx context.Context, summary string, configs []string, stage func(context.Context) error, probe txn.Probe) (txn.Status, error)
	Get(id string) (txn.Status, error)
	Apply(ctx context.Context, configs []string, stage func(context.Context) error) error
}

// Topology phases in the state file. No file = default topology, nothing in flight.
const (
	phaseSwitching = "switching"
	phaseExpert    = "expert"
	phaseReverting = "reverting"
)

// topoState is written on switch and revert only (rare flash writes).
type topoState struct {
	Phase  string `json:"phase"`
	Target string `json:"target,omitempty"`
	TxnID  string `json:"txn_id,omitempty"`
	// Hosts AGH binds in expert mode; Saved the default lines it replaced; Edits the
	// expert plan's UCI changes (their inverse is the way back).
	Hosts          []string  `json:"hosts"`
	Saved          SavedDNS  `json:"saved"`
	Edits          []UCIEdit `json:"edits"`
	LayerWasActive bool      `json:"layer_was_active,omitempty"`
}

// TopologyStatus is part of the AdGuard status.
type TopologyStatus struct {
	Topology string   `json:"topology" enum:"default,expert" doc:"default: dnsmasq on :53 forwards to AGH; expert: AGH on :53 forwards to dnsmasq (ADR 0021)"`
	Hosts    []string `json:"topology_hosts,omitempty" doc:"Addresses AGH answers on port 53 in expert mode"`
}

// SetTopology is the user's switch. Expert runs as one transaction the user confirms;
// back to default too, so a failed return to dnsmasq on :53 restores expert exactly.
func (s *Service) SetTopology(ctx context.Context, mode string) (txn.Status, error) {
	if mode != TopologyDefault && mode != TopologyExpert {
		return txn.Status{}, ErrTopologyInvalid
	}
	if s.UCI == nil || s.Txns == nil || s.DNS == nil || s.ProbeAt == nil {
		return txn.Status{}, ErrTopologyUnavailable
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.idle(); err != nil {
		return txn.Status{}, err
	}
	st, err := s.loadTopology()
	if err != nil {
		return txn.Status{}, err
	}
	if st.Phase == phaseSwitching || st.Phase == phaseReverting {
		return txn.Status{}, ErrBusy
	}
	if (st.Phase == phaseExpert) == (mode == TopologyExpert) {
		return txn.Status{}, txn.ErrNoChanges
	}
	cur, err := os.ReadFile(s.Config)
	if err != nil {
		return txn.Status{}, err
	}
	if mode == TopologyDefault {
		next, err := ToDefault(cur, st.Saved)
		if err != nil {
			return txn.Status{}, err
		}
		st.Phase, st.Target = phaseSwitching, TopologyDefault
		return s.switchTopology(ctx, st, cur, next, Inverse(st.Edits), "AdGuard Home off port 53 (default DNS topology)")
	}
	st, next, err := s.planExpert(ctx, cur)
	if err != nil {
		return txn.Status{}, err
	}
	if st.LayerWasActive, err = s.DNS.popForExpert(ctx); err != nil {
		return txn.Status{}, err
	}
	return s.switchTopology(ctx, st, cur, next, st.Edits, "AdGuard Home on port 53 (expert DNS topology)")
}

func (s *Service) planExpert(ctx context.Context, cur []byte) (topoState, []byte, error) {
	if _, err := s.Packages.Version(ctx, Package); err != nil {
		return topoState{}, nil, errors.Join(ErrTopologyNotReady, err)
	}
	if running, err := s.Services.ServiceRunning(ctx, Package); err != nil || !running {
		return topoState{}, nil, errors.Join(ErrTopologyNotReady, err)
	}
	if l, err := ParseListen(cur); err != nil || !l.IsLoopback() {
		return topoState{}, nil, errors.Join(ErrTopologyNotReady, err)
	}
	var cfg [3][]device.Section
	for i, name := range []string{"dhcp", "network", TopologyConfig} {
		secs, err := s.UCI.Show(ctx, name)
		if err != nil && !(name == TopologyConfig && errors.Is(err, device.ErrNotFound)) {
			return topoState{}, nil, err
		}
		cfg[i] = secs
	}
	plan, err := PlanExpert(cfg[0], cfg[1], cfg[2])
	if err != nil {
		return topoState{}, nil, err
	}
	next, saved, err := ToExpert(cur, plan.Hosts)
	if err != nil {
		return topoState{}, nil, err
	}
	return topoState{Phase: phaseSwitching, Target: TopologyExpert, Hosts: plan.Hosts, Saved: saved, Edits: plan.Edits}, next, nil
}

// switchTopology runs the transaction: dhcp and the mode are staged and committed, then the
// probe swaps AGH's YAML and proves DNS on port 53. Every outcome but "pending" is settled
// here; a pending one is settled by TopologyTick once it is confirmed or restored.
func (s *Service) switchTopology(ctx context.Context, st topoState, cur, next []byte, edits []UCIEdit, summary string) (txn.Status, error) {
	if err := s.Guards.Acquire(TopologyGuard); err != nil {
		return txn.Status{}, errors.Join(err, s.restoreLayer(ctx, st))
	}
	if err := atomicfile.Write(s.TopologyBackup, cur, 0o600); err != nil {
		return txn.Status{}, errors.Join(err, s.undoUnstarted(ctx, st))
	}
	if err := s.saveTopology(st); err != nil {
		return txn.Status{}, errors.Join(err, s.undoUnstarted(ctx, st))
	}
	stage := func(ctx context.Context) error { return StageEdits(ctx, s.UCI, edits) }
	res, err := s.Txns.Begin(ctx, summary, TopologyConfigs, stage, func(ctx context.Context) error { return s.probeSwitch(ctx, st, next) })
	switch {
	case res.ID == "":
		return res, errors.Join(err, s.undoUnstarted(ctx, st))
	case res.State == txn.RolledBack:
		return res, errors.Join(err, s.rollBackTopology(context.WithoutCancel(ctx), st))
	}
	st.TxnID = res.ID
	return res, errors.Join(err, s.saveTopology(st))
}

// undoUnstarted cleans up when nothing was committed: AGH's YAML was not touched yet.
func (s *Service) undoUnstarted(ctx context.Context, st topoState) error {
	return errors.Join(s.restoreLayer(ctx, st), s.endSwitch(prevTopology(st)))
}

// restoreLayer puts back the `adguard` layer a failed switch to expert took off.
func (s *Service) restoreLayer(ctx context.Context, st topoState) error {
	if st.Target != TopologyExpert || !st.LayerWasActive {
		return nil
	}
	return s.DNS.restoreLayer(context.WithoutCancel(ctx))
}

// prevTopology is the settled state a switch started from.
func prevTopology(st topoState) topoState {
	if st.Target == TopologyExpert {
		return topoState{}
	}
	st.Phase, st.Target, st.TxnID = phaseExpert, "", ""
	return st
}

// probeSwitch runs after dhcp is committed and reloaded. Expert: dnsmasq answers on its new
// port first, then AGH takes :53. Default: AGH leaves :53, then dnsmasq is restarted to bind
// it. Then a fresh name must resolve on port 53 of every address clients use.
func (s *Service) probeSwitch(ctx context.Context, st topoState, next []byte) error {
	if st.Target == TopologyExpert {
		if err := s.retryProbe(ctx, ExpertUpstream); err != nil {
			return fmt.Errorf("dnsmasq on %s: %w", ExpertUpstream, err)
		}
	}
	if err := atomicfile.Write(s.Config, next, 0o600); err != nil {
		return err
	}
	if err := s.Services.ServiceAction(ctx, Package, "restart"); err != nil {
		return err
	}
	if err := s.waitReady(ctx); err != nil {
		return fmt.Errorf("%s not answering: %w", Package, err)
	}
	if st.Target == TopologyDefault {
		if err := s.Services.ServiceAction(ctx, "dnsmasq", "restart"); err != nil {
			return err
		}
	}
	for _, h := range st.Hosts {
		addr := net.JoinHostPort(h, "53")
		if err := s.retryProbe(ctx, addr); err != nil {
			return fmt.Errorf("DNS on %s: %w", addr, err)
		}
	}
	return nil
}

// retryProbe asks addr until a fresh name resolves or the settle wait ends.
func (s *Service) retryProbe(ctx context.Context, addr string) error {
	wait := s.SettleWait
	if wait <= 0 {
		wait = DefaultSettleWait
	}
	ctx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	probe := s.ProbeAt(addr)
	for {
		err := probe(ctx)
		if err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return err
		case <-time.After(time.Second):
		}
	}
}

// rollBackTopology puts AGH's kept YAML back after the transaction restored dhcp: restart
// AGH (it leaves or retakes :53), then dnsmasq (it may have failed to bind :53 while AGH
// still held it). Any failure keeps the guard; TopologyTick retries.
func (s *Service) rollBackTopology(ctx context.Context, st topoState) error {
	old, err := os.ReadFile(s.TopologyBackup)
	if err == nil {
		err = atomicfile.Write(s.Config, old, 0o600)
	}
	if err == nil {
		err = s.Services.ServiceAction(ctx, Package, "restart")
	}
	if err == nil {
		err = s.waitReady(ctx)
	}
	if err == nil {
		err = s.Services.ServiceAction(ctx, "dnsmasq", "restart")
	}
	if err == nil {
		err = s.restoreLayer(ctx, st)
	}
	if err != nil {
		return fmt.Errorf("adguard: restore topology: %w (guard %s kept)", err, TopologyGuard)
	}
	return s.endSwitch(prevTopology(st))
}

// endSwitch persists the settled state, drops the kept YAML and releases the guard.
func (s *Service) endSwitch(st topoState) error {
	err := s.saveTopology(st)
	if err == nil {
		if rerr := os.Remove(s.TopologyBackup); rerr != nil && !errors.Is(rerr, os.ErrNotExist) {
			err = rerr
		}
	}
	if err != nil {
		return fmt.Errorf("adguard: settle topology: %w (guard %s kept)", err, TopologyGuard)
	}
	return s.Guards.Release(TopologyGuard)
}

func (s *Service) loadTopology() (topoState, error) {
	var st topoState
	if s.TopologyState == "" {
		return st, nil
	}
	err := atomicfile.ReadJSON(s.TopologyState, &st)
	if errors.Is(err, os.ErrNotExist) {
		return topoState{}, nil
	}
	return st, err
}

// saveTopology writes the state; the zero state (default, settled) removes the file.
func (s *Service) saveTopology(st topoState) error {
	if st.Phase == "" {
		if err := os.Remove(s.TopologyState); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	return atomicfile.WriteJSON(s.TopologyState, st, 0o600)
}

// topologyStatus never mutates.
func (s *Service) topologyStatus() (TopologyStatus, string, error) {
	st, err := s.loadTopology()
	if err != nil {
		return TopologyStatus{Topology: TopologyDefault}, "", err
	}
	out := TopologyStatus{Topology: TopologyDefault}
	if st.Phase == phaseExpert || (st.Phase == phaseSwitching && st.Target == TopologyDefault) || st.Phase == phaseReverting {
		out.Topology, out.Hosts = TopologyExpert, slices.Clone(st.Hosts)
	}
	return out, st.Phase, nil
}

// expertGuard refuses changes that would break DNS while AGH owns (or may own) port 53.
func (s *Service) expertGuard() error {
	_, phase, err := s.topologyStatus()
	if err == nil && phase != "" {
		err = ErrExpertTopology
	}
	return err
}

// guardHeld reports whether name is held (the txn guard while a transaction is open).
func (s *Service) guardHeld(name string) (bool, error) {
	g, err := s.Guards.Stale()
	return slices.ContainsFunc(g, func(x guard.Info) bool { return x.Name == name }), err
}
