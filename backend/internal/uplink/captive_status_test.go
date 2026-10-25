package uplink

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/guard"
	"github.com/openwrt-travel-gui/backend/internal/platform"
)

// countingProber answers 204 after a delay and counts its calls.
type countingProber struct {
	calls atomic.Int32
	delay time.Duration
}

func (p *countingProber) Do(string) (int, string, string, error) {
	p.calls.Add(1)
	time.Sleep(p.delay)
	return 204, "", "", nil
}

func routeRunner(route string) *platform.MockCommandRunner {
	return &platform.MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		return []byte(route), nil
	}}
}

func newStatusTestService(t *testing.T, prober platform.HTTPProber, route string) *CaptiveService {
	t.Helper()
	return NewCaptiveService(CaptiveDeps{
		Prober: prober,
		Cmd:    routeRunner(route),
		Guard:  guard.In(t.TempDir(), captiveDNSGuardFeature),
	})
}

// Without a default route nothing upstream can answer, and the probe would
// only wait out its timeout (3 s on the device) on DNS.
func TestStatusSkipsTheProbeWithoutADefaultRoute(t *testing.T) {
	prober := &countingProber{}
	svc := newStatusTestService(t, prober, "")

	st, err := svc.Status()
	if err != nil {
		t.Fatal(err)
	}
	if prober.calls.Load() != 0 {
		t.Errorf("probe ran %d times without a default route", prober.calls.Load())
	}
	if st.STAConnected || st.CanReachInternet || st.Detected {
		t.Errorf("status = %+v, want everything false", st)
	}
}

// A WireGuard default route has no gateway ("default dev wg0"); it is still
// an upstream, or the UI would report no internet whenever the VPN is up.
func TestStatusCountsADeviceOnlyDefaultRouteAsUpstream(t *testing.T) {
	prober := &countingProber{}
	svc := newStatusTestService(t, prober, "default dev wg0 scope link\n")

	st, err := svc.Status()
	if err != nil {
		t.Fatal(err)
	}
	if !st.STAConnected || !st.CanReachInternet {
		t.Errorf("status = %+v, want an upstream that reaches the internet", st)
	}
}

// The dashboard, the WiFi page and every open tab poll the status; parallel
// polls must share one outbound probe.
func TestStatusSharesOneProbeBetweenConcurrentCallers(t *testing.T) {
	prober := &countingProber{delay: 50 * time.Millisecond}
	svc := newStatusTestService(t, prober, "default via 10.0.1.1 dev phy0-sta0\n")

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if _, err := svc.Status(); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if n := prober.calls.Load(); n != 1 {
		t.Errorf("probe ran %d times for 8 concurrent callers, want 1", n)
	}
}

// After a DNS bypass or restore the next poll must see the new state, not a
// cached answer from before the action.
func TestStatusIsProbedAgainAfterInvalidation(t *testing.T) {
	prober := &countingProber{}
	svc := newStatusTestService(t, prober, "default via 10.0.1.1 dev phy0-sta0\n")

	_, _ = svc.Status()
	_, _ = svc.Status()
	if n := prober.calls.Load(); n != 1 {
		t.Fatalf("probe ran %d times within the cache window, want 1", n)
	}
	svc.invalidateStatus()
	_, _ = svc.Status()
	if n := prober.calls.Load(); n != 2 {
		t.Errorf("probe ran %d times after invalidation, want 2", n)
	}
}
