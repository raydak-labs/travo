package services

import (
	"encoding/json"
	"os"
	"os/exec"
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

// writeReconnectScriptFixture stages the REAL generated auto-reconnect script in
// a temp bin dir with a stub `wifi` that records every invocation, and points the
// guard directory at guardDir. The stub is what makes the assertion possible:
// the script is otherwise a root cron job that talks to ubus.
//
// The paths inside the script are rewritten so the fixture never touches the test
// machine's /etc/trafo (paths.replace would corrupt them).
func writeReconnectScriptFixture(t *testing.T, guardDir string) (dir, script, marker string) {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no /bin/sh available")
	}

	binDir := t.TempDir()
	marker = filepath.Join(binDir, "wifi-invoked")
	scriptPath := filepath.Join(binDir, "wifi-reconnect.sh")

	// The stub records the call and succeeds, so the script takes its "reconnect
	// worked" branch.
	stub := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"" + marker + "\"\nexit 0\n"
	if err := os.WriteFile(filepath.Join(binDir, "wifi"), []byte(stub), 0o755); err != nil {
		t.Fatalf("write wifi stub: %v", err)
	}
	// No wwan address on the test machine: the stub reports an interface with no
	// ipv4-address, which is the "connection dropped" branch the guard exists for.
	for _, name := range []string{"ubus", "jsonfilter"} {
		body := "#!/bin/sh\nexit 0\n"
		if name == "ubus" {
			body = "#!/bin/sh\necho '{\"up\":false}'\n"
		}
		if err := os.WriteFile(filepath.Join(binDir, name), []byte(body), 0o755); err != nil {
			t.Fatalf("write %s stub: %v", name, err)
		}
	}

	script = strings.ReplaceAll(reconnectScriptContent, crashGuardDir, guardDir)
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write reconnect script: %v", err)
	}
	dir = binDir
	return dir, scriptPath, marker
}

// The crash guard and the MAX_FAIL counter are the only things standing between
// cron and an endless `wifi up` on a broken saved network. Both live in /etc/trafo,
// which nothing in packaging/, install.sh or backend startup guarantees exists,
// and cron can fire this script before the backend has ever run.
//
// Without `mkdir -p` the guard write fails silently (there is no `set -e`), the
// leftover-guard check cannot see a stale guard, and the script runs `wifi up`
// UNGUARDED with a counter that never increments — the one sanctioned `wifi up` in
// the tree, every minute, forever. The script must refuse to run instead.
func TestAutoReconnectScriptRefusesToRunWhenTheGuardDirIsMissing(t *testing.T) {
	// A regular file where the guard directory has to be: mkdir -p fails with
	// ENOTDIR, and so does the guard write — the exact state on a device that
	// never got /etc/trafo created.
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	guardDir := filepath.Join(blocker, "trafo")

	binDir, scriptPath, wifiMarker := writeReconnectScriptFixture(t, guardDir)

	cmd := exec.Command("sh", scriptPath)
	cmd.Env = append(os.Environ(), "PATH="+binDir+":"+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("the script exited 0 although it could not create its guard directory:\n%s", out)
	}
	if _, statErr := os.Stat(wifiMarker); statErr == nil {
		t.Error("`wifi up` ran without a crash guard and without a fail counter")
	}
}

// Control for the test above: with a writable guard directory the same script
// does reach `wifi up`, so the refusal above is caused by the missing directory
// and not by the fixture.
func TestAutoReconnectScriptRunsWifiWhenTheGuardDirIsWritable(t *testing.T) {
	guardDir := filepath.Join(t.TempDir(), "trafo")

	binDir, scriptPath, wifiMarker := writeReconnectScriptFixture(t, guardDir)

	cmd := exec.Command("sh", scriptPath)
	cmd.Env = append(os.Environ(), "PATH="+binDir+":"+os.Getenv("PATH"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("the script failed with a writable guard dir: %v\n%s", err, out)
	}
	calls, err := os.ReadFile(wifiMarker)
	if err != nil {
		t.Fatalf("`wifi` was never invoked, so the fixture proves nothing: %v", err)
	}
	if !strings.Contains(string(calls), "up") {
		t.Errorf("expected `wifi up`, stub recorded: %q", calls)
	}
	// A successful reconnect clears both the guard and the counter.
	if _, err := os.Stat(filepath.Join(guardDir, "autoreconnect-crash-guard")); !os.IsNotExist(err) {
		t.Error("a successful reconnect must clear the crash guard")
	}
}

// scheduleWindowIsOff backs the startup AP repair: it must be able to tell that
// the operator's schedule currently says "WiFi off", including across midnight.
func TestScheduleWindowIsOff(t *testing.T) {
	day := func(h, m int) time.Time {
		return time.Date(2026, 10, 4, h, m, 0, 0, time.Local)
	}
	for _, tc := range []struct {
		name    string
		now     time.Time
		on, off string
		wantOff bool
	}{
		{"same day window, inside", day(13, 0), "22:00", "12:00", true},
		{"same day window, before off", day(8, 0), "22:00", "12:00", false},
		{"same day window, after on", day(23, 0), "22:00", "12:00", false},
		{"wrapping window, late evening", day(23, 30), "07:00", "22:00", true},
		{"wrapping window, after midnight", day(3, 0), "07:00", "22:00", true},
		{"wrapping window, daytime", day(13, 0), "07:00", "22:00", false},
		{"degenerate equal times", day(13, 0), "07:00", "07:00", false},
		{"malformed times", day(13, 0), "7:00", "23:00", false},
	} {
		if got := scheduleWindowIsOff(tc.now, tc.on, tc.off); got != tc.wantOff {
			t.Errorf("%s: scheduleWindowIsOff(%s, on=%q, off=%q) = %v, want %v",
				tc.name, tc.now.Format("15:04"), tc.on, tc.off, got, tc.wantOff)
		}
	}
}

// A backend restart inside the schedule's off window must NOT re-enable the
// radios. The startup repair runs ~30s after boot and used to commit
// wireless.<radio>.disabled=0 whenever an AP iface was enabled — which is
// exactly the shape the generated toggle helper leaves behind when it turns WiFi
// off (radios disabled, AP ifaces still enabled). The result was a WiFi schedule
// that silently stopped applying.
func TestEnsureAPRunning_KeepsRadiosOffDuringScheduleOffWindow(t *testing.T) {
	dir := t.TempDir()
	// One hour either side of now, so "now" is a full hour away from both
	// boundaries and the assertion cannot depend on the minute it runs in.
	now := time.Now()
	writeSchedule := func(on, off string) *WifiService {
		path := filepath.Join(dir, "wifi-schedule.json")
		data, err := json.Marshal(models.WiFiSchedule{Enabled: true, OnTime: on, OffTime: off})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return &WifiService{scheduleFile: path}
	}
	hhmm := func(offset time.Duration) string {
		return now.Add(offset).Format("15:04")
	}

	t.Run("off window leaves the radios disabled", func(t *testing.T) {
		u := uci.NewMockUCI()
		// radio1 has no STA, so only the schedule can keep it off.
		if err := u.Set("wireless", "radio1", "disabled", "1"); err != nil {
			t.Fatalf("setup: %v", err)
		}
		svc := newAPHealthTestService(u)
		// Off an hour ago, on an hour from now: now is inside the off window.
		svc.scheduleFile = writeSchedule(hhmm(time.Hour), hhmm(-time.Hour)).scheduleFile

		if _, _, err := svc.EnsureAPRunning(); err != nil {
			t.Fatalf("EnsureAPRunning: %v", err)
		}
		sections, _ := u.GetSections("wireless")
		if sections["radio1"]["disabled"] != "1" {
			t.Error("startup AP repair re-enabled a radio the WiFi schedule had turned off")
		}
	})

	t.Run("on window still enables the radios", func(t *testing.T) {
		u := uci.NewMockUCI()
		if err := u.Set("wireless", "radio1", "disabled", "1"); err != nil {
			t.Fatalf("setup: %v", err)
		}
		svc := newAPHealthTestService(u)
		// On an hour ago, off an hour from now: the off window runs from now+1h
		// round to now-1h, so it does not contain now.
		svc.scheduleFile = writeSchedule(hhmm(-time.Hour), hhmm(time.Hour)).scheduleFile

		if _, _, err := svc.EnsureAPRunning(); err != nil {
			t.Fatalf("EnsureAPRunning: %v", err)
		}
		sections, _ := u.GetSections("wireless")
		if sections["radio1"]["disabled"] != "0" {
			t.Error("a radio with an enabled AP must still be re-enabled outside the off window")
		}
	})

	t.Run("disabled schedule does not block the repair", func(t *testing.T) {
		u := uci.NewMockUCI()
		if err := u.Set("wireless", "radio1", "disabled", "1"); err != nil {
			t.Fatalf("setup: %v", err)
		}
		svc := newAPHealthTestService(u)
		path := filepath.Join(dir, "wifi-schedule.json")
		if err := os.WriteFile(path, []byte(`{"enabled":false}`), 0o644); err != nil {
			t.Fatal(err)
		}
		svc.scheduleFile = path

		if _, _, err := svc.EnsureAPRunning(); err != nil {
			t.Fatalf("EnsureAPRunning: %v", err)
		}
		sections, _ := u.GetSections("wireless")
		if sections["radio1"]["disabled"] != "0" {
			t.Error("with no active schedule the repair must still enable the radios")
		}
	})
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
