package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
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

// The auto-reconnect cron entry and the WiFi/LED schedules all rewrite the one
// crontab file in place. They must therefore share crontabMu: without it, a
// SetAutoReconnect concurrent with a SetWiFiSchedule has both read the same base
// and each writes its own version, so one silently drops the other's entries.
// main.go reconciles auto-reconnect at 5s and the schedules at 6s, so the two
// writers really do overlap on every boot.
func TestAutoReconnectWaitsForTheCrontabLock(t *testing.T) {
	dir := t.TempDir()

	var ran bool
	cmd := &MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		ran = true
		return nil, nil
	}}

	w := NewWifiServiceForTesting(uci.NewMockUCI(), ubus.NewMockUbus(), &NoopWifiReloader{}, cmd,
		filepath.Join(dir, "priorities.json"),
		filepath.Join(dir, "autoreconnect.json"),
		filepath.Join(dir, "wifi-reconnect.sh"))

	// Hold the lock the schedule writers take, then ask for a crontab rewrite.
	crontabMu.Lock()
	done := make(chan error, 1)
	go func() { done <- w.SetAutoReconnect(true) }()

	select {
	case <-done:
		crontabMu.Unlock()
		t.Fatal("auto-reconnect rewrote the crontab while another writer held the lock")
	case <-time.After(200 * time.Millisecond):
		// Expected: the rewrite is waiting, not racing.
	}
	crontabMu.Unlock()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("SetAutoReconnect: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("SetAutoReconnect never completed after the lock was released")
	}
	if !ran {
		t.Error("expected the crontab command to run once the lock was free")
	}
}

// busybox crond re-reads /etc/crontabs/root while it runs, so removing the
// schedule lines must swap the file by rename rather than truncate it in place:
// a crond that reads a half-written file parses it as the real crontab and drops
// the auto-reconnect and LED entries. A truncating os.WriteFile shows up as the
// same inode before and after the rewrite.
func TestRemoveWiFiScheduleCronLinesReplacesTheFileAtomically(t *testing.T) {
	dir := t.TempDir()
	crontab := filepath.Join(dir, "root")
	seed := "* * * * * /etc/trafo/wifi-reconnect.sh\n"
	if err := os.WriteFile(crontab, []byte(seed), 0o600); err != nil {
		t.Fatalf("seed crontab: %v", err)
	}
	before, err := os.Stat(crontab)
	if err != nil {
		t.Fatalf("stat crontab: %v", err)
	}

	w := &WifiService{
		crontabFile:  crontab,
		scheduleFile: filepath.Join(dir, "wifi-schedule.json"),
	}
	if err := w.removeWiFiScheduleCronLines(); err != nil {
		t.Fatalf("removeWiFiScheduleCronLines: %v", err)
	}

	after, err := os.Stat(crontab)
	if err != nil {
		t.Fatalf("stat crontab after: %v", err)
	}
	if os.SameFile(before, after) {
		t.Error("the crontab was rewritten in place; it must be replaced by rename so crond never reads a partial file")
	}
	got, err := os.ReadFile(crontab)
	if err != nil {
		t.Fatalf("read crontab: %v", err)
	}
	if !strings.Contains(string(got), "/etc/trafo/wifi-reconnect.sh") {
		t.Errorf("unrelated crontab entries were lost:\n%s", got)
	}
	if _, err := os.Stat(crontab + ".tmp"); !os.IsNotExist(err) {
		t.Error("the atomic write left its temp file behind")
	}
}
