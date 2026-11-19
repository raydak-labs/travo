package firmware

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/guard"
)

func fixture(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", path))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// validReply flips only the verbatim capture's verdict (a zero file is not an image).
func validReply(t *testing.T) []byte {
	t.Helper()
	b := fixture(t, "container/openwrt-25.12.6/sysupgrade/validate-zero-image.json")
	out := bytes.Replace(b, []byte(`"valid": false`), []byte(`"valid": true`), 1)
	if bytes.Equal(out, b) {
		t.Fatal("fixture shape changed")
	}
	return out
}

// device records the order of events: the guard must exist before sysupgrade is scheduled.
type device struct {
	t          *testing.T
	guards     guard.Dir
	validation []byte
	upgrades   []bool
	sawGuard   bool
	upgradeErr error
	stored     bool
	info       []byte
}

func (d *device) StoreFirmware(_ context.Context, src io.Reader, _ string) (Stored, error) {
	n, err := io.Copy(io.Discard, src)
	d.stored = err == nil
	return Stored{Size: n, SHA256: strings.Repeat("a", 64)}, err
}

func (d *device) RemoveFirmware(context.Context, string) error {
	if !d.stored {
		return os.ErrNotExist
	}
	d.stored = false
	return nil
}

func (d *device) ValidateFirmware(context.Context, string) ([]byte, error) { return d.validation, nil }

func (d *device) PackageWorlds(context.Context) ([]byte, []byte, error) {
	w := fixture(d.t, "device/axt1800-25.12.6/sysupgrade/etc-apk-world.txt")
	return w, fixture(d.t, "device/axt1800-25.12.6/sysupgrade/rom-etc-apk-world.txt"), nil
}

func (d *device) Sysupgrade(_ context.Context, _ string, keep bool) error {
	d.upgrades = append(d.upgrades, keep)
	_, err := os.Stat(filepath.Join(d.guards.Path, GuardName))
	d.sawGuard = err == nil
	return d.upgradeErr
}

func (d *device) Call(_ context.Context, object, method string, _ any) (json.RawMessage, error) {
	switch object + "." + method {
	case "system.info":
		if d.info != nil {
			return d.info, nil
		}
		return fixture(d.t, "device/axt1800-25.12.6/system-info.json"), nil
	case "system.board":
		return fixture(d.t, "device/axt1800-25.12.6/system-board.json"), nil
	}
	return nil, errors.New("unexpected ubus call")
}

func setup(t *testing.T) (*Service, *device) {
	t.Helper()
	g := guard.Dir{Path: t.TempDir(), Now: time.Now}
	d := &device{t: t, guards: g, validation: validReply(t)}
	return &Service{Device: d, Guards: g}, d
}

func staged(t *testing.T, s *Service) Image {
	t.Helper()
	if err := s.Prepare(context.Background(), 1024); err != nil {
		t.Fatal(err)
	}
	defer s.FinishUpload()
	img, err := s.Upload(context.Background(), strings.NewReader("image"))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func heldNames(t *testing.T, s *Service) []string {
	t.Helper()
	infos, err := s.Guards.Stale()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, i := range infos {
		names = append(names, i.Name)
	}
	return names
}

func TestParseValidationVerbatimReply(t *testing.T) {
	v, err := ParseValidation(fixture(t, "container/openwrt-25.12.6/sysupgrade/validate-zero-image.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := Validation{Valid: false, Forceable: true, AllowBackup: true,
		Tests: map[string]bool{"fwtool_signature": true, "fwtool_device_match": true}}
	if !reflect.DeepEqual(v, want) {
		t.Fatalf("%+v", v)
	}
	for _, bad := range []string{`{}`, `{"valid":true,"forceable":true}`, `not json`} {
		if _, err := ParseValidation([]byte(bad)); err == nil {
			t.Errorf("%s parsed", bad)
		}
	}
}

func TestDroppedPackagesComparesTheWorlds(t *testing.T) {
	w := fixture(t, "device/axt1800-25.12.6/sysupgrade/etc-apk-world.txt")
	if got := DroppedPackages(w, fixture(t, "device/axt1800-25.12.6/sysupgrade/rom-etc-apk-world.txt")); len(got) != 0 {
		t.Fatalf("router has no user packages, got %v", got)
	}
	world := append(append([]byte{}, w...), "tcpdump\nkmod-usb-net-rndis\n"...)
	if got := DroppedPackages(world, w); !reflect.DeepEqual(got, []string{"kmod-usb-net-rndis", "tcpdump"}) {
		t.Fatalf("%v", got)
	}
	// A version pin on either side still names the same package.
	if got := DroppedPackages([]byte("kernel=6.12.113\n"), []byte("kernel=6.12.112\n")); len(got) != 0 {
		t.Fatalf("%v", got)
	}
}

func TestUploadDescribesTheImage(t *testing.T) {
	s, _ := setup(t)
	img := staged(t, s)
	if img.Size != 5 || img.Board != "glinet,gl-axt1800" || !img.Validation.Valid || !img.PackagesKnown || len(img.Dropped) != 0 {
		t.Fatalf("%+v", img)
	}
	st, err := s.Status(context.Background())
	if err != nil || st.Staged == nil || st.Staged.SHA256 != img.SHA256 || st.Pending || st.MaxImageBytes != MaxImageBytes {
		t.Fatalf("%+v %v", st, err)
	}
}

func TestUploadRemovesAnImageTheValidatorCannotDescribe(t *testing.T) {
	s, d := setup(t)
	first := staged(t, s)
	d.validation = []byte(`{}`)
	if _, err := s.Upload(context.Background(), strings.NewReader("x")); err == nil {
		t.Fatal("unparsable validation accepted")
	}
	if d.stored {
		t.Fatal("image kept")
	}
	// The earlier image's file is gone too: Apply must not flash it.
	if err := s.Apply(context.Background(), first.SHA256, true, Phrase); !errors.Is(err, ErrNotStaged) || len(d.upgrades) != 0 {
		t.Fatalf("stale image: %v", err)
	}
}

func TestPrepareHoldsOneUploadSlotUntilFinished(t *testing.T) {
	s, _ := setup(t)
	ctx := context.Background()
	if err := s.Prepare(ctx, 1024); err != nil {
		t.Fatal(err)
	}
	if err := s.Prepare(ctx, 1024); !errors.Is(err, ErrUploadBusy) {
		t.Fatalf("second admission: %v", err)
	}
	s.FinishUpload()
	if err := s.Prepare(ctx, 1024); err != nil {
		t.Fatalf("after finish: %v", err)
	}
}

func TestPrepareRefusesWhatCannotFit(t *testing.T) {
	s, d := setup(t)
	if err := s.Prepare(context.Background(), MaxRequestBytes+1); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("%v", err)
	}
	// Verbatim capture: 149 MB RAM, 207 MB /tmp available; a full 64 MiB image with its
	// multipart envelope plus headroom fits.
	if err := s.Prepare(context.Background(), MaxRequestBytes); err != nil {
		t.Fatal(err)
	}
	s.FinishUpload()
	d.info = []byte(`{"uptime":1,"load":[1,2,3],"memory":{"total":100000000,"available":50000000},"root":{"total":8,"avail":4},"tmp":{"avail":200000}}`)
	if err := s.Prepare(context.Background(), 20<<20); !errors.Is(err, ErrNoRoom) {
		t.Fatalf("%v", err)
	}
}

func TestApplyWritesTheGuardFirstAndNeverRemovesIt(t *testing.T) {
	s, d := setup(t)
	img := staged(t, s)
	if err := s.Apply(context.Background(), img.SHA256, true, Phrase); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(d.upgrades, []bool{true}) || !d.sawGuard {
		t.Fatalf("upgrades %v, guard present at flash %v", d.upgrades, d.sawGuard)
	}
	if got := heldNames(t, s); !reflect.DeepEqual(got, []string{GuardName}) {
		t.Fatalf("guards after success %v", got)
	}
	if err := s.Apply(context.Background(), img.SHA256, true, Phrase); !errors.Is(err, ErrPending) || len(d.upgrades) != 1 {
		t.Fatalf("second: %v", err)
	}
	// A pending flash also blocks replacing the image it reads.
	if err := s.Prepare(context.Background(), 1); !errors.Is(err, ErrPending) {
		t.Fatalf("prepare: %v", err)
	}
	if st, _ := s.Status(context.Background()); !st.Pending {
		t.Fatal("status not pending")
	}
}

func TestApplyKeepsTheGuardWhenSchedulingFails(t *testing.T) {
	s, d := setup(t)
	img := staged(t, s)
	d.upgradeErr = errors.New("setsid: exit 1")
	if err := s.Apply(context.Background(), img.SHA256, false, Phrase); !errors.Is(err, ErrFailed) {
		t.Fatalf("%v", err)
	}
	if got := heldNames(t, s); len(got) != 1 {
		t.Fatalf("guard removed after a failed schedule: %v", got)
	}
}

type failingGuards struct{ guard.Dir }

func (failingGuards) AcquireExclusive(string, func([]guard.Info) error) error {
	return errors.New("read-only file system")
}

// An upgrade racing another guarded operation: exactly one proceeds, and a refused upgrade
// leaves no firmware guard behind (nothing was flashed).
func TestApplyRacingAnotherGuardedOperationHasOneWinner(t *testing.T) {
	for range 30 {
		s, d := setup(t)
		img := staged(t, s)
		var applyErr, otherErr error
		var wg sync.WaitGroup
		start := make(chan struct{})
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			applyErr = s.Apply(context.Background(), img.SHA256, true, Phrase)
		}()
		go func() {
			defer wg.Done()
			<-start
			otherErr = d.guards.AcquireExclusive("catalog-package-in-progress", guard.RefuseTerminal)
		}()
		close(start)
		wg.Wait()
		if (applyErr == nil) == (otherErr == nil) {
			t.Fatalf("apply %v, other %v: want exactly one winner", applyErr, otherErr)
		}
		if applyErr != nil && (!errors.Is(applyErr, ErrBusy) || len(d.upgrades) != 0 || slices.Contains(heldNames(t, s), GuardName)) {
			t.Fatalf("refused upgrade: %v, upgrades %v, guards %v", applyErr, d.upgrades, heldNames(t, s))
		}
	}
}

func TestApplyAbortsWhenTheGuardCannotBeWritten(t *testing.T) {
	s, d := setup(t)
	img := staged(t, s)
	s.Guards = failingGuards{d.guards}
	if err := s.Apply(context.Background(), img.SHA256, true, Phrase); !errors.Is(err, ErrFailed) {
		t.Fatalf("%v", err)
	}
	if len(d.upgrades) != 0 {
		t.Fatal("flashed without a guard")
	}
}

func TestApplyRefusals(t *testing.T) {
	s, d := setup(t)
	img := staged(t, s)
	ctx := context.Background()
	for _, p := range []string{"", "Upgrade Firmware", "upgrade firmware ", "yes"} {
		if err := s.Apply(ctx, img.SHA256, true, p); !errors.Is(err, ErrConfirm) {
			t.Fatalf("%q: %v", p, err)
		}
	}
	if err := s.Apply(ctx, strings.Repeat("b", 64), true, Phrase); !errors.Is(err, ErrNotStaged) {
		t.Fatalf("other sha: %v", err)
	}
	if err := d.guards.Acquire("reboot-in-progress"); err != nil {
		t.Fatal(err)
	}
	if err := s.Apply(ctx, img.SHA256, true, Phrase); !errors.Is(err, ErrBusy) {
		t.Fatalf("held guard: %v", err)
	}
	if err := d.guards.Release("reboot-in-progress"); err != nil {
		t.Fatal(err)
	}
	d.validation = fixture(t, "container/openwrt-25.12.6/sysupgrade/validate-zero-image.json")
	img = staged(t, s)
	if err := s.Apply(ctx, img.SHA256, true, Phrase); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid: %v", err)
	}
	d.validation = bytes.Replace(validReply(t), []byte(`"allow_backup": true`), []byte(`"allow_backup": false`), 1)
	img = staged(t, s)
	if err := s.Apply(ctx, img.SHA256, true, Phrase); !errors.Is(err, ErrNoBackup) {
		t.Fatalf("no backup: %v", err)
	}
	if len(d.upgrades) != 0 || len(heldNames(t, s)) != 0 {
		t.Fatalf("upgrades %v guards %v", d.upgrades, heldNames(t, s))
	}
}
