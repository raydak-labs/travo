package vpndns

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device/cli"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/dnslayer"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/vpnprofiles"
	"github.com/blackdark-org/travo-greenfield/internal/vpnstatus"
)

var ctx = context.Background()

// The router's captured dnsmasq config (testdata/device/axt1800-25.12.6/PROVENANCE.md).
const dhcpFixture = "../../testdata/device/axt1800-25.12.6/uci-export-dhcp.txt"

type commitApplier struct {
	d    *mock.Device
	fail bool
}

// Mirrors txn.Manager.Apply: stage, commit, revert on failure.
func (a *commitApplier) Apply(ctx context.Context, configs []string, stage func(context.Context) error) error {
	if err := stage(ctx); err != nil {
		return errors.Join(err, a.d.Revert(ctx, "dhcp"))
	}
	if a.fail {
		return errors.Join(errors.New("apply failed"), a.d.Revert(ctx, "dhcp"))
	}
	for _, c := range configs {
		if err := a.d.Commit(ctx, c); err != nil {
			return err
		}
	}
	return nil
}

type tunnel struct {
	status vpnstatus.TunnelStatus
	err    error
}

func (t *tunnel) Status(context.Context) (vpnstatus.TunnelStatus, error) { return t.status, t.err }

type profiles struct {
	dns []netip.Addr
	err error
}

func (p *profiles) SelectedConfig() (vpnprofiles.Config, error) {
	return vpnprofiles.Config{DNS: p.dns}, p.err
}

type env struct {
	svc     *Service
	layers  *dnslayer.Service
	dev     *mock.Device
	apply   *commitApplier
	tunnel  *tunnel
	profile *profiles
	txnOpen bool
	lookups []string
	lookErr error
}

func setup(t *testing.T) *env {
	t.Helper()
	raw, err := os.ReadFile(dhcpFixture)
	if err != nil {
		t.Fatal(err)
	}
	secs, err := cli.ParseExport("dhcp", string(raw))
	if err != nil {
		t.Fatal(err)
	}
	d := mock.New()
	if err := d.Load(ctx, "dhcp", secs); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	e := &env{dev: d, apply: &commitApplier{d: d},
		tunnel:  &tunnel{status: vpnstatus.TunnelStatus{State: vpnstatus.Connected}},
		profile: &profiles{dns: []netip.Addr{netip.MustParseAddr("10.64.0.1")}}}
	e.layers = &dnslayer.Service{Device: d, Guards: guard.Dir{Path: filepath.Join(dir, "guards"), Now: time.Now},
		RecordPath: filepath.Join(dir, "dns-layers.json"), Apply: e.apply}
	e.svc = &Service{Tunnel: e.tunnel, Profiles: e.profile, Layers: e.layers,
		ReadDNS: func(ctx context.Context) (dnslayer.State, error) {
			_, st, err := dnslayer.ReadState(ctx, d)
			return st, err
		},
		TxnOpen: func() (bool, error) { return e.txnOpen, nil },
		Lookup: func(_ context.Context, host string) error {
			e.lookups = append(e.lookups, host)
			return e.lookErr
		},
		Nonce: func() string { return "n1" }}
	return e
}

func (e *env) live(t *testing.T) dnslayer.State {
	t.Helper()
	_, st, err := dnslayer.ReadState(ctx, e.dev)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func (e *env) reconcile(t *testing.T, want Action) {
	t.Helper()
	got, err := e.svc.Reconcile(ctx)
	if err != nil || got != want {
		t.Fatalf("reconcile: %v %v, want %v", got, err, want)
	}
}

func TestReconcilePushesThenPopsToBase(t *testing.T) {
	e := setup(t)
	base := e.live(t)
	e.reconcile(t, Push)
	st := e.live(t)
	if !slices.Equal(st.Server.Values, []string{"10.64.0.1"}) || st.NoResolv.Value != "1" {
		t.Fatalf("vpn layer not live: %+v", st)
	}
	e.reconcile(t, Keep) // idempotent
	e.tunnel.status.State = vpnstatus.NoHandshake
	e.reconcile(t, Keep)
	e.tunnel.status.State = vpnstatus.Disabled
	e.reconcile(t, Pop)
	if got := e.live(t); !reflect.DeepEqual(got, base) {
		t.Fatalf("base not restored:\n got %+v\nwant %+v", got, base)
	}
	if _, err := os.Stat(e.layers.RecordPath); !os.IsNotExist(err) {
		t.Fatalf("record kept after last pop: %v", err)
	}
}

func TestReconcileUnreadableInputsChangeNothing(t *testing.T) {
	e := setup(t)
	e.tunnel.err = errors.New("ubus timeout")
	e.reconcile(t, Keep)
	e.tunnel.err = nil
	e.profile.err = errors.New("corrupt store")
	e.reconcile(t, Keep)
	if _, err := os.Stat(e.layers.RecordPath); !os.IsNotExist(err) {
		t.Fatal("layer written from unreadable inputs")
	}
}

func TestReconcileNoProfileMeansNoResolvers(t *testing.T) {
	e := setup(t)
	e.reconcile(t, Push)
	e.profile.err = vpnprofiles.ErrNotFound
	e.reconcile(t, Pop)
}

func TestFailedApplyIsReportedAndNotRetried(t *testing.T) {
	e := setup(t)
	e.apply.fail = true
	if _, err := e.svc.Reconcile(ctx); err == nil {
		t.Fatal("want apply error")
	}
	e.apply.fail = false
	e.reconcile(t, Keep) // guard kept: waits for the explicit retry
	st, err := e.svc.Status(ctx)
	if err != nil || st.Layer != Failed {
		t.Fatalf("%+v %v", st, err)
	}
	v, err := e.svc.Verify(ctx)
	if err != nil || v.Result != LayerFailed {
		t.Fatalf("%+v %v", v, err)
	}
}

// Coexistence with captive and AdGuard: precedence is by name, pops remove only their own entry.
func TestCoexistenceWithOtherLayers(t *testing.T) {
	e := setup(t)
	base := e.live(t)
	adguard := dnslayer.Layer{Name: dnslayer.AdGuard, Servers: []string{"127.0.0.1#5353"}, NoResolv: true}
	if err := e.layers.Push(ctx, adguard); err != nil {
		t.Fatal(err)
	}
	e.reconcile(t, Push)
	if got := e.live(t).Server.Values; !slices.Equal(got, adguard.Servers) {
		t.Fatalf("adguard must win over vpn: %v", got)
	}
	v, err := e.svc.Verify(ctx)
	if err != nil || v.Result != Overridden || v.Status.Top == nil || *v.Status.Top != "adguard" {
		t.Fatalf("%+v %v", v, err)
	}
	captive := dnslayer.Layer{Name: dnslayer.Captive, Servers: []string{"192.0.2.53"}, DisableRebind: true}
	if err := e.layers.Push(ctx, captive); err != nil {
		t.Fatal(err)
	}
	if got := e.live(t).Server.Values; !slices.Equal(got, captive.Servers) {
		t.Fatalf("captive must win: %v", got)
	}
	if err := e.layers.Pop(ctx, dnslayer.Captive); err != nil {
		t.Fatal(err)
	}
	if err := e.layers.Pop(ctx, dnslayer.AdGuard); err != nil {
		t.Fatal(err)
	}
	if got := e.live(t).Server.Values; !slices.Equal(got, []string{"10.64.0.1"}) {
		t.Fatalf("vpn layer lost by other pops: %v", got)
	}
	e.tunnel.status.State = vpnstatus.Configured
	e.reconcile(t, Pop)
	if got := e.live(t); !reflect.DeepEqual(got, base) {
		t.Fatalf("base not restored: %+v", got)
	}
}

func TestVerify(t *testing.T) {
	e := setup(t)
	v, err := e.svc.Verify(ctx)
	if err != nil || v.Result != NotActive || v.Status.Layer != Inactive || len(e.lookups) != 0 {
		t.Fatalf("%+v %v %v", v, err, e.lookups)
	}
	e.reconcile(t, Push)
	v, err = e.svc.Verify(ctx)
	if err != nil || v.Result != OK || !slices.Equal(e.lookups, []string{"n1." + VerifyDomain}) {
		t.Fatalf("%+v %v %v", v, err, e.lookups)
	}
	if !slices.Equal(v.Status.Effective, []string{"10.64.0.1"}) || !slices.Equal(v.Status.Resolvers, []string{"10.64.0.1"}) {
		t.Fatalf("%+v", v.Status)
	}
	e.lookErr = errors.New("i/o timeout")
	if v, _ = e.svc.Verify(ctx); v.Result != LookupFailed {
		t.Fatalf("%+v", v)
	}
	e.profile.dns = []netip.Addr{netip.MustParseAddr("10.8.0.1")} // selected another profile
	if v, _ = e.svc.Verify(ctx); v.Result != Mismatch {
		t.Fatalf("%+v", v)
	}
}

func TestStatusWithoutDnsmasqStillReportsLayer(t *testing.T) {
	e := setup(t)
	e.svc.ReadDNS = func(context.Context) (dnslayer.State, error) { return dnslayer.State{}, errors.New("uci") }
	st, err := e.svc.Status(ctx)
	if err != nil || st.Effective != nil || st.Layer != Inactive || st.Top != nil {
		t.Fatalf("%+v %v", st, err)
	}
	if _, err := e.svc.Verify(ctx); err != nil {
		t.Fatalf("inactive layer needs no live read: %v", err)
	}
}
