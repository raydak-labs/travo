package captive

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/connectivity"
	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
)

type fakeConn struct {
	internet    connectivity.Internet
	invalidated atomic.Int32
}

func (f *fakeConn) Status(context.Context) connectivity.Status {
	return connectivity.Status{Internet: f.internet}
}
func (f *fakeConn) Invalidate() { f.invalidated.Add(1) }

// scriptedRoutes answers each read from a script; the last entry repeats.
type scriptedRoutes struct {
	script []device.IPv4DefaultRoute
	reads  int
}

func (s *scriptedRoutes) DefaultIPv4Route(context.Context) (device.IPv4DefaultRoute, bool, error) {
	r := s.script[min(s.reads, len(s.script)-1)]
	s.reads++
	return r, r.Device != "", nil
}

type renewRig struct {
	renewer *Renewer
	dev     *mock.Device
	guards  guard.Dir
	routes  *scriptedRoutes
	conn    *fakeConn
	clock   *clock
	slept   time.Duration
}

var viaWWAN = device.IPv4DefaultRoute{Device: "phy0-sta0"}

// newRenewRig uses the captured interface dump: wwan is up on phy0-sta0.
func newRenewRig(t *testing.T) *renewRig {
	t.Helper()
	dev := mock.New()
	dump, err := os.ReadFile(fixtures + "interface-dump.json")
	if err != nil {
		t.Fatal(err)
	}
	dev.SetUbusReply("network.interface", "dump", dump)
	r := &renewRig{dev: dev, guards: guard.Dir{Path: filepath.Join(t.TempDir(), "guards"), Now: time.Now},
		routes: &scriptedRoutes{script: []device.IPv4DefaultRoute{viaWWAN}}, conn: &fakeConn{internet: connectivity.Captive},
		clock: &clock{t: time.Unix(1000, 0)}}
	sleep := func(_ context.Context, d time.Duration) error {
		r.slept += d
		r.clock.t = r.clock.t.Add(d)
		return nil
	}
	r.renewer = NewRenewer(r.guards, dev, dev, r.routes, r.conn, r.clock.now, sleep)
	return r
}

func (r *renewRig) guardHeld(t *testing.T) bool {
	t.Helper()
	held, err := r.renewer.held()
	if err != nil {
		t.Fatal(err)
	}
	return held
}

func TestRenewProvesRouteAndReleasesGuard(t *testing.T) {
	r := newRenewRig(t)
	if err := r.renewer.Renew(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(r.dev.Renews(), []string{"wwan"}) || r.guardHeld(t) || r.conn.invalidated.Load() != 1 {
		t.Fatalf("renews %v guard %v invalidated %d", r.dev.Renews(), r.guardHeld(t), r.conn.invalidated.Load())
	}
	if r.slept < renewSettle {
		t.Fatalf("read the route as proof before netifd could act: slept %s", r.slept)
	}
}

func TestRenewWaitsBoundedThenKeepsGuard(t *testing.T) {
	r := newRenewRig(t)
	r.routes.script = []device.IPv4DefaultRoute{viaWWAN, {}} // precheck passes, then the route is gone
	if err := r.renewer.Renew(t.Context()); !errors.Is(err, ErrRenewUnproven) {
		t.Fatal(err)
	}
	if !r.guardHeld(t) {
		t.Fatal("unproven renew released its guard")
	}
	if r.slept > RenewWait {
		t.Fatalf("wait unbounded: %s", r.slept)
	}
	if got := r.renewer.Availability(t.Context(), connectivity.Status{}); got != RenewBlocked {
		t.Fatal(got)
	}
	// The explicit request is the retry path: a proven route now releases the kept guard.
	r.routes.script, r.routes.reads = []device.IPv4DefaultRoute{viaWWAN}, 0
	if err := r.renewer.Renew(t.Context()); err != nil {
		t.Fatal(err)
	}
	if r.guardHeld(t) || len(r.dev.Renews()) != 2 {
		t.Fatal("retry did not finish")
	}
}

func TestRenewRefusesWhenNotApplicable(t *testing.T) {
	for name, setup := range map[string]func(*renewRig){
		"online":             func(r *renewRig) { r.conn.internet = connectivity.Online },
		"unknown":            func(r *renewRig) { r.conn.internet = connectivity.Unknown },
		"uplink is not wwan": func(r *renewRig) { r.routes.script = []device.IPv4DefaultRoute{{Device: "eth0"}} },
		"no default route":   func(r *renewRig) { r.routes.script = []device.IPv4DefaultRoute{{}} },
	} {
		t.Run(name, func(t *testing.T) {
			r := newRenewRig(t)
			setup(r)
			if err := r.renewer.Renew(t.Context()); !errors.Is(err, ErrRenewNotApplicable) {
				t.Fatal(err)
			}
			if len(r.dev.Renews()) != 0 || r.guardHeld(t) {
				t.Fatal("refusal renewed or kept the guard")
			}
		})
	}
}

func TestRenewFailureOfNetifdKeepsGuard(t *testing.T) {
	r := newRenewRig(t)
	r.dev.RenewErr = errors.New("ubus timed out")
	if err := r.renewer.Renew(t.Context()); !errors.Is(err, ErrRenewUnproven) {
		t.Fatal(err)
	}
	if !r.guardHeld(t) {
		t.Fatal("unknown renew outcome released the guard")
	}
}

func TestRenewIgnoresCallerCancellationAfterRenew(t *testing.T) {
	r := newRenewRig(t)
	ctx, cancel := context.WithCancel(t.Context())
	r.renewer.sleep = func(c context.Context, d time.Duration) error {
		cancel() // the HTTP client went away mid-wait
		if c.Err() != nil {
			return c.Err()
		}
		r.clock.t = r.clock.t.Add(d)
		return nil
	}
	if err := r.renewer.Renew(ctx); err != nil {
		t.Fatal(err)
	}
	if r.guardHeld(t) {
		t.Fatal("a cancelled caller stranded the guard")
	}
}

func TestRenewAvailability(t *testing.T) {
	r := newRenewRig(t)
	sta, eth := "phy0-sta0", "eth0"
	for _, tc := range []struct {
		st   connectivity.Status
		want RenewState
	}{
		{connectivity.Status{ActiveInterface: &sta, Internet: connectivity.Captive}, RenewAvailable},
		{connectivity.Status{ActiveInterface: &sta, Internet: connectivity.Offline}, RenewAvailable},
		{connectivity.Status{ActiveInterface: &sta, Internet: connectivity.Online}, RenewNotApplicable},
		{connectivity.Status{ActiveInterface: &sta, Internet: connectivity.Unknown}, RenewNotApplicable},
		{connectivity.Status{ActiveInterface: &eth, Internet: connectivity.Captive}, RenewNotApplicable},
		{connectivity.Status{Internet: connectivity.Offline}, RenewNotApplicable},
	} {
		if got := r.renewer.Availability(t.Context(), tc.st); got != tc.want {
			t.Errorf("%+v: got %s want %s", tc.st, got, tc.want)
		}
	}
	if len(r.dev.Renews()) != 0 {
		t.Fatal("availability renewed")
	}
}
