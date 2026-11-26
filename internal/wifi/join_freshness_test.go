package wifi

import (
	"context"
	"errors"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

type joinLinks func() (map[string]int, error)

func (f joinLinks) LinkIndices(context.Context) (map[string]int, error) { return f() }

func fixedJoinLinks(index int) joinLinks {
	return func() (map[string]int, error) { return map[string]int{"lo": 1, "phy0-sta0": index}, nil }
}

type joinRouteFunc func() string

func (f joinRouteFunc) DefaultIPv4Route(ctx context.Context) (device.IPv4DefaultRoute, bool, error) {
	return (joinRoute{f()}).DefaultIPv4Route(ctx)
}

func TestJoinRequiresReplacementKernelLink(t *testing.T) {
	for _, tc := range []struct {
		name    string
		old     map[int]struct{}
		samples []int
		want    bool
	}{
		{"first join", map[int]struct{}{1: {}}, []int{2, 2}, true},
		{"stale healthy link", map[int]struct{}{2: {}}, []int{2}, false},
		{"reused name, fresh index", map[int]struct{}{2: {}}, []int{3, 3}, true},
		{"replaced during proof", map[int]struct{}{2: {}}, []int{3, 4}, false},
		{"disappeared during proof", map[int]struct{}{2: {}}, []int{3, 0}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			ubus := summaryUbus(t, "wifi-scan/connected/wireless-status.json")
			ubus.replies[ubusKey{"network.device", "status", "phy0-sta0"}] = fixture(t, "device-status-phy0-sta0.json")
			links := joinLinks(func() (map[string]int, error) {
				if len(tc.samples) == 0 {
					t.Fatal("sample sequence exhausted")
				}
				link := map[string]int{"phy0-sta0": tc.samples[0]}
				tc.samples = tc.samples[1:]
				return link, nil
			})
			s := New(ctx, ubus, nil).WithJoin(JoinDeps{Ubus: ubus, Routes: joinRoute{"phy0-sta0"}, Links: links})
			if got := s.provenFreshLink(ctx, "phy0-sta0", tc.old); got != tc.want {
				t.Fatalf("fresh proof = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestJoinUnreadableKernelSnapshotDoesNotStage(t *testing.T) {
	ctx := context.Background()
	s, _ := noSTAScanService(t)
	d := mock.New()
	seedJoinDevice(t, d)
	begin := joinBeginFunc(func(ctx context.Context, _ string, _ []string, stage func(context.Context) error, _ txn.Probe) (txn.Status, error) {
		return txn.Status{}, stage(ctx)
	})
	links := joinLinks(func() (map[string]int, error) { return nil, errors.New("read failed") })
	s.WithJoin(JoinDeps{UCI: d, Ubus: s.ubus, Txns: begin, Keys: d, Links: links})
	_, err := s.Join(ctx, JoinRequest{SSID: joinFixtureSSID(t, s), Key: "test-password", Security: SecurityWPA2})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	for _, cfg := range []string{"wireless", "network", "firewall"} {
		if pending, err := d.Pending(ctx, cfg); err != nil || pending {
			t.Fatalf("%s: pending=%t err=%v", cfg, pending, err)
		}
	}
}
