package system

import (
	"os"
	"os/exec"
	"strings"

	"github.com/openwrt-travel-gui/backend/internal/execx"
	"github.com/openwrt-travel-gui/backend/internal/models"
)

// PackageManager abstracts package install/remove operations.
type PackageManager interface {
	// Update refreshes the package index. On OpenWrt the opkg lists live in
	// /tmp (RAM), so after every reboot an install without a prior update
	// fails — callers run this best-effort before installing.
	Update() (string, error)
	Install(pkg string) (string, error)
	Remove(pkg string) (string, error)
	IsInstalled(pkg string) bool
	InstallStream(pkg string, logFn func(string)) error
	RemoveStream(pkg string, logFn func(string)) error
	// Name is "apk", "opkg" or "none".
	Name() string
	// ListUpgradable reads the installed packages with a newer version in the
	// index; run Update first.
	ListUpgradable() ([]models.PackageUpgrade, error)
	UpgradeStream(pkgs []string, logFn func(string)) error
}

// SystemProbe abstracts init.d and process checks.
type SystemProbe interface {
	HasInitScript(name string) bool
	IsRunning(initName string) bool
	Start(initName string) (string, error)
	Stop(initName string) (string, error)
	IsAutoStart(initName string) bool
	Enable(initName string) error
	Disable(initName string) error
}

// detectPackageManager checks which package manager is available.
func detectPackageManager() PackageManager {
	if _, err := exec.LookPath("apk"); err == nil {
		return &ApkPackageManager{}
	}
	if _, err := exec.LookPath("opkg"); err == nil {
		return &OpkgPackageManager{}
	}
	return &NoopPackageManager{}
}

// ApkPackageManager uses apk (OpenWrt 25.x+).
type ApkPackageManager struct{}

func (a *ApkPackageManager) Update() (string, error) {
	out, err := execx.CombinedOutput(execx.Package, "apk", "update")
	return string(out), err
}

func (a *ApkPackageManager) Install(pkg string) (string, error) {
	out, err := execx.CombinedOutput(execx.Package, "apk", "add", pkg)
	return string(out), err
}

func (a *ApkPackageManager) Remove(pkg string) (string, error) {
	out, err := execx.CombinedOutput(execx.Package, "apk", "del", pkg)
	return string(out), err
}

func (a *ApkPackageManager) IsInstalled(pkg string) bool {
	err := execx.Run(execx.Quick, "apk", "info", "-e", pkg)
	return err == nil
}

func (a *ApkPackageManager) InstallStream(pkg string, logFn func(string)) error {
	return execx.Stream(execx.Package, logFn, "apk", "add", pkg)
}

func (a *ApkPackageManager) RemoveStream(pkg string, logFn func(string)) error {
	return execx.Stream(execx.Package, logFn, "apk", "del", pkg)
}

func (a *ApkPackageManager) Name() string { return "apk" }

func (a *ApkPackageManager) ListUpgradable() ([]models.PackageUpgrade, error) {
	out, err := execx.CombinedOutput(execx.Slow, "apk", "list", "--upgradable")
	if err != nil {
		return nil, err
	}
	return parseApkUpgradable(string(out)), nil
}

func (a *ApkPackageManager) UpgradeStream(pkgs []string, logFn func(string)) error {
	return execx.Stream(execx.Package, logFn, "apk", append([]string{"upgrade"}, pkgs...)...)
}

// OpkgPackageManager uses opkg (OpenWrt <25).
type OpkgPackageManager struct{}

func (o *OpkgPackageManager) Update() (string, error) {
	out, err := execx.CombinedOutput(execx.Package, "opkg", "update")
	return string(out), err
}

func (o *OpkgPackageManager) Install(pkg string) (string, error) {
	out, err := execx.CombinedOutput(execx.Package, "opkg", "install", pkg)
	return string(out), err
}

func (o *OpkgPackageManager) Remove(pkg string) (string, error) {
	out, err := execx.CombinedOutput(execx.Package, "opkg", "remove", pkg)
	return string(out), err
}

func (o *OpkgPackageManager) IsInstalled(pkg string) bool {
	out, err := execx.CombinedOutput(execx.Quick, "opkg", "list-installed", pkg)
	return err == nil && strings.Contains(string(out), pkg)
}

func (o *OpkgPackageManager) InstallStream(pkg string, logFn func(string)) error {
	return execx.Stream(execx.Package, logFn, "opkg", "install", pkg)
}

func (o *OpkgPackageManager) RemoveStream(pkg string, logFn func(string)) error {
	return execx.Stream(execx.Package, logFn, "opkg", "remove", pkg)
}

func (o *OpkgPackageManager) Name() string { return "opkg" }

func (o *OpkgPackageManager) ListUpgradable() ([]models.PackageUpgrade, error) {
	out, err := execx.CombinedOutput(execx.Slow, "opkg", "list-upgradable")
	if err != nil {
		return nil, err
	}
	return parseOpkgUpgradable(string(out)), nil
}

func (o *OpkgPackageManager) UpgradeStream(pkgs []string, logFn func(string)) error {
	return execx.Stream(execx.Package, logFn, "opkg", append([]string{"upgrade"}, pkgs...)...)
}

// RealSystemProbe checks init.d scripts and running processes.
type RealSystemProbe struct{}

func (r *RealSystemProbe) HasInitScript(name string) bool {
	_, err := os.Stat("/etc/init.d/" + name)
	return err == nil
}

func (r *RealSystemProbe) IsRunning(initName string) bool {
	// OpenWrt init.d scripts return 0 for "running" status
	err := execx.Run(execx.Quick, "/etc/init.d/"+initName, "status")
	return err == nil
}

func (r *RealSystemProbe) Start(initName string) (string, error) {
	out, err := execx.CombinedOutput(execx.Slow, "/etc/init.d/"+initName, "start")
	return string(out), err
}

func (r *RealSystemProbe) Stop(initName string) (string, error) {
	out, err := execx.CombinedOutput(execx.Slow, "/etc/init.d/"+initName, "stop")
	return string(out), err
}

func (r *RealSystemProbe) IsAutoStart(initName string) bool {
	err := execx.Run(execx.Quick, "/etc/init.d/"+initName, "enabled")
	return err == nil
}

func (r *RealSystemProbe) Enable(initName string) error {
	return execx.Run(execx.Quick, "/etc/init.d/"+initName, "enable")
}

func (r *RealSystemProbe) Disable(initName string) error {
	return execx.Run(execx.Quick, "/etc/init.d/"+initName, "disable")
}
