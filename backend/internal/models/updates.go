package models

// UpdateStatus is the result of the last update check.
type UpdateStatus struct {
	Firmware FirmwareUpdate `json:"firmware"`
	Packages PackageUpdates `json:"packages"`
	// CheckedAt is RFC 3339; empty until the first check has run.
	CheckedAt string `json:"checked_at"`
	// Errors lists the parts of the last check that failed (no internet,
	// index refresh); the other parts still report.
	Errors []string `json:"errors"`
}

// FirmwareUpdate compares the running OpenWrt release with the published ones.
type FirmwareUpdate struct {
	Current string `json:"current"`
	// Latest is empty when the device runs the newest release.
	Latest string `json:"latest,omitempty"`
	// Kind is "patch" for a newer release in the running series (25.12.3 →
	// 25.12.6) and "release" for a newer series (24.10 → 25.12).
	Kind string `json:"kind,omitempty" enum:"patch,release"`
	// AttendedSysupgrade reports whether LuCI's Attended Sysupgrade page,
	// which builds an image that keeps the installed packages, is installed.
	AttendedSysupgrade bool `json:"attended_sysupgrade"`
}

// PackageUpdates lists the installed packages that have a newer version.
type PackageUpdates struct {
	Manager    string           `json:"manager" enum:"apk,opkg,none"`
	Upgradable []PackageUpgrade `json:"upgradable"`
}

// PackageUpgrade is one upgradable package.
type PackageUpgrade struct {
	Name      string `json:"name"`
	Current   string `json:"current"`
	Available string `json:"available"`
}

// PackageUpgradeRequest names the packages to upgrade; empty means all.
type PackageUpgradeRequest struct {
	Packages []string `json:"packages,omitempty"`
}
