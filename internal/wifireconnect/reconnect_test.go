package wifireconnect

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/wifi"
)

const deviceDir = "../../testdata/device/axt1800-25.12.6"

var errInjected = errors.New("injected")

func capture(t *testing.T, name string) json.RawMessage {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(deviceDir, name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

type fakeUplink struct {
	enabled          bool
	assoc            wifi.Association
	enabledErr, aErr error
}

func (f *fakeUplink) UplinkEnabled(context.Context) (bool, error) { return f.enabled, f.enabledErr }
func (f *fakeUplink) AssociatedUplink(context.Context) (wifi.Association, error) {
	return f.assoc, f.aErr
}

type fakeUbus struct {
	reply json.RawMessage
	err   error
}

func (f *fakeUbus) Call(_ context.Context, object, method string, _ any) (json.RawMessage, error) {
	if object != "network.interface.wwan" || method != "status" {
		return nil, device.ErrNotFound
	}
	return f.reply, f.err
}

type fakeReconnect struct {
	calls int
	err   error
}

func (f *fakeReconnect) ReconnectUplink(context.Context) error { f.calls++; return f.err }

type harness struct {
	svc       *Service
	uplink    *fakeUplink
	ubus      *fakeUbus
	reconnect *fakeReconnect
	guards    guard.Dir
	path      string
}

// newHarness starts with the enabled STA disconnected (verbatim wwan-down capture).
func newHarness(t *testing.T) *harness {
	t.Helper()
	dir := t.TempDir()
	h := &harness{
		uplink:    &fakeUplink{enabled: true},
		ubus:      &fakeUbus{reply: capture(t, "wwan-down/interface-wwan-status-down.json")},
		reconnect: &fakeReconnect{},
		guards:    guard.Dir{Path: filepath.Join(dir, "guards"), Now: time.Now},
		path:      filepath.Join(dir, "wifi-reconnect.json"),
	}
	h.svc = New(h.path, h.uplink, h.ubus, h.guards, h.reconnect, func() time.Time { return time.Unix(1_800_000_000, 0) })
	return h
}

func (h *harness) connect(t *testing.T) {
	h.uplink.assoc = wifi.Association{SSID: "HomeUplink", BSSID: "94:83:c4:1f:28:3a"}
	h.ubus.reply = capture(t, "interface-wwan-status.json")
}

func (h *harness) ticks(n int) {
	for range n {
		h.svc.Tick(context.Background())
	}
}

func (h *harness) status(t *testing.T) Status {
	t.Helper()
	st, err := h.svc.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestDecide(t *testing.T) {
	down := Observation{Readable: true, Enabled: true}
	for _, tc := range []struct {
		name     string
		attempts int
		obs      Observation
		wasDown  bool
		want     Action
	}{
		{"unreadable does nothing", 2, Observation{}, true, ActionNone},
		{"address clears counter", 3, Observation{Readable: true, Enabled: true, Associated: true, Addressed: true}, true, ActionClear},
		{"address with zero counter writes nothing", 0, Observation{Readable: true, Addressed: true}, true, ActionNone},
		{"first disconnected tick waits for wpa_supplicant", 0, down, false, ActionNone},
		{"second disconnected tick attempts", 0, down, true, ActionAttempt},
		{"associated without address waits for DHCP", 0, Observation{Readable: true, Enabled: true, Associated: true}, true, ActionNone},
		{"no enabled STA", 0, Observation{Readable: true}, true, ActionNone},
		{"cap reached", MaxAttempts, down, true, ActionNone},
		{"last attempt under cap", MaxAttempts - 1, down, true, ActionAttempt},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Decide(tc.attempts, tc.obs, tc.wasDown); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCapStopsAfterMaxAttemptsAndReleasesGuard(t *testing.T) {
	h := newHarness(t)
	h.ticks(1)
	if h.reconnect.calls != 0 || h.status(t).State != StateIdle {
		t.Fatalf("first disconnected tick must not act: %d", h.reconnect.calls)
	}
	h.ticks(2 * MaxAttempts)
	if h.reconnect.calls != MaxAttempts {
		t.Fatalf("calls = %d", h.reconnect.calls)
	}
	h.ticks(10)
	if h.reconnect.calls != MaxAttempts {
		t.Fatalf("worker passed the cap: %d", h.reconnect.calls)
	}
	if st := h.status(t); st != (Status{State: StateExhausted, Attempts: MaxAttempts, MaxAttempts: MaxAttempts}) {
		t.Fatalf("status %+v", st)
	}
	if active, _ := h.guards.Stale(); len(active) != 0 {
		t.Fatalf("guard left behind: %v", active)
	}
}

func TestCounterSurvivesRestartAndClearsOnlyOnAddress(t *testing.T) {
	h := newHarness(t)
	h.ticks(2 * MaxAttempts)
	restarted := New(h.path, h.uplink, h.ubus, h.guards, h.reconnect, time.Now)
	for range 4 {
		restarted.Tick(context.Background())
	}
	if h.reconnect.calls != MaxAttempts {
		t.Fatalf("restart reset the cap: %d", h.reconnect.calls)
	}
	// Association alone (no DHCP address yet) keeps the counter.
	h.uplink.assoc = wifi.Association{SSID: "HomeUplink", BSSID: "94:83:c4:1f:28:3a"}
	restarted.Tick(context.Background())
	if st, _ := restarted.Status(context.Background()); st.State != StateExhausted {
		t.Fatalf("cleared without an address: %+v", st)
	}
	h.connect(t)
	restarted.Tick(context.Background())
	if st, _ := restarted.Status(context.Background()); st != (Status{State: StateIdle, MaxAttempts: MaxAttempts}) {
		t.Fatalf("status after address %+v", st)
	}
}

func TestRetryingStatusAndPersistedShape(t *testing.T) {
	h := newHarness(t)
	h.ticks(2)
	if st := h.status(t); st.State != StateRetrying || st.Attempts != 1 {
		t.Fatalf("status %+v", st)
	}
	b, err := os.ReadFile(h.path)
	if err != nil {
		t.Fatal(err)
	}
	var stored map[string]any
	if err := json.Unmarshal(b, &stored); err != nil || len(stored) != 2 || stored["attempts"] != float64(1) || stored["last_attempt"] != "2027-01-15T08:00:00Z" {
		t.Fatalf("stored %s", b)
	}
}

func TestNoActionWhenStateUnreadableOrGuarded(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*testing.T, *harness)
	}{
		{"uplink unreadable", func(_ *testing.T, h *harness) { h.uplink.aErr = errInjected }},
		{"UCI unreadable", func(_ *testing.T, h *harness) { h.uplink.enabledErr = errInjected }},
		{"interface unreadable", func(_ *testing.T, h *harness) { h.ubus.err = errInjected }},
		{"interface payload malformed", func(_ *testing.T, h *harness) { h.ubus.reply = json.RawMessage(`{}`) }},
		{"no enabled STA", func(_ *testing.T, h *harness) { h.uplink.enabled = false }},
		{"transaction guard active", func(t *testing.T, h *harness) {
			if err := h.guards.Acquire("txn-in-progress"); err != nil {
				t.Fatal(err)
			}
		}},
		{"counter torn", func(t *testing.T, h *harness) {
			if err := os.WriteFile(h.path, []byte(`{"attempts":`), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			tc.setup(t, h)
			h.ticks(4)
			if h.reconnect.calls != 0 {
				t.Fatalf("acted %d times", h.reconnect.calls)
			}
		})
	}
}

// A transaction takes the STA down while it applies (seen on the AXT1800: wwan down
// for 5 s during a profile select). That tick must not count as the first disconnected
// tick, or an outage of a single tick right after the change triggers an attempt.
func TestGuardedDownTickDoesNotCountTowardsAttempt(t *testing.T) {
	h := newHarness(t)
	if err := h.guards.Acquire("txn-in-progress"); err != nil {
		t.Fatal(err)
	}
	h.ticks(1)
	if err := h.guards.Release("txn-in-progress"); err != nil {
		t.Fatal(err)
	}
	h.ticks(1)
	if h.reconnect.calls != 0 {
		t.Fatalf("attempted after one unguarded disconnected tick")
	}
	h.ticks(1)
	if h.reconnect.calls != 1 {
		t.Fatalf("want an attempt after two unguarded disconnected ticks, got %d", h.reconnect.calls)
	}
}

func TestTornCounterIsUnavailableNotEmpty(t *testing.T) {
	h := newHarness(t)
	if err := os.WriteFile(h.path, []byte(`{"attempts":-1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.Status(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v", err)
	}
}

func TestFailedRecoveryKeepsGuardAndStopsFeature(t *testing.T) {
	h := newHarness(t)
	h.reconnect.err = errInjected
	h.ticks(6)
	if h.reconnect.calls != 1 {
		t.Fatalf("calls = %d", h.reconnect.calls)
	}
	active, err := h.guards.Stale()
	if err != nil || len(active) != 1 || active[0].Name != GuardName {
		t.Fatalf("guards %v, %v", active, err)
	}
}

func TestCounterWriteFailureSkipsRecovery(t *testing.T) {
	h := newHarness(t)
	h.svc.writeJSON = func(string, any, os.FileMode) error { return errInjected }
	h.ticks(4)
	if h.reconnect.calls != 0 {
		t.Fatalf("recovered without a persisted counter: %d", h.reconnect.calls)
	}
}

func TestStoppedContextDoesNothing(t *testing.T) {
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for range 4 {
		h.svc.Tick(ctx)
	}
	if h.reconnect.calls != 0 {
		t.Fatal("acted after stop")
	}
}
