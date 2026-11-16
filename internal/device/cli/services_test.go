package cli

import (
	"context"
	"os"
	"slices"
	"testing"
)

func TestParseServiceRunningDeviceFixtures(t *testing.T) {
	for file, want := range map[string]bool{
		"service-list-adguardhome.json": true,
		"service-list-absent.json":      false,
	} {
		raw, err := os.ReadFile("../../../testdata/device/axt1800-25.12.6/adguard/" + file)
		if err != nil {
			t.Fatal(err)
		}
		got, err := ParseServiceRunning(raw, "adguardhome")
		if err != nil || got != want {
			t.Fatalf("%s: got %v, %v; want %v", file, got, err, want)
		}
	}
}

func TestServiceActionRunsInitScriptAndRejectsOthers(t *testing.T) {
	var calls [][]string
	d := &Device{Run: func(_ context.Context, name string, args ...string) (string, int, error) {
		calls = append(calls, append([]string{name}, args...))
		return "", 0, nil
	}}
	if err := d.ServiceAction(context.Background(), "adguardhome", "restart"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(calls[0], []string{"/etc/init.d/adguardhome", "restart"}) {
		t.Fatalf("calls = %v", calls)
	}
	for _, bad := range [][2]string{{"adguardhome", "reload; rm"}, {"../x", "start"}, {"network", "restart"}, {"adguardhome", "boot"}} {
		if err := d.ServiceAction(context.Background(), bad[0], bad[1]); err == nil {
			t.Fatalf("%v accepted", bad)
		}
	}
	if len(calls) != 1 {
		t.Fatalf("rejected actions ran: %v", calls)
	}
}
