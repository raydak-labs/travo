package statshistory

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

const fixtures = "../../testdata/device/axt1800-25.12.6/"

var t0 = time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)

func obs(at time.Time, uptime time.Duration, uplinks map[string]Counter) Observation {
	return Observation{At: at, Uptime: uptime, Uplinks: uplinks}
}

func TestRecordComputesDeltasPerUplink(t *testing.T) {
	h := NewHistory(time.Time{}, nil, PersistenceOK)
	h.Record(obs(t0, time.Hour, map[string]Counter{"wwan": {"phy0-sta0", 100, 50}}))
	h.Record(obs(t0.Add(Interval), time.Hour+Interval, map[string]Counter{
		"wwan": {"phy0-sta0", 1100, 250},
		"wan":  {"eth0", 10, 10}, // new uplink: no previous counter
	}))
	points, _ := h.Points(time.Time{})
	if len(points) != 2 || len(points[0].Uplinks) != 0 {
		t.Fatalf("first point has no deltas: %+v", points)
	}
	want := []UplinkBytes{{Name: "wwan", RXBytes: 1000, TXBytes: 200}}
	if got := points[1].Uplinks; len(got) != 1 || got[0] != want[0] {
		t.Fatalf("uplinks = %+v, want %+v", got, want)
	}
}

func TestRecordDropsResetsDeviceChangesAndReboots(t *testing.T) {
	cases := map[string]struct {
		uptime time.Duration
		cur    Counter
	}{
		"counter reset":  {time.Hour + Interval, Counter{"phy0-sta0", 5, 5}},
		"device changed": {time.Hour + Interval, Counter{"phy1-sta0", 2000, 2000}},
		"reboot":         {time.Minute, Counter{"phy0-sta0", 2000, 2000}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			h := NewHistory(time.Time{}, nil, PersistenceOK)
			h.Record(obs(t0, time.Hour, map[string]Counter{"wwan": {"phy0-sta0", 100, 100}}))
			h.Record(obs(t0.Add(Interval), c.uptime, map[string]Counter{"wwan": c.cur}))
			points, _ := h.Points(time.Time{})
			if len(points[1].Uplinks) != 0 {
				t.Fatalf("no provable delta expected: %+v", points[1].Uplinks)
			}
		})
	}
}

func TestRecordSkipsImplausibleAndRewoundClock(t *testing.T) {
	h := NewHistory(t0, nil, PersistenceOK)
	h.Record(obs(t0.Add(-time.Hour), time.Hour, nil)) // below the build-time floor
	h.Record(obs(t0.Add(Interval), time.Hour, map[string]Counter{"wwan": {"d", 1, 1}}))
	h.Record(obs(t0, time.Hour+Interval, map[string]Counter{"wwan": {"d", 9, 9}})) // rewound
	h.Record(obs(t0.Add(2*Interval), time.Hour+2*Interval, map[string]Counter{"wwan": {"d", 20, 20}}))
	points, _ := h.Points(time.Time{})
	if len(points) != 2 {
		t.Fatalf("points = %+v, want 2", points)
	}
	if len(points[1].Uplinks) != 0 {
		t.Fatalf("a rewind restarts the delta chain: %+v", points[1].Uplinks)
	}
}

func TestSkipRestartsDeltaChain(t *testing.T) {
	h := NewHistory(time.Time{}, nil, PersistenceOK)
	h.Record(obs(t0, time.Hour, map[string]Counter{"wwan": {"d", 1, 1}}))
	h.Skip()
	h.Record(obs(t0.Add(2*Interval), time.Hour+2*Interval, map[string]Counter{"wwan": {"d", 9, 9}}))
	points, _ := h.Points(time.Time{})
	if len(points[1].Uplinks) != 0 {
		t.Fatalf("a delta across a failed sample spans two buckets: %+v", points[1].Uplinks)
	}
}

func TestRetentionAndSince(t *testing.T) {
	h := NewHistory(time.Time{}, nil, PersistenceOK)
	end := t0.Add(Retention + 5*Interval)
	for at := t0; !at.After(end); at = at.Add(Interval) {
		h.Record(obs(at, at.Sub(t0)+time.Hour, nil))
	}
	all, _ := h.Points(time.Time{})
	if len(all) > maxPoints+1 || time.Unix(all[0].At, 0).Before(end.Add(-Retention)) {
		t.Fatalf("retention: %d points from %v", len(all), time.Unix(all[0].At, 0))
	}
	day, _ := h.Points(end.Add(-24 * time.Hour))
	if len(day) != 24*6+1 {
		t.Fatalf("last 24 h = %d points, want %d", len(day), 24*6+1)
	}
}

type fakeUbus map[string][]byte

func (f fakeUbus) Call(_ context.Context, object, method string, _ any) (json.RawMessage, error) {
	if b, ok := f[object+" "+method]; ok {
		return b, nil
	}
	return nil, errors.New("no reply")
}

type fakeCounters map[string]device.InterfaceCounters

func (f fakeCounters) ReadInterfaceCounters(context.Context) (map[string]device.InterfaceCounters, error) {
	return f, nil
}

type fakeNeigh []byte

func (f fakeNeigh) Neighbours(context.Context) (json.RawMessage, error) {
	return json.RawMessage(f), nil
}

func read(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(fixtures + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestObserveFromCaptures(t *testing.T) {
	src := Sources{
		Ubus: fakeUbus{
			"system info":            read(t, "system-info.json"),
			"network.interface dump": read(t, "interface-dump.json"),
		},
		Counters:   fakeCounters{"phy0-sta0": {RXBytes: 7, TXBytes: 3}, "br-lan": {RXBytes: 1, TXBytes: 1}},
		Neighbours: fakeNeigh(read(t, "with-ap-client/ip-neigh.json")),
	}
	o, err := Observe(context.Background(), src, t0)
	if err != nil {
		t.Fatal(err)
	}
	if o.Uptime != 224*time.Second {
		t.Fatalf("uptime = %v", o.Uptime)
	}
	// wan is down in the capture; wwan is phy0-sta0; br-lan (lan) is not an uplink.
	if len(o.Uplinks) != 1 || o.Uplinks["wwan"] != (Counter{"phy0-sta0", 7, 3}) {
		t.Fatalf("uplinks = %+v", o.Uplinks)
	}
	// two br-lan neighbours; the upstream gateway on phy0-sta0 is not a client.
	if o.Clients == nil || *o.Clients != 2 {
		t.Fatalf("clients = %v", o.Clients)
	}
}

func TestObserveWithoutNeighboursLeavesClientsNull(t *testing.T) {
	src := Sources{
		Ubus:     fakeUbus{"system info": read(t, "system-info.json"), "network.interface dump": read(t, "interface-dump.json")},
		Counters: fakeCounters{},
	}
	o, err := Observe(context.Background(), src, t0)
	if err != nil || o.Clients != nil {
		t.Fatalf("o = %+v, err = %v", o, err)
	}
	if _, err := Observe(context.Background(), Sources{Ubus: fakeUbus{}, Counters: fakeCounters{}}, t0); err == nil {
		t.Fatal("unreadable system info must fail the sample")
	}
}

func TestStoreFlushCadenceAndShutdown(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stats-history.json")
	mono := t0
	s := Open(path, time.Time{}, func() time.Time { return mono })
	writes := 0
	write := s.write
	s.write = func(p string, b []byte, m os.FileMode) error { writes++; return write(p, b, m) }

	s.Record(obs(t0, time.Hour, nil))
	s.MaybeFlush()
	if writes != 0 {
		t.Fatal("no flush before FlushEvery")
	}
	mono = mono.Add(FlushEvery)
	s.MaybeFlush()
	s.MaybeFlush() // nothing new: no second write
	if writes != 1 {
		t.Fatalf("writes = %d, want 1", writes)
	}
	s.Record(obs(t0.Add(Interval), time.Hour+Interval, nil))
	s.Flush() // shutdown
	if writes != 2 {
		t.Fatalf("writes = %d, want 2", writes)
	}
	reopened := Open(path, time.Time{}, time.Now)
	points, persistence := reopened.Points(time.Time{})
	if persistence != PersistenceOK || len(points) != 2 {
		t.Fatalf("reload: %v %d points", persistence, len(points))
	}
}

func TestTornFileDegradesAndIsNeverOverwritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stats-history.json")
	torn := []byte(`{"version":1,"interval_seconds":600,"points":[{"at":`)
	if err := os.WriteFile(path, torn, 0o600); err != nil {
		t.Fatal(err)
	}
	s := Open(path, time.Time{}, time.Now)
	s.Record(obs(t0, time.Hour, nil))
	s.Flush()
	if b, _ := os.ReadFile(path); string(b) != string(torn) {
		t.Fatal("a degraded store overwrote the torn file")
	}
	points, persistence := s.Points(time.Time{})
	if persistence != PersistenceDegraded || len(points) != 1 {
		t.Fatalf("RAM collection continues while degraded: %v %d", persistence, len(points))
	}
	if err := s.Reset(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("reset removes the file")
	}
	if points, persistence := s.Points(time.Time{}); persistence != PersistenceOK || len(points) != 0 {
		t.Fatalf("after reset: %v %d", persistence, len(points))
	}
}

func TestFlushFailureDegrades(t *testing.T) {
	s := Open(filepath.Join(t.TempDir(), "h.json"), time.Time{}, time.Now)
	s.write = func(string, []byte, os.FileMode) error { return errors.New("no space") }
	s.Record(obs(t0, time.Hour, nil))
	s.Flush()
	if _, p := s.Points(time.Time{}); p != PersistenceDegraded {
		t.Fatalf("persistence = %v", p)
	}
}

func TestLoadRejectsForeignShapes(t *testing.T) {
	for name, body := range map[string]string{
		"interval": `{"version":1,"interval_seconds":60,"points":[]}`,
		"order":    `{"version":1,"interval_seconds":600,"points":[{"at":2,"uptime_seconds":1,"clients":null,"uplinks":[]},{"at":1,"uptime_seconds":1,"clients":null,"uplinks":[]}]}`,
		"uplink":   `{"version":1,"interval_seconds":600,"points":[{"at":2,"uptime_seconds":1,"clients":null,"uplinks":[{"name":"lan","rx_bytes":1,"tx_bytes":1}]}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "h.json")
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, p := load(path); p != PersistenceDegraded {
				t.Fatalf("persistence = %v", p)
			}
		})
	}
}
