package pkgmgr

import (
	"context"
	"fmt"
	"strings"
)

// OPKG drives opkg (OpenWrt 24.10 and older).
type OPKG struct{ Run Runner }

func (*OPKG) Name() string { return "opkg" }

func (o *OPKG) Update(ctx context.Context) error {
	_, _, err := run(ctx, o.Run, TierUpdate, "opkg", "update")
	return err
}

func (o *OPKG) Install(ctx context.Context, pkg string) error {
	if err := validPkg(pkg); err != nil {
		return err
	}
	_, _, err := run(ctx, o.Run, TierInstall, "opkg", "install", pkg)
	return err
}

func (o *OPKG) InstallFile(ctx context.Context, path string) error {
	if err := validFile(path); err != nil {
		return err
	}
	_, _, err := run(ctx, o.Run, TierInstall, "opkg", "install", path)
	return err
}

func (o *OPKG) Remove(ctx context.Context, pkg string) error {
	if err := validPkg(pkg); err != nil {
		return err
	}
	_, _, err := run(ctx, o.Run, TierInstall, "opkg", "remove", pkg)
	return err
}

func (o *OPKG) Installed(ctx context.Context) ([]Package, error) {
	out, _, err := run(ctx, o.Run, TierQuery, "opkg", "list-installed")
	if err != nil {
		return nil, err
	}
	return ParseOPKGList(out)
}

func (o *OPKG) Version(ctx context.Context, pkg string) (string, error) {
	if err := validPkg(pkg); err != nil {
		return "", err
	}
	pkgs, err := o.Installed(ctx)
	if err != nil {
		return "", err
	}
	return find(pkgs, pkg)
}

// ParseOPKGList parses `opkg list-installed` lines of the form `name - version`.
func ParseOPKGList(out string) ([]Package, error) {
	pkgs := []Package{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name, ver, ok := strings.Cut(line, " - ")
		if !ok || name == "" || ver == "" {
			return nil, fmt.Errorf("opkg list-installed: malformed line %q", line)
		}
		pkgs = append(pkgs, Package{Name: name, Version: strings.TrimSpace(ver)})
	}
	return pkgs, nil
}
