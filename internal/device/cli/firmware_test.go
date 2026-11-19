package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The fake runner only records argv: nothing is executed, nothing is flashed.
func TestSysupgradeSchedulesDetachedFlash(t *testing.T) {
	for keep, want := range map[bool]string{
		true:  "sh -c (sleep 2; sysupgrade /tmp/travo-firmware.bin) </dev/null >/dev/null 2>&1 &",
		false: "sh -c (sleep 2; sysupgrade -n /tmp/travo-firmware.bin) </dev/null >/dev/null 2>&1 &",
	} {
		var calls []call
		d := &Device{Run: fake(&calls, nil, "")}
		if err := d.Sysupgrade(context.Background(), "/tmp/travo-firmware.bin", keep); err != nil {
			t.Fatal(err)
		}
		if len(calls) != 1 || calls[0].name != "setsid" || calls[0].args != want {
			t.Fatalf("keep %v: %+v", keep, calls)
		}
	}
}

func TestFirmwarePathsStayPlainTmpFiles(t *testing.T) {
	var calls []call
	d := &Device{Run: fake(&calls, nil, "")}
	for _, p := range []string{"/tmp/a b", "/tmp/x;reboot", "/etc/firmware.bin", "/tmp/../etc/x", "tmp/x", "/tmp/sub/x", "/tmp/.."} {
		if d.Sysupgrade(context.Background(), p, true) == nil {
			t.Errorf("%q accepted", p)
		}
		if _, err := d.ValidateFirmware(context.Background(), p); err == nil {
			t.Errorf("validate %q accepted", p)
		}
	}
	if len(calls) != 0 {
		t.Fatalf("ran %+v", calls)
	}
}

func TestValidateFirmwareCallsUbus(t *testing.T) {
	var calls []call
	d := &Device{Run: fake(&calls, map[string]string{
		`ubus -t 55 call system validate_firmware_image {"path":"/tmp/travo-firmware.bin"}`: `{"valid":true}`,
	}, "")}
	out, err := d.ValidateFirmware(context.Background(), "/tmp/travo-firmware.bin")
	if err != nil || string(out) != `{"valid":true}` {
		t.Fatalf("%s %v %+v", out, err, calls)
	}
}

func TestPackageWorldsReadsOverlayAndRom(t *testing.T) {
	d := &Device{ReadFile: func(p string) ([]byte, error) {
		switch p {
		case "/etc/apk/world":
			return []byte("a\n"), nil
		case "/rom/etc/apk/world":
			return []byte("b\n"), nil
		}
		return nil, os.ErrNotExist
	}}
	w, r, err := d.PackageWorlds(context.Background())
	if err != nil || string(w) != "a\n" || string(r) != "b\n" {
		t.Fatalf("%q %q %v", w, r, err)
	}
}

func TestStoreFirmwareStreamsOrMovesAndHashes(t *testing.T) {
	dir := t.TempDir()
	d := &Device{TmpDir: dir}
	dst := filepath.Join(dir, "travo-firmware.bin")
	const sumABC = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	got, err := d.StoreFirmware(context.Background(), strings.NewReader("abc"), dst)
	if err != nil || got.Size != 3 || got.SHA256 != sumABC {
		t.Fatalf("%+v %v", got, err)
	}
	// An uploaded multipart temp file on the same tmpfs is moved, not copied.
	src, err := os.CreateTemp(dir, "multipart-")
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	if _, err := src.WriteString("abc"); err != nil {
		t.Fatal(err)
	}
	if got, err = d.StoreFirmware(context.Background(), src, dst); err != nil || got.SHA256 != sumABC {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := os.Stat(src.Name()); !os.IsNotExist(err) {
		t.Fatalf("temp file still there: %v", err)
	}
	if err := d.RemoveFirmware(context.Background(), dst); err != nil {
		t.Fatal(err)
	}
	if err := d.RemoveFirmware(context.Background(), dst); !os.IsNotExist(err) {
		t.Fatalf("second remove: %v", err)
	}
}
