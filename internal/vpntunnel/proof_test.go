package vpntunnel

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

type proofDevice struct {
	handshake      device.PeerHandshake
	handshakeReads int
	delayHandshake bool
	ambiguous      bool
	link           bool
	failDown       bool
	routeCalls     int
	ups            []string
}

func (d *proofDevice) Call(_ context.Context, _, method string, _ any) (json.RawMessage, error) {
	if method == "dump" {
		raw, err := os.ReadFile("../../testdata/device/axt1800-25.12.6/wireguard/connected/interface-dump.json")
		if err != nil || !d.ambiguous {
			return raw, err
		}
		var dump struct {
			Interface []json.RawMessage `json:"interface"`
		}
		if err := json.Unmarshal(raw, &dump); err != nil {
			return nil, err
		}
		dump.Interface = append(dump.Interface, dump.Interface...)
		return json.Marshal(dump)
	}
	return json.RawMessage(`{"up":true}`), nil
}

func (d *proofDevice) LatestHandshakes(context.Context, string) ([]device.PeerHandshake, error) {
	d.handshakeReads++
	if d.delayHandshake && d.handshakeReads == 1 {
		return nil, nil
	}
	return []device.PeerHandshake{d.handshake}, nil
}
func (d *proofDevice) ReloadNetwork(context.Context) error                { return nil }
func (d *proofDevice) WireGuardLinkPresent(context.Context) (bool, error) { return d.link, nil }
func (d *proofDevice) SetInterfaceUp(_ context.Context, logical string, up bool) error {
	if !up {
		d.ups = append(d.ups, logical+" down")
		if d.failDown {
			return errProof
		}
		return nil
	}
	d.ups = append(d.ups, logical+" up")
	return nil
}
func (d *proofDevice) DefaultIPv4Route(context.Context) (device.IPv4DefaultRoute, bool, error) {
	d.routeCalls++
	return device.IPv4DefaultRoute{Device: "phy0-sta0"}, d.routeCalls > 1, nil
}

func TestEnableRequiresFreshHandshakeFromConfiguredPeer(t *testing.T) {
	now := time.Unix(100, 0)
	for _, tc := range []struct {
		name, peer string
		at         int64
		want       bool
	}{
		{"matching new peer", "selected", 100, true},
		{"unknown peer", "other", 100, false},
		{"equal baseline", "selected", 99, false},
		{"future", "selected", 101, false},
		{"no handshake", "selected", 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := &proofDevice{handshake: device.PeerHandshake{PublicKey: tc.peer}}
			if tc.at != 0 {
				d.handshake.At = time.Unix(tc.at, 0)
			}
			s := Service{ubus: d, handshakes: d, control: d, now: func() time.Time { return now }}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			err := s.proveEnabled(ctx, enableProof{started: 99,
				previous:   map[string]time.Time{"selected": time.Unix(99, 0)},
				configured: map[string]struct{}{"selected": {}},
			})
			if (err == nil) != tc.want {
				t.Fatalf("proof result %v, want success %t", err, tc.want)
			}
		})
	}
}

func TestDisableRecoversUniqueCapturedUplink(t *testing.T) {
	d := &proofDevice{}
	s := Service{ubus: d, control: d, routes: d}
	if err := s.proveDisabled(context.Background()); err != nil {
		t.Fatal(err)
	}
	if d.routeCalls != 2 || !slices.Equal(d.ups, []string{"wwan down", "wwan up"}) {
		t.Fatalf("route reads %d, restarted %v", d.routeCalls, d.ups)
	}
}

func TestDisableRequiresLinkAbsenceBeforeRouteProof(t *testing.T) {
	d := &proofDevice{link: true, routeCalls: 1}
	s := Service{ubus: d, control: d, routes: d}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := s.proveDisabled(ctx); !errors.Is(err, errProof) || d.routeCalls != 1 || len(d.ups) != 0 {
		t.Fatalf("error %v, route reads %d, restarted %v", err, d.routeCalls, d.ups)
	}
}

func TestDisableStopsWhenUplinkCannotBeBroughtDown(t *testing.T) {
	d := &proofDevice{failDown: true}
	s := Service{ubus: d, control: d, routes: d}
	if err := s.proveDisabled(context.Background()); !errors.Is(err, errProof) {
		t.Fatalf("proof result %v, want failure", err)
	}
	if d.routeCalls != 1 || !slices.Equal(d.ups, []string{"wwan down"}) {
		t.Fatalf("route reads %d, calls %v", d.routeCalls, d.ups)
	}
}

func TestEnableWaitsForHandshake(t *testing.T) {
	now := time.Unix(100, 0)
	d := &proofDevice{delayHandshake: true, handshake: device.PeerHandshake{PublicKey: "selected", At: now}}
	s := Service{ubus: d, control: d, handshakes: d, now: func() time.Time { return now }}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := s.proveEnabled(ctx, enableProof{started: 99, configured: map[string]struct{}{"selected": {}}}); err != nil || d.handshakeReads != 2 {
		t.Fatalf("result %v, handshake reads %d", err, d.handshakeReads)
	}
}

func TestEnableStopsWhenRestartFails(t *testing.T) {
	d := &proofDevice{failDown: true}
	s := Service{ubus: d, control: d, handshakes: d}
	if err := s.proveEnabled(context.Background(), enableProof{wasEnabled: true}); !errors.Is(err, errProof) || d.handshakeReads != 0 || !slices.Equal(d.ups, []string{"wg0 down"}) {
		t.Fatalf("result %v, calls %v", err, d.ups)
	}
}

func TestDisableRefusesAmbiguousUplink(t *testing.T) {
	d := &proofDevice{ambiguous: true}
	s := Service{ubus: d, control: d, routes: d}
	if err := s.proveDisabled(context.Background()); !errors.Is(err, errProof) || len(d.ups) != 0 {
		t.Fatalf("result %v, restarted %v", err, d.ups)
	}
}
