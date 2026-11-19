package mock

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"io"
	"os"

	"github.com/blackdark-org/travo-greenfield/internal/firmware"
)

//go:embed testdata/validate-firmware-zero-image.json
var validateFirmwareZeroImage []byte

//go:embed testdata/apk-world.txt
var apkWorld []byte

// StoreFirmware hashes the upload and keeps only its size and checksum; mock mode writes nothing.
func (d *Device) StoreFirmware(_ context.Context, src io.Reader, dst string) (firmware.Stored, error) {
	h := sha256.New()
	n, err := io.Copy(h, src)
	if err != nil {
		return firmware.Stored{}, err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.firmwareStored = dst
	return firmware.Stored{Size: n, SHA256: hex.EncodeToString(h.Sum(nil))}, nil
}

func (d *Device) RemoveFirmware(_ context.Context, path string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.firmwareStored != path {
		return os.ErrNotExist
	}
	d.firmwareStored = ""
	return nil
}

// ValidateFirmware answers FirmwareValidation, else the verbatim zero-image capture.
func (d *Device) ValidateFirmware(context.Context, string) ([]byte, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.FirmwareValidation != nil {
		return d.FirmwareValidation, nil
	}
	return validateFirmwareZeroImage, nil
}

func (d *Device) PackageWorlds(context.Context) (world, rom []byte, err error) {
	return apkWorld, apkWorld, nil
}

// Sysupgrade records the request; mock mode never flashes anything.
func (d *Device) Sysupgrade(_ context.Context, _ string, keepSettings bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Sysupgrades = append(d.Sysupgrades, keepSettings)
	return d.SysupgradeErr
}
