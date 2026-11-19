// Package catalog installs, removes and switches optional OpenWrt services from one
// data table. Configuration stays in LuCI; Travo only installs and turns them on or off.
package catalog

// MiB is 2^20 bytes.
const MiB = 1 << 20

// Headroom is the free flash kept after an install (invariants §15).
const Headroom = 20 * MiB

// Entry is one catalog service. Packages install in order and remove in reverse; the
// first is the service itself, later ones its LuCI app.
type Entry struct {
	ID       string
	Packages []string
	// InitScript is the /etc/init.d name; empty for a tool without a service.
	InitScript string
	// Daemon is true when procd tracks a running instance (sqm only runs at start).
	Daemon bool
	// Size is the installed size with dependencies, rounded up to whole MiB.
	Size uint64
	// LuCIPath is the LuCI menu path for the settings, or empty.
	LuCIPath string
}

// Entries is the catalog. Package names and sizes come from the OpenWrt 25.12.6 feed:
// testdata/device/axt1800-25.12.6/catalog/apk-add-simulate.txt.
var Entries = []Entry{
	{ID: "sqm", Packages: []string{"sqm-scripts", "luci-app-sqm"}, InitScript: "sqm", Size: 1 * MiB,
		LuCIPath: "admin/network/sqm"},
	{ID: "cloudflared", Packages: []string{"cloudflared", "luci-app-cloudflared"}, InitScript: "cloudflared",
		Daemon: true, Size: 25 * MiB, LuCIPath: "admin/vpn/cloudflared"},
	{ID: "vnstat", Packages: []string{"vnstat2", "luci-app-vnstat2"}, InitScript: "vnstat", Daemon: true,
		Size: 3 * MiB, LuCIPath: "admin/status/vnstat2"},
	{ID: "watchcat", Packages: []string{"watchcat", "luci-app-watchcat"}, InitScript: "watchcat", Daemon: true,
		Size: 1 * MiB, LuCIPath: "admin/services/watchcat"},
	{ID: "speedtest", Packages: []string{"speedtestcpp"}, Size: 3 * MiB},
}

// Find returns the entry with id.
func Find(id string) (Entry, bool) {
	for _, e := range Entries {
		if e.ID == id {
			return e, true
		}
	}
	return Entry{}, false
}
