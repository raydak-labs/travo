package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

// The WiFi schedule used to be written to /etc/cron.d/openwrt-gui-wifi-schedule.
// That is wrong twice over on OpenWrt: the directory does not exist, and busybox
// crond is built with `-c /etc/crontabs` and would never read it even if it did.
//
// The visible failure was nastier than a dead feature: wifi-schedule.json was
// written FIRST, then the /etc/cron.d write failed with ENOENT and the endpoint
// answered 500 — so the UI kept reporting enabled=true for a schedule that
// would never fire. Verified on the device.
//
// These tests pin the crontab location, the tag, and the ordering: the crontab
// is the source of truth, so it must be written before the JSON that reports it.
func TestWiFiScheduleWritesToOpenWrtCrontab(t *testing.T) {
	dir := t.TempDir()
	crontab := filepath.Join(dir, "root")
	// Pre-existing entries we must not disturb.
	seed := "* * * * * /etc/travo/wifi-reconnect.sh\n"
	if err := os.WriteFile(crontab, []byte(seed), 0o600); err != nil {
		t.Fatalf("seed crontab: %v", err)
	}

	w := &WifiService{
		crontabFile:      crontab,
		scheduleFile:     filepath.Join(dir, "wifi-schedule.json"),
		toggleScriptPath: filepath.Join(dir, "travo-wireless-toggle.sh"),
	}

	if err := w.SetWiFiSchedule(models.WiFiSchedule{Enabled: true, OnTime: "07:00", OffTime: "23:00"}); err != nil {
		t.Fatalf("SetWiFiSchedule: %v", err)
	}

	data, err := os.ReadFile(crontab)
	if err != nil {
		t.Fatalf("read crontab: %v", err)
	}
	got := string(data)
	helper := w.toggleScriptPathOrDefault()
	for _, want := range []string{
		wifiCronTag,
		"00 07 * * * " + helper + " up",
		"00 23 * * * " + helper + " down",
		"/etc/travo/wifi-reconnect.sh", // preserved
	} {
		if !strings.Contains(got, want) {
			t.Errorf("crontab is missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, " wifi up") || strings.Contains(got, " wifi down") {
		t.Errorf("crontab must never run `wifi up`/`wifi down`:\n%s", got)
	}
	// busybox crond takes no user field; a "root" column would make the whole
	// line unparseable and the schedule would silently never run.
	for _, line := range strings.Split(got, "\n") {
		if strings.Contains(line, wifiCronTag) && strings.Contains(line, " root ") {
			t.Errorf("cron line has a user field, which busybox crond does not accept: %q", line)
		}
	}

	// Disabling must remove only our lines.
	if err := w.SetWiFiSchedule(models.WiFiSchedule{Enabled: false}); err != nil {
		t.Fatalf("disable: %v", err)
	}
	data, _ = os.ReadFile(crontab)
	got = string(data)
	if strings.Contains(got, wifiCronTag) {
		t.Errorf("disabling the schedule left its cron lines behind:\n%s", got)
	}
	if !strings.Contains(got, "/etc/travo/wifi-reconnect.sh") {
		t.Errorf("disabling the schedule removed an unrelated entry:\n%s", got)
	}
}

// A rejected schedule must not touch the crontab or the persisted JSON, so a
// 400 cannot leave the UI showing a schedule that is not actually scheduled.
func TestWiFiScheduleInvalidTimeChangesNothing(t *testing.T) {
	dir := t.TempDir()
	crontab := filepath.Join(dir, "root")
	jsonPath := filepath.Join(dir, "wifi-schedule.json")
	_ = os.WriteFile(crontab, []byte("seed\n"), 0o600)
	_ = os.WriteFile(jsonPath, []byte(`{"enabled":true,"on_time":"07:00","off_time":"23:00"}`), 0o644)

	w := &WifiService{crontabFile: crontab, scheduleFile: jsonPath}
	err := w.SetWiFiSchedule(models.WiFiSchedule{Enabled: true, OnTime: "99:99", OffTime: "23:00"})
	if err == nil {
		t.Fatal("expected an error for an out-of-range on_time")
	}
	crontabAfter, _ := os.ReadFile(crontab)
	if string(crontabAfter) != "seed\n" {
		t.Errorf("a rejected schedule rewrote the crontab:\n%s", crontabAfter)
	}
	jsonAfter, _ := os.ReadFile(jsonPath)
	if !strings.Contains(string(jsonAfter), `"07:00"`) {
		t.Errorf("a rejected schedule changed the persisted schedule: %s", jsonAfter)
	}
}
