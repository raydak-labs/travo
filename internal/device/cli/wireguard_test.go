package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

func wireGuardFixture(t *testing.T, state, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("../../../testdata/device/axt1800-25.12.6/wireguard", state, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestLatestHandshakesFromFixtures(t *testing.T) {
	for _, tc := range []struct {
		state string
		at    time.Time
	}{
		{state: "connected", at: time.Unix(1794610022, 0).UTC()},
		{state: "no-handshake", at: time.Time{}},
	} {
		t.Run(tc.state, func(t *testing.T) {
			var calls []call
			d := &Device{Run: fake(&calls, map[string]string{
				"wg show wg0 latest-handshakes": wireGuardFixture(t, tc.state, "wg-latest-handshakes.txt"),
			}, "")}
			got, err := d.LatestHandshakes(context.Background(), "wg0")
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 || got[0].PublicKey != "KgTUh3KLijVluDvNpzDCJJfrJ7EyLzYLmdHCksG4sRg=" || !got[0].At.Equal(tc.at) {
				t.Fatalf("got %+v", got)
			}
			if len(calls) != 1 || calls[0].name != "wg" || calls[0].args != "show wg0 latest-handshakes" {
				t.Fatalf("argv: %+v", calls)
			}
		})
	}
}

func TestLatestHandshakesMapsMissingDevice(t *testing.T) {
	message := strings.TrimSpace(wireGuardFixture(t, "disabled", "wg-latest-handshakes.txt"))
	d := &Device{Run: func(context.Context, string, ...string) (string, int, error) {
		return "", 1, failedCommand("wg", message)
	}}
	_, err := d.LatestHandshakes(context.Background(), "wg0")
	if !errors.Is(err, device.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestLatestHandshakesDoesNotMapDifferentDiagnosticContainingNoSuchDevice(t *testing.T) {
	d := &Device{Run: func(context.Context, string, ...string) (string, int, error) {
		return "", 1, failedCommand("wg", "Unable to access interface: No such device or address")
	}}
	_, err := d.LatestHandshakes(context.Background(), "wg0")
	if errors.Is(err, device.ErrNotFound) {
		t.Fatalf("unrecognized diagnostic mapped to ErrNotFound: %v", err)
	}
}

func TestLatestHandshakesRejectsInvalidInterfaceWithoutRunning(t *testing.T) {
	for _, iface := range []string{"wg0 --x", "all", "interfaces"} {
		t.Run(iface, func(t *testing.T) {
			runs := 0
			d := &Device{Run: func(context.Context, string, ...string) (string, int, error) {
				runs++
				return "", 0, nil
			}}
			if _, err := d.LatestHandshakes(context.Background(), iface); err == nil {
				t.Fatal("accepted invalid interface")
			}
			if runs != 0 {
				t.Fatalf("ran %d commands", runs)
			}
		})
	}
}

func TestLatestHandshakesRejectsMalformedLine(t *testing.T) {
	var calls []call
	d := &Device{Run: fake(&calls, map[string]string{
		"wg show wg0 latest-handshakes": wireGuardFixture(t, "connected", "date.txt"),
	}, "")}
	if _, err := d.LatestHandshakes(context.Background(), "wg0"); err == nil {
		t.Fatal("accepted malformed handshake output")
	}
}

func TestCallMapsMissingUbusObject(t *testing.T) {
	message := strings.TrimSpace(wireGuardFixture(t, "disabled", "interface-wg0-status.json"))
	d := &Device{Run: func(context.Context, string, ...string) (string, int, error) {
		return "", 1, failedCommand("ubus", message)
	}}
	_, err := d.Call(context.Background(), "network.interface.wg0", "status", nil)
	if !errors.Is(err, device.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}
