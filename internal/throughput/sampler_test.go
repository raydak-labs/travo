package throughput

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

func TestCapturedCounters(t *testing.T) {
	file, err := os.Open("../../testdata/device/axt1800-25.12.6/wwan-down/net-counters.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	rows := map[string][]sample{}
	scan := bufio.NewScanner(file)
	for scan.Scan() {
		var seconds int64
		var name string
		var counters device.InterfaceCounters
		if _, err := fmt.Sscan(scan.Text(), &seconds, &name, &counters.RXBytes, &counters.TXBytes); err != nil {
			t.Fatal(err)
		}
		rows[name] = append(rows[name], sample{at: time.Unix(seconds, 0), counters: counters})
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]float64{"br-lan": 0, "lo": 688.0 / 3} {
		t.Run(name, func(t *testing.T) {
			if len(rows[name]) != 2 {
				t.Fatalf("expected exactly two captured rows for %s", name)
			}
			s := New()
			for i, row := range rows[name] {
				s.record(int64(i), row.at, map[string]device.InterfaceCounters{name: row.counters}, nil)
			}
			checkRate(t, s, name, want, want)
		})
	}
}

func TestMissingAndResetSamples(t *testing.T) {
	s := New()
	at := time.Now()
	counters := func(rx, tx uint64) map[string]device.InterfaceCounters {
		return map[string]device.InterfaceCounters{"lan1": {RXBytes: rx, TXBytes: tx}}
	}
	s.record(0, at, counters(100, 100), nil)
	checkUnknown(t, s)
	s.record(1, at.Add(Interval), counters(50, 110), nil)
	checkRate(t, s, "lan1", 0, 5)
	s.record(3, at.Add(3*Interval), counters(55, 120), nil)
	checkUnknown(t, s)
	s.record(4, at.Add(3*Interval), counters(60, 125), nil)
	checkUnknown(t, s)
	s.record(5, at.Add(5*Interval), nil, errors.New("failed read"))
	if _, ok := s.Latest(); ok {
		t.Fatal("failed snapshot must be unavailable")
	}
	s.record(6, at.Add(6*Interval), counters(65, 130), nil)
	checkUnknown(t, s)
	s.record(7, at.Add(7*Interval), nil, nil)
	if rates, ok := s.Latest(); !ok || rates == nil || len(rates) != 0 {
		t.Fatalf("empty successful snapshot: %+v, %v", rates, ok)
	}
	s.record(8, at.Add(8*Interval), counters(70, 135), nil)
	checkUnknown(t, s)
	s.record(9, at.Add(9*Interval), counters(74, 141), nil)
	checkRate(t, s, "lan1", 2, 3)
}

func checkRate(t *testing.T, s *Sampler, name string, rx, tx float64) {
	t.Helper()
	rates, ok := s.Latest()
	if !ok || len(rates) != 1 || rates[0].Name != name ||
		rates[0].RXBytesPerSecond == nil || *rates[0].RXBytesPerSecond != rx ||
		rates[0].TXBytesPerSecond == nil || *rates[0].TXBytesPerSecond != tx {
		t.Fatalf("expected %s RX %v TX %v, got %+v, %v", name, rx, tx, rates, ok)
	}
}

func checkUnknown(t *testing.T, s *Sampler) {
	t.Helper()
	rates, ok := s.Latest()
	if !ok || len(rates) != 1 || rates[0].RXBytesPerSecond != nil || rates[0].TXBytesPerSecond != nil {
		t.Fatalf("expected unknown rates, got %+v, %v", rates, ok)
	}
}

func TestRetentionAndOrdering(t *testing.T) {
	s := New()
	at := time.Now()
	for i := 0; i <= maxSamples; i++ {
		s.record(int64(i), at.Add(time.Duration(i)*Interval), map[string]device.InterfaceCounters{
			"wan": {}, "br-lan": {},
		}, nil)
	}
	if len(s.history["wan"]) != maxSamples || s.history["wan"][0].slot != 1 {
		t.Fatal("oldest sample was not evicted at the history cap")
	}
	rates, _ := s.Latest()
	if len(rates) != 2 || rates[0].Name != "br-lan" || rates[1].Name != "wan" {
		t.Fatalf("unexpected ordering: %+v", rates)
	}
	s.record(301, at.Add(retention+time.Duration(maxSamples)*Interval+time.Second), nil, nil)
	if len(s.history) != 0 {
		t.Fatal("disappeared interface histories must expire")
	}
}

type counterReader func(context.Context) (map[string]device.InterfaceCounters, error)

func (r counterReader) ReadInterfaceCounters(ctx context.Context) (map[string]device.InterfaceCounters, error) {
	return r(ctx)
}

func TestRunSamplesImmediatelyAndHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	s := New()
	s.Run(ctx, counterReader(func(ctx context.Context) (map[string]device.InterfaceCounters, error) {
		calls++
		cancel()
		return nil, ctx.Err()
	}), time.Now)
	if calls != 1 {
		t.Fatalf("expected one immediate sample, got %d", calls)
	}
	if _, ok := s.Latest(); ok {
		t.Fatal("canceled sample must be unavailable")
	}
}
