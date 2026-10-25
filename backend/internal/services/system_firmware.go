package services

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/execx"
)

// Firmware image validation. A `.bin` extension check is not validation: a
// junk file with the right name is the textbook way to brick a router, and a
// travel router has no serial console to recover from. sysupgrade does re-check
// the image, but it does so in the background AFTER this endpoint answered 200,
// so doing it here turns a brick into a 400.
var (
	// ErrInvalidFirmware marks an image that must not be flashed. Handlers map
	// it to 400.
	ErrInvalidFirmware = errors.New("invalid firmware image")
	// ErrNoFirmwareMetadata is the "this is not an OpenWrt sysupgrade image"
	// case: no metadata block was found in the header.
	ErrNoFirmwareMetadata = errors.New("no OpenWrt firmware metadata block")
	// ErrUnsupportedFirmware is a readable image for other hardware.
	ErrUnsupportedFirmware = errors.New("firmware does not support this device")
)

// FirmwareMetadata is the OpenWrt metadata block that follows the kernel image
// in a sysupgrade image. It is what `sysupgrade -l` and the build system read to
// decide which boards an image may be flashed onto.
type FirmwareMetadata struct {
	// Model is the image's own model string, e.g. "Linksys EA8300".
	Model string
	// SupportedDevices is the image's supported_devices list, split on commas
	// for display. Note that OpenWrt board names themselves contain a comma
	// ("glinet,gl-mt3000"), so SupportedDevicesRaw is what device matching uses.
	SupportedDevices []string
	// SupportedDevicesRaw is the supported_devices value verbatim.
	SupportedDevicesRaw string
	// Version is the firmware version, e.g. "23.05.2".
	Version string
	// Distname is the distribution name, e.g. "OpenWrt".
	Distname string
	// CompatVersion is the metadata compat_version field.
	CompatVersion uint32
}

const (
	// firmwareMetadataMagic is the literal that opens the metadata payload,
	// NUL padded to 10 bytes.
	firmwareMetadataMagic = "metadata\x00\x00"
	// firmwareMetadataVersion is the only metadata block version mkimage emits.
	firmwareMetadataVersion = 1
	// maxFirmwareMetadataScan bounds how far into the image the metadata block is
	// searched. It sits directly after the (4-byte aligned) kernel image, so a
	// few MiB is generous; the bound keeps a junk upload from being read whole.
	maxFirmwareMetadataScan = 8 << 20
	// firmwareMetadataReadChunk is the streaming read size.
	firmwareMetadataReadChunk = 64 << 10
)

// ParseFirmwareMetadata reads the OpenWrt metadata block from the head of a
// sysupgrade image and returns it.
//
// Layout (as written by tools/mkimage and read by scripts/json_overview_image_info.py):
//
//	uint32 block size   (big endian, counts everything below including these 8 bytes)
//	uint32 version      (big endian, 1)
//	char   magic[10]    "metadata\0\0"
//	uint32 compat_version (big endian)
//	char   pairs[]      NUL-terminated key/value strings, key\0value\0…
func ParseFirmwareMetadata(r io.Reader) (FirmwareMetadata, error) {
	block, err := scanFirmwareMetadataBlock(r)
	if err != nil {
		return FirmwareMetadata{}, err
	}
	return parseFirmwareMetadataBlock(block)
}

// scanFirmwareMetadataBlock streams the head of the image and returns the
// complete metadata block, or ErrNoFirmwareMetadata.
func scanFirmwareMetadataBlock(r io.Reader) ([]byte, error) {
	magic := []byte(firmwareMetadataMagic)
	// Keep this much context around the magic so the 8-byte prefix and the
	// following fields are available even when the magic straddles a read.
	keep := len(magic) + 8 + 4
	buf := make([]byte, 0, firmwareMetadataReadChunk+keep)
	chunk := make([]byte, firmwareMetadataReadChunk)
	scanned := 0
	for scanned < maxFirmwareMetadataScan {
		n, err := r.Read(chunk)
		if n > 0 {
			scanned += n
			buf = append(buf, chunk[:n]...)
			if i := bytes.Index(buf, magic); i >= 8 {
				start := i - 8
				size := int(binary.BigEndian.Uint32(buf[start : start+4]))
				if size >= 8+len(magic)+4 && size <= maxFirmwareMetadataScan {
					need := start + size
					for len(buf) < need {
						n, rerr := r.Read(chunk)
						if n > 0 {
							buf = append(buf, chunk[:n]...)
							continue
						}
						if errors.Is(rerr, io.EOF) {
							break
						}
						if rerr != nil {
							return nil, rerr
						}
					}
					if len(buf) < need {
						return nil, fmt.Errorf("%w: truncated metadata block", ErrNoFirmwareMetadata)
					}
					return buf[start:need], nil
				}
			}
			// Keep the tail so a magic split across two reads is still found.
			if len(buf) > keep {
				buf = buf[len(buf)-keep:]
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	return nil, fmt.Errorf("%w: no \"metadata\" block in the first %d bytes", ErrNoFirmwareMetadata, maxFirmwareMetadataScan)
}

// parseFirmwareMetadataBlock decodes one metadata block.
func parseFirmwareMetadataBlock(block []byte) (FirmwareMetadata, error) {
	if len(block) < 8+len(firmwareMetadataMagic)+4 {
		return FirmwareMetadata{}, fmt.Errorf("%w: block is too short", ErrNoFirmwareMetadata)
	}
	if string(block[8:8+len(firmwareMetadataMagic)]) != firmwareMetadataMagic {
		return FirmwareMetadata{}, fmt.Errorf("%w: bad block magic", ErrNoFirmwareMetadata)
	}
	if version := binary.BigEndian.Uint32(block[4:8]); version != firmwareMetadataVersion {
		return FirmwareMetadata{}, fmt.Errorf("%w: unsupported metadata version %d", ErrNoFirmwareMetadata, version)
	}
	meta := FirmwareMetadata{CompatVersion: binary.BigEndian.Uint32(block[8+len(firmwareMetadataMagic):])}
	rest := block[8+len(firmwareMetadataMagic)+4:]
	for len(rest) > 0 {
		key, remainder, ok := cutNUL(rest)
		if !ok || key == "" {
			break
		}
		value, remainder, ok := cutNUL(remainder)
		if !ok {
			break
		}
		switch key {
		case "model":
			meta.Model = value
		case "version":
			meta.Version = value
		case "distname":
			meta.Distname = value
		case "supported_devices":
			meta.SupportedDevicesRaw = value
			for _, dev := range strings.Split(value, ",") {
				if dev = strings.TrimSpace(dev); dev != "" {
					meta.SupportedDevices = append(meta.SupportedDevices, dev)
				}
			}
		}
		rest = remainder
	}
	if meta.Model == "" {
		return FirmwareMetadata{}, fmt.Errorf("%w: metadata block has no model", ErrNoFirmwareMetadata)
	}
	return meta, nil
}

func cutNUL(b []byte) (value string, rest []byte, ok bool) {
	i := bytes.IndexByte(b, 0)
	if i < 0 {
		return "", nil, false
	}
	return string(b[:i]), b[i+1:], true
}

// SupportsDevice reports whether the image declares boardName (or any alias in
// aliases) as a supported device.
//
// Names are compared case-insensitively with spaces/underscores folded to
// dashes, because "GL.iNet GL-MT3000", "gl-mt3000" and "glinet,gl-mt3000" all
// name one board.
//
// The comma is the load-bearing part. OpenWrt board names contain one
// ("glinet,gl-mt3000") and the supported_devices list is itself comma
// separated, so a list entry can only be told apart from a board name by
// comparing WHOLE entries. Substring matching over the raw list is what
// OpenWrt's own patsubstr() does, and it is unsafe here: board names are
// prefixes of each other ("gl-mt3000" is a prefix of "gl-mt3000-nand"), so a
// NAND-only image passes on a NOR router — a flash that leaves the device
// unbootable with no serial console. Aliases are therefore matched for
// equality, and the one deliberate exception is the device's own boardName
// string, which OpenWrt may pack several models into ("glinet,gl-mt3000" for
// an image that declares "glinet,gl-mt3000,gl-mt3000-nor"); it may match the
// leading comma-delimited run of entries, still only on a comma boundary.
func (m FirmwareMetadata) SupportsDevice(boardName string, aliases []string) bool {
	list := normaliseBoardName(m.SupportedDevicesRaw)
	if list == "" {
		return false
	}
	if boardNameMatchesRun(list, normaliseBoardName(boardName)) {
		return true
	}
	for _, a := range aliases {
		if listHasDevice(list, normaliseBoardName(a)) {
			return true
		}
	}
	return false
}

// listHasDevice reports whether the normalised, comma-separated
// supported_devices list contains name as a whole entry.
func listHasDevice(list, name string) bool {
	if list == "" || name == "" {
		return false
	}
	for _, entry := range strings.Split(list, ",") {
		if entry == name {
			return true
		}
	}
	return false
}

// boardNameMatchesRun reports whether the possibly multi-model board name heads
// a whole run of comma-delimited entries in the list. It accepts an exact
// whole-list match and a match followed by a comma; it never accepts a match
// that runs into a longer entry, which is what keeps "gl-mt3000" from claiming
// "gl-mt3000-nand".
func boardNameMatchesRun(list, boardName string) bool {
	if list == "" || boardName == "" {
		return false
	}
	return list == boardName || strings.HasPrefix(list, boardName+",")
}

func normaliseBoardName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.NewReplacer(" ", "-", "_", "-").Replace(s)
	return s
}

// boardIdentity returns the strings this device may legitimately be named by in
// an image's supported_devices list: the ubus board_name ("glinet,gl-mt3000"),
// and the model ("GL.iNet GL-MT3000") together with its trailing model token.
//
// The comma-separated PARTS of board_name are deliberately not returned as
// separate aliases. "glinet,gl-mt3000" is one board whose name is comma
// separated, and its first part is a vendor prefix that every GL.iNet entry
// carries: splitting the name hands out "glinet", which then matches the
// vendor part of "glinet,gl-mt3000-nand" and re-admits a sibling board's
// image — the exact flash this check must refuse. The whole name is matched as
// one unit by SupportsDevice instead.
func (s *SystemService) boardIdentity() (string, []string, error) {
	board, err := s.ubus.Call("system", "board", nil)
	if err != nil {
		return "", nil, err
	}
	boardName, _ := board["board_name"].(string)
	model, _ := board["model"].(string)
	if boardName == "" && model == "" {
		return "", nil, fmt.Errorf("ubus system board returned no board identity")
	}
	primary := boardName
	if primary == "" {
		primary = model
	}
	var aliases []string
	if model != "" {
		aliases = append(aliases, model)
		// "GL.iNet GL-MT3000" → also try the trailing model token.
		if fields := strings.Fields(model); len(fields) > 1 {
			aliases = append(aliases, fields[len(fields)-1])
		}
	}
	return primary, aliases, nil
}

// ValidateFirmwareImage parses the OpenWrt metadata of an uploaded image and
// checks it against this board. It is exported so a handler can validate an
// upload before staging it.
func (s *SystemService) ValidateFirmwareImage(file io.Reader) (FirmwareMetadata, error) {
	meta, err := ParseFirmwareMetadata(file)
	if err != nil {
		return FirmwareMetadata{}, fmt.Errorf("%w: %w", ErrInvalidFirmware, err)
	}
	boardName, aliases, err := s.boardIdentity()
	if err != nil {
		return meta, fmt.Errorf("%w: cannot determine this board's identity, refusing to flash: %w", ErrInvalidFirmware, err)
	}
	if len(meta.SupportedDevices) == 0 {
		// Without the list nothing can be checked, and an unverifiable image on
		// a router with no serial console is a coin flip with the device on the
		// wrong side. sysupgrade would happily try.
		return meta, fmt.Errorf("%w: image %q declares no supported_devices, so it cannot be verified for %s",
			ErrInvalidFirmware, meta.Model, boardName)
	}
	if !meta.SupportsDevice(boardName, aliases) {
		return meta, fmt.Errorf("%w: %w: image %q supports %v, this device is %s",
			ErrInvalidFirmware, ErrUnsupportedFirmware, meta.Model, meta.SupportedDevicesRaw, boardName)
	}
	return meta, nil
}

// UpgradeFirmware saves the uploaded firmware image and flashes it via sysupgrade.
// If keepSettings is true, current configuration is preserved (-v flag).
// If keepSettings is false, settings are discarded (-n flag).
//
// The image is staged at a unique path, then its OpenWrt metadata is parsed and
// checked against this board BEFORE the crash guard is written and long before
// sysupgrade runs, so a junk or foreign image is a 400 and never reaches the
// flash. The parsed metadata is returned so the caller can tell the operator
// which image was flashed.
//
// The flash is asynchronous — it takes minutes and reboots the device — and
// runs under a crash guard so a power cut mid-write is visible after reboot.
func (s *SystemService) UpgradeFirmware(file io.Reader, keepSettings bool) (FirmwareMetadata, error) {
	firmwarePath, err := uniqueTempPath("/tmp", "firmware-*.bin")
	if err != nil {
		return FirmwareMetadata{}, err
	}
	out, err := os.Create(firmwarePath)
	if err != nil {
		return FirmwareMetadata{}, fmt.Errorf("creating firmware file: %w", err)
	}
	if _, err := io.Copy(out, file); err != nil {
		_ = out.Close()
		_ = os.Remove(firmwarePath)
		return FirmwareMetadata{}, fmt.Errorf("saving firmware file: %w", err)
	}
	_ = out.Close()

	staged, err := os.Open(firmwarePath)
	if err != nil {
		_ = os.Remove(firmwarePath)
		return FirmwareMetadata{}, fmt.Errorf("re-reading firmware file: %w", err)
	}
	meta, err := s.ValidateFirmwareImage(staged)
	_ = staged.Close()
	if err != nil {
		_ = os.Remove(firmwarePath)
		return meta, err
	}

	var args []string
	if keepSettings {
		args = []string{"-v", firmwarePath}
	} else {
		args = []string{"-n", firmwarePath}
	}

	reason := "sysupgrade " + strings.Join(args, " ") + " (" + meta.Model + ")"
	if err := s.crashGuard(firmwareUpgradeGuard).Write(reason); err != nil {
		_ = os.Remove(firmwarePath)
		return meta, err
	}

	// Run sysupgrade asynchronously — the device will reboot. The guard is
	// intentionally NOT removed: the marker must survive the reboot so an
	// interrupted flash is discoverable.
	go func() {
		time.Sleep(500 * time.Millisecond)
		// Firmware writes to the MTD partition take minutes on a router.
		if err := execx.Run(execx.Package, "sysupgrade", args...); err != nil {
			slog.Error("sysupgrade failed; crash guard kept", "image", firmwarePath, "err", err, "guard", s.crashGuard(firmwareUpgradeGuard).Path())
		}
	}()

	return meta, nil
}
