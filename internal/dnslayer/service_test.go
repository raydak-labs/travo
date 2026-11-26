package dnslayer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device/cli"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
)

type commitApplier struct {
	d    *mock.Device
	fail bool
	n    int
}

// commitApplier mirrors txn.Manager.Apply semantics: stage, then commit; revert on failure.
func (a *commitApplier) Apply(ctx context.Context, configs []string, stage func(context.Context) error) error {
	a.n++
	if err := stage(ctx); err != nil {
		return errors.Join(err, a.d.Revert(ctx, "dhcp"))
	}
	if a.fail {
		return errors.Join(errors.New("apply failed"), a.d.Revert(ctx, "dhcp"))
	}
	for _, c := range configs {
		if err := a.d.Commit(ctx, c); err != nil {
			return err
		}
	}
	return nil
}

var ctx = context.Background()

// setup loads a captured `uci export dhcp` fixture (see internal/device/cli/testdata/PROVENANCE.md).
func setup(t *testing.T, fixture string) (*Service, *mock.Device, *commitApplier) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "device", "cli", "testdata", fixture))
	if err != nil {
		t.Fatal(err)
	}
	secs, err := cli.ParseExport("dhcp", string(b))
	if err != nil {
		t.Fatal(err)
	}
	d := mock.New()
	if err := d.Load(ctx, "dhcp", secs); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	a := &commitApplier{d: d}
	return &Service{
		Device:     d,
		Guards:     guard.Dir{Path: filepath.Join(dir, "guards"), Now: time.Now},
		RecordPath: filepath.Join(dir, "dns-layers.json"),
		Apply:      a,
	}, d, a
}

const listFixture, scalarFixture = "uci-export-dhcp.txt", "uci-export-dhcp-scalar-server.txt"

// deviceFixture is the real router's dnsmasq: a ONE-element `list server` (old Travo's
// AdGuard forwarding). Restoring it as an option silently broke all DNS on the device.
const deviceFixture = "../../../../testdata/device/axt1800-25.12.6/uci-export-dhcp.txt"

func live(t *testing.T, d *mock.Device) State {
	t.Helper()
	_, st, err := ReadState(ctx, d)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func guarded(t *testing.T, s *Service) bool {
	t.Helper()
	st, err := s.Status()
	if err != nil {
		t.Fatal(err)
	}
	return st.Pending
}

func TestPushThenPopRestoresBaseExactly(t *testing.T) {
	for _, fx := range []string{listFixture, scalarFixture, deviceFixture} {
		t.Run(fx, func(t *testing.T) {
			s, d, _ := setup(t, fx)
			before := live(t, d)
			for _, n := range []Name{AdGuard, Captive, Tailscale} {
				if err := s.Push(ctx, layers[n]); err != nil {
					t.Fatal(err)
				}
			}
			st := live(t, d)
			if !reflect.DeepEqual(st.Server.Values, []string{"10.0.0.1", "/ts.net/100.100.100.100"}) || st.RebindProtection.Value != "0" {
				t.Fatalf("captive+tailscale not effective: %+v", st)
			}
			for _, n := range []Name{Captive, Tailscale, AdGuard} {
				if err := s.Pop(ctx, n); err != nil {
					t.Fatal(err)
				}
			}
			if after := live(t, d); !reflect.DeepEqual(before, after) {
				t.Fatalf("base not restored (shape included):\n%+v\n%+v", before, after)
			}
			if _, err := os.Stat(s.RecordPath); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("record not deleted after last pop")
			}
			if guarded(t, s) {
				t.Fatal("guard left")
			}
		})
	}
}

func TestFailedApplyRevertsStagingKeepsGuardAndResumeFinishes(t *testing.T) {
	s, d, a := setup(t, listFixture)
	a.fail = true
	if err := s.Push(ctx, layers[VPN]); err == nil {
		t.Fatal("want error")
	}
	if p, _ := d.Pending(ctx, "dhcp"); p {
		t.Fatal("staged changes left behind: a foreign commit would make them live unguarded")
	}
	if !guarded(t, s) {
		t.Fatal("guard removed after failed apply")
	}
	if st, _ := s.Status(); !reflect.DeepEqual(st.Layers, []Name{VPN}) {
		t.Fatalf("intent record lost: %+v", st)
	}
	if err := s.Push(ctx, layers[AdGuard]); !errors.Is(err, guard.ErrActive) {
		t.Fatalf("feature must skip while guarded: %v", err)
	}
	a.fail = false
	if err := s.Resume(ctx); err != nil {
		t.Fatal(err)
	}
	if guarded(t, s) || !reflect.DeepEqual(live(t, d).Server.Values, []string{"10.2.0.1"}) {
		t.Fatalf("resume did not finish: %+v", live(t, d))
	}
}

func TestInterruptedLastPopResumesToBase(t *testing.T) {
	s, d, a := setup(t, listFixture)
	before := live(t, d)
	_ = s.Push(ctx, layers[Captive])
	a.fail = true
	if err := s.Pop(ctx, Captive); err == nil {
		t.Fatal("restore reported success while bypass may be in force")
	}
	if _, err := os.Stat(s.RecordPath); err != nil {
		t.Fatal("record deleted before base was restored")
	}
	if st, _ := s.Status(); len(st.Layers) != 0 || !st.Pending {
		t.Fatalf("status must show restore pending: %+v", st)
	}
	a.fail = false
	if err := s.Resume(ctx); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, live(t, d)) {
		t.Fatal("resume re-applied layers instead of restoring base")
	}
	if _, err := os.Stat(s.RecordPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("record not deleted")
	}
}

func TestPushDuringRestoringKeepsSavedBase(t *testing.T) {
	s, d, a := setup(t, listFixture)
	before := live(t, d)
	_ = s.Push(ctx, layers[Captive])
	a.fail = true
	_ = s.Pop(ctx, Captive)
	_ = s.Guards.Clear(guardName) // operator retry path
	a.fail = false
	if err := s.Push(ctx, layers[VPN]); err != nil {
		t.Fatal(err)
	}
	_ = s.Pop(ctx, VPN)
	if !reflect.DeepEqual(before, live(t, d)) {
		t.Fatal("base recaptured from half-restored live state")
	}
}

func TestResumeWithoutGuardIsNoOp(t *testing.T) {
	s, _, a := setup(t, listFixture)
	if err := s.Resume(ctx); err != nil || a.n != 0 {
		t.Fatal(err, a.n)
	}
}

func TestForeignStagedChangesRefusedAndUntouched(t *testing.T) {
	s, d, a := setup(t, listFixture)
	_ = d.Set(ctx, "dhcp", "lan", "leasetime", "1h") // someone else's staged edit
	if err := s.Push(ctx, layers[VPN]); !errors.Is(err, ErrForeignStaged) {
		t.Fatal(err)
	}
	if a.n != 0 || guarded(t, s) {
		t.Fatal("applied or kept guard")
	}
	secs, _ := d.Show(ctx, "dhcp")
	for _, sec := range secs {
		if sec.Name == "lan" && sec.Options["leasetime"] != "1h" {
			t.Fatal("foreign staged change reverted")
		}
	}
}

type failingPending struct{ *mock.Device }

func (failingPending) Pending(context.Context, string) (bool, error) {
	return false, errors.New("ubus timeout")
}

func TestTransientReadErrorReleasesGuard(t *testing.T) {
	s, d, a := setup(t, listFixture)
	s.Device = failingPending{d}
	if err := s.Push(ctx, layers[VPN]); err == nil {
		t.Fatal("want error")
	}
	if guarded(t, s) || a.n != 0 {
		t.Fatal("guard kept although nothing was written")
	}
}

func TestCorruptRecordRefusesAndKeepsGuard(t *testing.T) {
	s, _, a := setup(t, listFixture)
	_ = os.WriteFile(s.RecordPath, []byte("{"), 0o600)
	for _, op := range []func() error{
		func() error { return s.Pop(ctx, VPN) },
		func() error { return s.Resume(ctx) }, // guard kept by the Pop above
	} {
		if err := op(); err == nil {
			t.Fatal("accepted corrupt record")
		}
	}
	st, err := s.Status()
	if err != nil || !st.Pending || st.Corrupt == "" || a.n != 0 {
		t.Fatalf("status must expose pending + corrupt: %+v %v (applied %d)", st, err, a.n)
	}
}

func TestInvalidLayerWritesNothing(t *testing.T) {
	s, _, a := setup(t, listFixture)
	for _, l := range []Layer{
		{Name: "evil", Servers: []string{"1.1.1.1"}},
		{Name: Tailscale, Servers: []string{"/ts.net/100.100.100.100"}, NoResolv: true},
		{Name: VPN},
	} {
		if err := s.Push(ctx, l); err == nil {
			t.Errorf("accepted %+v", l)
		}
	}
	if a.n != 0 || guarded(t, s) {
		t.Fatal("applied or guard left after validation error")
	}
}

func TestStatusIsReadOnly(t *testing.T) {
	s, _, a := setup(t, listFixture)
	_ = s.Push(ctx, layers[VPN])
	n := a.n
	for i := 0; i < 3; i++ {
		if _, err := s.Status(); err != nil {
			t.Fatal(err)
		}
	}
	if a.n != n {
		t.Fatal("read mutated DNS")
	}
}

func TestResumeRefusesForeignStagedChanges(t *testing.T) {
	s, d, a := setup(t, listFixture)
	a.fail = true
	_ = s.Push(ctx, layers[VPN])
	a.fail = false
	_ = d.Set(ctx, "dhcp", "lan", "leasetime", "1h") // staged while the guard is held
	n := a.n
	if err := s.Resume(ctx); !errors.Is(err, ErrForeignStaged) {
		t.Fatal(err)
	}
	if a.n != n || !guarded(t, s) {
		t.Fatal("resume applied foreign changes or released the guard")
	}
	if p, _ := d.Pending(ctx, "dhcp"); !p {
		t.Fatal("resume reverted foreign changes")
	}
}

// Regression for the S5 finding: the device's one-element list must come back as a list.
func TestDeviceSingleElementListRestoresAsList(t *testing.T) {
	s, d, _ := setup(t, deviceFixture)
	_, before, _ := ReadState(ctx, d)
	if before.Server.Scalar || !reflect.DeepEqual(before.Server.Values, []string{"127.0.0.1#5353"}) {
		t.Fatalf("base read wrong: %+v", before.Server)
	}
	_ = s.Push(ctx, layers[Captive])
	_ = s.Pop(ctx, Captive)
	secs, _ := d.Show(ctx, "dhcp")
	if _, isOpt := secs[0].Options["server"]; isOpt || len(secs[0].Lists["server"]) != 1 {
		t.Fatalf("restored as option: dnsmasq would ignore it: %+v", secs[0])
	}
}

func TestAfterApplyRunsBeforeGuardCleanup(t *testing.T) {
	for _, fail := range []bool{false, true} {
		s, _, applier := setup(t, listFixture)
		applier.fail = fail
		calls := 0
		s.AfterApply = func() {
			calls++
			if applier.n != 1 || !guarded(t, s) {
				t.Error("expected callback after apply while guard is held")
			}
		}
		if err := s.Push(ctx, layers[AdGuard]); (err != nil) != fail || calls != 1 {
			t.Fatalf("failure=%v: apply error %v, callbacks %d", fail, err, calls)
		}
	}
}
