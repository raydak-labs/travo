//go:build integration

package txn

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device/cli"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/uciwrite"
)

func realManager(window time.Duration) *Manager {
	return &Manager{
		Device: cli.New(), Locks: &uciwrite.Locks{},
		Guards:    guard.Dir{Path: "/tmp/itest/guards", Now: time.Now},
		Dir:       "/tmp/itest/txn",
		ConfigDir: "/etc/config", Reload: []string{"reload_config"}, UCI: "uci",
		Spawner:     ShellSpawner{Shell: "/bin/sh", RunTimeout: 60 * time.Second},
		Clock:       realClock{},
		ArmLimit:    75 * time.Second,
		ApplyBudget: 65 * time.Second,
		Settle:      15 * time.Second,
		NewID: func() string {
			b := make([]byte, 8)
			_, _ = rand.Read(b)
			return hex.EncodeToString(b)
		},
		Window: window,
	}
}

// shrink lets integration tests use a short window (Manager enforces MinWindow).
type shrink struct {
	ShellSpawner
	secs string
}

func (s shrink) Start(script string, args ...string) (int, error) {
	args = append([]string(nil), args...)
	args[2] = s.secs
	return s.ShellSpawner.Start(script, args...)
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

func uciGet(t *testing.T, key string) string {
	out, _ := exec.Command("uci", "-q", "get", key).Output()
	return strings.TrimSpace(string(out))
}

// Child mode: begin a transaction and die without confirming (SIGKILL-equivalent: os.Exit
// skips every defer). The parent test checks that the detached watchdog restores.
func TestHelperBeginAndDie(t *testing.T) {
	if os.Getenv("TXN_HELPER") != "1" {
		t.Skip("helper")
	}
	m := realManager(MinWindow)
	secs := "2"
	if os.Getenv("TXN_STAY") == "1" {
		secs = "8" // long enough for the parent to stop the service while pending
	}
	m.Spawner = shrink{ShellSpawner: m.Spawner.(ShellSpawner), secs: secs}
	_, err := m.Begin(context.Background(), "itest", []string{"dhcp"}, func(ctx context.Context) error {
		return m.Device.Set(ctx, "dhcp", "lan", "leasetime", "1h")
	}, nil)
	if err != nil {
		os.Exit(3)
	}
	if os.Getenv("TXN_STAY") == "1" {
		select {} // stay up like a daemon until procd stops us
	}
	os.Exit(0)
}

func TestRealWatchdogRestoresAfterDaemonDeath(t *testing.T) {
	_ = os.RemoveAll("/tmp/itest")
	before := uciGet(t, "dhcp.lan.leasetime")
	cmd := exec.Command(os.Args[0], "-test.run", "TestHelperBeginAndDie")
	cmd.Env = append(os.Environ(), "TXN_HELPER=1")
	if err := cmd.Run(); err != nil {
		t.Fatalf("helper: %v", err)
	}
	if got := uciGet(t, "dhcp.lan.leasetime"); got != "1h" {
		t.Fatalf("change not committed: %q", got)
	}
	deadline := time.Now().Add(15 * time.Second)
	for uciGet(t, "dhcp.lan.leasetime") != before {
		if time.Now().After(deadline) {
			t.Fatal("watchdog did not restore after daemon death")
		}
		time.Sleep(200 * time.Millisecond)
	}
	// The config is back before the script records its result (reload runs in between);
	// wait for the result, as the UI does, before asking recovery.
	m := realManager(MinWindow)
	waitResult := time.Now().Add(15 * time.Second)
	for {
		entries, _ := os.ReadDir(m.Dir)
		if len(entries) == 1 {
			if _, err := os.Stat(filepath.Join(m.Dir, entries[0].Name(), "result")); err == nil {
				break
			}
		}
		if time.Now().After(waitResult) {
			t.Fatal("watchdog never wrote its result")
		}
		time.Sleep(100 * time.Millisecond)
	}
	out, err := m.Recover(context.Background(), ProcAlive)
	if err != nil || len(out) != 1 || out[0].State != RolledBack {
		t.Fatalf("recover after watchdog restore: %+v %v", out, err)
	}
	if st, _ := m.Guards.Stale(); len(st) != 0 {
		t.Fatalf("guard left: %v", st)
	}
	if p, _ := m.Device.Pending(context.Background(), "dhcp"); p {
		t.Fatal("staged changes left")
	}
}

func TestRealConfirmKeepsChange(t *testing.T) {
	_ = os.RemoveAll("/tmp/itest")
	m := realManager(MinWindow)
	m.Spawner = shrink{ShellSpawner: m.Spawner.(ShellSpawner), secs: "2"}
	ctx := context.Background()
	st, err := m.Begin(ctx, "itest", []string{"dhcp"}, func(ctx context.Context) error {
		return m.Device.Set(ctx, "dhcp", "lan", "limit", "77")
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Confirm(st.ID); err != nil {
		t.Fatal(err)
	}
	time.Sleep(4 * time.Second)
	if uciGet(t, "dhcp.lan.limit") != "77" {
		t.Fatal("confirmed change restored")
	}
	if _, err := os.Stat(filepath.Join(m.Dir, st.ID)); !os.IsNotExist(err) {
		t.Fatal("txn dir left")
	}
}

// procd stop of the daemon's service must not kill the detached watchdog (own session).
func TestRealWatchdogSurvivesProcdStop(t *testing.T) {
	if _, err := os.Stat("/sbin/procd"); err != nil {
		t.Skip("no procd")
	}
	_ = os.RemoveAll("/tmp/itest")
	script := `#!/bin/sh /etc/rc.common
USE_PROCD=1
start_service() {
	procd_open_instance
	procd_set_param command env TXN_HELPER=1 TXN_STAY=1 ` + os.Args[0] + ` -test.run TestHelperBeginAndDie
	procd_close_instance
}
`
	if err := os.WriteFile("/etc/init.d/txnitest", []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	defer os.Remove("/etc/init.d/txnitest")
	before := uciGet(t, "dhcp.lan.leasetime")
	if out, err := exec.Command("/etc/init.d/txnitest", "start").CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	deadline := time.Now().Add(10 * time.Second)
	for uciGet(t, "dhcp.lan.leasetime") != "1h" {
		if time.Now().After(deadline) {
			t.Fatal("helper never committed")
		}
		time.Sleep(100 * time.Millisecond)
	}
	if out, err := exec.Command("/etc/init.d/txnitest", "stop").CombinedOutput(); err != nil {
		t.Fatalf("stop: %v %s", err, out)
	}
	if uciGet(t, "dhcp.lan.leasetime") != "1h" {
		t.Fatal("restored before stop: test window too short")
	}
	deadline = time.Now().Add(20 * time.Second)
	for uciGet(t, "dhcp.lan.leasetime") != before {
		if time.Now().After(deadline) {
			t.Fatal("watchdog died with the procd service")
		}
		time.Sleep(200 * time.Millisecond)
	}
}
