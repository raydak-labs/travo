// Package api registers every HTTP route; nothing is routed outside New.
// Default-deny: a route is protected unless it is registered with public=true.
package api

import (
	"context"
	"net/http"
	"net/netip"
	"reflect"
	"sync"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"github.com/blackdark-org/travo-greenfield/internal/auth"
	"github.com/blackdark-org/travo-greenfield/internal/connectivity"
	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/reboot"
	"github.com/blackdark-org/travo-greenfield/internal/throughput"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/vpnprofiles"
	"github.com/blackdark-org/travo-greenfield/internal/vpnstatus"
	"github.com/blackdark-org/travo-greenfield/internal/wifi"
)

// Txns is the slice of txn.Manager the API needs.
type Txns interface {
	Get(id string) (txn.Status, error)
	Confirm(id string) (txn.Status, error)
	Rollback(id string) (txn.Status, error)
}

// Rebooter is the confirmed system restart operation exposed by reboot.Service.
type Rebooter interface {
	Reboot(context.Context) (reboot.Accepted, error)
}

// ThroughputReader reads the latest sampled interface rates.
type ThroughputReader interface {
	Latest() ([]throughput.Rate, bool)
}

type ConnectivityReader interface {
	Status(context.Context) connectivity.Status
}

type VPNTunnelSetter interface {
	Set(context.Context, bool) (txn.Status, error)
}

type VPNStatusReader interface {
	Status(context.Context) (vpnstatus.TunnelStatus, error)
}

// WiFiService is the read-only wireless summary and site-survey service.
type WiFiJoiner interface {
	Join(context.Context, wifi.JoinRequest) (txn.Status, error)
}

// WiFiAPEditor lists and edits configured access points through guarded transactions.
type WiFiAPEditor interface {
	AccessPoints(context.Context) ([]wifi.APConfig, error)
	UpdateAP(context.Context, wifi.APRequest) (txn.Status, error)
}

// WiFiGuestEditor reads and applies the guest AP through guarded transactions.
type WiFiGuestEditor interface {
	Guest(context.Context) (wifi.GuestConfig, error)
	SetGuest(context.Context, wifi.GuestRequest) (txn.Status, error)
}

// WiFiMACEditor reads and applies the uplink STA MAC policy.
type WiFiMACEditor interface {
	UplinkMAC(context.Context) (wifi.MACPolicy, error)
	SetUplinkMAC(context.Context, wifi.MACRequest) (txn.Status, error)
}

type WiFiService interface {
	Summary(context.Context) (wifi.Summary, error)
	Scan(context.Context) (wifi.ScanResult, error)
}

// Deps are everything the routes need; the daemon builds them, tests fake them.
type Deps struct {
	VPNProfiles *vpnprofiles.Service
	VPNStatus   VPNStatusReader
	VPNTunnel   VPNTunnelSetter
	VPNSplit    VPNSplit
	// VPNKillSwitch is nil when the device cannot change firewall config.
	VPNKillSwitch VPNKillSwitch
	VPNDNS        VPNDNS
	// DaemonContext stops live streams and their producers before HTTP shutdown drains.
	DaemonContext context.Context
	UpdateStage   UpdateStager
	UpdateApply   UpdateApplier
	Rebooter      Rebooter
	Updates       UpdateChecker
	Version       string
	// BootID returns the current opaque boot id, or empty when unknown.
	BootID   func(context.Context) string
	Accounts device.Accounts
	// Ubus feeds read-only system facts; nil answers 503 device_unavailable.
	Ubus       device.Ubus
	Backup     device.ConfigBackupper
	Throughput ThroughputReader
	// StatsHistory is the persisted dashboard history; nil answers 503.
	StatsHistory StatsHistory
	Connectivity ConnectivityReader
	// Captive is the portal DNS bypass; nil answers 503 captive_unavailable.
	Captive      CaptiveAssist
	CaptiveRenew CaptiveRenewer
	// CaptiveAccept is the auto-accept walker; nil answers 503 captive_unavailable.
	CaptiveAccept CaptiveAcceptor
	WiFi          WiFiService
	WiFiJoin      WiFiJoiner
	WiFiProfiles  WiFiProfileService
	// AdGuard is nil when the daemon cannot supervise the package (no service port).
	AdGuard AdGuard
	// Tailscale is nil when the daemon cannot supervise the package (no service port).
	Tailscale Tailscale
	Txns      Txns
	Sessions  *auth.Store
	// Tokens are the automation API tokens; nil disables bearer auth and answers 503 on management.
	Tokens  APITokens
	Limiter *auth.RateLimiter
	Clock   auth.Clock
	// WiFiReconnect is the capped auto-reconnect counter; nil answers 503 device_unavailable.
	WiFiReconnect WiFiReconnectReader
	WiFiAPs       WiFiAPEditor
	WiFiGuest     WiFiGuestEditor
	WiFiMAC       WiFiMACEditor
	// WiFiSchedule is the AP on/off schedule; nil answers 503.
	WiFiSchedule WiFiSchedule
	// SSHKeys is root's authorized_keys; nil answers 503.
	SSHKeys SSHKeys
	// FactoryReset wipes the router; nil answers 503.
	FactoryReset FactoryResetter
	// Firmware uploads, validates and flashes a sysupgrade image; nil answers 503.
	Firmware Firmware
	// SystemLog reads logread; nil answers 503.
	SystemLog SystemLog
	// USBTether is nil when the device cannot list netdevs; it answers 503 usbtether_unavailable.
	USBTether USBTether
	// Alerts are derived operator alerts; nil answers 503.
	Alerts Alerts
	// OpenWrtUpdates is the OpenWrt release and package check; nil answers 503.
	OpenWrtUpdates OpenWrtUpdates
	// SystemSettings is hostname, time zone and NTP; nil answers 503.
	SystemSettings SystemSettings
	// LEDs are the router LEDs and their night schedule; nil answers 503 leds_unavailable.
	LEDs LEDs
	// Buttons are the hardware buttons and the slide switch action; nil answers 503 buttons_unavailable.
	Buttons Buttons
	// Failover is nil without package and service ports; it answers 503 failover_unavailable.
	Failover Failover
	// Clients is the LAN client list and its actions; nil answers 503 clients_unavailable.
	Clients Clients
	// NetworkView is the read-only interface and zone view; nil answers 503 network_unavailable.
	NetworkView NetworkView
	// InterfaceRestart restarts uplinks; nil answers 503 interface_restart_unavailable.
	InterfaceRestart InterfaceRestarter
	// Diagnostics runs ping/traceroute/lookup; nil answers 503 diagnostics_unavailable.
	Diagnostics Diagnostics
	// WakeOnLAN sends magic packets; nil answers 503 wol_unavailable.
	WakeOnLAN WakeOnLAN
	// DHCP is the lan DHCP server and local DNS names; nil answers 503 dhcp_unavailable.
	DHCP DHCPServer
	// WAN is network.wan (DHCP, static, PPPoE); nil answers 503 wan_unavailable.
	WAN WANSettings
	// IPv6 is the IPv6 switch (wan6/wwan6, odhcpd on lan); nil answers 503 ipv6_unavailable.
	IPv6 IPv6Settings
	// LAN is the router's lan address (ADR 0028); nil answers 503 lan_unavailable.
	LAN LANAddress
	// PortForwards is firewall redirects wan -> lan; nil answers 503 port_forwards_unavailable.
	PortForwards PortForwards
	// Trusted reports whether a source address may reach the admin API (internal/trust).
	Trusted func(netip.Addr) bool
	// RouterIPs are the router's own lan addresses (internal/trust); nil before the first read.
	RouterIPs func() []netip.Addr
	// HostAllow are extra Host names accepted besides IP literals of the router and localhost.
	HostAllow []string
	// SaveEpoch persists the session epoch; it runs BEFORE a password change replies.
	SaveEpoch func(uint64) error
	// UI serves the web UI at "/" (internal/static); nil disables it. /api/ routes keep precedence.
	UI http.Handler
}

type server struct {
	Deps
	sessions *auth.Store
	clock    auth.Clock
	trusted  func(netip.Addr) bool
	events   *dashboardEventService
	// accountMu serialises setup and password change: "is it set?" and "set it" are one step.
	accountMu sync.Mutex
}

type healthOutput struct {
	Body struct {
		Status  string `json:"status" enum:"ok" doc:"Always ok when the daemon answers"`
		Version string `json:"version" doc:"Running build version"`
		BootID  string `json:"boot_id" doc:"Opaque id of the current boot; empty when unknown"`
	}
}

// register is the only way to add a route: it records whether the route is public.
// Protected routes take the cookie session or an API token (docs/adr/0008).
func register[I, O any](s *server, public bool, op huma.Operation, a huma.API, h func(context.Context, *I) (*O, error)) {
	if op.Metadata == nil {
		op.Metadata = map[string]any{}
	}
	op.Metadata[publicKey] = public
	if !public {
		op.Security = []map[string][]string{{"cookieAuth": {}}, {"bearerAuth": {}}}
	}
	huma.Register(a, op, h)
}

// registerSessionOnly adds a protected route that an API token may never call: managing
// tokens, the password and the session itself needs the browser login.
func registerSessionOnly[I, O any](s *server, op huma.Operation, a huma.API, h func(context.Context, *I) (*O, error)) {
	op.Metadata = map[string]any{publicKey: false, sessionOnlyKey: true}
	op.Security = []map[string][]string{{"cookieAuth": {}}}
	huma.Register(a, op, h)
}

// New builds the router and its generated OpenAPI 3.1 spec.
func New(d Deps) (http.Handler, huma.API) {
	s := &server{Deps: d, sessions: d.Sessions, clock: d.Clock, trusted: d.Trusted}
	mux := http.NewServeMux()
	cfg := huma.DefaultConfig("travo-greenfield", d.Version)
	cfg.OpenAPIPath = "/api/openapi"
	cfg.DocsPath = ""
	cfg.SchemasPath = ""
	cfg.CreateHooks = nil // no  links: that route does not exist
	cfg.Components.Schemas = huma.NewMapRegistry("#/components/schemas/", schemaNamer)
	cfg.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"cookieAuth": {Type: "apiKey", In: "cookie", Name: cookieName},
		"bearerAuth": {Type: "http", Scheme: "bearer", Description: "API token (tgt_...): read scope = GET only, admin = all but session-only routes"},
	}
	a := humago.New(mux, cfg)
	a.UseMiddleware(s.gate)

	register(s, true, huma.Operation{
		OperationID: "getHealth", Method: http.MethodGet, Path: "/api/health",
		Summary: "Liveness check", Tags: []string{"system"},
	}, a, func(ctx context.Context, _ *struct{}) (*healthOutput, error) {
		out := &healthOutput{}
		out.Body.Status = "ok"
		out.Body.Version = d.Version
		if d.BootID != nil {
			out.Body.BootID = d.BootID(ctx)
		}
		return out, nil
	})
	s.registerAuth(a)
	s.registerAPITokens(a)
	s.registerSetup(a)
	s.registerTxn(a)
	s.registerSystem(a)
	s.registerSystemSettings(a)
	s.registerLEDs(a)
	s.registerButtons(a)
	s.registerOpenWrtUpdates(a)
	s.registerSystemAlerts(a)
	s.registerSSHKeys(a)
	s.registerSystemLog(a)
	s.registerClients(a)
	s.registerNetworkView(a)
	s.registerInterfaceRestart(a)
	s.registerDiagnostics(a)
	s.registerWakeOnLAN(a)
	s.registerDHCP(a)
	s.registerWAN(a)
	s.registerIPv6(a)
	s.registerLAN(a)
	s.registerPortForwards(a)
	s.registerReboot(a)
	s.registerFactoryReset(a)
	s.registerFirmware(a)
	s.registerVPNProfiles(a)
	s.registerVPNStatus(a)
	s.registerVPNTunnel(a)
	s.registerVPNSplit(a)
	s.registerVPNKillSwitch(a)
	s.registerVPNDNS(a)
	s.registerUpdateCheck(a)
	s.registerUpdateStage(a)
	s.registerUpdateApply(a)
	s.registerBackup(a)
	s.registerThroughput(a)
	s.registerStatsHistory(a)
	s.registerConnectivity(a)
	s.registerCaptive(a)
	s.registerWiFi(a)
	s.registerWiFiProfiles(a)
	s.registerAdGuard(a)
	s.registerTailscale(a)
	s.registerWiFiReconnect(a)
	s.registerWiFiSchedule(a)
	s.registerUSBTether(a)
	s.registerFailover(a)
	s.registerDashboardEvents(a)
	if d.UI != nil {
		mux.Handle("/", d.UI)
	}
	return s.outer(mux), a
}

// schemaNamer disambiguates the WiFi and saved-uplink models from the VPN profile models,
// the AdGuard and Tailscale models from the transaction Status.
func schemaNamer(t reflect.Type, hint string) string {
	name := huma.DefaultSchemaNamer(t, hint)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.PkgPath() {
	case "github.com/blackdark-org/travo-greenfield/internal/wifi":
		if t.Name() == "Summary" {
			return "WiFi" + name
		}
	case "github.com/blackdark-org/travo-greenfield/internal/wifiprofiles":
		return "WiFiProfile" + name
	case "github.com/blackdark-org/travo-greenfield/internal/wifireconnect":
		return "WiFiReconnect" + name
	case "github.com/blackdark-org/travo-greenfield/internal/usbtether":
		return "USBTether" + name
	case "github.com/blackdark-org/travo-greenfield/internal/wifischedule":
		return "WiFiSchedule" + name
	case "github.com/blackdark-org/travo-greenfield/internal/statshistory":
		return "StatsHistory" + name
	case "github.com/blackdark-org/travo-greenfield/internal/tailscale":
		return "Tailscale" + name
	case "github.com/blackdark-org/travo-greenfield/internal/failover":
		return "Failover" + name
	case "github.com/blackdark-org/travo-greenfield/internal/wansettings":
		return "WAN" + name
	case "github.com/blackdark-org/travo-greenfield/internal/ipv6settings":
		return "IPv6" + name
	case "github.com/blackdark-org/travo-greenfield/internal/portfwd":
		return "Port" + name
	case "github.com/blackdark-org/travo-greenfield/internal/lanaddress":
		return "LAN" + name
	case "github.com/blackdark-org/travo-greenfield/internal/leds":
		return "LED" + name
	case "github.com/blackdark-org/travo-greenfield/internal/firmware":
		return "Firmware" + name
	case "github.com/blackdark-org/travo-greenfield/internal/buttons":
		if name == "Button" {
			return "HardwareButton"
		}
		return "Button" + name
	}
	if t.PkgPath() == "github.com/blackdark-org/travo-greenfield/internal/adguard" && t.Name() == "Status" {
		return "AdGuard" + name
	}
	return name
}

// op builds an Operation with the documented extra error statuses.
func op(id, method, path, summary, tag string, errs ...int) huma.Operation {
	return huma.Operation{OperationID: id, Method: method, Path: path, Summary: summary, Tags: []string{tag}, Errors: errs}
}
