package tailscale

import (
	"context"
	"errors"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/dnslayer"
)

func running(suffix string, enabled bool) Status {
	return Status{Installed: true, Running: true, Backend: Backend{State: StateRunning, MagicDNS: MagicDNS{Enabled: enabled, Suffix: suffix}}}
}

func TestWant(t *testing.T) {
	l, known := Want(running("tail0000.ts.net", true))
	if !known || l == nil || l.Name != dnslayer.Tailscale || l.Servers[0] != "/tail0000.ts.net/100.100.100.100" || l.NoResolv {
		t.Fatalf("%+v %v", l, known)
	}
	for name, st := range map[string]Status{
		"absent":         {},
		"daemon stopped": {Installed: true},
		"logged out":     {Installed: true, Running: true, Backend: Backend{State: StateNeedsLogin}},
		"down":           {Installed: true, Running: true, Backend: Backend{State: StateStopped}},
		"magic off":      running("tail0000.ts.net", false),
		"bad suffix":     running("evil/1.2.3.4#53", true),
		"bare label":     running("ts", true),
	} {
		if l, known := Want(st); l != nil || !known {
			t.Errorf("%s: %+v %v", name, l, known)
		}
	}
	if _, known := Want(Status{Installed: true, Running: true, Backend: Backend{State: "Starting"}}); known {
		t.Error("transient state decided")
	}
}

func TestDecide(t *testing.T) {
	want := &dnslayer.Layer{Name: dnslayer.Tailscale, Servers: []string{"/a.ts.net/100.100.100.100"}}
	other := &dnslayer.Layer{Name: dnslayer.Tailscale, Servers: []string{"/b.ts.net/100.100.100.100"}}
	cases := []struct {
		want    *dnslayer.Layer
		known   bool
		current *dnslayer.Layer
		out     Action
	}{
		{want, true, nil, PushLayer},
		{want, true, want, Keep},
		{want, true, other, PushLayer},
		{nil, true, want, PopLayer},
		{nil, true, nil, Keep},
		{nil, false, want, Keep},
	}
	for i, c := range cases {
		if got := Decide(c.want, c.known, c.current); got != c.out {
			t.Errorf("case %d: got %v want %v", i, got, c.out)
		}
	}
}

type fakeLayers struct {
	rec    *dnslayer.Record
	status dnslayer.Status
	pushed []dnslayer.Layer
	popped []dnslayer.Name
}

func (f *fakeLayers) Load() (*dnslayer.Record, error)  { return f.rec, nil }
func (f *fakeLayers) Status() (dnslayer.Status, error) { return f.status, nil }
func (f *fakeLayers) Push(_ context.Context, l dnslayer.Layer) error {
	f.pushed = append(f.pushed, l)
	f.rec = &dnslayer.Record{Layers: []dnslayer.Layer{l}}
	return nil
}
func (f *fakeLayers) Pop(_ context.Context, n dnslayer.Name) error {
	f.popped = append(f.popped, n)
	f.rec = nil
	return nil
}

func TestTickFollowsLoginAndLogout(t *testing.T) {
	r := newRig(t)
	r.installed(t)
	layers := &fakeLayers{}
	d := &DNS{Service: r.svc, Layers: layers}
	ctx := context.Background()
	if a, err := d.Tick(ctx); a != Keep || err != nil {
		t.Fatalf("logged out: %v %v", a, err)
	}
	if _, err := r.svc.Login(ctx); err != nil {
		t.Fatal(err)
	}
	if a, err := d.Tick(ctx); a != PushLayer || err != nil {
		t.Fatalf("running: %v %v", a, err)
	}
	if a, _ := d.Tick(ctx); a != Keep {
		t.Fatalf("second tick rewrote: %v", a)
	}
	if _, err := r.svc.Logout(ctx); err != nil {
		t.Fatal(err)
	}
	if a, err := d.Tick(ctx); a != PopLayer || err != nil {
		t.Fatalf("logout: %v %v", a, err)
	}
}

type brokenStatus struct{ *Mock }

func (brokenStatus) Status(context.Context) ([]byte, error) { return nil, errors.New("timeout") }

func TestTickChangesNothingOnBrokenReadsOrHeldGuards(t *testing.T) {
	r := newRig(t)
	r.installed(t)
	if _, err := r.svc.Login(context.Background()); err != nil {
		t.Fatal(err)
	}
	active := &dnslayer.Record{Layers: []dnslayer.Layer{{Name: dnslayer.Tailscale, Servers: []string{"/x.ts.net/100.100.100.100"}}}}
	r.svc.CLI = brokenStatus{r.cli}
	layers := &fakeLayers{rec: active}
	d := &DNS{Service: r.svc, Layers: layers}
	if a, err := d.Tick(context.Background()); a != Keep || err != nil {
		t.Fatalf("broken read: %v %v", a, err)
	}
	r.svc.CLI = r.cli
	layers.status.Pending = true
	if a, _ := d.Tick(context.Background()); a != Keep {
		t.Fatalf("pending stack: %v", a)
	}
	layers.status.Pending = false
	if err := r.svc.Guards.Acquire(ChangeGuard); err != nil {
		t.Fatal(err)
	}
	if a, _ := d.Tick(context.Background()); a != Keep || len(layers.pushed)+len(layers.popped) != 0 {
		t.Fatalf("held guard: %v", a)
	}
}
