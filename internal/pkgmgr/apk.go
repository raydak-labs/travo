package pkgmgr

import (
	"context"
	"encoding/json"
	"fmt"
)

// APK drives apk-tools 3 (OpenWrt 25.x).
type APK struct{ Run Runner }

func (*APK) Name() string { return "apk" }

func (a *APK) Update(ctx context.Context) error {
	_, _, err := run(ctx, a.Run, TierUpdate, "apk", "update")
	return err
}

func (a *APK) Install(ctx context.Context, pkg string) error {
	if err := validPkg(pkg); err != nil {
		return err
	}
	_, _, err := run(ctx, a.Run, TierInstall, "apk", "add", pkg)
	return err
}

func (a *APK) InstallFile(ctx context.Context, path string) error {
	if err := validFile(path); err != nil {
		return err
	}
	_, _, err := run(ctx, a.Run, TierInstall, "apk", "add", "--allow-untrusted", path)
	return err
}

func (a *APK) Remove(ctx context.Context, pkg string) error {
	if err := validPkg(pkg); err != nil {
		return err
	}
	_, _, err := run(ctx, a.Run, TierInstall, "apk", "del", pkg)
	return err
}

func (a *APK) Installed(ctx context.Context) ([]Package, error) {
	out, _, err := run(ctx, a.Run, TierQuery, "apk", "query", "--installed", "--fields", "name,version", "--format", "json", "*")
	if err != nil {
		return nil, err
	}
	return ParseAPKQuery(out)
}

func (a *APK) Version(ctx context.Context, pkg string) (string, error) {
	if err := validPkg(pkg); err != nil {
		return "", err
	}
	pkgs, err := a.Installed(ctx)
	if err != nil {
		return "", err
	}
	return find(pkgs, pkg)
}

// ParseAPKQuery parses `apk query --format json` output.
func ParseAPKQuery(out string) ([]Package, error) {
	pkgs := []Package{}
	if err := json.Unmarshal([]byte(out), &pkgs); err != nil {
		return nil, fmt.Errorf("apk query: %w", err)
	}
	for _, p := range pkgs {
		if p.Name == "" {
			return nil, fmt.Errorf("apk query: entry without name")
		}
	}
	return pkgs, nil
}

func find(pkgs []Package, name string) (string, error) {
	for _, p := range pkgs {
		if p.Name == name {
			return p.Version, nil
		}
	}
	return "", ErrNotInstalled
}
