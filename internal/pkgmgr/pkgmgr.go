// Package pkgmgr is the PackageManager port with apk (OpenWrt 25.x) and opkg (older) adapters.
package pkgmgr

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrNotInstalled means the package is absent. A failed query is a different error and
// must never be read as "absent" (invariants §13).
var ErrNotInstalled = errors.New("package not installed")

// ErrNoManager means neither apk nor opkg was found.
var ErrNoManager = errors.New("no supported package manager (apk or opkg) found")

// Package is an installed package.
type Package struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// PackageManager is the port. Install and Remove are live-state operations: callers guard them.
type PackageManager interface {
	Name() string
	// Update refreshes package lists; they live in /tmp and vanish on reboot. Best effort for callers.
	Update(ctx context.Context) error
	Install(ctx context.Context, pkg string) error
	// InstallFile installs a local package file whose integrity the CALLER verified
	// (apk skips its signature check: feed packages are signed only through the index).
	InstallFile(ctx context.Context, path string) error
	Remove(ctx context.Context, pkg string) error
	Installed(ctx context.Context) ([]Package, error)
	// Version returns ErrNotInstalled when absent.
	Version(ctx context.Context, pkg string) (string, error)
}

// Detect picks the manager at runtime; nothing is hardcoded to a release.
// lookPath is exec.LookPath in production.
func Detect(lookPath func(string) (string, error), r Runner) (PackageManager, error) {
	if _, err := lookPath("apk"); err == nil {
		return &APK{Run: r}, nil
	}
	if _, err := lookPath("opkg"); err == nil {
		return &OPKG{Run: r}, nil
	}
	return nil, ErrNoManager
}

// DetectSystem is Detect against the real PATH.
func DetectSystem(r Runner) (PackageManager, error) { return Detect(exec.LookPath, r) }

// validFile accepts an absolute path that cannot be read as an option.
func validFile(path string) error {
	if !strings.HasPrefix(path, "/") || strings.ContainsAny(path, "\x00\n") {
		return fmt.Errorf("invalid package file %q", path)
	}
	return nil
}

func validPkg(name string) error {
	if name == "" {
		return errors.New("empty package name")
	}
	for _, c := range name {
		ok := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.' || c == '+'
		if !ok {
			return fmt.Errorf("invalid package name %q", name)
		}
	}
	if name[0] == '-' {
		return fmt.Errorf("invalid package name %q", name)
	}
	return nil
}
