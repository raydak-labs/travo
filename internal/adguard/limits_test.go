package adguard

import (
	"bytes"
	"context"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/cli"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
)

const uciFixture = "../../testdata/device/axt1800-25.12.6/adguard/uci-adguardhome.txt"

func TestLimitsCapsTheDeviceConfig(t *testing.T) {
	out, changed := Limits(readFixture(t))
	if !changed {
		t.Fatal("the captured config (90d, 1000 entries) must be capped")
	}
	for _, want := range []string{
		"querylog:\n  dir_path: \"\"\n  ignored: []\n  interval: 6h\n  size_memory: 100\n",
		"statistics:\n  dir_path: \"\"\n  ignored: []\n  interval: 1d\n",
		"  cache_size: 4194304\n",
	} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("missing %q", want)
		}
	}
	if again, changed := Limits(out); changed || !bytes.Equal(again, out) {
		t.Fatal("second run must be a no-op")
	}
}

func TestLimitsKeepSmallerValuesAndCapLargerOnes(t *testing.T) {
	in := strings.NewReplacer(
		"  interval: 90d\n  size_memory: 1000\n", "  interval: 1h\n  size_memory: 20\n",
		"  cache_size: 4194304\n", "  cache_size: 16777216\n",
		"statistics:\n  dir_path: \"\"\n  ignored: []\n  interval: 1d\n", "statistics:\n  dir_path: \"\"\n  ignored: []\n  interval: 2160h0m0s\n",
	).Replace(string(readFixture(t)))
	out, changed := Limits([]byte(in))
	if !changed {
		t.Fatal("want caps")
	}
	s := string(out)
	for _, want := range []string{"  interval: 1h\n  size_memory: 20\n", "  cache_size: 4194304\n", "ignored: []\n  interval: 1d\n  enabled"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in\n%s", want, s)
		}
	}
}

func TestLimitsLeavesUnknownShapesAlone(t *testing.T) {
	for _, in := range []string{"", "dns:\n  port: 5353\n", "querylog:\n\tinterval: 90d\n", "querylog:\n  interval: \"90d\"\n"} {
		if out, changed := Limits([]byte(in)); changed || string(out) != in {
			t.Fatalf("%q changed to %q", in, out)
		}
	}
}

func TestMemLimitTarget(t *testing.T) {
	secs := uciSections(t)
	if v, ok := MemLimitTarget(secs); !ok || v != "67108864" {
		t.Fatalf("captured memlimit 0 → %q %v", v, ok)
	}
	for in, want := range map[string]bool{"": true, "x": true, "33554432": false, "67108864": false, "1073741824": true} {
		s := slices.Clone(secs)
		s[0].Options = map[string]string{MemLimitOption: in}
		if _, ok := MemLimitTarget(s); ok != want {
			t.Fatalf("memlimit %q: ok=%v want %v", in, ok, want)
		}
	}
	if _, ok := MemLimitTarget(nil); ok {
		t.Fatal("no section, nothing to write")
	}
}

func uciSections(t *testing.T) []device.Section {
	t.Helper()
	b, err := os.ReadFile(uciFixture)
	if err != nil {
		t.Fatal(err)
	}
	secs, err := cli.ParseShow(UCIConfig, string(b))
	if err != nil {
		t.Fatal(err)
	}
	return secs
}

// capsRig is an installed, running AGH with a loopback config that still lacks the caps and
// the captured UCI options (memlimit 0).
func capsRig(t *testing.T, running bool) *rig {
	t.Helper()
	r := newRig(t)
	loop, _, err := Loopback(readFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	r.installed(t, loop)
	r.dev.SetService(Package, true, running)
	if err := r.dev.Load(context.Background(), UCIConfig, uciSections(t)); err != nil {
		t.Fatal(err)
	}
	r.svc.UCI, r.svc.Txns = r.dev, applyTxns{dev: r.dev}
	return r
}

// applyTxns is txn.Manager's direct path without locks: stage, then commit.
type applyTxns struct {
	TopologyTxns
	dev *mock.Device
}

func (a applyTxns) Apply(ctx context.Context, configs []string, stage func(context.Context) error) error {
	if err := stage(ctx); err != nil {
		return err
	}
	for _, c := range configs {
		if err := a.dev.Commit(ctx, c); err != nil {
			return err
		}
	}
	return nil
}

func TestReconcileCapsAGHWithOneRestart(t *testing.T) {
	r := capsRig(t, true)
	ctx := context.Background()
	if err := r.svc.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(r.svc.Config); !bytes.Equal(got, Seed) {
		t.Fatal("caps not written")
	}
	if secs, _ := r.dev.Show(ctx, UCIConfig); secs[0].Options[MemLimitOption] != "67108864" {
		t.Fatalf("memlimit = %+v", secs)
	}
	if !slices.Equal(r.dev.ServiceActions, []string{"adguardhome restart"}) {
		t.Fatalf("actions = %v", r.dev.ServiceActions)
	}
	if err := r.svc.Reconcile(ctx); err != nil || len(r.dev.ServiceActions) != 1 {
		t.Fatalf("an applied state must not restart again: %v %v", err, r.dev.ServiceActions)
	}
}

func TestReconcileMemLimitAloneRestartsOnce(t *testing.T) {
	r := capsRig(t, true)
	if err := os.WriteFile(r.svc.Config, Seed, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := r.svc.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(r.dev.ServiceActions, []string{"adguardhome restart"}) {
		t.Fatalf("actions = %v", r.dev.ServiceActions)
	}
}

func TestReconcileCapsAStoppedAGHWithoutStartingIt(t *testing.T) {
	r := capsRig(t, false)
	if err := r.svc.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(r.svc.Config); !bytes.Equal(got, Seed) {
		t.Fatal("caps not written")
	}
	if len(r.dev.ServiceActions) != 0 {
		t.Fatalf("a stopped AGH must stay stopped: %v", r.dev.ServiceActions)
	}
}

func TestReconcileCapsOnceKeepsUserRaiseAndReappliesMemLimit(t *testing.T) {
	r := capsRig(t, true)
	ctx := context.Background()
	if err := r.svc.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(r.svc.CapsMarker); err != nil || string(b) != CapsVersion {
		t.Fatalf("caps marker = %q %v", b, err)
	}
	raised := []byte(strings.Replace(string(Seed), "  interval: 6h\n  size_memory: 100\n", "  interval: 90d\n  size_memory: 1000\n", 1))
	if err := os.WriteFile(r.svc.Config, raised, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := r.svc.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(r.svc.Config); !bytes.Equal(got, raised) {
		t.Fatal("a value the user raised after the caps were applied must stay")
	}
	if st, _ := r.svc.Status(ctx); st.MigrationFailed || len(r.dev.ServiceActions) != 1 {
		t.Fatalf("no second restart: %+v %v", st, r.dev.ServiceActions)
	}
	// memlimit is router safety: reconciled on every tick.
	if err := r.dev.Set(ctx, UCIConfig, UCISection, MemLimitOption, "0"); err != nil {
		t.Fatal(err)
	}
	if err := r.dev.Commit(ctx, UCIConfig); err != nil {
		t.Fatal(err)
	}
	if err := r.svc.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	if secs, _ := r.dev.Show(ctx, UCIConfig); secs[0].Options[MemLimitOption] != "67108864" {
		t.Fatalf("memlimit = %+v", secs)
	}
	if got, _ := os.ReadFile(r.svc.Config); !bytes.Equal(got, raised) || len(r.dev.ServiceActions) != 2 {
		t.Fatalf("memlimit alone: yaml kept, one restart: %v", r.dev.ServiceActions)
	}
}
