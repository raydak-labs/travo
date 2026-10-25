package system

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/guard"
	"github.com/openwrt-travel-gui/backend/internal/models"
)

// versionsFeedURL lists every OpenWrt release and names the stable one.
const versionsFeedURL = "https://downloads.openwrt.org/.versions.json"

// attendedSysupgradeMenu exists when luci-app-attendedsysupgrade is installed.
const attendedSysupgradeMenu = "/usr/share/luci/menu.d/luci-app-attendedsysupgrade.json"

// UpdateService finds OpenWrt releases newer than the running one and
// upgradable packages, and upgrades packages. A check needs the internet (the
// release feed, the package index), so it runs in the background and Status
// serves the last result. Flashing a new release stays with LuCI.
type UpdateService struct {
	pkg     PackageManager
	version func() (string, error)
	feed    func() ([]byte, error)
	// guardDir overrides the crash-guard directory (tests).
	guardDir string

	mu     sync.Mutex
	status models.UpdateStatus
	// opMu serialises checks and upgrades: an upgrade must not race the
	// index refresh of a check, and its package list must be current.
	opMu sync.Mutex
}

// NewUpdateService takes the package manager, a reader of the running
// OpenWrt version, and the release feed; a nil package manager is the one the
// device has, and a nil feed fetches versionsFeedURL.
func NewUpdateService(pkg PackageManager, version func() (string, error), feed func() ([]byte, error)) *UpdateService {
	if pkg == nil {
		pkg = detectPackageManager()
	}
	if feed == nil {
		feed = fetchVersionsFeed
	}
	u := &UpdateService{pkg: pkg, version: version, feed: feed}
	u.status = models.UpdateStatus{
		Packages: models.PackageUpdates{Manager: pkg.Name()},
		Errors:   []string{},
	}
	return u
}

// guard is the package-operation crash guard ServiceManager installs use too
// (ADR 0003 §2): an interrupted upgrade blocks every later package operation
// until the explicit redeploy retry path clears it.
func (u *UpdateService) guard() guard.Guard { return guard.In(u.guardDir, "pkg-install") }

// Status returns the result of the last check.
func (u *UpdateService) Status() models.UpdateStatus {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.status
}

// Check looks for a newer release and upgradable packages now. A part that
// fails (offline, index refresh) is reported in Errors; the other still is.
func (u *UpdateService) Check() models.UpdateStatus {
	u.opMu.Lock()
	defer u.opMu.Unlock()

	st := models.UpdateStatus{
		Firmware: models.FirmwareUpdate{AttendedSysupgrade: fileExists(attendedSysupgradeMenu)},
		Packages: models.PackageUpdates{Manager: u.pkg.Name()},
		Errors:   []string{},
	}
	if current, err := u.version(); err != nil {
		st.Errors = append(st.Errors, "reading the OpenWrt version: "+err.Error())
	} else {
		st.Firmware.Current = current
		if feed, err := u.feed(); err != nil {
			st.Errors = append(st.Errors, "fetching the OpenWrt release list: "+err.Error())
		} else if st.Firmware.Latest, st.Firmware.Kind, err = newestFirmware(current, feed); err != nil {
			st.Errors = append(st.Errors, "reading the OpenWrt release list: "+err.Error())
		}
	}
	if st.Packages.Manager != "none" {
		if out, err := u.pkg.Update(); err != nil {
			st.Errors = append(st.Errors, "refreshing the package index: "+firstLine(out, err))
		}
		list, err := u.pkg.ListUpgradable()
		if err != nil {
			st.Errors = append(st.Errors, "listing upgradable packages: "+err.Error())
		}
		st.Packages.Upgradable = list
	}
	st.CheckedAt = time.Now().UTC().Format(time.RFC3339)

	u.mu.Lock()
	u.status = st
	u.mu.Unlock()
	return st
}

// Upgrade upgrades pkgs, or every package the last check listed when pkgs is
// empty. Only names from that list are accepted: they reach apk or opkg as
// arguments. The package list is refreshed afterwards.
func (u *UpdateService) Upgrade(pkgs []string, logFn func(string)) error {
	u.opMu.Lock()
	defer u.opMu.Unlock()

	listed := u.Status().Packages.Upgradable
	if len(pkgs) == 0 {
		for _, p := range listed {
			pkgs = append(pkgs, p.Name)
		}
		if len(pkgs) == 0 {
			return errors.New("no upgradable packages; run a check first")
		}
	}
	for _, name := range pkgs {
		if !slices.ContainsFunc(listed, func(p models.PackageUpgrade) bool { return p.Name == name }) {
			return fmt.Errorf("%q is not in the list of upgradable packages", name)
		}
	}

	if err := u.guard().Create("upgrade " + strings.Join(pkgs, " ")); err != nil {
		return fmt.Errorf("another package operation is in progress or was interrupted: %w", err)
	}
	logFn(fmt.Sprintf("Upgrading %d package(s)…", len(pkgs)))
	if err := u.pkg.UpgradeStream(pkgs, logFn); err != nil {
		return err // the guard stays: the upgrade may be half applied
	}
	if err := u.guard().Clear(); err != nil {
		return err
	}

	list, err := u.pkg.ListUpgradable()
	u.mu.Lock()
	if err == nil {
		u.status.Packages.Upgradable = list
	}
	u.mu.Unlock()
	return nil
}

// newestFirmware returns the release to offer for current: the newest final
// release of the running series ("patch"), else the stable release when it is
// newer ("release"), else nothing. A version that is not a release (SNAPSHOT)
// gets nothing.
func newestFirmware(current string, feed []byte) (latest, kind string, err error) {
	var f struct {
		Stable   string   `json:"stable_version"`
		Versions []string `json:"versions_list"`
	}
	if err := json.Unmarshal(feed, &f); err != nil {
		return "", "", err
	}
	cur, ok := parseRelease(current)
	if !ok {
		return "", "", nil
	}
	var best release
	for _, v := range f.Versions {
		r, ok := parseRelease(v)
		if ok && r.rc == 0 && r.series() == cur.series() && cur.less(r) && best.less(r) {
			best, latest = r, v
		}
	}
	if latest != "" {
		return latest, "patch", nil
	}
	if r, ok := parseRelease(f.Stable); ok && cur.less(r) {
		return f.Stable, "release", nil
	}
	return "", "", nil
}

// release is a parsed OpenWrt version such as 25.12.3 or 25.12.0-rc1.
type release struct{ major, minor, patch, rc int }

func parseRelease(v string) (release, bool) {
	base, rc, hasRC := strings.Cut(v, "-rc")
	parts := strings.Split(base, ".")
	if len(parts) != 3 {
		return release{}, false
	}
	var nums [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return release{}, false
		}
		nums[i] = n
	}
	r := release{major: nums[0], minor: nums[1], patch: nums[2]}
	if hasRC {
		n, err := strconv.Atoi(rc)
		if err != nil || n < 1 {
			return release{}, false
		}
		r.rc = n
	}
	return r, true
}

func (r release) series() [2]int { return [2]int{r.major, r.minor} }

// less orders releases; a release candidate comes before its final release.
func (r release) less(o release) bool {
	if r.major != o.major {
		return r.major < o.major
	}
	if r.minor != o.minor {
		return r.minor < o.minor
	}
	if r.patch != o.patch {
		return r.patch < o.patch
	}
	return r.rc != 0 && (o.rc == 0 || r.rc < o.rc)
}

func fetchVersionsFeed() ([]byte, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(versionsFeedURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", versionsFeedURL, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

// parseApkUpgradable reads `apk list --upgradable`:
//
//	adguardhome-0.107.79-r1 aarch64_cortex-a53 {…} (…) [upgradable from: adguardhome-0.107.74-r1]
//
// Both tokens are name-version; the name is their common prefix up to the
// last "-" that a version digit follows (names contain dashes and digits).
func parseApkUpgradable(out string) []models.PackageUpgrade {
	list := []models.PackageUpgrade{}
	for line := range strings.Lines(out) {
		newTok, _, _ := strings.Cut(line, " ")
		_, from, ok := strings.Cut(line, "[upgradable from: ")
		if !ok {
			continue
		}
		oldTok, _, _ := strings.Cut(from, "]")
		n := 0
		for n < len(newTok) && n < len(oldTok) && newTok[n] == oldTok[n] {
			n++
		}
		cut := -1
		for i := n - 1; i > 0; i-- {
			if newTok[i] == '-' && i+1 < len(newTok) && i+1 < len(oldTok) &&
				isDigit(newTok[i+1]) && isDigit(oldTok[i+1]) {
				cut = i
				break
			}
		}
		if cut < 0 {
			continue
		}
		list = append(list, models.PackageUpgrade{
			Name: newTok[:cut], Current: oldTok[cut+1:], Available: newTok[cut+1:],
		})
	}
	return list
}

// parseOpkgUpgradable reads `opkg list-upgradable`: "name - current - available".
func parseOpkgUpgradable(out string) []models.PackageUpgrade {
	list := []models.PackageUpgrade{}
	for line := range strings.Lines(out) {
		f := strings.Split(strings.TrimSpace(line), " - ")
		if len(f) == 3 {
			list = append(list, models.PackageUpgrade{Name: f[0], Current: f[1], Available: f[2]})
		}
	}
	return list
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// firstLine is the first line of a command's output, or the error.
func firstLine(out string, err error) string {
	if line, _, _ := strings.Cut(strings.TrimSpace(out), "\n"); line != "" {
		return line
	}
	return err.Error()
}
