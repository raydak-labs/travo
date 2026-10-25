package system

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

// From the GL-AXT1800 (OpenWrt 25.12.3, apk-tools 3.0.5).
const apkUpgradable = `adguardhome-0.107.79-r1 aarch64_cortex-a53 {feeds/packages/feeds/packages/net/adguardhome} (GPL-3.0-only) [upgradable from: adguardhome-0.107.74-r1]
libblobmsg-json20260213-2026.06.19~7dd12784-r1 aarch64_cortex-a53 {feeds/base/libs/libubox} (ISC) [upgradable from: libblobmsg-json20260213-2026.03.13~81563384-r2]
hostapd-common-2025.08.26~ca266cc2-r6 aarch64_cortex-a53 {feeds/base/network/services/hostapd} (BSD-3-Clause) [upgradable from: hostapd-common-2025.08.26~ca266cc2-r1]
`

func TestParseApkUpgradable(t *testing.T) {
	got := parseApkUpgradable(apkUpgradable)
	want := []models.PackageUpgrade{
		{Name: "adguardhome", Current: "0.107.74-r1", Available: "0.107.79-r1"},
		{Name: "libblobmsg-json20260213", Current: "2026.03.13~81563384-r2", Available: "2026.06.19~7dd12784-r1"},
		{Name: "hostapd-common", Current: "2025.08.26~ca266cc2-r1", Available: "2025.08.26~ca266cc2-r6"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestParseOpkgUpgradable(t *testing.T) {
	got := parseOpkgUpgradable("dnsmasq - 2.90-r3 - 2.90-r4\nluci-base - 25.001 - 25.044\n")
	want := []models.PackageUpgrade{
		{Name: "dnsmasq", Current: "2.90-r3", Available: "2.90-r4"},
		{Name: "luci-base", Current: "25.001", Available: "25.044"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

const versionsFeed = `{"stable_version":"25.12.6","oldstable_version":"24.10.8",
"versions_list":["25.12.6","25.12.5","25.12.0-rc1","25.12.0","24.10.8","24.10.7","24.10.0"]}`

func TestNewestFirmware(t *testing.T) {
	for _, tc := range []struct {
		current, latest, kind string
	}{
		{"25.12.3", "25.12.6", "patch"},     // newer in the running series first
		{"25.12.6", "", ""},                 // up to date
		{"24.10.8", "25.12.6", "release"},   // series is done: offer the stable one
		{"24.10.2", "24.10.8", "patch"},     // a patch beats a jump to a new series
		{"25.12.0-rc1", "25.12.6", "patch"}, // a release candidate is older than its release
		{"SNAPSHOT", "", ""},                // snapshots have no release to compare with
	} {
		latest, kind, err := newestFirmware(tc.current, []byte(versionsFeed))
		if err != nil {
			t.Fatalf("%s: %v", tc.current, err)
		}
		if latest != tc.latest || kind != tc.kind {
			t.Errorf("%s: got %q %q, want %q %q", tc.current, latest, kind, tc.latest, tc.kind)
		}
	}
}

type fakeUpgrader struct {
	MockPackageManager
	list     []models.PackageUpgrade
	upgraded []string
}

func (f *fakeUpgrader) Name() string                                     { return "apk" }
func (f *fakeUpgrader) ListUpgradable() ([]models.PackageUpgrade, error) { return f.list, nil }
func (f *fakeUpgrader) UpgradeStream(pkgs []string, logFn func(string)) error {
	f.upgraded = append(f.upgraded, pkgs...)
	f.list = nil
	return nil
}

func newTestUpdates(t *testing.T, pkg PackageManager, feedErr error) *UpdateService {
	t.Helper()
	u := NewUpdateService(pkg,
		func() (string, error) { return "25.12.3", nil },
		func() ([]byte, error) { return []byte(versionsFeed), feedErr })
	u.guardDir = t.TempDir()
	return u
}

func TestCheckReportsFirmwareAndPackages(t *testing.T) {
	pkg := &fakeUpgrader{list: []models.PackageUpgrade{{Name: "htop", Current: "3.4.1-r1", Available: "3.5.1-r1"}}}
	u := newTestUpdates(t, pkg, nil)

	st := u.Check()
	if st.Firmware.Latest != "25.12.6" || st.Firmware.Kind != "patch" {
		t.Errorf("firmware = %+v", st.Firmware)
	}
	if st.Packages.Manager != "apk" || len(st.Packages.Upgradable) != 1 {
		t.Errorf("packages = %+v", st.Packages)
	}
	if st.CheckedAt == "" || len(st.Errors) != 0 {
		t.Errorf("checked_at %q, errors %v", st.CheckedAt, st.Errors)
	}
	if got := u.Status(); got.CheckedAt != st.CheckedAt {
		t.Error("Status does not return the last check")
	}
}

// Offline the feed fails, but the package part is still reported.
func TestCheckKeepsTheOtherPartWhenOneFails(t *testing.T) {
	pkg := &fakeUpgrader{list: []models.PackageUpgrade{{Name: "htop"}}}
	u := newTestUpdates(t, pkg, errors.New("no route to host"))

	st := u.Check()
	if len(st.Errors) != 1 || !strings.Contains(st.Errors[0], "no route to host") {
		t.Errorf("errors = %v", st.Errors)
	}
	if st.Firmware.Current != "25.12.3" || len(st.Packages.Upgradable) != 1 {
		t.Errorf("status = %+v", st)
	}
}

// The names reach apk/opkg as arguments: anything not in the last check's
// list (a flag such as --force-broken-world, a package that is not
// installed) is refused before a command runs.
func TestUpgradeOnlyAcceptsListedPackages(t *testing.T) {
	pkg := &fakeUpgrader{list: []models.PackageUpgrade{{Name: "htop"}, {Name: "curl"}}}
	u := newTestUpdates(t, pkg, nil)
	u.Check()

	if err := u.Upgrade([]string{"--force-broken-world"}, func(string) {}); err == nil {
		t.Fatal("upgrade accepted a name that is not upgradable")
	}
	if len(pkg.upgraded) != 0 {
		t.Fatalf("upgraded %v after refusing", pkg.upgraded)
	}
	if err := u.Upgrade([]string{"htop"}, func(string) {}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(pkg.upgraded, []string{"htop"}) {
		t.Errorf("upgraded %v, want [htop]", pkg.upgraded)
	}
	if len(u.Status().Packages.Upgradable) != 0 {
		t.Error("the package list was not refreshed after the upgrade")
	}
}

// An empty selection upgrades everything the last check listed, and the
// crash guard is gone once the upgrade succeeded.
func TestUpgradeAllClearsTheGuard(t *testing.T) {
	pkg := &fakeUpgrader{list: []models.PackageUpgrade{{Name: "htop"}, {Name: "curl"}}}
	u := newTestUpdates(t, pkg, nil)
	u.Check()

	if err := u.Upgrade(nil, func(string) {}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(pkg.upgraded, []string{"htop", "curl"}) {
		t.Errorf("upgraded %v", pkg.upgraded)
	}
	if u.guard().Present() {
		t.Error("crash guard left behind after a successful upgrade")
	}
}
