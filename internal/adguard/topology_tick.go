package adguard

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"

	"github.com/blackdark-org/travo-greenfield/internal/atomicfile"
	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/dnslayer"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

// ExpertProbeAddr is where clients and the router itself resolve in expert mode.
const ExpertProbeAddr = LoopbackDNS + ":" + ExpertDNSPort

// LivenessTick is the liveness worker's round (every 15 s). Default topology: the `adguard`
// layer's probe and fail-open (DNS.Tick). A switch in flight: settle it once its transaction
// is confirmed or restored. Expert: probe AGH on 127.0.0.1:53 and revert to default after
// FailThreshold failures. An unfinished revert: retry it.
func (s *Service) LivenessTick(ctx context.Context) error {
	st, err := s.loadTopology()
	if err != nil {
		return err
	}
	switch st.Phase {
	case phaseSwitching:
		return s.settleSwitch(ctx)
	case phaseExpert:
		return s.watchExpert(ctx)
	case phaseReverting:
		return s.revertExpert(ctx, nil)
	}
	if s.DNS == nil {
		return nil
	}
	return s.DNS.Tick(ctx)
}

// RecoverTopology runs at startup: a switch or revert a daemon restart interrupted is
// settled or retried before the workers start.
func (s *Service) RecoverTopology(ctx context.Context) error {
	st, err := s.loadTopology()
	switch {
	case err != nil:
		return err
	case st.Phase == phaseSwitching:
		return s.settleSwitch(ctx)
	case st.Phase == phaseReverting:
		return s.revertExpert(ctx, nil)
	}
	return nil
}

// settleSwitch decides a switch whose transaction is no longer pending by the committed
// mode: the watchdog restores dhcp and the mode together, so the mode tells which side won.
func (s *Service) settleSwitch(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.loadTopology()
	if err != nil || st.Phase != phaseSwitching {
		return err
	}
	if st.TxnID != "" {
		res, err := s.Txns.Get(st.TxnID)
		switch {
		case errors.Is(err, txn.ErrNotFound):
		case err != nil:
			return err
		case res.State == txn.Pending, res.State == txn.Restoring:
			return nil
		case res.State == txn.RollbackFailed:
			return fmt.Errorf("adguard: topology transaction %s rollback failed (guard %s kept)", st.TxnID, TopologyGuard)
		}
	} else if held, err := s.guardHeld(txn.GuardName); err != nil || held {
		return err // crashed before Begin returned: wait for the transaction's own recovery
	}
	travo, err := s.UCI.Show(ctx, TopologyConfig)
	if err != nil && !errors.Is(err, device.ErrNotFound) {
		return err
	}
	if Mode(travo) != st.Target {
		slog.Warn("adguard: topology switch not kept; restoring AdGuard Home's config", "target", st.Target)
		return s.rollBackTopology(ctx, st)
	}
	slog.Info("adguard: topology switch confirmed", "topology", st.Target)
	next := topoState{}
	if st.Target == TopologyExpert {
		next = st
		next.Phase, next.Target, next.TxnID = phaseExpert, "", ""
	}
	s.expertFailures = 0
	return s.endSwitch(next)
}

// watchExpert is the expert liveness round. Like DNS.Tick it skips while a DNS change is
// unfinished, a captive portal hides the Internet or there is no uplink.
func (s *Service) watchExpert(ctx context.Context) error {
	if s.DNS == nil || s.ProbeAt == nil {
		return nil
	}
	ls, err := s.DNS.Layers.Status()
	if err != nil {
		return err
	}
	if ls.Pending || ls.Corrupt != "" || slices.Contains(ls.Layers, dnslayer.Captive) {
		return nil
	}
	if up, err := s.DNS.HasRoute(ctx); err != nil || !up {
		s.expertFailures = 0
		return err
	}
	perr := s.ProbeAt(ExpertProbeAddr)(ctx)
	if perr == nil {
		s.expertFailures = 0
		return nil
	}
	s.expertFailures++
	if s.expertFailures < FailThreshold {
		return nil
	}
	s.expertFailures = 0
	return s.revertExpert(ctx, perr)
}

// revertExpert is the one automatic DNS port change (ADR 0021): AGH leaves :53 first, then
// dnsmasq returns to it through the direct apply, then a restart makes sure it binds. The
// guard and the "reverting" phase stay until every step passed, so the next tick and
// startup retry it. cause is the failed probe (nil on a retry).
func (s *Service) revertExpert(ctx context.Context, cause error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.loadTopology()
	if err != nil || (st.Phase != phaseExpert && st.Phase != phaseReverting) {
		return err
	}
	if cause != nil {
		slog.Warn("adguard: expert topology failed open; dnsmasq takes port 53 back", "probe", cause)
	}
	if st.Phase == phaseExpert {
		if err := s.Guards.Acquire(TopologyGuard); err != nil {
			return err
		}
		st.Phase = phaseReverting
		if err := s.saveTopology(st); err != nil {
			return errors.Join(err, s.Guards.Release(TopologyGuard))
		}
	}
	if err := s.revertSteps(ctx, st); err != nil {
		return fmt.Errorf("adguard: revert expert topology: %w (guard %s kept)", err, TopologyGuard)
	}
	if err := s.DNS.markFailOpen(); err != nil {
		return fmt.Errorf("adguard: record fail-open: %w (guard %s kept)", err, TopologyGuard)
	}
	return s.endSwitch(topoState{})
}

func (s *Service) revertSteps(ctx context.Context, st topoState) error {
	cur, err := os.ReadFile(s.Config)
	if err != nil {
		return err
	}
	next, err := ToDefault(cur, st.Saved)
	if err != nil {
		return err
	}
	if err := atomicfile.Write(s.Config, next, 0o600); err != nil {
		return err
	}
	// AGH may be the broken part: a failed restart must not keep dnsmasq off :53.
	if err := s.Services.ServiceAction(ctx, Package, "restart"); err != nil {
		slog.Warn("adguard: restart during revert", "err", err)
	}
	if err := s.Txns.Apply(ctx, TopologyConfigs, func(ctx context.Context) error {
		return StageEdits(ctx, s.UCI, Inverse(st.Edits))
	}); err != nil {
		return err
	}
	return s.Services.ServiceAction(ctx, "dnsmasq", "restart")
}
