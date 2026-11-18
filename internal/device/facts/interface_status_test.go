package facts

import (
	"errors"
	"net/netip"
	"testing"
)

func TestInterfaceStatusFromDevice(t *testing.T) {
	up, err := ParseInterfaceStatus(read(t, "interface-wwan-status.json"))
	if err != nil || !up.Up || up.Uptime != 195 || len(up.IPv4) != 1 || up.IPv4[0] != netip.MustParsePrefix("192.168.90.241/24") {
		t.Fatalf("up %+v, %v", up, err)
	}
	down, err := ParseInterfaceStatus(read(t, "wwan-down/interface-wwan-status-down.json"))
	if err != nil || down.Up || down.Uptime != 0 || len(down.IPv4) != 0 {
		t.Fatalf("down %+v, %v", down, err)
	}
}

func TestInterfaceStatusRejectsUnknownShapes(t *testing.T) {
	for _, raw := range []string{
		`{}`,
		`not json`,
		`{"up":true,"ipv4-address":[{"address":"10.0.0.1"}]}`,
		`{"up":true,"ipv4-address":[{"address":"nope","mask":24}]}`,
		`{"up":true,"ipv4-address":[{"address":"10.0.0.1","mask":99}]}`,
	} {
		if _, err := ParseInterfaceStatus([]byte(raw)); err == nil {
			t.Fatalf("%s: expected error", raw)
		}
	}
	if _, err := ParseInterfaceStatus([]byte(`{}`)); !errors.Is(err, ErrShape) {
		t.Fatalf("missing up must be ErrShape: %v", err)
	}
}
