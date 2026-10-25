package system

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/guard"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

func TestUpgradeFirmware_SavesFile(t *testing.T) {
	ub := ubus.NewMockUbus()
	svc := New(ub, uci.NewMockUCI(), &MockStorageProvider{})
	svc.SetGuardDir(t.TempDir())
	// The flash goroutine must finish inside this test, or it would pick up a
	// later test's PATH and record a call that has nothing to do with it.
	marker := installFakeSysupgrade(t)

	image := buildFirmwareImage("GL.iNet GL-MT3000", mockBoardDevices())
	reader := bytes.NewReader(image)

	meta, err := svc.UpgradeFirmware(reader, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.Model != "GL.iNet GL-MT3000" {
		t.Errorf("parsed model = %q, want GL.iNet GL-MT3000", meta.Model)
	}
	waitForSysupgradeCall(t, marker)

	// The staged image must exist somewhere under /tmp with the uploaded bytes.
	data, err := findFirmwareImage(string(image))
	if err != nil {
		t.Fatalf("firmware file was not staged: %v", err)
	}
	_ = os.Remove(data)
}

// A .bin for another board must be refused BEFORE sysupgrade runs. sysupgrade
// only rejects it after this endpoint answered 200, by which point the operator
// has already been told the flash started — and on a travel router with no
// serial console the result is a brick.
func TestUpgradeFirmware_RejectsForeignDevice(t *testing.T) {
	svc, dir := newGuardedSystemService(t)
	marker := installFakeSysupgrade(t)

	image := buildFirmwareImage("Some Other Router", []string{"other,router-x1"})
	_, err := svc.UpgradeFirmware(bytes.NewReader(image), true)
	if err == nil {
		t.Fatal("a firmware image for another board was accepted")
	}
	if !errors.Is(err, ErrUnsupportedFirmware) {
		t.Errorf("error = %v, want ErrUnsupportedFirmware", err)
	}
	if !strings.Contains(err.Error(), "Some Other Router") {
		t.Errorf("the error must name the image model, got %v", err)
	}
	assertNoSysupgradeCall(t, marker)
	if _, err := os.Stat(guard.In(dir, firmwareUpgradeGuard).Path()); err == nil {
		t.Error("a rejected image must not leave a crash guard: nothing was flashed")
	}
	assertNoStagedFirmware(t)
}

// A junk file with a .bin name has no metadata block at all — the textbook
// brick. It must be a 400-equivalent error, not a flash attempt.
func TestUpgradeFirmware_RejectsJunkBinary(t *testing.T) {
	svc, _ := newGuardedSystemService(t)
	marker := installFakeSysupgrade(t)

	junk := make([]byte, 8192)
	for i := range junk {
		junk[i] = byte(i)
	}
	_, err := svc.UpgradeFirmware(bytes.NewReader(junk), true)
	if err == nil {
		t.Fatal("a junk .bin with no metadata block was accepted")
	}
	if !errors.Is(err, ErrNoFirmwareMetadata) {
		t.Errorf("error = %v, want ErrNoFirmwareMetadata", err)
	}
	assertNoSysupgradeCall(t, marker)
	assertNoStagedFirmware(t)
}

// An image that names no supported_devices cannot be verified at all. Flashing
// it anyway is a coin flip with the device on the wrong side.
func TestUpgradeFirmware_RejectsImageWithoutSupportedDevices(t *testing.T) {
	svc, _ := newGuardedSystemService(t)
	marker := installFakeSysupgrade(t)

	image := buildFirmwareImage("GL.iNet GL-MT3000", nil)
	if _, err := svc.UpgradeFirmware(bytes.NewReader(image), true); err == nil {
		t.Fatal("an image without supported_devices was accepted")
	}
	assertNoSysupgradeCall(t, marker)
}

func TestParseFirmwareMetadata(t *testing.T) {
	image := buildFirmwareImage("Linksys EA8300", []string{"linksys,ea8300", "linksys_e8300-ubi"})
	meta, err := ParseFirmwareMetadata(bytes.NewReader(image))
	if err != nil {
		t.Fatalf("a well-formed image was rejected: %v", err)
	}
	if meta.Model != "Linksys EA8300" {
		t.Errorf("model = %q", meta.Model)
	}
	if meta.Version != "23.05.2" || meta.Distname != "OpenWrt" {
		t.Errorf("version/distname = %q/%q", meta.Version, meta.Distname)
	}
	if meta.CompatVersion != 1 {
		t.Errorf("compat_version = %d, want 1", meta.CompatVersion)
	}
	if len(meta.SupportedDevices) != 3 || meta.SupportedDevicesRaw != "linksys,ea8300,linksys_e8300-ubi" {
		t.Errorf("supported_devices = %v (raw %q)", meta.SupportedDevices, meta.SupportedDevicesRaw)
	}
	// The mock board answers "glinet,gl-mt3000" with model "GL.iNet GL-MT3000".
	if meta.SupportsDevice("glinet,gl-mt3000", []string{"gl-mt3000", "GL.iNet GL-MT3000", "GL-MT3000"}) {
		t.Error("an EA8300 image must not claim support for a GL-MT3000 board")
	}
	own := buildFirmwareImage("GL.iNet GL-MT3000", []string{"gl-mt3000"})
	ownMeta, err := ParseFirmwareMetadata(bytes.NewReader(own))
	if err != nil {
		t.Fatalf("parsing the board's own image: %v", err)
	}
	if !ownMeta.SupportsDevice("glinet,gl-mt3000", []string{"gl-mt3000"}) {
		t.Error("a GL-MT3000 image must be accepted for the GL-MT3000 board")
	}
	// An image listing the full "glinet,gl-mt3000" board name matches too.
	full := buildFirmwareImage("GL.iNet GL-MT3000", []string{"glinet,gl-mt3000"})
	fullMeta, err := ParseFirmwareMetadata(bytes.NewReader(full))
	if err != nil {
		t.Fatalf("parsing the full-name image: %v", err)
	}
	if !fullMeta.SupportsDevice("glinet,gl-mt3000", nil) {
		t.Error("supported_devices=glinet,gl-mt3000 must match this board")
	}
}

func TestParseFirmwareMetadata_RejectsJunk(t *testing.T) {
	if _, err := ParseFirmwareMetadata(bytes.NewReader(make([]byte, 1024))); !errors.Is(err, ErrNoFirmwareMetadata) {
		t.Errorf("junk image: error = %v, want ErrNoFirmwareMetadata", err)
	}
	truncated := buildFirmwareImage("GL.iNet GL-MT3000", mockBoardDevices())
	truncated = truncated[:len(truncated)-8]
	if _, err := ParseFirmwareMetadata(bytes.NewReader(truncated)); err == nil {
		t.Error("a truncated metadata block was accepted")
	}
}

// UpgradeFirmware must leave a crash-guard marker: a power cut mid-flash
// leaves an unbootable device, and the marker is the only recovery hint.
func TestUpgradeFirmware_WritesCrashGuard(t *testing.T) {
	svc, dir := newGuardedSystemService(t)
	marker := installFakeSysupgrade(t)
	image := buildFirmwareImage("GL.iNet GL-MT3000", mockBoardDevices())
	if _, err := svc.UpgradeFirmware(bytes.NewReader(image), true); err != nil {
		t.Fatalf("UpgradeFirmware: %v", err)
	}
	waitForSysupgradeCall(t, marker)
	guard := guard.In(dir, firmwareUpgradeGuard).Path()
	data, err := os.ReadFile(guard)
	if err != nil {
		t.Fatalf("expected a crash guard at %s: %v", guard, err)
	}
	if !strings.Contains(string(data), "sysupgrade") {
		t.Errorf("guard should record what was running, got %q", data)
	}
	if !strings.Contains(string(data), "GL.iNet GL-MT3000") {
		t.Errorf("guard should record the image model, got %q", data)
	}
}

// Board names are prefixes of each other: "gl-mt3000" is a prefix of
// "gl-mt3000-nand". Matching the supported_devices list with a substring test
// therefore accepts a NAND-only image for a NOR router, and that flash is the
// brick this whole check exists to prevent. Entries must be compared whole.
func TestSupportsDevice_RequiresAWholeEntry(t *testing.T) {
	cases := []struct {
		name        string
		supported   []string
		boardName   string
		aliases     []string
		wantSupport bool
	}{
		{
			name:        "nand-only image on a nor board",
			supported:   []string{"glinet,gl-mt3000-nand"},
			boardName:   "glinet,gl-mt3000",
			aliases:     []string{"gl-mt3000", "GL.iNet GL-MT3000", "GL-MT3000"},
			wantSupport: false,
		},
		{
			name:        "nor-only image on a nand board",
			supported:   []string{"glinet,gl-mt3000"},
			boardName:   "glinet,gl-mt3000-nand",
			aliases:     []string{"gl-mt3000-nand"},
			wantSupport: false,
		},
		{
			name:        "own image listed beside a sibling variant",
			supported:   []string{"glinet,gl-mt3000", "glinet,gl-mt3000-nand"},
			boardName:   "glinet,gl-mt3000",
			aliases:     []string{"gl-mt3000", "GL.iNet GL-MT3000"},
			wantSupport: true,
		},
		{
			name:        "full multi-board name with no aliases",
			supported:   []string{"linksys,ea8300,linksys_e8300-ubi"},
			boardName:   "linksys,ea8300",
			aliases:     nil,
			wantSupport: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			meta, err := ParseFirmwareMetadata(bytes.NewReader(
				buildFirmwareImage("GL.iNet GL-MT3000", tc.supported)))
			if err != nil {
				t.Fatalf("parsing image: %v", err)
			}
			if got := meta.SupportsDevice(tc.boardName, tc.aliases); got != tc.wantSupport {
				t.Errorf("SupportsDevice(%q, %v) with supported_devices %q = %v, want %v",
					tc.boardName, tc.aliases, meta.SupportedDevicesRaw, got, tc.wantSupport)
			}
		})
	}
}

// The end-to-end consequence of the prefix bug: an image that only supports the
// NAND variant must never reach sysupgrade on this NOR board.
func TestUpgradeFirmware_RejectsSiblingVariantImage(t *testing.T) {
	svc, dir := newGuardedSystemService(t)
	marker := installFakeSysupgrade(t)

	image := buildFirmwareImage("GL.iNet GL-MT3000 (NAND)", []string{"glinet,gl-mt3000-nand"})
	_, err := svc.UpgradeFirmware(bytes.NewReader(image), true)
	if err == nil {
		t.Fatal("an image for the sibling NAND variant was accepted for this NOR board")
	}
	if !errors.Is(err, ErrUnsupportedFirmware) {
		t.Errorf("error = %v, want ErrUnsupportedFirmware", err)
	}
	assertNoSysupgradeCall(t, marker)
	if _, err := os.Stat(guard.In(dir, firmwareUpgradeGuard).Path()); err == nil {
		t.Error("a rejected image must not leave a crash guard: nothing was flashed")
	}
	assertNoStagedFirmware(t)
}

// board_name is comma separated ("glinet,gl-mt3000"), but it names ONE board.
// Handing its parts out as aliases hands out the vendor prefix "glinet", which
// matches every GL.iNet entry in a supported_devices list and would let a
// sibling board's image through.
func TestBoardIdentityDoesNotSplitTheVendorPrefix(t *testing.T) {
	svc, _ := newGuardedSystemService(t)
	name, aliases, err := svc.boardIdentity()
	if err != nil {
		t.Fatalf("boardIdentity: %v", err)
	}
	if name != "glinet,gl-mt3000" {
		t.Errorf("board name = %q, want the whole ubus value", name)
	}
	for _, a := range aliases {
		if a == "glinet" {
			t.Errorf("aliases %v must not contain the bare vendor prefix", aliases)
		}
	}
}
