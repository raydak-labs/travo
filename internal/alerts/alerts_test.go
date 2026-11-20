package alerts

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/datausage"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/wifireconnect"
)

const mountsCapture = "../../testdata/device/axt1800-25.12.6/system/proc-mounts.txt"

func capture(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(mountsCapture)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestDegradedFromMounts(t *testing.T) {
	healthy := capture(t)
	if Degraded(healthy) {
		t.Fatal("AXT1800 ubifs overlay read as degraded")
	}
	// The same capture with the overlay line as OpenWrt's fallbacks write it: a tmpfs
	// overlay after a failed flash mount, and a flash overlay remounted read-only.
	const ubi = "/dev/ubi0_2 /overlay ubifs rw,"
	if !strings.Contains(healthy, ubi) {
		t.Fatal("capture changed")
	}
	for _, line := range []string{"tmpfs /overlay tmpfs rw,", "/dev/ubi0_2 /overlay ubifs ro,"} {
		if !Degraded(strings.Replace(healthy, ubi, line, 1)) {
			t.Errorf("%q not degraded", line)
		}
	}
	if Degraded("/dev/sda2 / ext4 rw,relatime 0 0\n") {
		t.Fatal("no overlay is not degraded")
	}
}

func TestDeriveMapsEverySource(t *testing.T) {
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	low := uint64(OverlayMinFree - 1)
	mounts := "tmpfs /overlay tmpfs rw 0 0\n"
	failOpen := now.Add(-time.Hour)
	got := Derive(Inputs{
		Now: now,
		Guards: []guard.Info{
			{Name: "wifi-apply-in-progress", Started: now.Add(-time.Minute)}, // running operation
			{Name: "adguard-install-in-progress", Started: now.Add(-StaleGuardAfter)},
			{Name: "firmware-upgrade-in-progress"}, // unreadable start time
		},
		OverlayFree: &low,
		Mounts:      &mounts,
		FailOpen:    &failOpen,
		Reconnect:   &wifireconnect.Status{State: wifireconnect.StateExhausted, Attempts: 5, MaxAttempts: 5},
		DataBudget:  datausage.StateWarning,
	})
	ids := []string{}
	for _, a := range got {
		ids = append(ids, string(a.Severity)+" "+a.ID)
	}
	want := []string{
		"danger adguard_fail_open",
		"danger persistence_degraded",
		"danger stale_guard:adguard-install-in-progress",
		"danger stale_guard:firmware-upgrade-in-progress",
		"warning data_budget_warning",
		"warning overlay_low",
		"warning uplink_reconnect_exhausted",
	}
	if strings.Join(ids, "|") != strings.Join(want, "|") {
		t.Fatalf("got %q", ids)
	}
	if got[0].Since == nil || !got[0].Since.Equal(failOpen) || got[3].Since != nil {
		t.Fatalf("since %v %v", got[0].Since, got[3].Since)
	}
}

// The captured router (44 MiB available) is healthy; the bounds are strict.
func TestDeriveMemoryLow(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/device/axt1800-25.12.6/system/system-info-meminfo.json")
	if err != nil {
		t.Fatal(err)
	}
	info, err := facts.ParseSystemInfo(raw)
	if err != nil {
		t.Fatal(err)
	}
	meminfo, err := os.ReadFile("../../testdata/device/axt1800-25.12.6/system/proc-meminfo.txt")
	if err != nil || !strings.Contains(string(meminfo), "MemAvailable:      45480 kB") || info.MemAvailable != 45480*1024 {
		t.Fatalf("system info available %d is not the captured MemAvailable: %v", info.MemAvailable, err)
	}
	for avail, want := range map[uint64]string{
		info.MemAvailable:     "",
		MemoryWarnBelow:       "",
		MemoryWarnBelow - 1:   "warning memory_low",
		MemoryDangerBelow:     "warning memory_low",
		MemoryDangerBelow - 1: "danger memory_low",
	} {
		got := ""
		for _, a := range Derive(Inputs{Now: time.Now(), MemAvailable: &avail}) {
			got = string(a.Severity) + " " + a.ID
		}
		if got != want {
			t.Errorf("%d: %q, want %q", avail, got, want)
		}
	}
}

func TestDeriveDataBudget(t *testing.T) {
	for state, want := range map[datausage.State]string{
		datausage.StateExceeded: "danger data_budget_exceeded",
		datausage.StateWarning:  "warning data_budget_warning",
		datausage.StateOK:       "", datausage.StateOff: "", datausage.StateUnreadable: "",
	} {
		got := ""
		for _, a := range Derive(Inputs{Now: time.Now(), DataBudget: state}) {
			got = string(a.Severity) + " " + a.ID
		}
		if got != want {
			t.Errorf("%s: %q, want %q", state, got, want)
		}
	}
}

func TestDeriveFirmwareCompletedIsInfoAfterDanger(t *testing.T) {
	at := time.Date(2026, 11, 19, 8, 0, 0, 0, time.UTC)
	got := Derive(Inputs{Now: at, FirmwareCompleted: &at, DataBudget: datausage.StateWarning, FailOpen: &at})
	var order []string
	for _, a := range got {
		order = append(order, string(a.Severity)+" "+a.ID)
	}
	want := []string{"danger adguard_fail_open", "info firmware_upgrade_completed", "warning data_budget_warning"}
	if !slices.Equal(order, want) {
		t.Fatalf("%v, want %v", order, want)
	}
	if got[1].Since == nil || !got[1].Since.Equal(at) {
		t.Fatalf("since %v", got[1].Since)
	}
}

func TestDeriveHealthyIsEmpty(t *testing.T) {
	free := uint64(64 << 20)
	mounts := capture(t)
	got := Derive(Inputs{Now: time.Now(), OverlayFree: &free, Mounts: &mounts, Reconnect: &wifireconnect.Status{State: wifireconnect.StateRetrying}})
	if got == nil || len(got) != 0 {
		t.Fatalf("%v", got)
	}
}

func TestListSkipsFailedSources(t *testing.T) {
	boom := errors.New("boom")
	s := &Service{Sources: Sources{
		Now:          time.Now,
		Guards:       func() ([]guard.Info, error) { return nil, boom },
		OverlayFree:  func() (uint64, error) { return 0, boom },
		Mounts:       func() (string, error) { return "", boom },
		FailOpen:     func() (*time.Time, error) { return nil, boom },
		Reconnect:    func(context.Context) (wifireconnect.Status, error) { return wifireconnect.Status{}, boom },
		DataBudget:   func(context.Context) (datausage.State, error) { return datausage.StateExceeded, boom },
		MemAvailable: func(context.Context) (uint64, error) { return 0, boom },
	}}
	l, err := s.List(context.Background())
	if err != nil || len(l.Alerts) != 0 {
		t.Fatalf("%v %v", l, err)
	}
}
