//go:build hardware

package wifi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/netip"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device/cli"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/lockout"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/uciwrite"
)

// Run ON the router from Ethernet (docs/testing.md "Hardware layer"). Every test ends
// with an explicit rollback, so the router returns to its prior wireless config.
// HW_CALLER is the Ethernet test host's IP (an Ethernet caller is never blocked).

type hwClock struct{}

func (hwClock) Now() time.Time { return time.Now() }

func hwService(t *testing.T) (*Service, *txn.Manager) {
	t.Helper()
	d := cli.New()
	m := &txn.Manager{
		Device: d, Locks: &uciwrite.Locks{},
		Guards:    guard.Dir{Path: "/tmp/greenfield-hw/guards", Now: time.Now},
		Dir:       "/etc/greenfield-hw-txn",
		ConfigDir: "/etc/config", Reload: []string{"reload_config"}, UCI: "uci",
		Spawner: txn.ShellSpawner{Shell: "/bin/sh", RunTimeout: 120 * time.Second},
		Clock:   hwClock{},
		NewID:   func() string { b := make([]byte, 8); _, _ = rand.Read(b); return hex.EncodeToString(b) },
		Window:  txn.MinWindow, ArmLimit: 90 * time.Second, ApplyBudget: 75 * time.Second, Settle: 90 * time.Second,
	}
	s := New(context.Background(), d, d).WithJoin(JoinDeps{UCI: d, Ubus: d, Routes: d, Links: d, Keys: d, Neighbours: d, Txns: m})
	return s, m
}

func hwCaller(t *testing.T) netip.Addr {
	ip, err := netip.ParseAddr(os.Getenv("HW_CALLER"))
	if err != nil {
		t.Skip("HW_CALLER not set")
	}
	return ip
}

func uciGet(key string) string {
	out, _ := exec.Command("uci", "-q", "get", key).Output()
	return strings.TrimSpace(string(out))
}

func rollback(t *testing.T, m *txn.Manager, st txn.Status) {
	t.Helper()
	got, err := m.Rollback(st.ID)
	t.Logf("rollback: %s %v", got.State, err)
	if err != nil || got.State != txn.RolledBack {
		t.Fatalf("rollback %+v %v", got, err)
	}
}

func TestHWAPRenameProvesNewSSIDThenRollsBack(t *testing.T) {
	s, m := hwService(t)
	ctx := context.Background()
	aps, err := s.AccessPoints(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var ap APConfig
	for _, a := range aps {
		if a.Enabled && !a.Guest {
			ap = a
		}
	}
	if ap.Section == "" {
		t.Skip("no enabled AP")
	}
	start := time.Now()
	st, err := s.UpdateAP(ctx, APRequest{Section: ap.Section, SSID: ap.SSID + "-hw", Enabled: true, Caller: hwCaller(t)})
	t.Logf("rename: %s %v after %s", st.State, err, time.Since(start))
	if err != nil || st.State != txn.Pending {
		t.Fatal(err)
	}
	rollback(t, m, st)
	if got := uciGet("wireless." + ap.Section + ".ssid"); got != ap.SSID {
		t.Fatalf("ssid after rollback %q", got)
	}
}

func TestHWAPDisableRefusesUnknownCaller(t *testing.T) {
	s, _ := hwService(t)
	ctx := context.Background()
	aps, err := s.AccessPoints(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range aps {
		if !a.Enabled || a.Guest {
			continue
		}
		// An address with no neighbour entry classifies Unknown and fails closed.
		_, err := s.UpdateAP(ctx, APRequest{Section: a.Section, SSID: a.SSID, Enabled: false, Caller: netip.MustParseAddr("192.168.1.253")})
		if !errors.Is(err, lockout.ErrLockoutRisk) {
			t.Fatalf("got %v", err)
		}
		if out, _ := exec.Command("uci", "changes", "wireless").Output(); len(out) != 0 {
			t.Fatalf("refusal staged %s", out)
		}
		return
	}
	t.Skip("no enabled AP")
}

func TestHWGuestSetupThenRollsBack(t *testing.T) {
	s, m := hwService(t)
	ctx := context.Background()
	if uciGet("wireless.guest") != "" {
		t.Skip("guest already configured")
	}
	start := time.Now()
	st, err := s.SetGuest(ctx, GuestRequest{SSID: "TGGuest-hw", Key: "hw-test-only-key", Enabled: true, Caller: hwCaller(t)})
	t.Logf("guest: %s %v after %s", st.State, err, time.Since(start))
	if err != nil || st.State != txn.Pending {
		t.Fatal(err)
	}
	for _, cmd := range [][]string{
		{"ubus", "call", "network.interface.guest", "status"},
		{"ubus", "call", "network.wireless", "status"},
	} {
		out, err := exec.Command(cmd[0], cmd[1:]...).Output()
		t.Logf("%s: %v\n%s", strings.Join(cmd, " "), err, out)
	}
	rollback(t, m, st)
	for _, key := range []string{"wireless.guest", "network.guest", "dhcp.guest", "firewall.guest"} {
		if uciGet(key) != "" {
			t.Fatalf("%s survived rollback", key)
		}
	}
}
