package tailscale

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/vpnstatus"
)

func loggedIn(t *testing.T) *rig {
	t.Helper()
	r := newRig(t)
	r.installed(t)
	if _, err := r.svc.Login(context.Background()); err != nil {
		t.Fatal(err)
	}
	if st, _ := r.svc.Status(context.Background()); st.State != StateRunning {
		t.Fatalf("mock login did not complete: %q", st.State)
	}
	return r
}

func lastCall(r *rig) []string { return r.cli.Calls[len(r.cli.Calls)-1] }

func TestSetSettingsWritesFullPrefSet(t *testing.T) {
	r := loggedIn(t)
	st, err := r.svc.SetSettings(context.Background(), Settings{ExitNode: "nMock1CNTRL", AdvertiseLAN: true, SSH: true})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"set", "--accept-dns=false", "--exit-node=100.64.0.2", "--exit-node-allow-lan-access=true", "--advertise-routes=192.168.1.0/24", "--ssh=true"}
	if !slices.Equal(lastCall(r), want) {
		t.Fatalf("call %v", lastCall(r))
	}
	if st.Settings != (Settings{ExitNode: "nMock1CNTRL", AdvertiseLAN: true, SSH: true}) || r.held(t, ChangeGuard) {
		t.Fatalf("settings %+v", st.Settings)
	}
	if _, err := r.svc.SetSettings(context.Background(), Settings{}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(lastCall(r), append([]string{"set"}, Flags(Desired{})...)) {
		t.Fatalf("clear %v", lastCall(r))
	}
}

func TestSetSettingsRefusesBeforeAnyWrite(t *testing.T) {
	for name, tc := range map[string]struct {
		in     Settings
		tunnel vpnstatus.State
		want   error
	}{
		"unknown exit node":  {Settings{ExitNode: "nGone"}, vpnstatus.Disabled, ErrExitNodeUnknown},
		"not an exit node":   {Settings{ExitNode: "nMock2CNTRL"}, vpnstatus.Disabled, ErrExitNodeUnknown},
		"wireguard enabled":  {Settings{ExitNode: "nMock1CNTRL"}, vpnstatus.NoHandshake, ErrExitNodeConflict},
		"wireguard up":       {Settings{ExitNode: "nMock1CNTRL"}, vpnstatus.Connected, ErrExitNodeConflict},
		"wireguard selected": {Settings{ExitNode: "nMock1CNTRL"}, vpnstatus.Configured, nil},
	} {
		t.Run(name, func(t *testing.T) {
			r := loggedIn(t)
			r.svc.Tunnel = fakeTunnel{state: tc.tunnel}
			calls := len(r.cli.Calls)
			_, err := r.svc.SetSettings(context.Background(), tc.in)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v", err)
			}
			if tc.want != nil && (len(r.cli.Calls) != calls || r.held(t, ChangeGuard)) {
				t.Fatalf("wrote %v", r.cli.Calls[calls:])
			}
		})
	}
	r := loggedIn(t)
	r.svc.Tunnel = fakeTunnel{err: errors.New("ubus timeout")}
	if _, err := r.svc.SetSettings(context.Background(), Settings{ExitNode: "nMock1CNTRL"}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unreadable tunnel must fail closed: %v", err)
	}
}

func TestExitNodeWithoutInternetRestoresPrefs(t *testing.T) {
	r := loggedIn(t)
	if _, err := r.svc.SetSettings(context.Background(), Settings{SSH: true}); err != nil {
		t.Fatal(err)
	}
	r.svc.Online = func(context.Context) bool { return false }
	r.svc.ProbeWait = 5 * r.svc.Every
	_, err := r.svc.SetSettings(context.Background(), Settings{ExitNode: "nMock1CNTRL", SSH: true})
	if !errors.Is(err, ErrExitNodeOffline) {
		t.Fatalf("got %v", err)
	}
	if !slices.Contains(lastCall(r), "--exit-node=") || !slices.Contains(lastCall(r), "--ssh=true") {
		t.Fatalf("not restored: %v", lastCall(r))
	}
	if r.held(t, ChangeGuard) {
		t.Fatal("a restored change must release the guard")
	}
}

// The restore must run even when the request was cancelled during the probe, and the
// probe must not reuse a cached result from before the route change (#201 review).
func TestExitNodeRestoreSurvivesCancelAndProbeIsFresh(t *testing.T) {
	r := loggedIn(t)
	if _, err := r.svc.SetSettings(context.Background(), Settings{SSH: true}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	invalidated := false
	r.svc.Invalidate = func() { invalidated = true }
	r.svc.Online = func(context.Context) bool {
		if !invalidated {
			t.Error("probe ran before the cached result was dropped")
		}
		cancel()
		return false
	}
	r.svc.ProbeWait = 5 * r.svc.Every
	_, err := r.svc.SetSettings(ctx, Settings{ExitNode: "nMock1CNTRL", SSH: true})
	if !errors.Is(err, ErrExitNodeOffline) {
		t.Fatalf("got %v", err)
	}
	if !slices.Contains(lastCall(r), "--exit-node=") {
		t.Fatalf("not restored after cancel: %v", lastCall(r))
	}
}

func TestApplyFirewallRefusesWhileStopped(t *testing.T) {
	r := loggedIn(t)
	r.svc.Txns = &stageNow{}
	r.dev.SetService(Package, true, false)
	if _, err := r.svc.ApplyFirewall(context.Background()); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("got %v", err)
	}
}

type fakeTunnel struct {
	state vpnstatus.State
	err   error
}

func (f fakeTunnel) Status(context.Context) (vpnstatus.TunnelStatus, error) {
	return vpnstatus.TunnelStatus{State: f.state}, f.err
}

type stageNow struct {
	dev     device.UCI
	summary string
}

func (s *stageNow) Begin(ctx context.Context, summary string, configs []string, stage func(context.Context) error, _ txn.Probe) (txn.Status, error) {
	s.summary = summary
	if err := stage(ctx); err != nil {
		return txn.Status{}, err
	}
	return txn.Status{ID: "0123456789abcdef", State: txn.Pending, Configs: configs}, nil
}

func TestApplyFirewallStagesZoneOnce(t *testing.T) {
	r := loggedIn(t)
	tx := &stageNow{}
	r.svc.Txns = tx
	st, _ := r.svc.Status(context.Background())
	if st.FirewallInSync {
		t.Fatal("captured firewall has no travo_ts")
	}
	if got, err := r.svc.ApplyFirewall(context.Background()); err != nil || got.State != txn.Pending {
		t.Fatalf("%+v %v", got, err)
	}
	if st, _ = r.svc.Status(context.Background()); !st.FirewallInSync {
		t.Fatal("zone not in sync after staging")
	}
	if _, err := r.svc.ApplyFirewall(context.Background()); !errors.Is(err, txn.ErrNoChanges) {
		t.Fatalf("re-apply: %v", err)
	}
	if _, err := r.svc.SetSettings(context.Background(), Settings{AdvertiseLAN: true}); err != nil {
		t.Fatal(err)
	}
	if st, _ = r.svc.Status(context.Background()); st.FirewallInSync {
		t.Fatal("advertising the lan needs the inbound forwarding")
	}
	if _, err := r.svc.ApplyFirewall(context.Background()); err != nil {
		t.Fatal(err)
	}
	fw, _ := r.dev.Show(context.Background(), "firewall")
	if !FirewallInSync(fw, FirewallSections(true)) {
		t.Fatal("inbound forwarding missing")
	}
}

func TestApplyFirewallRefusesForeignSections(t *testing.T) {
	r := loggedIn(t)
	r.svc.Txns = &stageNow{}
	ctx := context.Background()
	if err := r.dev.SetType(ctx, "firewall", LANForward, "rule"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.svc.ApplyFirewall(ctx); !errors.Is(err, ErrFirewallConflict) {
		t.Fatalf("got %v", err)
	}
}
