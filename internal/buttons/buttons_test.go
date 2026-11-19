package buttons

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/hook"
	"github.com/blackdark-org/travo-greenfield/internal/lockout"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/wifi"
)

func captureKeys(t *testing.T) []device.Button {
	t.Helper()
	b, err := os.ReadFile("../../testdata/device/axt1800-25.12.6/buttons/devicetree-keys.tsv")
	if err != nil {
		t.Fatal(err)
	}
	k, err := mock.ParseButtonsTSV(b)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestViewFromCapture(t *testing.T) {
	st := View(captureKeys(t), map[string]Action{"switch": VPN, "reset": GuestWiFi}, nil)
	want := []Button{
		{Name: "reset", Label: "reset", Hotplug: "reset", Action: None},
		{Name: "switch", Label: "switch", Hotplug: "BTN_0", Configurable: true, Action: VPN},
	}
	if len(st.Buttons) != 2 || st.Buttons[0] != want[0] || st.Buttons[1] != want[1] {
		t.Fatalf("got %+v", st.Buttons)
	}
}

func TestNames(t *testing.T) {
	if got := ActionName("BTN_0", "pressed"); got != "button-btn-0-pressed" {
		t.Fatal(got)
	}
	if HotplugName(0x105) != "BTN_5" || HotplugName(0x110) != "" || HotplugName(0x211) != "wps" {
		t.Fatal("hotplug names")
	}
	if SectionName("slide-switch@1") != "button_slide_switch_1" {
		t.Fatal(SectionName("slide-switch@1"))
	}
	// A BTN code on a push button is not a switch.
	if Configurable(device.Button{Code: 0x100, InputType: 1}) {
		t.Fatal("EV_KEY configurable")
	}
}

type commitApply struct{ dev *mock.Device }

func (a commitApply) Apply(ctx context.Context, configs []string, stage func(context.Context) error) error {
	if err := stage(ctx); err != nil {
		return err
	}
	for _, c := range configs {
		if err := a.dev.Commit(ctx, c); err != nil {
			return err
		}
	}
	return nil
}

type fakeTunnel struct {
	calls []bool
	st    txn.Status
	err   error
}

func (f *fakeTunnel) Set(_ context.Context, on bool) (txn.Status, error) {
	f.calls = append(f.calls, on)
	return f.st, f.err
}

type fakeGuest struct {
	cfg  wifi.GuestConfig
	reqs []wifi.GuestRequest
	err  error
}

func (f *fakeGuest) Guest(context.Context) (wifi.GuestConfig, error) { return f.cfg, nil }
func (f *fakeGuest) SetGuest(_ context.Context, r wifi.GuestRequest) (txn.Status, error) {
	f.reqs = append(f.reqs, r)
	if f.err != nil {
		return txn.Status{}, f.err
	}
	return txn.Status{ID: "t1", State: txn.Pending}, nil
}

type fakeTxns struct{ confirmed []string }

func (f *fakeTxns) Confirm(id string) (txn.Status, error) {
	f.confirmed = append(f.confirmed, id)
	return txn.Status{ID: id, State: txn.Confirmed}, nil
}

type fakeGuards struct{ held []guard.Info }

func (f fakeGuards) Stale() ([]guard.Info, error) { return f.held, nil }

func newService(t *testing.T) (*Service, *fakeTunnel, *fakeGuest, *fakeTxns) {
	t.Helper()
	dev := mock.New()
	tun := &fakeTunnel{st: txn.Status{ID: "t0", State: txn.Pending}}
	g := &fakeGuest{cfg: wifi.GuestConfig{Configured: true, Enabled: true, SSID: "TGGuest", Radio: "radio1"}}
	tx := &fakeTxns{}
	return &Service{UCI: dev, Keys: dev, Apply: commitApply{dev}, Tunnel: tun, Guest: g, Txns: tx, Guards: fakeGuards{}}, tun, g, tx
}

func TestSetStoresAndClearsAction(t *testing.T) {
	ctx := context.Background()
	s, _, _, _ := newService(t)
	st, err := s.Set(ctx, "switch", GuestWiFi)
	if err != nil || st.Buttons[1].Action != GuestWiFi {
		t.Fatalf("%+v %v", st, err)
	}
	secs, _ := s.UCI.Show(ctx, config)
	if len(secs) != 1 || secs[0].Name != "button_switch" || secs[0].Options["node"] != "switch" || secs[0].Options["action"] != "guest_wifi" {
		t.Fatalf("sections %+v", secs)
	}
	if st, err = s.Set(ctx, "switch", None); err != nil || st.Buttons[1].Action != None {
		t.Fatalf("%+v %v", st, err)
	}
	if secs, _ := s.UCI.Show(ctx, config); len(secs) != 0 {
		t.Fatalf("none left %+v", secs)
	}
}

func TestSetRefusesResetAndUnknown(t *testing.T) {
	ctx := context.Background()
	s, _, _, _ := newService(t)
	if _, err := s.Set(ctx, "reset", VPN); !errors.Is(err, ErrNotConfigurable) {
		t.Fatal(err)
	}
	if _, err := s.Set(ctx, "nope", VPN); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.Set(ctx, "switch", "factory_reset"); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
}

func actions(t *testing.T, s *Service) map[string]hook.Action {
	t.Helper()
	a, err := s.Actions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 2 || a["button-btn-0-pressed"] == nil || a["button-btn-0-released"] == nil {
		t.Fatalf("actions %v: only the switch, both positions", a)
	}
	return a
}

func TestSwitchRunsVPN(t *testing.T) {
	ctx := context.Background()
	s, tun, _, tx := newService(t)
	a := actions(t, s)
	if err := a["button-btn-0-pressed"](ctx); err != nil || len(tun.calls) != 0 {
		t.Fatalf("no action configured: %v %v", err, tun.calls)
	}
	if _, err := s.Set(ctx, "switch", VPN); err != nil {
		t.Fatal(err)
	}
	if err := a["button-btn-0-pressed"](ctx); err != nil {
		t.Fatal(err)
	}
	tun.err, tun.st = txn.ErrNoChanges, txn.Status{}
	if err := a["button-btn-0-released"](ctx); err != nil {
		t.Fatal(err)
	}
	if len(tun.calls) != 2 || !tun.calls[0] || tun.calls[1] || len(tx.confirmed) != 1 {
		t.Fatalf("calls %v confirmed %v", tun.calls, tx.confirmed)
	}
	st, _ := s.Get(ctx)
	if l := st.Buttons[1].Last; l == nil || l.Position != "released" || l.Outcome != Unchanged {
		t.Fatalf("last %+v", l)
	}
}

func TestSwitchGuestFollowsLockoutAndGuards(t *testing.T) {
	ctx := context.Background()
	s, _, g, _ := newService(t)
	if _, err := s.Set(ctx, "switch", GuestWiFi); err != nil {
		t.Fatal(err)
	}
	a := actions(t, s)
	if err := a["button-btn-0-pressed"](ctx); err != nil || len(g.reqs) != 0 {
		t.Fatalf("already on: %v %v", err, g.reqs)
	}
	g.err = lockout.ErrLockoutRisk
	err := a["button-btn-0-released"](ctx)
	if hook.Code(err) != "skipped_lockout" || len(g.reqs) != 1 || g.reqs[0].Acknowledge || g.reqs[0].Enabled || g.reqs[0].Key != "" {
		t.Fatalf("%v %+v", err, g.reqs)
	}
	s.Guards = fakeGuards{held: []guard.Info{{}}}
	if err := a["button-btn-0-released"](ctx); hook.Code(err) != "skipped_guard" || len(g.reqs) != 1 {
		t.Fatalf("guard: %v", err)
	}
}
