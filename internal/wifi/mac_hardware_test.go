//go:build hardware

package wifi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device/cli"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/uciwrite"
)

type macHWClock struct{}

func (macHWClock) Now() time.Time { return time.Now() }

// Run ON the router from Ethernet with a working STA uplink. Ends with a rollback.
func TestHWUplinkMACRandomThenRollsBack(t *testing.T) {
	d := cli.New()
	m := &txn.Manager{
		Device: d, Locks: &uciwrite.Locks{},
		Guards:    guard.Dir{Path: "/tmp/greenfield-hw/guards", Now: time.Now},
		Dir:       "/etc/greenfield-hw-txn",
		ConfigDir: "/etc/config", Reload: []string{"reload_config"}, UCI: "uci",
		Spawner: txn.ShellSpawner{Shell: "/bin/sh", RunTimeout: 120 * time.Second},
		Clock:   macHWClock{},
		NewID:   func() string { b := make([]byte, 8); _, _ = rand.Read(b); return hex.EncodeToString(b) },
		Window:  txn.MinWindow, ArmLimit: 90 * time.Second, ApplyBudget: 75 * time.Second, Settle: 90 * time.Second,
	}
	s := New(context.Background(), d, d).WithJoin(JoinDeps{UCI: d, Ubus: d, Routes: d, Links: d, Keys: d, Neighbours: d, Txns: m})
	ctx := context.Background()
	before, err := s.UplinkMAC(ctx)
	if err != nil || before.Mode != MACDefault || before.Effective == nil {
		t.Skipf("needs a default-MAC uplink: %+v %v", before, err)
	}
	t.Logf("factory MAC is locally administered: %v", locallyAdministered(*before.Effective))
	start := time.Now()
	st, err := s.SetUplinkMAC(ctx, MACRequest{Mode: MACRandom})
	t.Logf("random: %s %v after %s", st.State, err, time.Since(start))
	if err != nil || st.State != txn.Pending {
		t.Fatal(err)
	}
	during, _ := s.UplinkMAC(ctx)
	if during.Effective == nil || !locallyAdministered(*during.Effective) {
		t.Fatalf("random MAC not live: %+v", during)
	}
	got, err := m.Rollback(st.ID)
	if err != nil || got.State != txn.RolledBack {
		t.Fatalf("rollback %+v %v", got, err)
	}
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		if after, err := s.UplinkMAC(ctx); err == nil && after.Effective != nil && *after.Effective == *before.Effective {
			t.Logf("factory MAC back after %s", time.Since(start))
			return
		}
		time.Sleep(time.Second)
	}
	t.Fatal("factory MAC did not return after rollback")
}
