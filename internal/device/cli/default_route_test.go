package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

func TestDefaultRoutesFromDevice(t *testing.T) {
	const root = "../../../testdata/device/axt1800-25.12.6"
	for _, tc := range []struct {
		fixture string
		found   bool
		route   device.IPv4DefaultRoute
	}{
		{"ip-route-default.txt", true, device.IPv4DefaultRoute{Device: "phy0-sta0", Gateway: "192.168.90.2", Source: "192.168.90.241"}},
		{"wwan-down/ip-route-default-none.txt", false, device.IPv4DefaultRoute{}},
	} {
		t.Run(tc.fixture, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(root, tc.fixture))
			if err != nil {
				t.Fatal(err)
			}
			route, found, err := parseDefaultRoutes(string(raw))
			if err != nil || found != tc.found || route != tc.route {
				t.Fatalf("got %+v, %v, %v", route, found, err)
			}
		})
	}
}

func TestDefaultRouteRanking(t *testing.T) {
	for _, tc := range []struct{ label, routes, winner string }{
		{"lowest", "default dev lan metric 10\ndefault dev wan metric 2", "wan"},
		{"implicit zero", "default dev lan metric 10\ndefault dev wan", "wan"},
		{"identical ties", "default dev wan\ndefault dev wan", "wan"},
		{"different ties", "default dev lan metric 5\ndefault dev wan metric 5", ""},
		{"unsupported winner", "blackhole default metric 3\ndefault dev wan metric 4", ""},
		{"invalid metric", "default dev lan metric nope\ndefault dev wan", ""},
		{"linkdown winner", "default dev lan metric 1 linkdown\ndefault dev wan metric 2", ""},
		{"missing device", "default via 192.0.2.1\ndefault dev wan metric 4", ""},
		{"multiline multipath", "default metric 3\n  nexthop via 192.0.2.1 dev lan weight 1\n  nexthop via 192.0.2.2 dev wan weight 1\ndefault dev wan metric 4", ""},
	} {
		t.Run(tc.label, func(t *testing.T) {
			got, found, err := parseDefaultRoutes(tc.routes)
			if tc.winner == "" {
				if found || err == nil {
					t.Fatalf("wanted unknown, got %+v, %v, %v", got, found, err)
				}
			} else if err != nil || !found || got.Device != tc.winner {
				t.Fatalf("got %+v, %v, %v", got, found, err)
			}
		})
	}
}
