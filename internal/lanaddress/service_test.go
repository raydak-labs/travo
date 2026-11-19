package lanaddress

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

// fakeTxns stages, commits and probes like BeginWindow without the watchdog.
type fakeTxns struct {
	d      *mock.Device
	window time.Duration
	state  txn.State
}

func (f *fakeTxns) BeginWindow(ctx context.Context, _ string, configs []string, window time.Duration, stage func(context.Context) error, probe txn.Probe) (txn.Status, error) {
	f.window = window
	if !slices.Equal(configs, []string{"network"}) {
		return txn.Status{}, errors.New("unexpected configs")
	}
	if err := stage(ctx); err != nil {
		return txn.Status{}, errors.Join(err, f.d.Revert(ctx, "network"))
	}
	if err := f.d.Commit(ctx, "network"); err != nil {
		return txn.Status{}, err
	}
	meta := txn.Meta{ID: "0123456789abcdef", WindowSeconds: int(window / time.Second)}
	if err := probe(ctx); err != nil {
		f.state = txn.RolledBack
		return txn.Status{Meta: meta, State: txn.RolledBack}, errors.Join(txn.ErrProbeFailed, err)
	}
	f.state = txn.Pending
	return txn.Status{Meta: meta, State: txn.Pending}, nil
}

func (f *fakeTxns) Get(id string) (txn.Status, error) {
	if id != "0123456789abcdef" {
		return txn.Status{}, txn.ErrNotFound
	}
	return txn.Status{Meta: txn.Meta{ID: id}, State: f.state}, nil
}

// movingUbus answers the captured interface dump; when moves is set and network.lan holds
// 192.168.8.1/24, lan carries it (the same capture with lan's address value replaced).
type movingUbus struct {
	d             *mock.Device
	moves         bool
	before, after json.RawMessage
}

func (u *movingUbus) Call(ctx context.Context, object, method string, args any) (json.RawMessage, error) {
	if object != "network.interface" || method != "dump" {
		return u.d.Call(ctx, object, method, args)
	}
	if secs, err := u.d.Show(ctx, "network"); err == nil && u.moves {
		if cur, err := read(secs); err == nil && cur.prefix.String() == "192.168.8.1/24" {
			return u.after, nil
		}
	}
	return u.before, nil
}

type clock struct{ now time.Time }

func (c *clock) Now() time.Time { return c.now }

func setup(t *testing.T, moves bool) (*Service, *mock.Device, *fakeTxns, *clock) {
	t.Helper()
	d := mock.New()
	in := inputs(t)
	for name, secs := range map[string][]device.Section{"network": in.Network, "dhcp": in.DHCP, "firewall": in.Firewall} {
		if err := d.Load(t.Context(), name, secs); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(captures + "network/interface-dump.json")
	if err != nil {
		t.Fatal(err)
	}
	u := &movingUbus{d: d, moves: moves, before: raw,
		after: bytes.Replace(raw, []byte(`"address": "192.168.1.1"`), []byte(`"address": "192.168.8.1"`), 1)}
	f := &fakeTxns{d: d}
	c := &clock{now: time.Unix(1000, 0)}
	s := &Service{UCI: d, Ubus: u, Txns: f, Now: c.Now, ProbeWait: 50 * time.Millisecond, Tick: time.Millisecond}
	return s, d, f, c
}

func TestSetMovesLANAndReportsPending(t *testing.T) {
	s, d, f, c := setup(t, true)
	refreshed := false
	s.OnMoved = func(context.Context) { refreshed = true }
	st, err := s.Set(t.Context(), Request{Address: "192.168.8.1/24"})
	if err != nil || st.State != txn.Pending || f.window != Window || !refreshed {
		t.Fatalf("%+v %v window=%v refreshed=%v", st, err, f.window, refreshed)
	}
	secs, _ := d.Show(t.Context(), "network")
	l := secs[slices.IndexFunc(secs, func(s device.Section) bool { return s.Name == Interface })]
	if !slices.Equal(l.Lists["ipaddr"], []string{"192.168.8.1/24"}) || l.Options["ipaddr"] != "" {
		t.Fatalf("lan = %+v", l)
	}
	c.now = c.now.Add(30 * time.Second)
	v, err := s.Settings(t.Context())
	if err != nil || v.Address != "192.168.8.1/24" || v.Subnet != "192.168.8.0/24" || v.Pending == nil ||
		*v.Pending != (Pending{ID: st.ID, Address: "192.168.8.1/24", RemainingSeconds: 150}) {
		t.Fatalf("view = %+v %v", v, err)
	}
	f.state = txn.Confirmed
	if v, _ := s.Settings(t.Context()); v.Pending != nil {
		t.Fatalf("settled change still pending: %+v", v.Pending)
	}
}

func TestSetRollsBackWhenLANKeepsTheOldAddress(t *testing.T) {
	s, _, _, _ := setup(t, false)
	st, err := s.Set(t.Context(), Request{Address: "192.168.8.1/24"})
	if !errors.Is(err, ErrNotProven) || st.State != txn.RolledBack {
		t.Fatalf("%+v %v", st, err)
	}
	if v, _ := s.Settings(t.Context()); v.Pending != nil {
		t.Fatal("a rolled-back change is not pending")
	}
}

func TestSetRefusesBeforeAnyWrite(t *testing.T) {
	s, d, f, _ := setup(t, true)
	for _, addr := range []string{"8.8.8.1/24", "192.168.2.1/24", "192.168.1.1/24"} {
		if _, err := s.Set(t.Context(), Request{Address: addr}); err == nil {
			t.Errorf("%s accepted", addr)
		}
	}
	if pending, _ := d.Pending(t.Context(), "network"); pending || f.window != 0 {
		t.Fatal("a refused change staged or began a transaction")
	}
}
