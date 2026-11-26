package adguard

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/pkgmgr"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

// fakeTxns keeps a passing switch staged (pending): commit = confirm, revert = the
// watchdog's restore. A failed probe reverts at once like txn.Manager's immediate restore.
type fakeTxns struct {
	r        *dnsRig
	getState txn.State
	getErr   error
	applyErr error
}

const topoTxnID = "0123456789abcdef"

func (f *fakeTxns) Begin(ctx context.Context, summary string, configs []string, stage func(context.Context) error, probe txn.Probe) (txn.Status, error) {
	if err := stage(ctx); err != nil {
		return txn.Status{}, errors.Join(err, f.revert(ctx))
	}
	meta := txn.Meta{ID: topoTxnID, Configs: configs, Summary: summary, WindowSeconds: 60}
	if err := probe(ctx); err != nil {
		return txn.Status{Meta: meta, State: txn.RolledBack}, errors.Join(txn.ErrProbeFailed, err, f.revert(ctx))
	}
	f.getState, f.getErr = txn.Pending, nil
	return txn.Status{Meta: meta, State: txn.Pending}, nil
}

func (f *fakeTxns) Get(string) (txn.Status, error) {
	return txn.Status{Meta: txn.Meta{ID: topoTxnID}, State: f.getState}, f.getErr
}

func (f *fakeTxns) Apply(ctx context.Context, configs []string, stage func(context.Context) error) error {
	if f.applyErr != nil {
		return f.applyErr
	}
	if err := stage(ctx); err != nil {
		return errors.Join(err, f.revert(ctx))
	}
	return f.commit(ctx)
}

func (f *fakeTxns) commit(ctx context.Context) error {
	f.getErr = txn.ErrNotFound
	return errors.Join(f.r.dev.Commit(ctx, "dhcp"), f.r.dev.Commit(ctx, TopologyConfig))
}

func (f *fakeTxns) revert(ctx context.Context) error {
	f.getErr = txn.ErrNotFound
	return errors.Join(f.r.dev.Revert(ctx, "dhcp"), f.r.dev.Revert(ctx, TopologyConfig))
}

type topoRig struct {
	*dnsRig
	svc    *Service
	txns   *fakeTxns
	probes map[string]error
	asked  []string
}

func newTopoRig(t *testing.T) *topoRig {
	t.Helper()
	ctx := context.Background()
	r := &topoRig{dnsRig: newDNSRig(t), probes: map[string]error{}}
	r.txns = &fakeTxns{r: r.dnsRig}
	if err := r.dev.Load(ctx, "network", fixtureSections(t, "network", false, "uci-show-network.txt")); err != nil {
		t.Fatal(err)
	}
	if err := r.dev.Commit(ctx, "network"); err != nil {
		t.Fatal(err)
	}
	r.dev.SetService(Package, true, true)
	r.dev.SetService("dnsmasq", true, true)
	pkgs := pkgmgr.NewMock()
	if err := pkgs.Install(ctx, Package); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	r.svc = &Service{
		Packages: pkgs, Services: r.dev, Guards: guard.Dir{Path: filepath.Join(dir, "guards"), Now: time.Now},
		Config: filepath.Join(dir, "adguardhome.yaml"), Backup: filepath.Join(dir, "adguardhome.yaml.old"),
		Ready: func(context.Context) error { return nil }, DNS: r.d,
		UCI: r.dev, Txns: r.txns, SettleWait: 20 * time.Millisecond,
		TopologyState: filepath.Join(dir, "adguard-topology.json"), TopologyBackup: filepath.Join(dir, "adguardhome.yaml.topology"),
		ProbeAt: func(addr string) func(context.Context) error {
			return func(context.Context) error { r.asked = append(r.asked, addr); return r.probes[addr] }
		},
	}
	if err := os.WriteFile(r.svc.Config, Seed, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := r.d.Tick(ctx); err != nil { // adopt old Travo's forwarding as the adguard layer
		t.Fatal(err)
	}
	r.requireLayer(t, true)
	return r
}

func (r *topoRig) requireLayer(t *testing.T, want bool) {
	t.Helper()
	st, err := r.d.Status()
	if err != nil || st.DefaultDNS != want {
		t.Fatalf("adguard layer active=%v, want %v (%v)", st.DefaultDNS, want, err)
	}
}

func (r *topoRig) config(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(r.svc.Config)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func (r *topoRig) option(t *testing.T, config, section, option string) (string, bool) {
	t.Helper()
	secs, err := r.dev.Show(context.Background(), config)
	if err != nil && !errors.Is(err, device.ErrNotFound) {
		t.Fatal(err)
	}
	for _, s := range secs {
		if s.Name == section || (section == "dnsmasq" && s.Type == "dnsmasq") {
			v, ok := s.Options[option]
			return v, ok
		}
	}
	return "", false
}

func (r *topoRig) status(t *testing.T) Status {
	t.Helper()
	st, err := r.svc.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return st
}

// expert switches and confirms.
func (r *topoRig) expert(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	if st, err := r.svc.SetTopology(ctx, TopologyExpert); err != nil || st.State != txn.Pending {
		t.Fatalf("%+v %v", st, err)
	}
	if err := r.txns.commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := r.svc.LivenessTick(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestTopologyExpertConfirmed(t *testing.T) {
	r := newTopoRig(t)
	ctx := context.Background()
	st, err := r.svc.SetTopology(ctx, TopologyExpert)
	if err != nil || st.State != txn.Pending || st.ID != topoTxnID {
		t.Fatalf("%+v %v", st, err)
	}
	hosts := []string{"127.0.0.1", "192.168.1.1"}
	if !IsExpert(r.config(t), hosts) {
		t.Fatal("AGH must bind :53 once the probe passed")
	}
	if want := []string{"127.0.0.1:5354", "127.0.0.1:53", "192.168.1.1:53"}; !slices.Equal(r.asked, want) {
		t.Fatalf("dnsmasq must answer on its new port before AGH takes :53, then every address: %v", r.asked)
	}
	r.requireLayer(t, false) // AGH -> dnsmasq -> AGH would loop
	if v, _ := r.option(t, "dhcp", "dnsmasq", "port"); v != "5354" {
		t.Fatalf("dnsmasq port %q", v)
	}
	if s := r.status(t); !s.Busy || s.Topology != TopologyDefault {
		t.Fatalf("pending switch: %+v", s)
	}
	if err := r.svc.LivenessTick(ctx); err != nil || !r.status(t).Busy {
		t.Fatalf("a pending transaction is left to the user: %v", err)
	}
	if err := r.txns.commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := r.svc.LivenessTick(ctx); err != nil {
		t.Fatal(err)
	}
	s := r.status(t)
	if s.Busy || s.Topology != TopologyExpert || !slices.Equal(s.Hosts, hosts) || !s.Loopback {
		t.Fatalf("confirmed: %+v", s)
	}
	if _, err := os.Stat(r.svc.TopologyBackup); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("kept YAML must go once settled: %v", err)
	}
	if err := r.svc.Reconcile(ctx); err != nil || !IsExpert(r.config(t), hosts) {
		t.Fatalf("Reconcile must never migrate the expert config back: %v", err)
	}
	for name, err := range map[string]error{
		"doh":         r.svc.Idle(),
		"default dns": r.svc.SetDefaultDNS(ctx, true),
		"stop":        r.svc.SetEnabled(ctx, false),
	} {
		if !errors.Is(err, ErrExpertTopology) {
			t.Errorf("%s in expert mode: %v", name, err)
		}
	}
	if _, err := r.svc.SetTopology(ctx, TopologyExpert); !errors.Is(err, txn.ErrNoChanges) {
		t.Fatalf("same mode: %v", err)
	}
}

func TestTopologyExpertProbeFailureRestoresBoth(t *testing.T) {
	r := newTopoRig(t)
	ctx := context.Background()
	r.probes["192.168.1.1:53"] = ErrProbe
	r.dev.ServiceActions = nil
	st, err := r.svc.SetTopology(ctx, TopologyExpert)
	if st.State != txn.RolledBack || !errors.Is(err, txn.ErrProbeFailed) {
		t.Fatalf("%+v %v", st, err)
	}
	if !bytes.Equal(r.config(t), Seed) {
		t.Fatal("AGH's YAML must be restored exactly")
	}
	if v, _ := r.option(t, "dhcp", "dnsmasq", "port"); v != "53" {
		t.Fatalf("dnsmasq port %q", v)
	}
	want := []string{"adguardhome restart", "adguardhome restart", "dnsmasq restart"}
	if !slices.Equal(r.dev.ServiceActions, want) {
		t.Fatalf("AGH leaves :53, then dnsmasq retakes it: %v", r.dev.ServiceActions)
	}
	r.requireLayer(t, true)
	if s := r.status(t); s.Busy || s.Topology != TopologyDefault {
		t.Fatalf("settled default: %+v", s)
	}
}

func TestTopologyExpertNotConfirmedRestoresBoth(t *testing.T) {
	r := newTopoRig(t)
	ctx := context.Background()
	if _, err := r.svc.SetTopology(ctx, TopologyExpert); err != nil {
		t.Fatal(err)
	}
	if err := r.txns.revert(ctx); err != nil { // the watchdog's restore after the window
		t.Fatal(err)
	}
	r.dev.ServiceActions = nil
	if err := r.svc.LivenessTick(ctx); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(r.config(t), Seed) || !slices.Equal(r.dev.ServiceActions, []string{"adguardhome restart", "dnsmasq restart"}) {
		t.Fatalf("restore: %v", r.dev.ServiceActions)
	}
	r.requireLayer(t, true)
	if s := r.status(t); s.Busy || s.Topology != TopologyDefault {
		t.Fatalf("settled default: %+v", s)
	}
}

func TestTopologyExpertFailsOpenToDnsmasq(t *testing.T) {
	r := newTopoRig(t)
	ctx := context.Background()
	r.expert(t)
	r.probes[ExpertProbeAddr] = ErrProbe
	r.txns.applyErr = errors.New("uci: locked")
	for range FailThreshold {
		_ = r.svc.LivenessTick(ctx)
	}
	if s := r.status(t); !s.Busy || s.Topology != TopologyExpert {
		t.Fatalf("a failed revert keeps its guard for the retry: %+v", s)
	}
	r.txns.applyErr = nil
	if err := r.svc.LivenessTick(ctx); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(r.config(t), Seed) {
		t.Fatal("AGH back on 127.0.0.1:5353 with its own upstreams")
	}
	if v, _ := r.option(t, "dhcp", "dnsmasq", "port"); v != "53" {
		t.Fatalf("dnsmasq port %q", v)
	}
	if _, ok := r.option(t, TopologyConfig, TopologySection, TopologyOption); ok {
		t.Fatal("mode must be back to default")
	}
	s := r.status(t)
	if s.Busy || s.Topology != TopologyDefault || s.FailOpenSince == nil || s.DefaultDNS {
		t.Fatalf("after fail-open: %+v", s)
	}
}

func TestTopologyBackToDefault(t *testing.T) {
	r := newTopoRig(t)
	ctx := context.Background()
	r.expert(t)
	r.asked, r.dev.ServiceActions = nil, nil
	st, err := r.svc.SetTopology(ctx, TopologyDefault)
	if err != nil || st.State != txn.Pending {
		t.Fatalf("%+v %v", st, err)
	}
	if !bytes.Equal(r.config(t), Seed) || !slices.Equal(r.dev.ServiceActions, []string{"adguardhome restart", "dnsmasq restart"}) {
		t.Fatalf("AGH leaves :53 before dnsmasq is restarted onto it: %v", r.dev.ServiceActions)
	}
	if err := r.txns.commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := r.svc.LivenessTick(ctx); err != nil {
		t.Fatal(err)
	}
	if s := r.status(t); s.Busy || s.Topology != TopologyDefault {
		t.Fatalf("%+v", s)
	}
	if _, err := os.Stat(r.svc.TopologyState); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("default keeps no topology state")
	}
	if err := r.svc.LivenessTick(ctx); err != nil {
		t.Fatal(err)
	}
	r.requireLayer(t, true) // the default-DNS logic of ADR 0006 runs again
}

func TestTopologyRefusals(t *testing.T) {
	r := newTopoRig(t)
	ctx := context.Background()
	if _, err := r.svc.SetTopology(ctx, "auto"); !errors.Is(err, ErrTopologyInvalid) {
		t.Fatal(err)
	}
	if _, err := r.svc.SetTopology(ctx, TopologyDefault); !errors.Is(err, txn.ErrNoChanges) {
		t.Fatal(err)
	}
	r.dev.SetService(Package, true, false)
	if _, err := r.svc.SetTopology(ctx, TopologyExpert); !errors.Is(err, ErrTopologyNotReady) {
		t.Fatalf("AGH not running: %v", err)
	}
	r.requireLayer(t, true)
}
