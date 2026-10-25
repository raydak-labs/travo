package network

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/platform"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

// ErrUnknownInterface is returned for an interface name netifd does not know.
var ErrUnknownInterface = errors.New("unknown interface")

// Service provides network status and configuration.
type Service struct {
	uci       uci.UCI
	ubus      ubus.Ubus
	aliasFile string
	cmd       platform.CommandRunner

	// portForwardsFile overrides the const path (tests only), following the
	// same pattern as aliasFile.
	portForwardsFile string

	// arpFile is the neighbour table the client classifier reads an IP to a MAC
	// through. Overridable so a test can answer with the table captured from the
	// device instead of the build machine's.
	arpFile string

	// ddnsInitScript is the ddns-scripts init script. Its absence is how we know
	// the package is not installed, because that is the only thing on the device
	// that can service a `ddns` config. Overridable for tests.
	ddnsInitScript string

	// wifiMACsMu guards wifiMACsSeen.
	wifiMACsMu sync.Mutex
	// wifiMACsSeen tracks MACs recently seen in WiFi station dumps, with the
	// timestamp of their last appearance. Used to suppress "disconnected WiFi
	// client appears as LAN" during the ARP cache staleness window.
	wifiMACsSeen map[string]time.Time

	// portForwardsMu serialises the port-forward read-modify-write.
	//
	// This is NOT the per-config UCI lock (see lockUCIConfigs), which guards the
	// process-global /tmp/.uci/<config>/changes file. Port forwards are a
	// read-modify-write over BOTH /etc/travo/port-forwards.json and the firewall
	// config, and the config lock cannot cover the JSON file. Unserialised, two
	// concurrent adds each read the same rule list and each wrote its own
	// version, so one add silently vanished; and because os.WriteFile truncates
	// before writing, a reader could observe a half-written file and fail with
	// "unexpected end of JSON input". Both were reproduced on the device with
	// 24 concurrent writers: 35 successful creates, 1 rule on disk.
	portForwardsMu sync.Mutex
}

// wifiMACTTL is how long a MAC stays in the wifiMACsSeen set after it was
// last observed in a station dump. Once expired the device may reappear as
// a wired LAN client if it reconnects that way.
const wifiMACTTL = 5 * time.Minute

// initdPath is the path to OpenWRT init.d scripts directory.
const initdPath = "/etc/init.d/"

// stringFromBytes converts a byte slice to a trimmed string.
func stringFromBytes(data []byte) string {
	return strings.TrimSpace(string(data))
}

// Deps are the collaborators of a network Service. Zero values take the
// production defaults.
type Deps struct {
	UCI  uci.UCI
	Ubus ubus.Ubus
	// Cmd runs shell commands; default platform.RealCommandRunner.
	Cmd platform.CommandRunner
	// AliasFile stores client aliases; default /etc/travo/aliases.json.
	AliasFile string
}

// New creates a network Service.
func New(d Deps) *Service {
	if d.Cmd == nil {
		d.Cmd = &platform.RealCommandRunner{}
	}
	if d.AliasFile == "" {
		d.AliasFile = "/etc/travo/aliases.json"
	}
	return &Service{
		uci: d.UCI, ubus: d.Ubus, aliasFile: d.AliasFile, cmd: d.Cmd, arpFile: ProcNetARP,
		wifiMACsSeen:     make(map[string]time.Time),
		portForwardsFile: defaultPortForwardsFile,
		ddnsInitScript:   ddnsInitScriptPath,
	}
}

// uciSet is a helper for setting UCI values with consistent error wrapping.
func (n *Service) uciSet(config, section, option, value string) error {
	if err := n.uci.Set(config, section, option, value); err != nil {
		return fmt.Errorf("set %s.%s.%s: %w", config, section, option, err)
	}
	return nil
}

// uciCommit commits a UCI config with consistent error wrapping.
func (n *Service) uciCommit(config string) error {
	if err := n.uci.Commit(config); err != nil {
		return fmt.Errorf("commit %s: %w", config, err)
	}
	return nil
}

// restartService restarts an OpenWRT init.d service.
func (n *Service) restartService(service string) error {
	_, err := n.cmd.Run(initdPath+service, "restart")
	return err
}

// allowedInterfaces is the set of interface names that can be toggled.
var allowedInterfaces = map[string]bool{
	"wan":  true,
	"lan":  true,
	"wwan": true,
}

// SetInterfaceState brings a network interface up or down via ifup/ifdown.
func (n *Service) SetInterfaceState(iface string, up bool) error {
	if !allowedInterfaces[iface] {
		return fmt.Errorf("%w: %s", ErrUnknownInterface, iface)
	}
	cmd := "ifdown"
	if up {
		cmd = "ifup"
	}
	if _, err := n.cmd.Run(cmd, iface); err != nil {
		return fmt.Errorf("%s %s: %w", cmd, iface, err)
	}
	return nil
}

func maskToNetmask(mask float64) string {
	bits := int(mask)
	if bits <= 0 || bits > 32 {
		return "255.255.255.0"
	}
	m := uint32(0xFFFFFFFF) << (32 - bits)
	return fmt.Sprintf("%d.%d.%d.%d", (m>>24)&0xFF, (m>>16)&0xFF, (m>>8)&0xFF, m&0xFF)
}

// GetNetworkStatus returns the overall network status.
func (n *Service) GetNetworkStatus() (models.NetworkStatus, error) {
	var status models.NetworkStatus

	wanData, err := n.ubus.Call("network.interface.wan", "status", nil)
	if err == nil {
		wan := parseInterface("wan", "eth0", wanData, n.ubus)
		status.WAN = &wan
	}

	lanData, err := n.ubus.Call("network.interface.lan", "status", nil)
	if err == nil {
		status.LAN = parseInterface("lan", "br-lan", lanData, n.ubus)
	}

	// Also check wwan (WiFi uplink) — common on travel routers
	wwanData, wwanErr := n.ubus.Call("network.interface.wwan", "status", nil)

	// USB tethering (typically a phone) creates its own logical interface in
	// the wan zone. It has to appear in Interfaces, or a phone-tethered router
	// reports the uplink as down while it is up.
	usbData, usbErr := n.ubus.Call("network.interface."+USBTetherInterface, "status", nil)

	status.Interfaces = []models.NetworkInterface{}
	if status.WAN != nil {
		status.Interfaces = append(status.Interfaces, *status.WAN)
	}
	status.Interfaces = append(status.Interfaces, status.LAN)

	if wwanErr == nil {
		// parseInterface maps the wireless device to the "wifi" discriminator.
		wwanIface := parseInterface("wwan", "phy0-sta0", wwanData, n.ubus)
		status.Interfaces = append(status.Interfaces, wwanIface)
		// If wwan is up and wan is not, use wwan as the effective WAN
		if wwanIface.IsUp && (status.WAN == nil || !status.WAN.IsUp) {
			status.WAN = &wwanIface
		}
	}

	if usbErr == nil {
		usbIface := parseInterface(USBTetherInterface, USBTetherInterface, usbData, n.ubus)
		status.Interfaces = append(status.Interfaces, usbIface)
		if usbIface.IsUp && (status.WAN == nil || !status.WAN.IsUp) {
			status.WAN = &usbIface
		}
	}

	// Fetch DHCP clients from ubus
	status.Clients = n.fetchDHCPClients()

	status.InternetReachable = status.WAN != nil && status.WAN.IsUp
	return status, nil
}

// interfaceType maps a ubus network interface onto the discriminator the API
// contract declares in shared/src/api/network.ts:
// 'wan' | 'lan' | 'wifi' | 'vpn' | 'usb'.
//
// Rules (in order):
//   - logical interfaces named wan/lan keep their name, whatever device carries
//     them, so the WAN/LAN cards keep working;
//   - wireguard/tunnel devices (proto=wireguard, wg*, tun*, tap*) are "vpn";
//   - USB tethering devices (usbtether, usb*, rndis*, cdc_*) are "usb";
//   - wireless devices (phy*, wlan*, *-sta, *-ap) and the wwan uplink are
//     "wifi";
//   - anything else keeps the interface name.
func interfaceType(name, device, proto string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	d := strings.ToLower(strings.TrimSpace(device))
	p := strings.ToLower(strings.TrimSpace(proto))
	if n == "wan" || n == "lan" {
		return n
	}
	switch {
	case p == "wireguard" ||
		strings.HasPrefix(d, "wg") || strings.HasPrefix(d, "tun") || strings.HasPrefix(d, "tap") ||
		strings.HasPrefix(n, "wg") || strings.HasPrefix(n, "vpn"):
		return "vpn"
	case n == "usbtether" || strings.HasPrefix(d, "usb") ||
		strings.HasPrefix(d, "rndis") || strings.HasPrefix(d, "cdc_"):
		return "usb"
	case n == "wwan" || strings.HasPrefix(d, "phy") || strings.HasPrefix(d, "wlan") ||
		strings.Contains(d, "-sta") || strings.Contains(d, "-ap"):
		return "wifi"
	default:
		return n
	}
}

func parseInterface(name, device string, data map[string]any, ub ubus.Ubus) models.NetworkInterface {
	iface := models.NetworkInterface{
		Name: name,
	}
	if up, ok := data["up"].(bool); ok {
		iface.IsUp = up
	}
	// Get device name from ubus response
	devName, _ := data["device"].(string)
	if devName == "" {
		devName, _ = data["l3_device"].(string)
	}
	if devName == "" {
		devName = device
	}
	proto, _ := data["proto"].(string)
	iface.Type = interfaceType(name, devName, proto)

	// Fetch device stats for MAC and traffic
	if ub != nil && devName != "" {
		if devData, err := ub.Call("network.device", "status", map[string]any{"name": devName}); err == nil {
			if mac, ok := devData["macaddr"].(string); ok && mac != "" {
				iface.MACAddress = mac
			}
			if stats, ok := devData["statistics"].(map[string]any); ok {
				if rxBytes, ok := stats["rx_bytes"].(float64); ok {
					iface.RxBytes = int64(rxBytes)
				}
				if txBytes, ok := stats["tx_bytes"].(float64); ok {
					iface.TxBytes = int64(txBytes)
				}
			}
		}
	}

	if addrs, ok := data["ipv4-address"].([]any); ok && len(addrs) > 0 {
		if a, ok := addrs[0].(map[string]any); ok {
			iface.IPAddress, _ = a["address"].(string)
			if mask, ok := a["mask"].(float64); ok {
				iface.Netmask = maskToNetmask(mask)
			}
		}
	}
	if routes, ok := data["route"].([]any); ok {
		for _, r := range routes {
			if rm, ok := r.(map[string]any); ok {
				if gw, ok := rm["nexthop"].(string); ok && gw != "" {
					iface.Gateway = gw
					break
				}
			}
		}
	}
	if dns, ok := data["dns-server"].([]any); ok {
		iface.DNSServers = make([]string, 0, len(dns))
		for _, d := range dns {
			if s, ok := d.(string); ok {
				iface.DNSServers = append(iface.DNSServers, s)
			}
		}
	}
	return iface
}

// GetClients returns connected LAN clients with aliases merged.
func (n *Service) GetClients() ([]models.Client, error) {
	status, err := n.GetNetworkStatus()
	if err != nil {
		return nil, err
	}
	aliases := n.loadAliases()
	clients := status.Clients
	for i := range clients {
		if alias, ok := aliases[strings.ToUpper(clients[i].MACAddress)]; ok {
			clients[i].Alias = alias
		}
	}
	return clients, nil
}
