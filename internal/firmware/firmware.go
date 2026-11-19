// Package firmware stages an uploaded sysupgrade image in /tmp, validates it, and starts
// the flash. Invariants §1: the guard is written FIRST, a failed guard write aborts, and
// the guard is never removed (an interrupted flash must stay visible; the keep-list carries
// it across a flash that keeps settings).
package firmware

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sync"

	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
)

const (
	// GuardName stays after the flash; reinstalling Travo clears it by name.
	GuardName = "firmware-upgrade-in-progress"
	// Phrase is what the operator types to confirm.
	Phrase = "upgrade firmware"
	// ImagePath is the only place an image is staged (tmpfs, gone after reboot).
	ImagePath = "/tmp/travo-firmware.bin"
	// MaxImageBytes caps the upload; AXT1800 images are about 20 MB.
	MaxImageBytes = 64 << 20
	// MaxRequestBytes adds the multipart boundary and part headers around the image.
	MaxRequestBytes = MaxImageBytes + 64<<10
	// Headroom stays free in RAM and /tmp beside the image so the router keeps working
	// while the image waits and sysupgrade has room for its ramfs.
	Headroom = 32 << 20
)

var (
	ErrConfirm     = errors.New("firmware: confirmation phrase does not match")
	ErrTooLarge    = errors.New("firmware: image too large")
	ErrNoRoom      = errors.New("firmware: not enough free memory for the image")
	ErrNotStaged   = errors.New("firmware: no validated image with that checksum")
	ErrInvalid     = errors.New("firmware: image failed validation")
	ErrNoBackup    = errors.New("firmware: settings cannot be kept with this image")
	ErrPending     = errors.New("firmware: upgrade already started")
	ErrUploadBusy  = errors.New("firmware: another upload is in progress")
	ErrBusy        = errors.New("firmware: another guarded operation is in progress")
	ErrUnavailable = errors.New("firmware: unavailable")
	ErrFailed      = errors.New("firmware: could not start the upgrade")
)

// Stored is what landed at ImagePath.
type Stored struct {
	Size   int64
	SHA256 string
}

// Device is the port. The daemon runs on the router, so "device" is its own /tmp and CLI.
type Device interface {
	// StoreFirmware moves (same file system) or streams src to dst and hashes it.
	StoreFirmware(ctx context.Context, src io.Reader, dst string) (Stored, error)
	RemoveFirmware(ctx context.Context, path string) error
	// ValidateFirmware is `ubus call system validate_firmware_image`: a test, never a flash.
	ValidateFirmware(ctx context.Context, path string) ([]byte, error)
	// PackageWorlds returns /etc/apk/world and /rom/etc/apk/world; os.ErrNotExist on opkg.
	PackageWorlds(ctx context.Context) (world, rom []byte, err error)
	// Sysupgrade schedules `sysupgrade [-n] path` detached and returns before it runs.
	// A terminal operation: no timeout tier.
	Sysupgrade(ctx context.Context, path string, keepSettings bool) error
	Call(ctx context.Context, object, method string, args any) (json.RawMessage, error)
}

// Guards is the subset of guard.Dir the service needs.
type Guards interface {
	AcquireExclusive(name string, conflicts func(held []guard.Info) error) error
	Stale() ([]guard.Info, error)
}

// Image is a staged, validated upload.
type Image struct {
	SHA256     string     `json:"sha256"`
	Size       int64      `json:"size"`
	Board      string     `json:"board" doc:"This router's board_name; fwtool_device_match says whether the image is for it"`
	Model      string     `json:"model"`
	Validation Validation `json:"validation"`
	// PackagesKnown is false when the user-installed packages cannot be derived (opkg, no /rom).
	PackagesKnown bool     `json:"packages_known"`
	Dropped       []string `json:"dropped_packages" nullable:"false" doc:"Packages installed after the running image; the new image does not keep them"`
}

// Service serialises upload and apply. The staged image lives in memory only: a daemon
// restart forgets it and the operator uploads again.
type Service struct {
	Device  Device
	Guards  Guards
	Stopped func() bool
	mu      sync.Mutex
	staged  *Image
	// uploading holds the one upload slot from Prepare until FinishUpload, so a second
	// request cannot pass the room check against the same free memory.
	uploading bool
}

// Prepare runs before the body is read, with the request size: it refuses a pending
// flash, a second concurrent upload or an oversized request, drops the previous image and
// checks RAM and /tmp for size plus Headroom. On success it holds the upload slot until
// FinishUpload, which the caller must call once the request ends.
func (s *Service) Prepare(ctx context.Context, size int64) error {
	if size > MaxRequestBytes {
		return ErrTooLarge
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.uploading {
		return ErrUploadBusy
	}
	if err := s.refuseOwn(); err != nil {
		return err
	}
	s.staged = nil
	if err := s.Device.RemoveFirmware(ctx, ImagePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	raw, err := s.Device.Call(ctx, "system", "info", nil)
	if err != nil {
		return err
	}
	info, err := facts.ParseSystemInfo(raw)
	if err != nil {
		return err
	}
	need := uint64(size) + Headroom
	if info.MemAvailable < need || info.TmpAvail < need {
		return ErrNoRoom
	}
	s.uploading = true
	return nil
}

// FinishUpload frees the upload slot Prepare took; the body is staged or discarded.
func (s *Service) FinishUpload() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.uploading = false
}

// Upload stores, validates and describes the image. An unparsable validation removes it.
func (s *Service) Upload(ctx context.Context, src io.Reader) (Image, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refuseOwn(); err != nil {
		return Image{}, err
	}
	// The HTTP layer bounds the body; src stays unwrapped so the device can rename the file.
	stored, err := s.Device.StoreFirmware(ctx, src, ImagePath)
	if err == nil && stored.Size > MaxImageBytes {
		err = ErrTooLarge
	}
	var img Image
	if err == nil {
		img, err = s.describe(ctx, stored)
	}
	if err != nil {
		_ = s.Device.RemoveFirmware(ctx, ImagePath)
		s.staged = nil // the file is gone; never let Apply flash a missing image
		return Image{}, err
	}
	s.staged = &img
	return img, nil
}

func (s *Service) describe(ctx context.Context, stored Stored) (Image, error) {
	raw, err := s.Device.ValidateFirmware(ctx, ImagePath)
	if err != nil {
		return Image{}, err
	}
	v, err := ParseValidation(raw)
	if err != nil {
		return Image{}, err
	}
	img := Image{SHA256: stored.SHA256, Size: stored.Size, Validation: v, Dropped: []string{}}
	if raw, err := s.Device.Call(ctx, "system", "board", nil); err == nil {
		if b, err := facts.ParseBoard(raw); err == nil {
			img.Board, img.Model = b.BoardName, b.Model
		}
	}
	// A failed read only loses the list: the UI then warns that packages may be dropped.
	if world, rom, err := s.Device.PackageWorlds(ctx); err == nil {
		img.PackagesKnown = true
		if d := DroppedPackages(world, rom); d != nil {
			img.Dropped = d
		}
	}
	return img, nil
}

// Status is what the page needs before an upload.
type Status struct {
	Staged *Image `json:"staged,omitempty" doc:"Validated image waiting in /tmp; absent when none"`
	// MaxImageBytes is the largest image that fits now (RAM and /tmp minus Headroom, a
	// staged image counted as free since an upload replaces it), at most 64 MiB.
	MaxImageBytes int64 `json:"max_image_bytes"`
	Pending       bool  `json:"pending" doc:"A flash was started; the guard is held"`
}

// Status reports the staged image, the room for a new one and whether a flash started.
func (s *Service) Status(ctx context.Context) (Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var st Status
	if s.staged != nil {
		img := *s.staged
		st.Staged = &img
	}
	if err := s.refuseOwn(); errors.Is(err, ErrPending) {
		st.Pending = true
	} else if err != nil {
		return Status{}, err
	}
	raw, err := s.Device.Call(ctx, "system", "info", nil)
	if err != nil {
		return Status{}, err
	}
	info, err := facts.ParseSystemInfo(raw)
	if err != nil {
		return Status{}, err
	}
	free := min(info.MemAvailable, info.TmpAvail)
	if st.Staged != nil {
		free += uint64(st.Staged.Size)
	}
	if free > Headroom {
		st.MaxImageBytes = int64(min(free-Headroom, MaxImageBytes))
	}
	return st, nil
}

// Apply checks the phrase, the staged image and every guard, writes its guard, then
// schedules the flash. The guard is never released, also when scheduling fails.
func (s *Service) Apply(ctx context.Context, sha string, keepSettings bool, phrase string) error {
	if phrase != Phrase {
		return ErrConfirm
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Stopped != nil && s.Stopped() {
		return ErrUnavailable
	}
	switch {
	case s.staged == nil || s.staged.SHA256 != sha:
		return ErrNotStaged
	case !s.staged.Validation.Valid:
		return ErrInvalid
	case keepSettings && !s.staged.Validation.AllowBackup:
		return ErrNoBackup
	}
	// The scan and our guard are one step: a refusal writes nothing, and once our guard is
	// written no other guarded operation can start. From here on it is never removed.
	err := s.Guards.AcquireExclusive(GuardName, refuseOthers)
	switch {
	case errors.Is(err, ErrBusy):
		return ErrBusy
	case errors.Is(err, guard.ErrActive):
		return ErrPending
	case err != nil:
		return errors.Join(ErrFailed, err) // never degrade to unguarded
	}
	if err := s.Device.Sysupgrade(ctx, ImagePath, keepSettings); err != nil {
		return errors.Join(ErrFailed, err)
	}
	return nil
}

// refuseOwn fails while our guard exists: the image may already be flashing.
func (s *Service) refuseOwn() error {
	held, err := s.Guards.Stale()
	if err != nil {
		return errors.Join(ErrFailed, err)
	}
	for _, g := range held {
		if g.Name == GuardName {
			return ErrPending
		}
	}
	return nil
}

func refuseOthers(held []guard.Info) error {
	if len(held) > 0 {
		return ErrBusy
	}
	return nil
}
