package vpnstatus

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/cli"
	"github.com/blackdark-org/travo-greenfield/internal/runner"
	"github.com/blackdark-org/travo-greenfield/internal/vpnprofiles"
)

type configResult struct {
	sections []device.Section
	err      error
}

func (c *configResult) Show(context.Context, string) ([]device.Section, error) {
	return c.sections, c.err
}

type selectionResult struct {
	selected bool
	err      error
}

func (s selectionResult) List() (vpnprofiles.List, error) {
	list := vpnprofiles.List{Profiles: []vpnprofiles.Summary{}}
	if s.selected {
		id := "selected"
		list.SelectedID = &id
	}
	return list, s.err
}

func zeroElapsed() time.Duration { return 0 }

type ubusFunc func(context.Context, string, string, any) (json.RawMessage, error)

func (f ubusFunc) Call(ctx context.Context, object, method string, args any) (json.RawMessage, error) {
	return f(ctx, object, method, args)
}

type wireGuardFunc func(context.Context, string) ([]device.PeerHandshake, error)

func (f wireGuardFunc) LatestHandshakes(ctx context.Context, iface string) ([]device.PeerHandshake, error) {
	return f(ctx, iface)
}

type commandCall struct {
	name string
	args []string
}

func readFixture(t *testing.T, state, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("../../testdata/device/axt1800-25.12.6/wireguard", state, name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func fixtureTime(t *testing.T, state string) time.Time {
	t.Helper()
	now, err := time.Parse(time.UnixDate, strings.TrimSpace(string(readFixture(t, state, "date.txt"))))
	if err != nil {
		t.Fatal(err)
	}
	return now.UTC()
}

func fixtureSections(t *testing.T, state string) []device.Section {
	t.Helper()
	sections, err := cli.ParseShow("network", string(readFixture(t, state, "uci-show-network-wg0.txt")))
	if err != nil {
		t.Fatal(err)
	}
	return sections
}

func fixtureSection(t *testing.T, state string) *device.Section {
	t.Helper()
	for _, section := range fixtureSections(t, state) {
		if section.Name == Interface {
			copy := section
			return &copy
		}
	}
	t.Fatalf("fixture %s has no %s", state, Interface)
	return nil
}

func commandFailure(name string, args []string, diagnostic string) error {
	return &runner.CommandError{Name: name, Args: args, Err: errors.New("exit status 1"), Stderr: diagnostic}
}

func fixtureCLI(t *testing.T, state string) (*cli.Device, *[]commandCall) {
	t.Helper()
	calls := &[]commandCall{}
	d := &cli.Device{Run: func(ctx context.Context, name string, args ...string) (string, int, error) {
		if _, ok := ctx.Deadline(); !ok {
			t.Error("command without deadline")
		}
		*calls = append(*calls, commandCall{name: name, args: append([]string(nil), args...)})
		switch name + " " + strings.Join(args, " ") {
		case "ubus call network.interface.wg0 status {}":
			if state == "disabled" {
				diagnostic := strings.TrimSpace(string(readFixture(t, state, "interface-wg0-status.json")))
				return "", 1, commandFailure(name, args, diagnostic)
			}
			return string(readFixture(t, state, "interface-wg0-status.json")), 0, nil
		case "wg show wg0 latest-handshakes":
			if state == "disabled" {
				diagnostic := strings.TrimSpace(string(readFixture(t, state, "wg-latest-handshakes.txt")))
				return "", 1, commandFailure(name, args, diagnostic)
			}
			return string(readFixture(t, state, "wg-latest-handshakes.txt")), 0, nil
		default:
			return "", 1, errors.New("unexpected command")
		}
	}}
	return d, calls
}

func fixtureObservation(t *testing.T, state string) (Observation, time.Time) {
	t.Helper()
	d, _ := fixtureCLI(t, state)
	observation := Observation{Selected: true, Section: fixtureSection(t, state)}
	if state != "disabled" {
		raw, err := d.Call(context.Background(), "network.interface.wg0", "status", nil)
		if err != nil {
			t.Fatal(err)
		}
		var reply struct {
			Up *bool `json:"up"`
		}
		if err := json.Unmarshal(raw, &reply); err != nil {
			t.Fatal(err)
		}
		observation.NetifdUp = reply.Up
		observation.Handshakes, err = d.LatestHandshakes(context.Background(), Interface)
		if err != nil {
			t.Fatal(err)
		}
	}
	return observation, fixtureTime(t, state)
}

func TestServiceFixtureStates(t *testing.T) {
	for _, tc := range []struct {
		name     string
		fixture  string
		selected bool
		state    State
		age      *int64
	}{
		{name: "disabled", fixture: "disabled", state: Disabled},
		{name: "configured", fixture: "disabled", selected: true, state: Configured},
		{name: "no handshake", fixture: "no-handshake", selected: true, state: NoHandshake},
		{name: "connected", fixture: "connected", selected: true, state: Connected, age: new(int64)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.fixture == "connected" {
				*tc.age = 14
			}
			cfg := &configResult{sections: fixtureSections(t, tc.fixture)}
			d, calls := fixtureCLI(t, tc.fixture)
			now := fixtureTime(t, tc.fixture)
			service := New(cfg, d, d, selectionResult{selected: tc.selected}, func() time.Time { return now }, zeroElapsed)
			got, err := service.Status(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if got.State != tc.state || got.Stale || !got.ObservedAt.Equal(now) {
				t.Fatalf("got %+v", got)
			}
			if (got.HandshakeAgeSeconds == nil) != (tc.age == nil) || tc.age != nil && *got.HandshakeAgeSeconds != *tc.age {
				t.Fatalf("age got %v want %v", got.HandshakeAgeSeconds, tc.age)
			}
			if tc.fixture == "disabled" && len(*calls) != 0 {
				t.Fatalf("disabled tunnel made device calls: %+v", *calls)
			}
		})
	}
}

func TestServiceMissingNetworkConfig(t *testing.T) {
	for _, tc := range []struct {
		selected bool
		state    State
	}{
		{state: Disabled},
		{selected: true, state: Configured},
	} {
		d, calls := fixtureCLI(t, "disabled")
		service := New(&configResult{err: device.ErrNotFound}, d, d, selectionResult{selected: tc.selected}, func() time.Time { return fixtureTime(t, "disabled") }, zeroElapsed)
		got, err := service.Status(context.Background())
		if err != nil || got.State != tc.state {
			t.Fatalf("got %+v, %v", got, err)
		}
		if len(*calls) != 0 {
			t.Fatalf("absent config made device calls: %+v", *calls)
		}
	}
}

func TestServiceConfigReadFailureIsUnavailable(t *testing.T) {
	d := &cli.Device{Run: func(_ context.Context, name string, args ...string) (string, int, error) {
		if name != "uci" || strings.Join(args, " ") != "export network" {
			t.Fatalf("unexpected command: %s %s", name, strings.Join(args, " "))
		}
		return "", 1, errors.New("uci export network: exit status 1: uci: I/O error")
	}}
	service := New(d, d, d, selectionResult{}, func() time.Time { return fixtureTime(t, "connected") }, zeroElapsed)
	got, err := service.Status(context.Background())
	if !errors.Is(err, ErrUnavailable) || got.State != "" {
		t.Fatalf("transient config failure produced %+v, %v", got, err)
	}
}

func TestServiceMissingNetifdObject(t *testing.T) {
	cfg := &configResult{sections: fixtureSections(t, "no-handshake")}
	ubus := ubusFunc(func(context.Context, string, string, any) (json.RawMessage, error) {
		return nil, device.ErrNotFound
	})
	wg := wireGuardFunc(func(context.Context, string) ([]device.PeerHandshake, error) {
		t.Fatal("read handshakes without netifd up")
		return nil, nil
	})
	service := New(cfg, ubus, wg, selectionResult{selected: true}, func() time.Time { return fixtureTime(t, "no-handshake") }, zeroElapsed)
	got, err := service.Status(context.Background())
	if err != nil || got.State != EnabledNotUp {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestServiceMissingWireGuardDevice(t *testing.T) {
	cfg := &configResult{sections: fixtureSections(t, "connected")}
	ubus := ubusFunc(func(context.Context, string, string, any) (json.RawMessage, error) {
		return json.RawMessage(readFixture(t, "connected", "interface-wg0-status.json")), nil
	})
	wg := wireGuardFunc(func(context.Context, string) ([]device.PeerHandshake, error) {
		return nil, device.ErrNotFound
	})
	service := New(cfg, ubus, wg, selectionResult{selected: true}, func() time.Time { return fixtureTime(t, "connected") }, zeroElapsed)
	got, err := service.Status(context.Background())
	if err != nil || got.State != EnabledNotUp {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestClassifyReadFailuresDoNotProduceState(t *testing.T) {
	plainError := errors.New("read failed")
	up := true
	for _, tc := range []struct {
		name      string
		selection error
		config    error
		ubus      error
		wg        error
		netifdUp  *bool
	}{
		{name: "selection", selection: plainError},
		{name: "config", config: plainError},
		{name: "ubus", ubus: plainError},
		{name: "ubus shape", netifdUp: nil},
		{name: "wireguard", wg: plainError, netifdUp: &up},
	} {
		t.Run(tc.name, func(t *testing.T) {
			observation := Observation{Selected: true, Section: fixtureSection(t, "connected"), NetifdUp: tc.netifdUp, SelectionErr: tc.selection, ConfigErr: tc.config, NetifdErr: tc.ubus, HandshakeErr: tc.wg}
			if _, err := Classify(observation, fixtureTime(t, "connected")); err == nil {
				t.Fatal("read failure produced a state")
			}
		})
	}
}

func TestServiceKeepsLastGoodUntilStaleExpires(t *testing.T) {
	now := fixtureTime(t, "connected")
	elapsed := time.Duration(0)
	cfg := &configResult{sections: fixtureSections(t, "connected")}
	d, _ := fixtureCLI(t, "connected")
	service := New(cfg, d, d, selectionResult{selected: true}, func() time.Time { return now }, func() time.Duration { return elapsed })
	good, err := service.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	cfg.err = errors.New("read failed")
	stale, err := service.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stale.State != good.State || !stale.Stale || !stale.ObservedAt.Equal(good.ObservedAt) || *stale.HandshakeAgeSeconds != *good.HandshakeAgeSeconds {
		t.Fatalf("good %+v stale %+v", good, stale)
	}
	elapsed = MaxStale + time.Second
	if _, err := service.Status(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("got %v", err)
	}
}

func TestServiceFirstReadFailureIsUnavailable(t *testing.T) {
	d, _ := fixtureCLI(t, "connected")
	service := New(&configResult{err: errors.New("read failed")}, d, d, selectionResult{}, func() time.Time { return fixtureTime(t, "connected") }, zeroElapsed)
	if got, err := service.Status(context.Background()); !errors.Is(err, ErrUnavailable) || got.State != "" || got.Stale {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestServiceClockRollbackDoesNotExtendStaleCache(t *testing.T) {
	now := fixtureTime(t, "connected")
	elapsed := time.Duration(0)
	cfg := &configResult{sections: fixtureSections(t, "connected")}
	d, _ := fixtureCLI(t, "connected")
	service := New(cfg, d, d, selectionResult{selected: true}, func() time.Time { return now }, func() time.Duration { return elapsed })
	good, err := service.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	cfg.err = errors.New("read failed")

	elapsed = MaxStale - time.Second
	now = now.Add(-time.Hour)
	stale, err := service.Status(context.Background())
	if err != nil || !stale.Stale || !stale.ObservedAt.Equal(good.ObservedAt) {
		t.Fatalf("fresh stale observation changed: got %+v, %v", stale, err)
	}

	// The router clock is still behind the observation after the monotonic expiry.
	elapsed = MaxStale + time.Second
	if got, err := service.Status(context.Background()); !errors.Is(err, ErrUnavailable) || got.State != "" {
		t.Fatalf("clock rollback extended stale cache: got %+v, %v", got, err)
	}
}
