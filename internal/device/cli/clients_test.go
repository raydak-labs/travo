package cli

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

func TestDHCPLeasesReadsTheLeaseFileAndTreatsAbsentAsEmpty(t *testing.T) {
	d := Device{ReadFile: func(path string) ([]byte, error) {
		if path != LeaseFile {
			t.Fatal(path)
		}
		return []byte("1795000977 46:ba:e6:8d:7d:6b 192.168.1.119 REDACTED-HOST *\n"), nil
	}}
	if b, err := d.DHCPLeases(t.Context()); err != nil || len(b) == 0 {
		t.Fatal(b, err)
	}
	d.ReadFile = func(string) ([]byte, error) { return nil, os.ErrNotExist }
	if b, err := d.DHCPLeases(t.Context()); err != nil || len(b) != 0 {
		t.Fatal(b, err)
	}
	d.ReadFile = func(string) ([]byte, error) { return nil, os.ErrPermission }
	if _, err := d.DHCPLeases(t.Context()); err == nil {
		t.Fatal("an unreadable lease file read as empty")
	}
}

func TestKickStationCallsDelClient(t *testing.T) {
	var got []string
	d := Device{Run: func(ctx context.Context, name string, args ...string) (string, int, error) {
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("no timeout tier")
		}
		got = append([]string{name}, args...)
		return "", 0, nil
	}}
	if err := d.KickStation(t.Context(), "phy1-ap0", "22:4e:76:6c:2d:62"); err != nil {
		t.Fatal(err)
	}
	want := []string{"ubus", "call", "hostapd.phy1-ap0", "del_client", `{"addr":"22:4e:76:6c:2d:62","ban_time":0,"deauth":true,"reason":5}`}
	if !slices.Equal(got, want) {
		t.Fatal(got)
	}
}

func TestKickStationMapsNotFoundAndRefusesBadInput(t *testing.T) {
	d := Device{Run: func(context.Context, string, ...string) (string, int, error) {
		return "", 4, failedCommand("ubus", "Command failed: Not found")
	}}
	if err := d.KickStation(t.Context(), "phy1-ap0", "22:4e:76:6c:2d:62"); !errors.Is(err, device.ErrNotFound) {
		t.Fatal(err)
	}
	d.Run = func(context.Context, string, ...string) (string, int, error) {
		t.Fatal("ran with invalid input")
		return "", 0, nil
	}
	for _, tc := range [][2]string{{"phy1-ap0; reboot", "22:4e:76:6c:2d:62"}, {"phy1-ap0", "22:4E:76:6C:2D:62"}, {"phy1-ap0", "x"}} {
		if err := d.KickStation(t.Context(), tc[0], tc[1]); err == nil {
			t.Fatal(tc)
		}
	}
}
