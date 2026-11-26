package facts

import (
	"errors"
	"os"
	"testing"
	"time"
)

func TestParseLeasesCapture(t *testing.T) {
	raw, err := os.ReadFile("../../../testdata/device/axt1800-25.12.6/clients/dhcp-leases.txt")
	if err != nil {
		t.Fatal(err)
	}
	leases, err := ParseLeases(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(leases) != 2 {
		t.Fatal(leases)
	}
	l := leases[0]
	if l.MAC != "46:ba:e6:8d:7d:6b" || l.IP.String() != "192.168.1.119" || l.Hostname != "REDACTED-HOST" ||
		!l.Expires.Equal(time.Unix(1795000977, 0)) {
		t.Fatalf("%+v", l)
	}
}

func TestParseLeasesEdgeLines(t *testing.T) {
	text := "duid 00:01:00:01:aa\n0 AA:BB:CC:00:11:22 192.168.1.5 * *\n1700000000 aa:bb:cc:00:11:33 fd00::5 host *\n\n"
	leases, err := ParseLeases(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(leases) != 1 || leases[0].MAC != "aa:bb:cc:00:11:22" || leases[0].Hostname != "" || !leases[0].Expires.IsZero() {
		t.Fatalf("%+v", leases)
	}
	for _, bad := range []string{"1 aa:bb:cc:00:11:22", "x aa:bb:cc:00:11:22 192.168.1.5 h *", "1 aa:bb:cc:00:11:22 nope h *"} {
		if _, err := ParseLeases(bad); err == nil {
			t.Fatal(bad)
		}
	}
	if _, err := ParseLeases("1 aa"); !errors.Is(err, ErrShape) {
		t.Fatal(err)
	}
}

func TestParseHostapdStationsCapture(t *testing.T) {
	raw, err := os.ReadFile("../../../testdata/device/axt1800-25.12.6/with-ap-client/hostapd-phy1-ap0-get_clients.json")
	if err != nil {
		t.Fatal(err)
	}
	st, err := ParseHostapdStations(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(st) != 1 || st[0].MAC != "22:4e:76:6c:2d:62" || st[0].Signal == nil || *st[0].Signal != -38 {
		t.Fatalf("%+v", st)
	}
	if _, err := ParseHostapdStations([]byte(`{"clients":{}}`)); !errors.Is(err, ErrShape) {
		t.Fatal(err)
	}
}
