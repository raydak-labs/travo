//go:build hardware

package txn

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device/cli"
	dfacts "github.com/blackdark-org/travo-greenfield/internal/device/facts"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/uciwrite"
)

// Spike S4 on the router. Run from Ethernet only. Each test changes live wireless state
// and relies on the watchdog to put it back; it records timings to stdout as JSON lines.

func hwManager() *Manager {
	return &Manager{
		Device: cli.New(), Locks: &uciwrite.Locks{},
		Guards:    guard.Dir{Path: "/tmp/greenfield-hw/guards", Now: time.Now},
		Dir:       "/etc/greenfield-hw-txn", // flash: must survive reboot
		ConfigDir: "/etc/config", Reload: []string{"reload_config"}, UCI: "uci",
		Spawner:  ShellSpawner{Shell: "/bin/sh", RunTimeout: 120 * time.Second},
		Clock:    realClock{},
		NewID:    func() string { b := make([]byte, 8); _, _ = rand.Read(b); return hex.EncodeToString(b) },
		Window:   MinWindow,
		ArmLimit: 90 * time.Second, ApplyBudget: 75 * time.Second,
		Settle: 90 * time.Second,
	}
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

func emit(k string, v any) { b, _ := json.Marshal(map[string]any{k: v}); fmt.Println("HW", string(b)) }

func link(ifname string) (dfacts.Link, error) {
	out, err := exec.Command("ubus", "call", "network.device", "status", fmt.Sprintf(`{"name":%q}`, ifname)).Output()
	if err != nil {
		return dfacts.Link{}, err
	}
	return dfacts.ParseDeviceStatus(out)
}

// waitUp returns how long until ifname has present+up+carrier.
func waitUp(ifname string, max time.Duration) (time.Duration, bool) {
	start := time.Now()
	for time.Since(start) < max {
		if l, err := link(ifname); err == nil && l.Present && l.Up && l.Carrier {
			return time.Since(start), true
		}
		time.Sleep(250 * time.Millisecond)
	}
	return time.Since(start), false
}

// waitSSID returns how long until iwinfo shows the given SSID on ifname. procd runs the
// network reload triggered by reload_config asynchronously (debounced), so the live state
// lags the commit.
func waitSSID(ifname, ssid string, max time.Duration) (time.Duration, bool) {
	start := time.Now()
	for time.Since(start) < max {
		out, _ := exec.Command("iwinfo", ifname, "info").Output()
		if strings.Contains(string(out), `ESSID: "`+ssid+`"`) {
			return time.Since(start), true
		}
		time.Sleep(250 * time.Millisecond)
	}
	return time.Since(start), false
}

func defaultRoute() bool {
	out, _ := exec.Command("ip", "-4", "route", "show", "default").Output()
	return dfacts.ParseKernelDefaultRoute(string(out))
}

func uciGet(k string) string {
	out, _ := exec.Command("uci", "-q", "get", k).Output()
	return strings.TrimSpace(string(out))
}

// AP SSID change, not confirmed: watchdog must restore and the AP must come back via
// reload_config alone. Measures apply time, AP bring-up after apply and after restore.
func TestHWAPChangeRestoresViaReloadConfig(t *testing.T) {
	m := hwManager()
	ctx := context.Background()
	orig := uciGet("wireless.default_radio1.ssid")
	t0 := time.Now()
	st, err := m.Begin(ctx, "hw: ap ssid", []string{"wireless"}, func(ctx context.Context) error {
		return m.Device.Set(ctx, "wireless", "default_radio1", "ssid", orig+"-gf")
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	emit("apply_s", time.Since(t0).Seconds())
	d, ok := waitSSID("phy1-ap0", orig+"-gf", 60*time.Second)
	emit("new_ssid_live_after_apply_s", d.Seconds())
	emit("new_ssid_live_ok", ok)
	d, ok = waitUp("phy1-ap0", 60*time.Second)
	emit("ap_carrier_after_new_ssid_s", d.Seconds())
	emit("ap_carrier_ok", ok)
	sta, _ := link("phy0-sta0")
	emit("sta_carrier_during", sta.Carrier)

	// wait for the window to lapse and the watchdog to restore
	deadline := time.Now().Add(MinWindow + 90*time.Second)
	for {
		s, _ := m.Get(st.ID)
		if s.State == RolledBack || s.State == RollbackFailed {
			emit("restore_state", s.State)
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no restore")
		}
		time.Sleep(500 * time.Millisecond)
	}
	emit("restored_after_begin_s", time.Since(t0).Seconds())
	if uciGet("wireless.default_radio1.ssid") != orig {
		t.Fatal("uci not restored")
	}
	d, ok = waitSSID("phy1-ap0", orig, 60*time.Second)
	emit("orig_ssid_live_after_restore_s", d.Seconds())
	emit("orig_ssid_live_ok", ok)
	d, ok = waitUp("phy1-ap0", 60*time.Second)
	emit("ap_carrier_after_restore_s", d.Seconds())
	emit("ap_carrier_after_restore_ok", ok)
	if !ok {
		t.Fatal("AP did not come back")
	}
	d, ok = waitUp("phy0-sta0", 60*time.Second)
	emit("sta_up_after_restore_s", d.Seconds())
	emit("default_route_after_restore", defaultRoute())
}

// Confirm keeps the change; then a second transaction puts the original back (confirmed).
func TestHWConfirmKeepsAP(t *testing.T) {
	m := hwManager()
	ctx := context.Background()
	orig := uciGet("wireless.default_radio1.ssid")
	st, err := m.Begin(ctx, "hw: confirm", []string{"wireless"}, func(ctx context.Context) error {
		return m.Device.Set(ctx, "wireless", "default_radio1", "ssid", orig+"-gf")
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Confirm(st.ID); err != nil {
		t.Fatal(err)
	}
	time.Sleep(MinWindow + 5*time.Second)
	got := uciGet("wireless.default_radio1.ssid")
	emit("confirmed_kept", got == orig+"-gf")
	st2, err := m.Begin(ctx, "hw: undo", []string{"wireless"}, func(ctx context.Context) error {
		return m.Device.Set(ctx, "wireless", "default_radio1", "ssid", orig)
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Confirm(st2.ID); err != nil {
		t.Fatal(err)
	}
	d, ok := waitUp("phy1-ap0", 60*time.Second)
	emit("ap_up_after_undo_s", d.Seconds())
	emit("ap_up_after_undo_ok", ok)
	if got != orig+"-gf" {
		t.Fatal("confirm did not keep the change")
	}
}

// Child: begin an uplink change (wrong STA SSID) and exit without confirming, as a crash.
func TestHWHelperBeginSTAAndDie(t *testing.T) {
	if os.Getenv("HW_HELPER") != "1" {
		t.Skip("helper")
	}
	m := hwManager()
	_, err := m.Begin(context.Background(), "hw: bad sta", []string{"wireless"}, func(ctx context.Context) error {
		return m.Device.Set(ctx, "wireless", "wifinet3", "ssid", "TravoGreenfieldNoSuchNet")
	}, nil)
	if err != nil {
		os.Exit(3)
	}
	os.Exit(0)
}

// Wrong uplink SSID + daemon death: STA carrier must read false (device-facts §2) and the
// watchdog must restore the uplink and the kernel default route.
func TestHWBadUplinkDaemonDiesWatchdogRestores(t *testing.T) {
	origSSID := uciGet("wireless.wifinet3.ssid")
	t0 := time.Now()
	cmd := exec.Command(os.Args[0], "-test.run", "^TestHWHelperBeginSTAAndDie$")
	cmd.Env = append(os.Environ(), "HW_HELPER=1")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(12 * time.Second) // past measured association + DHCP
	sta, err := link("phy0-sta0")
	emit("bad_sta_link", sta)
	emit("bad_sta_link_err", fmt.Sprint(err))
	emit("bad_sta_default_route", defaultRoute())
	deadline := time.Now().Add(MinWindow + 120*time.Second)
	for uciGet("wireless.wifinet3.ssid") != origSSID {
		if time.Now().After(deadline) {
			t.Fatal("watchdog did not restore uplink")
		}
		time.Sleep(500 * time.Millisecond)
	}
	emit("uci_restored_after_s", time.Since(t0).Seconds())
	d, ok := waitUp("phy0-sta0", 90*time.Second)
	emit("sta_carrier_after_restore_s", d.Seconds())
	emit("sta_carrier_after_restore_ok", ok)
	start := time.Now()
	for !defaultRoute() && time.Since(start) < 60*time.Second {
		time.Sleep(250 * time.Millisecond)
	}
	emit("default_route_after_restore_s", time.Since(start).Seconds())
	emit("default_route_after_restore", defaultRoute())
	m := hwManager()
	out, err := m.Recover(context.Background(), ProcAlive)
	emit("recover", out)
	if err != nil || !ok || !defaultRoute() {
		t.Fatal("uplink not back", err)
	}
}

// Reboot part 1: begin an AP change and leave it pending (the caller then reboots).
func TestHWRebootPrepare(t *testing.T) {
	m := hwManager()
	orig := uciGet("wireless.default_radio1.ssid")
	_ = os.WriteFile("/etc/greenfield-hw-orig-ssid", []byte(orig), 0o600)
	st, err := m.Begin(context.Background(), "hw: reboot", []string{"wireless"}, func(ctx context.Context) error {
		return m.Device.Set(ctx, "wireless", "default_radio1", "ssid", orig+"-reboot")
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	emit("pending_id", st.ID)
}

// Boot hook: what the daemon's startup does. Runs from a temporary init script.
func TestHWRecoverAtBoot(t *testing.T) {
	out, err := hwManager().Recover(context.Background(), ProcAlive)
	b, _ := json.Marshal(map[string]any{"recover": out, "err": fmt.Sprint(err), "uptime": uptime()})
	_ = os.WriteFile("/etc/greenfield-hw-boot-result.json", b, 0o600)
}

func uptime() string { b, _ := os.ReadFile("/proc/uptime"); return strings.Fields(string(b))[0] }

// Reboot part 2: after boot, verify restore and AP state.
func TestHWRebootVerify(t *testing.T) {
	orig, _ := os.ReadFile("/etc/greenfield-hw-orig-ssid")
	res, _ := os.ReadFile("/etc/greenfield-hw-boot-result.json")
	emit("boot_result", string(res))
	emit("uci_ssid_restored", uciGet("wireless.default_radio1.ssid") == string(orig))
	d, ok := waitSSID("phy1-ap0", string(orig), 90*time.Second)
	emit("orig_ssid_live_s", d.Seconds())
	emit("orig_ssid_live_ok", ok)
	if uciGet("wireless.default_radio1.ssid") != string(orig) || !ok {
		t.Fatal("not restored after reboot")
	}
}
