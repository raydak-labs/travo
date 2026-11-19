// Package device is the only way the daemon touches OpenWrt: UCI, ubus and the kernel route table.
// Adapters: cli (real router) and mock (mock mode and tests).
package device

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// Section is one UCI section. Options hold strings, lists hold list options.
type Section struct {
	Name    string              `json:"name"`
	Type    string              `json:"type"`
	Options map[string]string   `json:"options"`
	Lists   map[string][]string `json:"lists"`
	// Order names each option and list once, in `uci export` order. Empty when the
	// adapter cannot tell (the mock).
	Order []string `json:"order,omitempty"`
}

// ErrNotFound is returned when a config, section or ubus object does not exist.
var ErrNotFound = errors.New("not found")

// UCI mirrors `uci` semantics. Set is create-or-update (upsert) like the real tool.
// Staged changes are process-global per config until Commit or Revert.
type UCI interface {
	Show(ctx context.Context, config string) ([]Section, error)
	// Set writes one option; the section must exist (use AddSection first).
	Set(ctx context.Context, config, section, option, value string) error
	// SetType creates the section if missing, or updates its type.
	SetType(ctx context.Context, config, section, typ string) error
	AddToList(ctx context.Context, config, section, option, value string) error
	DeleteOption(ctx context.Context, config, section, option string) error
	DeleteSection(ctx context.Context, config, section string) error
	Commit(ctx context.Context, config string) error
	Revert(ctx context.Context, config string) error
	// Pending reports whether config has staged, uncommitted changes (`uci changes`).
	Pending(ctx context.Context, config string) (bool, error)
}

// Ubus calls an object method and returns the raw JSON reply.
type Ubus interface {
	Call(ctx context.Context, object, method string, args any) (json.RawMessage, error)
}

// PeerHandshake is one peer's latest handshake. At is the zero time when none happened.
type PeerHandshake struct {
	PublicKey string
	At        time.Time // UTC
}

// WireGuardReader reads handshake evidence only. It never returns key material.
// ErrNotFound means the interface device does not exist.
type WireGuardReader interface {
	LatestHandshakes(ctx context.Context, iface string) ([]PeerHandshake, error)
}

// WirelessScanner runs `ubus call iwinfo scan {"device":name}` under its own timeout tier.
// name is a netdev ("phy1-ap0") or a radio name ("radio0"). Read-only (device-facts §2).
type WirelessScanner interface {
	ScanWireless(ctx context.Context, name string) (json.RawMessage, error)
}

// Routes reads the KERNEL table. netifd status is not a source for route truth.
type IPv4DefaultRoute struct {
	Device  string
	Gateway string
	Source  string
	Metric  uint64
}

// DefaultRouteReader returns found=false only when the kernel has no default route.
type DefaultRouteReader interface {
	DefaultIPv4Route(context.Context) (route IPv4DefaultRoute, found bool, err error)
}

// LinkIndexReader returns the kernel link name and index snapshot.
type LinkIndexReader interface {
	LinkIndices(context.Context) (map[string]int, error)
}

type Routes interface {
	HasDefaultRoute(ctx context.Context) (bool, error)
}

// Reloader applies committed config to running services: `reload_config`
// (procd ucitrack). Never `wifi` (device-facts §3).
type Reloader interface {
	Reload(ctx context.Context) error
}

// Accounts manages the root account. Passwords travel as values and never reach argv or logs.
type Accounts interface {
	// VerifyRootPassword is false for a wrong password; unknown-user and wrong-password are one result.
	VerifyRootPassword(ctx context.Context, password string) (bool, error)
	// RootPasswordSet reads /etc/shadow. An error means "unknown": callers must not open first-run.
	RootPasswordSet(ctx context.Context) (bool, error)
	SetRootPassword(ctx context.Context, password string) error
}

// ConfigBackupper creates a configuration archive at a caller-owned destination.
type ConfigBackupper interface {
	BackupConfig(ctx context.Context, destination string) error
}

// WirelessKeyWriter stages a WiFi key without passing it in process arguments.
type WirelessKeyWriter interface {
	SetWirelessKey(ctx context.Context, section, key string) error
}

// WANPasswordWriter stages network.wan.password (PPPoE) without passing it in process
// arguments; the value never appears in logs or errors.
type WANPasswordWriter interface {
	SetWANPassword(ctx context.Context, password string) error
}

// DDNSPasswordWriter stages ddns.travo.password (the dynamic DNS token) like the WAN
// password: never in process arguments, logs or errors.
type DDNSPasswordWriter interface {
	SetDDNSPassword(ctx context.Context, password string) error
}

// Rebooter schedules a router restart and returns before it happens, so the
// caller can flush its HTTP response. The reboot itself is a terminal operation.
type Rebooter interface {
	Reboot(ctx context.Context) error
}

// BootIDReader returns the kernel's random per-boot id, which changes on every boot.
type BootIDReader interface {
	BootID(ctx context.Context) (string, error)
}

// Device is the full port.
type Device interface {
	UCI
	Ubus
	Routes
	Reloader
	Accounts
}

// InterfaceCounters is a kernel interface's cumulative byte count.
type InterfaceCounters struct {
	RXBytes uint64
	TXBytes uint64
}

// InterfaceCounterReader reads a snapshot of the kernel's interface counters.
type InterfaceCounterReader interface {
	ReadInterfaceCounters(context.Context) (map[string]InterfaceCounters, error)
}

// ReleaseVerifier verifies a detached release manifest with the given public key.
type ReleaseVerifier interface {
	VerifyRelease(ctx context.Context, manifest, signature, key string) error
}

// LeaseRenewer asks netifd to renew a logical interface's DHCP lease
// (`ubus call network.interface.<name> renew`). It never takes the interface down.
type LeaseRenewer interface {
	RenewLease(ctx context.Context, logical string) error
}

// WireGuardControl changes netifd state inside a guarded tunnel transaction.
type WireGuardControl interface {
	ReloadNetwork(context.Context) error
	SetInterfaceUp(ctx context.Context, logical string, up bool) error
	WireGuardLinkPresent(context.Context) (bool, error)
}

// Services reads and drives procd services by init script name (e.g. "adguardhome").
// Actions are live-state changes: callers guard them. Never used for `network` or `wifi`.
type Services interface {
	// ServiceRunning is true when procd reports a running instance. An absent service is false.
	ServiceRunning(ctx context.Context, name string) (bool, error)
	// ServiceAction runs one of ServiceActions through the init script.
	ServiceAction(ctx context.Context, name, action string) error
}

// MWAN3Reader reads mwan3's runtime state as raw text; internal/failover parses it.
type MWAN3Reader interface {
	// MWAN3Interfaces is `mwan3 interfaces` (tracking verdict per interface section).
	MWAN3Interfaces(ctx context.Context) (string, error)
	// MangleRules is `iptables -t mangle -S`, where mwan3 keeps its IPv4 marking rules.
	// It fails when iptables-nft cannot read the table (a foreign native nft rule in it).
	MangleRules(ctx context.Context) (string, error)
}

// NftRule names one nft rule by its chain and handle.
type NftRule struct {
	Chain  string
	Handle int
}

// MangleCleaner removes what mwan3 left in the iptables-nft mangle tables when its own
// `iptables -S` reads failed (ADR 0018). internal/failover decides what goes.
type MangleCleaner interface {
	// NftRuleset is `nft -j list ruleset <family>` (ip or ip6): every table of the family.
	NftRuleset(ctx context.Context, family string) (string, error)
	// DeleteMangle deletes the rules, then flushes and deletes the chains, of table
	// <family> mangle in one nft transaction: all of it applies or nothing does.
	DeleteMangle(ctx context.Context, family string, rules []NftRule, chains []string) error
}

// ServiceAutostart reads whether an init script starts at boot.
type ServiceAutostart interface {
	// ServiceEnabled is `/etc/init.d/<name> enabled`: exit 0 true, exit 1 false
	// (testdata/device/axt1800-25.12.6/catalog/sqm-builtin.txt); anything else is an error.
	ServiceEnabled(ctx context.Context, name string) (bool, error)
}

// ServiceActions are the init script verbs Services accepts.
var ServiceActions = map[string]bool{"start": true, "stop": true, "restart": true, "enable": true, "disable": true}

// WireGuardKeyWriter stages private or preshared keys without passing them in argv.
type WireGuardKeyWriter interface {
	SetWireGuardKey(ctx context.Context, section, option, key string) error
}

// UplinkReconnector re-runs the managed STA uplink with `ifup wwan` (netifd: network
// reload, interface down, interface up). It writes no UCI and never runs `wifi`
// (device-facts §3); the capped auto-reconnect worker is its only caller (ADR 0014).
type UplinkReconnector interface {
	ReconnectUplink(context.Context) error
}

// InterfaceRestarter restarts one netifd interface with `ifup <name>` (network reload,
// interface down, interface up). It writes no UCI and never runs `wifi` (device-facts §3);
// internal/ifrestart limits it to uplinks.
type InterfaceRestarter interface {
	RestartInterface(ctx context.Context, name string) error
}

// NeighbourReader returns the kernel IPv4 neighbour table as `ip -j -4 neigh show` JSON.
// Lockout classification maps the caller's IP to a MAC with it (ADR 0011).
type NeighbourReader interface {
	Neighbours(context.Context) (json.RawMessage, error)
}

// NetLink is one kernel netdev as sysfs describes it (`/sys/class/net/<name>`). Bus and
// Driver are the basenames of the `device/subsystem` and `device/driver` links, empty for a
// netdev without a backing device (bridge, loopback, tunnel). DevType is `DEVTYPE` from
// `uevent`, empty when absent. The uplink medium comes from these, never from the name
// (device-facts §12).
type NetLink struct {
	Name    string
	Bus     string
	Driver  string
	DevType string
}

// NetLinkReader lists the kernel netdevs. Read-only.
type NetLinkReader interface {
	NetLinks(context.Context) ([]NetLink, error)
}

// UCIReorderer moves a section to a position in its config (`uci reorder`, staged like
// any other change). mwan3 evaluates rules in config order, so failover needs it.
type UCIReorderer interface {
	Reorder(ctx context.Context, config, section string, index int) error
}

// DHCPLeaseReader reads dnsmasq's IPv4 lease file (`/tmp/dhcp.leases`, device-facts §14).
// An absent file is an empty table, not an error. Read-only.
type DHCPLeaseReader interface {
	DHCPLeases(context.Context) ([]byte, error)
}

// StationKicker deauthenticates one station from one AP (`hostapd.<ifname> del_client`).
// It changes no config and the station may rejoin at once. ErrNotFound: no such AP or station.
type StationKicker interface {
	KickStation(ctx context.Context, ifname, mac string) error
}

// LED is one kernel LED as sysfs describes it (`/sys/class/leds/<name>`). Trigger is the
// raw `trigger` file: every available trigger, the active one in brackets.
type LED struct {
	Name          string
	Brightness    int
	MaxBrightness int
	Trigger       string
}

// LEDReader lists the kernel LEDs. Read-only: LED changes go through UCI `system` led
// sections and `/etc/init.d/led restart` (internal/leds).
type LEDReader interface {
	LEDs(context.Context) ([]LED, error)
}

// Button is one devicetree `gpio-keys` node (`/sys/firmware/devicetree/base/keys/<node>`).
// Code is `linux,code`; InputType is `linux,input-type` (1 = EV_KEY when absent, 5 = EV_SW
// for a slide switch).
type Button struct {
	Node      string
	Label     string
	Code      uint32
	InputType uint32
}

// ButtonReader lists the hardware buttons from the devicetree (device-facts §8). Read-only;
// `/etc/rc.button` is generic stock scripts, not a source of truth.
type ButtonReader interface {
	Buttons(context.Context) ([]Button, error)
}
