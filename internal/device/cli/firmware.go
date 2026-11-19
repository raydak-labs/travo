package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/blackdark-org/travo-greenfield/internal/firmware"
)

// firmwareName keeps every firmware path a plain file name in /tmp: it reaches a shell line.
var firmwareName = regexp.MustCompile(`^[A-Za-z0-9_-][A-Za-z0-9._-]*$`)

func (d *Device) checkFirmwarePath(path string) error {
	dir := d.TmpDir
	if dir == "" {
		dir = "/tmp"
	}
	if filepath.Dir(path) != dir || !firmwareName.MatchString(filepath.Base(path)) {
		return fmt.Errorf("firmware path %q is not a file in %s", path, dir)
	}
	return nil
}

// StoreFirmware renames an uploaded temp file into place when it already sits on the
// same tmpfs (no second copy in RAM), else streams it. Either way the result is hashed
// from dst, so the checksum is of what sysupgrade will read.
func (d *Device) StoreFirmware(_ context.Context, src io.Reader, dst string) (firmware.Stored, error) {
	if err := d.checkFirmwarePath(dst); err != nil {
		return firmware.Stored{}, err
	}
	if f, ok := src.(*os.File); !ok || os.Rename(f.Name(), dst) != nil {
		if err := copyFile(src, dst); err != nil {
			return firmware.Stored{}, err
		}
	}
	return hashFile(dst)
}

func copyFile(src io.Reader, dst string) error {
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".travo-firmware-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	_, err = io.Copy(tmp, src)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dst)
}

func hashFile(path string) (firmware.Stored, error) {
	f, err := os.Open(path)
	if err != nil {
		return firmware.Stored{}, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return firmware.Stored{}, err
	}
	return firmware.Stored{Size: n, SHA256: hex.EncodeToString(h.Sum(nil))}, nil
}

// RemoveFirmware deletes a staged image; os.ErrNotExist when there was none.
func (d *Device) RemoveFirmware(_ context.Context, path string) error {
	if err := d.checkFirmwarePath(path); err != nil {
		return err
	}
	return os.Remove(path)
}

// ValidateFirmware runs the read-only image checks (fwtool signature and board match,
// platform_check_image). It never flashes. The ubus timeout sits inside the apply tier.
func (d *Device) ValidateFirmware(ctx context.Context, path string) ([]byte, error) {
	if err := d.checkFirmwarePath(path); err != nil {
		return nil, err
	}
	args, err := json.Marshal(map[string]string{"path": path})
	if err != nil {
		return nil, err
	}
	out, err := d.run(ctx, TierApply, "ubus", "-t", "55", "call", "system", "validate_firmware_image", string(args))
	return []byte(out), err
}

// PackageWorlds reads the apk world of the overlay and of the read-only image.
func (d *Device) PackageWorlds(context.Context) (world, rom []byte, err error) {
	read := d.ReadFile
	if read == nil {
		read = os.ReadFile
	}
	if world, err = read("/etc/apk/world"); err != nil {
		return nil, nil, err
	}
	if rom, err = read("/rom/etc/apk/world"); err != nil {
		return nil, nil, err
	}
	return world, rom, nil
}

// sysupgradeShellCommand flashes after the HTTP reply flushed. `-n` drops all settings.
func sysupgradeShellCommand(path string, keepSettings bool) string {
	flag := ""
	if !keepSettings {
		flag = "-n "
	}
	return "(sleep 2; sysupgrade " + flag + path + ") </dev/null >/dev/null 2>&1 &"
}

// Sysupgrade schedules the detached flash. Scheduling is bounded; the flash and reboot
// are terminal operations without a timeout tier (invariants §1).
func (d *Device) Sysupgrade(ctx context.Context, path string, keepSettings bool) error {
	if err := d.checkFirmwarePath(path); err != nil {
		return err
	}
	_, err := d.run(ctx, TierWrite, "setsid", "sh", "-c", sysupgradeShellCommand(path, keepSettings))
	return err
}
