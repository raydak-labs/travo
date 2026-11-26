package connectivity

import (
	"context"
	"errors"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
)

type routeSource struct {
	route device.IPv4DefaultRoute
	err   error
}

func (r *routeSource) DefaultIPv4Route(context.Context) (device.IPv4DefaultRoute, bool, error) {
	return r.route, r.route.Device != "", r.err
}

func TestConnectivitySignals(t *testing.T) {
	dev := mock.New()
	raw, err := os.ReadFile("../../testdata/device/axt1800-25.12.6/device-status-phy0-sta0.json")
	if err != nil {
		t.Fatal(err)
	}
	dev.SetUbusReply("network.device", "status", raw)
	route := &routeSource{}
	var calls atomic.Int32
	probe := func(context.Context) Result { calls.Add(1); return Result{Internet: Offline} }
	service := New(t.Context(), route, dev, probe, time.Now)
	if got := service.Status(t.Context()); got.Internet != Offline || got.ActiveInterface != nil || got.Carrier != nil || calls.Load() != 0 {
		t.Fatalf("no route: %+v", got)
	}
	route.err = errors.New("route failed")
	if got := service.Status(t.Context()); got.Internet != Unknown || got.ActiveInterface != nil || got.Carrier != nil || calls.Load() != 0 {
		t.Fatalf("route error: %+v", got)
	}
	route.err = nil
	route.route.Device = "phy0-sta0"
	if got := service.Status(t.Context()); got.Internet != Offline || got.ActiveInterface == nil || *got.ActiveInterface != "phy0-sta0" || got.Carrier == nil || !*got.Carrier {
		t.Fatalf("carrier up and offline: %+v", got)
	}
	service = New(t.Context(), route, mock.New(), func(context.Context) Result { return Result{Internet: Online} }, time.Now)
	if got := service.Status(t.Context()); got.Internet != Online || got.Carrier != nil {
		t.Fatalf("unknown carrier and online: %+v", got)
	}
}

func TestCacheExpiry(t *testing.T) {
	route := &routeSource{route: device.IPv4DefaultRoute{Device: "wan"}}
	var tick atomic.Int64
	var calls atomic.Int32
	start := time.Now()
	service := New(t.Context(), route, mock.New(), func(context.Context) Result { calls.Add(1); return Result{Internet: Online} }, func() time.Time { return start.Add(time.Duration(tick.Load())) })
	service.Status(t.Context())
	tick.Store(int64(1999 * time.Millisecond))
	service.Status(t.Context())
	if calls.Load() != 1 {
		t.Fatal("cache missed early")
	}
	tick.Store(int64(2 * time.Second))
	service.Status(t.Context())
	if calls.Load() != 2 {
		t.Fatal("cache did not expire")
	}
}

func TestFlightGenerationsAndCancellation(t *testing.T) {
	daemon, cancel := context.WithCancel(t.Context())
	defer cancel()
	route := &routeSource{route: device.IPv4DefaultRoute{Device: "wan"}}
	started := make(chan chan Internet)
	service := New(daemon, route, mock.New(), func(ctx context.Context) Result {
		result := make(chan Internet, 1)
		select {
		case started <- result:
		case <-ctx.Done():
			return Result{Internet: Unknown}
		}
		select {
		case value := <-result:
			return Result{Internet: value}
		case <-ctx.Done():
			return Result{Internet: Unknown}
		}
	}, time.Now)

	_, old, _ := service.observeRoute(t.Context())
	first := receive(t, started)
	_, shared, _ := service.observeRoute(t.Context())
	if old != shared {
		t.Fatal("same generation did not coalesce")
	}
	caller, abort := context.WithCancel(t.Context())
	abort()
	if got := service.Status(caller); got.Internet != Unknown {
		t.Fatalf("canceled caller: %v", got)
	}

	route.route.Device = "other"
	_, second, _ := service.observeRoute(t.Context())
	receive(t, started)
	route.route.Device = "wan"
	_, third, _ := service.observeRoute(t.Context())
	thirdResult := receive(t, started)
	if old == third {
		t.Fatal("A to B to A reused a flight")
	}
	service.Invalidate()
	_, current, _ := service.observeRoute(t.Context())
	currentResult := receive(t, started)
	first <- Offline
	thirdResult <- Offline
	receive(t, old.done)
	receive(t, third.done)
	_, stillCurrent, _ := service.observeRoute(t.Context())
	if current != stillCurrent {
		t.Fatal("old result replaced current flight")
	}
	currentResult <- Online
	receive(t, current.done)
	if got := service.Status(t.Context()); got.Internet != Online {
		t.Fatal(got)
	}
	cancel()
	receive(t, second.done)
}

func receive[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(time.Second):
		t.Fatal("probe did not complete")
		var zero T
		return zero
	}
}

type routeReader func(context.Context) (device.IPv4DefaultRoute, bool, error)

func (r routeReader) DefaultIPv4Route(ctx context.Context) (device.IPv4DefaultRoute, bool, error) {
	return r(ctx)
}

func TestCanceledCallerDoesNotWaitForRouteReader(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	defer close(release)
	var calls atomic.Int32
	source := routeReader(func(context.Context) (device.IPv4DefaultRoute, bool, error) {
		if calls.Add(1) == 1 {
			close(entered)
			<-release
		}
		return device.IPv4DefaultRoute{}, false, nil
	})
	service := New(t.Context(), source, mock.New(), nil, time.Now)
	go service.Status(t.Context())
	receive(t, entered)
	caller, cancel := context.WithCancel(t.Context())
	done := make(chan Status, 1)
	go func() { done <- service.Status(caller) }()
	cancel()
	if got := receive(t, done); got.Internet != Unknown || calls.Load() != 1 {
		t.Fatalf("canceled caller got %+v with %d route reads", got, calls.Load())
	}
}

func TestPortalURLOnlyWhenCaptive(t *testing.T) {
	route := &routeSource{route: device.IPv4DefaultRoute{Device: "wan"}}
	result := Result{Internet: Captive, Portal: "http://portal.example/login"}
	service := New(t.Context(), route, mock.New(), func(context.Context) Result { return result }, time.Now)
	if got := service.Status(t.Context()); got.PortalURL == nil || *got.PortalURL != result.Portal {
		t.Fatalf("captive: %+v", got)
	}
	result = Result{Internet: Online, Portal: "http://ignored.example/"}
	service.Invalidate()
	if got := service.Status(t.Context()); got.PortalURL != nil {
		t.Fatalf("online carries a portal: %+v", got)
	}
	route.route = device.IPv4DefaultRoute{}
	if got := service.Status(t.Context()); got.PortalURL != nil {
		t.Fatalf("no route carries a portal: %+v", got)
	}
}
