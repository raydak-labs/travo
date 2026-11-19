package failover

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/pkgmgr"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

// fakeTxns commits and probes like Begin; rollback restores the snapshot when the probe fails.
type fakeTxns struct {
	d        *mock.Device
	guards   guard.Dir
	probeErr error
	hold     bool // keep the txn guard as if the user has not confirmed yet
}

func (f *fakeTxns) Begin(ctx context.Context, _ string, configs []string, stage func(context.Context) error, probe txn.Probe) (txn.Status, error) {
	before, _ := f.d.Show(ctx, Config)
	if err := stage(ctx); err != nil {
		_ = f.d.Revert(ctx, Config)
		return txn.Status{}, err
	}
	if err := f.d.Commit(ctx, Config); err != nil {
		return txn.Status{}, err
	}
	if f.probeErr = probe(ctx); f.probeErr != nil {
		after, _ := f.d.Show(ctx, Config) // the watchdog restores the whole file
		for _, s := range after {
			_ = f.d.DeleteSection(ctx, Config, s.Name)
		}
		_ = f.d.Load(ctx, Config, before)
		_ = f.d.Commit(ctx, Config)
		return txn.Status{Meta: txn.Meta{ID: "0123456789abcdef"}, State: txn.RolledBack}, txn.ErrProbeFailed
	}
	if f.hold {
		_ = f.guards.Acquire(txn.GuardName)
	}
	return txn.Status{Meta: txn.Meta{ID: "0123456789abcdef", Configs: configs}, State: txn.Pending}, nil
}

type rig struct {
	s      *Service
	d      *mock.Device
	pm     *pkgmgr.Mock
	txns   *fakeTxns
	online bool
}

func newRig(t *testing.T, installed bool) *rig {
	t.Helper()
	d := stockRouter(t)
	d.SetService(Package, false, false)
	pm := pkgmgr.NewMock()
	if installed {
		_ = pm.Install(context.Background(), Package)
	}
	g := guard.Dir{Path: filepath.Join(t.TempDir(), "guards"), Now: time.Now}
	r := &rig{d: d, pm: pm, online: true}
	r.txns = &fakeTxns{d: d, guards: g}
	r.s = &Service{UCI: d, Reorder: d, Ubus: d, Services: d, Runtime: d, Mangle: d, Packages: pm, Guards: g, Txns: r.txns, Exclusive: new(sync.Mutex),
		Online: func(context.Context) bool { return r.online }, Path: filepath.Join(t.TempDir(), "failover.json"),
		ProbeWait: 100 * time.Millisecond}
	return r
}

func TestEnableInstallsStopsStockThenAppliesAndProves(t *testing.T) {
	r := newRig(t, false)
	st, err := r.s.Enable(context.Background(), []string{"wwan", "wan"})
	if err != nil || st.State != txn.Pending {
		t.Fatalf("enable = %+v, %v (probe %v)", st, err, r.txns.probeErr)
	}
	if v, _ := r.pm.Version(context.Background(), Package); v == "" {
		t.Fatal("mwan3 not installed")
	}
	if !slices.Equal(r.d.ServiceActions, []string{"mwan3 stop", "mwan3 disable", "mwan3 restart"}) {
		t.Fatalf("service actions = %v (stock config must be stopped before any start)", r.d.ServiceActions)
	}
	set, _ := r.s.LoadSettings()
	if !set.Enabled || !slices.Equal(set.Uplinks, []string{"wwan", "wan"}) {
		t.Fatalf("settings = %+v", set)
	}
	if !Active(show(t, r.d)) {
		t.Fatal("config not active")
	}
}

func TestEnableRefusesOfflineAndBadSettings(t *testing.T) {
	r := newRig(t, true)
	r.online = false
	if _, err := r.s.Enable(context.Background(), []string{"wan", "wwan"}); !errors.Is(err, ErrOffline) {
		t.Fatalf("offline: %v", err)
	}
	r.online = true
	if _, err := r.s.Enable(context.Background(), []string{"wan"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("one uplink: %v", err)
	}
	if len(r.d.ServiceActions) != 0 {
		t.Fatalf("refusals acted: %v", r.d.ServiceActions)
	}
}

func TestProbeFailureRollsBackAndReconcilerStopsMwan3(t *testing.T) {
	r := newRig(t, true)
	calls := 0
	r.s.Online = func(context.Context) bool { calls++; return calls == 1 } // online before, not after
	st, err := r.s.Enable(context.Background(), []string{"wan", "wwan"})
	if !errors.Is(err, txn.ErrProbeFailed) || st.State != txn.RolledBack || !errors.Is(r.txns.probeErr, ErrNotUp) {
		t.Fatalf("enable = %+v, %v (probe %v)", st, err, r.txns.probeErr)
	}
	if Active(show(t, r.d)) {
		t.Fatal("rolled-back config still active")
	}
	if set, _ := r.s.LoadSettings(); set.Enabled {
		t.Fatal("a rolled-back enable was recorded")
	}
	// mwan3 was started by the probe and now runs the restored stock config.
	if err := r.s.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if running, _ := r.d.ServiceRunning(context.Background(), Package); running {
		t.Fatal("reconciler left mwan3 running on the stock config")
	}
}

// #277: a request through the main table is no proof. The probe needs every uplink tracked
// online and the mangle rules routing by travo_failover; otherwise the apply rolls back.
func TestProbeNeedsMwan3RoutingByThePolicy(t *testing.T) {
	for name, script := range map[string]func(*mock.Device){
		"backup offline": func(d *mock.Device) { d.MWAN3Offline = []string{"wwan"} },
		"mangle unreadable": func(d *mock.Device) {
			d.MangleErr = errors.New("iptables v1.8.10 (nf_tables): table `mangle' is incompatible, use 'nft' tool.")
		},
	} {
		t.Run(name, func(t *testing.T) {
			r := newRig(t, true)
			script(r.d)
			st, err := r.s.Enable(context.Background(), []string{"wan", "wwan"})
			if !errors.Is(err, txn.ErrProbeFailed) || st.State != txn.RolledBack || !errors.Is(r.txns.probeErr, ErrNotUp) {
				t.Fatalf("enable = %+v, %v (probe %v)", st, err, r.txns.probeErr)
			}
			if Active(show(t, r.d)) {
				t.Fatal("rolled-back config still active")
			}
		})
	}
}

func TestReconcilerWaitsForConfirmThenBootEnables(t *testing.T) {
	r := newRig(t, true)
	r.txns.hold = true
	if _, err := r.s.Enable(context.Background(), []string{"wan", "wwan"}); err != nil {
		t.Fatal(err)
	}
	n := len(r.d.ServiceActions)
	if err := r.s.Reconcile(context.Background()); err != nil || len(r.d.ServiceActions) != n {
		t.Fatalf("acted while pending: %v %v", r.d.ServiceActions, err)
	}
	if err := r.txns.guards.Release(txn.GuardName); err != nil { // the user confirmed
		t.Fatal(err)
	}
	if err := r.s.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := r.d.ServiceActions[n:]; !slices.Equal(got, []string{"mwan3 enable"}) {
		t.Fatalf("after confirm: %v", got)
	}
	if err := r.s.Reconcile(context.Background()); err != nil || len(r.d.ServiceActions) != n+1 {
		t.Fatalf("second tick acted again: %v", r.d.ServiceActions)
	}
}

func TestDisableStopsAndKeepsConfig(t *testing.T) {
	r := newRig(t, true)
	if _, err := r.s.Enable(context.Background(), []string{"wan", "wwan"}); err != nil {
		t.Fatal(err)
	}
	if err := r.s.Disable(context.Background()); err != nil {
		t.Fatal(err)
	}
	if running, _ := r.d.ServiceRunning(context.Background(), Package); running {
		t.Fatal("mwan3 still running")
	}
	if !Active(show(t, r.d)) {
		t.Fatal("disable removed the config")
	}
	st, err := r.s.Status(context.Background())
	if err != nil || st.Enabled || st.Running || !st.Active || !st.Installed {
		t.Fatalf("status = %+v, %v", st, err)
	}
}

func TestServiceFailureKeepsGuardAndBlocks(t *testing.T) {
	r := newRig(t, true)
	if _, err := r.s.Enable(context.Background(), []string{"wan", "wwan"}); err != nil {
		t.Fatal(err)
	}
	r.d.ServiceErr = errors.New("init script failed")
	if err := r.s.Disable(context.Background()); err == nil {
		t.Fatal("want error")
	}
	st, err := r.s.Status(context.Background())
	if err != nil || !st.Blocked {
		t.Fatalf("status = %+v, %v", st, err)
	}
	n := len(r.d.ServiceActions)
	if err := r.s.Reconcile(context.Background()); err != nil || len(r.d.ServiceActions) != n {
		t.Fatalf("reconciler acted past a kept guard: %v", err)
	}
}

func TestStatusWithoutPackage(t *testing.T) {
	r := newRig(t, false)
	st, err := r.s.Status(context.Background())
	if err != nil || st.Installed || st.Enabled || len(st.States) != 3 || !slices.Equal(st.Uplinks, []string{"wan", "wwan"}) {
		t.Fatalf("status = %+v, %v", st, err)
	}
	if _, err := (&Service{}).Status(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unwired: %v", err)
	}
}
