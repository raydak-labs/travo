package adguard

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
)

// The offline bundle pin is stamped by the release build (-ldflags -X, ADR 0017): the
// SHA-256 and file name of the adguardhome package the release ships beside the tarball.
// The signed release covers the binary, so the pin is the bundle's trust anchor; apk
// cannot check a feed package's signature on its own (it is signed only via the index).
var (
	BundleSHA256 = ""
	BundleName   = ""
)

// BundlePath is where install.sh puts a verified bundle: RAM, root-only, gone after reboot.
const BundlePath = "/tmp/travo-greenfield-bundle/adguardhome.apk"

// Bundle is the local package an offline first install may use.
type Bundle struct {
	Path   string
	SHA256 string // hex; empty disables the offline path
}

// ErrBundleMismatch means a bundle file exists but is not the one this release pinned.
var ErrBundleMismatch = errors.New("AdGuard Home bundle does not match this release")

// Verify returns nil when the file at Path has the pinned hash, os.ErrNotExist when there
// is no bundle (or no pin), and ErrBundleMismatch for any other file.
func (b Bundle) Verify() error {
	if b.SHA256 == "" || b.Path == "" {
		return os.ErrNotExist
	}
	f, err := os.Open(b.Path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != b.SHA256 {
		return fmt.Errorf("%w: sha256 %s, want %s", ErrBundleMismatch, got, b.SHA256)
	}
	return nil
}

// bundled reports a bundle matching the release pin. A mismatch is logged and the online
// path is used: a stray or stale file must never be installed (ADR 0017).
func (s *Service) bundled() bool {
	err := s.Bundle.Verify()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		slog.Warn("adguard: ignoring the offline bundle", "file", s.Bundle.Path, "err", err)
	}
	return err == nil
}

func (s *Service) installPackage(ctx context.Context, bundled bool) error {
	if !bundled {
		return s.Packages.Install(ctx, Package)
	}
	// Verified again right before the install: the check above ran before the flash check.
	if err := s.Bundle.Verify(); err != nil {
		return err
	}
	return s.Packages.InstallFile(ctx, s.Bundle.Path)
}
