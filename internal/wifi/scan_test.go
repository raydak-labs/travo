package wifi

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
)

type fakeScanner struct {
	mu       sync.Mutex
	replies  map[string]json.RawMessage
	failures map[string]error
	blocks   map[string]chan struct{}
	calls    []string
	started  chan string
}

func (f *fakeScanner) ScanWireless(ctx context.Context, name string) (json.RawMessage, error) {
	f.mu.Lock()
	f.calls = append(f.calls, name)
	block := f.blocks[name]
	started := f.started
	f.mu.Unlock()
	if started != nil {
		started <- name
	}
	if block != nil {
		select {
		case <-block:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if err := f.failures[name]; err != nil {
		return nil, err
	}
	return f.replies[name], nil
}

func (f *fakeScanner) called() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func scanFixtureService(t *testing.T, wireless string, replies map[string]string) (*Service, *fakeScanner) {
	t.Helper()
	scanner := &fakeScanner{replies: map[string]json.RawMessage{}}
	for name, path := range replies {
		scanner.replies[name] = fixture(t, path)
	}
	return New(context.Background(), summaryUbus(t, wireless), scanner), scanner
}

func noSTAScanService(t *testing.T) (*Service, *fakeScanner) {
	t.Helper()
	return scanFixtureService(t, "wifi-scan/no-sta/wireless-status.json", map[string]string{
		"radio0":   "wifi-scan/no-sta/iwinfo-scan-radio0.json",
		"phy1-ap0": "wifi-scan/no-sta/iwinfo-scan-phy1-ap0.json",
	})
}

func connectedScanService(t *testing.T) (*Service, *fakeScanner) {
	t.Helper()
	return scanFixtureService(t, "wifi-scan/connected/wireless-status.json", map[string]string{
		"phy0-sta0": "wifi-scan/connected/iwinfo-scan-phy0-sta0.json",
		"phy1-ap0":  "wifi-scan/no-sta/iwinfo-scan-phy1-ap0.json",
	})
}

func TestScanMergesNoSTABySSIDAndBand(t *testing.T) {
	s, scanner := noSTAScanService(t)
	got, err := s.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"radio0", "phy1-ap0"}; !reflect.DeepEqual(scanner.called(), want) {
		t.Fatalf("targets %v, want %v", scanner.called(), want)
	}
	wantUbusCalls := []ubusKey{{object: "network.wireless", method: "status"}}
	if calls := s.ubus.(*fakeUbus).calls; !reflect.DeepEqual(calls, wantUbusCalls) {
		t.Fatalf("ubus calls %v, want %v", calls, wantUbusCalls)
	}
	want := []struct {
		ssid   string
		signal int
		bands  []string
	}{
		{"HomeUplink", -31, []string{"2g", "5g"}},
		{"Neighbor2", -55, []string{"2g", "5g"}},
		{"Neighbor4", -69, []string{"2g", "5g"}},
		{"Neighbor1", -70, []string{"2g"}},
		{"Neighbor5", -76, []string{"2g"}},
		{"Neighbor3", -78, []string{"5g"}},
		{"Neighbor6", -82, []string{"5g"}},
	}
	if len(got.Networks) != len(want) {
		t.Fatalf("networks: %+v", got.Networks)
	}
	total := len(got.Hidden)
	for i, w := range want {
		n := got.Networks[i]
		if n.SSID != w.ssid || n.BestSignalDBM != w.signal || !reflect.DeepEqual(n.Bands, w.bands) {
			t.Errorf("%d: got %s %d %v, want %s %d %v", i, n.SSID, n.BestSignalDBM, n.Bands, w.ssid, w.signal, w.bands)
		}
		total += len(n.AccessPoints)
		if n.SSID == "Neighbor3" && n.Security != "wpa2_wpa3" {
			t.Errorf("Neighbor3 security %s", n.Security)
		}
	}
	if len(got.Hidden) != 4 || total != 14 {
		t.Fatalf("hidden/total %d/%d", len(got.Hidden), total)
	}
}

func TestScanConnectedStrongestAndSSIDTieBreak(t *testing.T) {
	s, scanner := connectedScanService(t)
	got, err := s.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"phy0-sta0", "phy1-ap0"}; !reflect.DeepEqual(scanner.called(), want) {
		t.Fatalf("targets %v, want %v", scanner.called(), want)
	}
	if got.Networks[0].SSID != "HomeUplink" || got.Networks[0].BestSignalDBM != -30 {
		t.Fatalf("first network: %+v", got.Networks[0])
	}
	positions := map[string]int{}
	for i, n := range got.Networks {
		positions[n.SSID] = i
	}
	if positions["Neighbor3"] > positions["Neighbor5"] || got.Networks[positions["Neighbor3"]].BestSignalDBM != -76 || got.Networks[positions["Neighbor5"]].BestSignalDBM != -76 {
		t.Fatalf("tie order: %+v", got.Networks)
	}
	if len(got.Hidden) != 4 {
		t.Fatalf("hidden: %+v", got.Hidden)
	}
}

func TestScanHiddenAPsAreSeparateAndSorted(t *testing.T) {
	s, _ := noSTAScanService(t)
	got, err := s.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range got.Networks {
		if n.SSID == "" {
			t.Fatal("hidden AP grouped under an empty SSID network")
		}
	}
	for i, ap := range got.Hidden {
		if ap.BSSID == "" {
			t.Fatal("hidden AP dropped BSSID")
		}
		if i > 0 {
			prev := got.Hidden[i-1]
			if prev.SignalDBM < ap.SignalDBM || (prev.SignalDBM == ap.SignalDBM && prev.BSSID > ap.BSSID) {
				t.Fatalf("hidden not sorted at %d: %+v", i, got.Hidden)
			}
		}
	}
}

func TestScanRadioFailuresAreIndependent(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		prepare    func(*Service, *fakeScanner)
	}{
		{name: "scanner", code: "scan_failed", prepare: func(_ *Service, s *fakeScanner) {
			s.failures = map[string]error{"radio0": errInjected}
		}},
		{name: "timeout", code: "scan_timeout", prepare: func(svc *Service, s *fakeScanner) {
			svc.RadioTimeout = 50 * time.Millisecond
			s.blocks = map[string]chan struct{}{"radio0": make(chan struct{})}
		}},
		{name: "garbage", code: "bad_payload", prepare: func(_ *Service, s *fakeScanner) {
			s.replies["radio0"] = json.RawMessage("garbage")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, scanner := noSTAScanService(t)
			tc.prepare(svc, scanner)
			got, err := svc.Scan(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Radios) != 2 || got.Radios[0].Status != "error" || got.Radios[0].Code != tc.code || got.Radios[1].Status != "ok" {
				t.Fatalf("radios: %+v", got.Radios)
			}
			if len(got.Networks) == 0 {
				t.Fatal("successful radio's networks were dropped")
			}
		})
	}
}

func TestScanMutatedFixtures(t *testing.T) {
	t.Run("mutated: disabled radios are skipped, not attempted", func(t *testing.T) {
		status := mutatedFixture(t, "wifi-scan/no-sta/wireless-status.json", func(payload map[string]any) {
			for _, entry := range payload {
				radio := entry.(map[string]any)
				radio["disabled"] = true
				radio["config"].(map[string]any)["disabled"] = true
			}
		})
		u := &fakeUbus{replies: map[ubusKey]json.RawMessage{
			{object: "network.wireless", method: "status"}: status,
		}}
		scanner := &fakeScanner{}
		got, err := New(context.Background(), u, scanner).Scan(context.Background())
		if err != nil {
			t.Fatalf("all-disabled scan must not fail as an attempted scan: %v", err)
		}
		wantRadios := []RadioScan{
			{Radio: "radio0", Band: "5g", Status: "skipped"},
			{Radio: "radio1", Band: "2g", Status: "skipped"},
		}
		if !reflect.DeepEqual(got.Radios, wantRadios) {
			t.Fatalf("radios: got %+v, want %+v", got.Radios, wantRadios)
		}
		if got.Networks == nil || got.Hidden == nil || len(got.Networks)+len(got.Hidden) != 0 {
			t.Fatalf("disabled radios returned scan data: %+v", got)
		}
		if calls := scanner.called(); len(calls) != 0 {
			t.Fatalf("disabled radios were scanned: %v", calls)
		}
		wantUbusCalls := []ubusKey{{object: "network.wireless", method: "status"}}
		if !reflect.DeepEqual(u.calls, wantUbusCalls) {
			t.Fatalf("ubus calls %v, want %v", u.calls, wantUbusCalls)
		}
	})

	t.Run("mutated: unknown numeric band is bad_payload", func(t *testing.T) {
		svc, scanner := noSTAScanService(t)
		scanner.replies["radio0"] = mutatedFixture(t, "wifi-scan/no-sta/iwinfo-scan-radio0.json", func(payload map[string]any) {
			results := payload["results"].([]any)
			results[0].(map[string]any)["band"] = 7
		})
		got, err := svc.Scan(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Radios) != 2 || got.Radios[0].Status != "error" || got.Radios[0].Code != "bad_payload" || got.Radios[1].Status != "ok" {
			t.Fatalf("radios: %+v", got.Radios)
		}
		if want := []string{"radio0", "phy1-ap0"}; !reflect.DeepEqual(scanner.called(), want) {
			t.Fatalf("targets %v, want %v", scanner.called(), want)
		}
	})
}

func TestScanAllAttemptedRadiosFailed(t *testing.T) {
	svc, scanner := noSTAScanService(t)
	scanner.failures = map[string]error{"radio0": errInjected, "phy1-ap0": errInjected}
	_, err := svc.Scan(context.Background())
	if !errors.Is(err, ErrScanFailed) {
		t.Fatalf("%v", err)
	}
}

func TestScanWithoutRadiosReturnsEmptyCollections(t *testing.T) {
	u := &fakeUbus{replies: map[ubusKey]json.RawMessage{{object: "network.wireless", method: "status"}: json.RawMessage(`{}`)}}
	scanner := &fakeScanner{}
	got, err := New(context.Background(), u, scanner).Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Networks == nil || got.Hidden == nil || got.Radios == nil || len(got.Networks)+len(got.Hidden)+len(got.Radios) != 0 {
		t.Fatalf("%#v", got)
	}
	if len(scanner.called()) != 0 {
		t.Fatalf("scanner calls: %v", scanner.called())
	}
}

func TestScanUnavailable(t *testing.T) {
	key := ubusKey{object: "network.wireless", method: "status"}
	u := &fakeUbus{replies: map[ubusKey]json.RawMessage{key: json.RawMessage("bad")}}
	_, err := New(context.Background(), u, &fakeScanner{}).Scan(context.Background())
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("%v", err)
	}
}

func TestSecurityFromCapturedShapes(t *testing.T) {
	aps, err := facts.ParseIwinfoScan(fixture(t, "wifi-scan/connected/iwinfo-scan-phy0-sta0.json"))
	if err != nil {
		t.Fatal(err)
	}
	var wpa2, mixed facts.Encryption
	for _, ap := range aps {
		if ap.SSID == "HomeUplink" {
			wpa2 = ap.Encryption
		}
		if ap.SSID == "Neighbor3" {
			mixed = ap.Encryption
		}
	}
	if got := securityLabel(wpa2); got != "wpa2" {
		t.Fatalf("wpa2: %s", got)
	}
	if got := securityLabel(mixed); got != "wpa2_wpa3" {
		t.Fatalf("mixed: %s", got)
	}
	// mutated: no open network captured yet
	open := wpa2
	open.Enabled = false
	if got := securityLabel(open); got != "open" {
		t.Fatalf("open: %s", got)
	}
}
