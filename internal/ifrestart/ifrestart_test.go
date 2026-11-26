package ifrestart

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/lockout"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

const captures = "../../testdata/device/axt1800-25.12.6/"

func read(t *testing.T, name string) json.RawMessage {
	t.Helper()
	b, err := os.ReadFile(captures + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// clock advances one second per Sleep; statuses are the replies in order, the last repeats.
type env struct {
	now      time.Time
	statuses []json.RawMessage
	calls    int
	dev      *mock.Device
	guards   guard.Dir
	lockout  *fakeLockout
}

type fakeLockout struct {
	err    error
	caller netip.Addr
	ack    bool
}

func (f *fakeLockout) CheckSTARestart(_ context.Context, ip netip.Addr, ack bool) error {
	f.caller, f.ack = ip, ack
	return f.err
}

func (e *env) Call(_ context.Context, object, method string, _ any) (json.RawMessage, error) {
	if object != "network.interface.wwan" || method != "status" {
		return nil, device.ErrNotFound
	}
	r := e.statuses[min(e.calls, len(e.statuses)-1)]
	e.calls++
	return r, nil
}

func newEnv(t *testing.T, statuses ...json.RawMessage) (*Service, *env) {
	e := &env{now: time.Unix(1_800_000_000, 0), statuses: statuses, dev: mock.New(), lockout: &fakeLockout{}}
	e.guards = guard.Dir{Path: t.TempDir(), Now: func() time.Time { return e.now }}
	s := &Service{Device: e.dev, Ubus: e, Guards: e.guards, Lockout: e.lockout, Now: func() time.Time { return e.now },
		Sleep: func(context.Context, time.Duration) error { e.now = e.now.Add(time.Second); return nil },
		Wait:  DefaultWait, Poll: DefaultPoll}
	return s, e
}

// withUptime rewrites only the uptime of the verbatim wwan status capture.
func withUptime(t *testing.T, raw json.RawMessage, uptime int) json.RawMessage {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	m["uptime"] = uptime
	b, _ := json.Marshal(m)
	return b
}

func held(t *testing.T, e *env) []string {
	t.Helper()
	infos, err := e.guards.Stale()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, g := range infos {
		names = append(names, g.Name)
	}
	return names
}

func TestRestartWaitsForAFreshUptime(t *testing.T) {
	up := read(t, "interface-wwan-status.json") // uptime 195: the state before the restart
	down := read(t, "wwan-down/interface-wwan-status-down.json")
	s, e := newEnv(t, up, up, down, withUptime(t, up, 1))
	caller := netip.MustParseAddr("192.168.1.2")
	res, err := s.Restart(context.Background(), Request{Name: "wwan", Caller: caller})
	if err != nil || !res.Up || res.ElapsedSeconds != 2 {
		t.Fatalf("%+v %v", res, err)
	}
	if strings.Join(e.dev.Restarts, ",") != "wwan" || e.lockout.caller != caller {
		t.Fatalf("restarts %v, lockout caller %v", e.dev.Restarts, e.lockout.caller)
	}
	if len(held(t, e)) != 0 {
		t.Fatal("guard left after success")
	}
}

func TestRestartReportsNotUpAfterTheWait(t *testing.T) {
	s, e := newEnv(t, read(t, "wwan-down/interface-wwan-status-down.json"))
	res, err := s.Restart(context.Background(), Request{Name: "wwan"})
	if err != nil || res.Up || res.ElapsedSeconds != 30 {
		t.Fatalf("%+v %v", res, err)
	}
	if len(held(t, e)) != 0 {
		t.Fatal("guard left after the bounded wait")
	}
}

func TestRestartRefusals(t *testing.T) {
	status := read(t, "interface-wwan-status.json")
	for _, tc := range []struct {
		name  string
		setup func(*Service, *env)
		req   Request
		want  error
	}{
		{"lan is not an uplink", nil, Request{Name: "lan"}, ErrNotRestartable},
		{"guest is not an uplink", nil, Request{Name: "guest"}, ErrNotRestartable},
		{"missing interface", nil, Request{Name: "usbtether"}, ErrNotFound},
		{"pending transaction", func(_ *Service, e *env) { _ = e.guards.Acquire(txn.GuardName) }, Request{Name: "wwan"}, ErrTxnPending},
		{"other live-state guard", func(_ *Service, e *env) { _ = e.guards.Acquire("wifi-reconnect-in-progress") }, Request{Name: "wwan"}, ErrBusy},
		{"lockout", func(_ *Service, e *env) { e.lockout.err = lockout.ErrLockoutRisk }, Request{Name: "wwan"}, lockout.ErrLockoutRisk},
		{"restart running", func(s *Service, _ *env) { s.mu.Lock() }, Request{Name: "wwan"}, ErrBusy},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, e := newEnv(t, status)
			if tc.setup != nil {
				tc.setup(s, e)
			}
			if _, err := s.Restart(context.Background(), tc.req); !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
			if len(e.dev.Restarts) != 0 {
				t.Fatalf("ifup ran: %v", e.dev.Restarts)
			}
		})
	}
}

func TestFailedIfupKeepsTheGuardUntilTheNextRestart(t *testing.T) {
	up := read(t, "interface-wwan-status.json")
	s, e := newEnv(t, up, withUptime(t, up, 0))
	e.dev.RestartErr = errors.New("ifup: exit 1")
	if _, err := s.Restart(context.Background(), Request{Name: "wwan"}); !errors.Is(err, ErrFailed) {
		t.Fatal(err)
	}
	if got := held(t, e); len(got) != 1 || got[0] != GuardName {
		t.Fatalf("guard after failure: %v", got)
	}
	e.dev.RestartErr = nil
	if res, err := s.Restart(context.Background(), Request{Name: "wwan"}); err != nil || !res.Up {
		t.Fatalf("retry: %+v %v", res, err)
	}
	if len(held(t, e)) != 0 {
		t.Fatal("retry left the guard")
	}
}

func TestWANRestartSkipsTheSTALockout(t *testing.T) {
	s, e := newEnv(t)
	e.lockout.err = lockout.ErrLockoutRisk
	s.Ubus = &statusFor{name: "wan", raw: read(t, "interface-wan-status.json")}
	if _, err := s.Restart(context.Background(), Request{Name: "wan"}); err != nil {
		t.Fatal(err)
	}
}

type statusFor struct {
	name string
	raw  json.RawMessage
}

func (f *statusFor) Call(_ context.Context, object, _ string, _ any) (json.RawMessage, error) {
	if object != "network.interface."+f.name {
		return nil, device.ErrNotFound
	}
	return f.raw, nil
}
