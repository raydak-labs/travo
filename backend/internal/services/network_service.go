package services

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

// NetworkService provides network status and configuration.
type NetworkService struct {
	uci  uci.UCI
	ubus ubus.Ubus
}

// NewNetworkService creates a new NetworkService.
func NewNetworkService(u uci.UCI, ub ubus.Ubus) *NetworkService {
	return &NetworkService{uci: u, ubus: ub}
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
func (n *NetworkService) GetNetworkStatus() (models.NetworkStatus, error) {
	var status models.NetworkStatus

	wanData, err := n.ubus.Call("network.interface.wan", "status", nil)
	if err == nil {
		wan := parseInterface("wan", "eth0", wanData)
		status.WAN = &wan
	}

	lanData, err := n.ubus.Call("network.interface.lan", "status", nil)
	if err == nil {
		status.LAN = parseInterface("lan", "br-lan", lanData)
	}

	status.Interfaces = []models.NetworkInterface{}
	if status.WAN != nil {
		status.Interfaces = append(status.Interfaces, *status.WAN)
	}
	status.Interfaces = append(status.Interfaces, status.LAN)

	// Fetch DHCP clients from ubus
	status.Clients = n.fetchDHCPClients()

	status.InternetReachable = status.WAN != nil && status.WAN.IsUp
	return status, nil
}

// fetchDHCPClients queries ubus for DHCP lease information.
func (n *NetworkService) fetchDHCPClients() []models.Client {
	var clients []models.Client

	data, err := n.ubus.Call("dhcp", "ipv4leases", nil)
	if err != nil {
		return clients
	}

	device, ok := data["device"].(map[string]interface{})
	if !ok {
		return clients
	}

	for ifaceName, ifaceData := range device {
		ifaceMap, ok := ifaceData.(map[string]interface{})
		if !ok {
			continue
		}
		leases, ok := ifaceMap["leases"].([]interface{})
		if !ok {
			continue
		}
		for _, lease := range leases {
			lm, ok := lease.(map[string]interface{})
			if !ok {
				continue
			}
			ip, _ := lm["ip"].(string)
			mac, _ := lm["mac"].(string)
			hostname, _ := lm["hostname"].(string)

			clients = append(clients, models.Client{
				IPAddress:     ip,
				MACAddress:    mac,
				Hostname:      hostname,
				InterfaceName: ifaceName,
			})
		}
	}

	return clients
}

func parseInterface(name, device string, data map[string]interface{}) models.NetworkInterface {
	iface := models.NetworkInterface{
		Name: name, Type: name, MACAddress: "00:00:00:00:00:00",
	}
	if up, ok := data["up"].(bool); ok {
		iface.IsUp = up
	}
	if addrs, ok := data["ipv4-address"].([]interface{}); ok && len(addrs) > 0 {
		if a, ok := addrs[0].(map[string]interface{}); ok {
			iface.IPAddress, _ = a["address"].(string)
			if mask, ok := a["mask"].(float64); ok {
				iface.Netmask = maskToNetmask(mask)
			}
		}
	}
	if routes, ok := data["route"].([]interface{}); ok {
		for _, r := range routes {
			if rm, ok := r.(map[string]interface{}); ok {
				if gw, ok := rm["nexthop"].(string); ok && gw != "" {
					iface.Gateway = gw
					break
				}
			}
		}
	}
	if dns, ok := data["dns-server"].([]interface{}); ok {
		iface.DNSServers = make([]string, 0, len(dns))
		for _, d := range dns {
			if s, ok := d.(string); ok {
				iface.DNSServers = append(iface.DNSServers, s)
			}
		}
	}
	_ = device
	return iface
}

// GetWanConfig returns the WAN configuration.
func (n *NetworkService) GetWanConfig() (models.WanConfig, error) {
	opts, err := n.uci.GetAll("network", "wan")
	if err != nil {
		return models.WanConfig{}, err
	}
	var dnsServers []string
	if dns, ok := opts["dns"]; ok && dns != "" {
		dnsServers = strings.Split(dns, " ")
	}
	mtu := 1500
	if m, ok := opts["mtu"]; ok && m != "" {
		if v, err := strconv.Atoi(m); err == nil {
			mtu = v
		}
	}
	return models.WanConfig{
		Type: opts["proto"], InterfaceName: opts["ifname"],
		IPAddress: opts["ip4addr"], Netmask: opts["netmask"],
		Gateway: opts["gateway"], DNSServers: dnsServers, MTU: mtu,
	}, nil
}

// SetWanConfig updates the WAN configuration.
func (n *NetworkService) SetWanConfig(config models.WanConfig) error {
	if config.Type != "" {
		if err := n.uci.Set("network", "wan", "proto", config.Type); err != nil {
			return fmt.Errorf("setting WAN proto: %w", err)
		}
	}
	if config.IPAddress != "" {
		if err := n.uci.Set("network", "wan", "ip4addr", config.IPAddress); err != nil {
			return fmt.Errorf("setting WAN ip4addr: %w", err)
		}
	}
	if config.Netmask != "" {
		if err := n.uci.Set("network", "wan", "netmask", config.Netmask); err != nil {
			return fmt.Errorf("setting WAN netmask: %w", err)
		}
	}
	if config.Gateway != "" {
		if err := n.uci.Set("network", "wan", "gateway", config.Gateway); err != nil {
			return fmt.Errorf("setting WAN gateway: %w", err)
		}
	}
	return n.uci.Commit("network")
}

// GetClients returns connected LAN clients.
func (n *NetworkService) GetClients() ([]models.Client, error) {
	status, err := n.GetNetworkStatus()
	if err != nil {
		return nil, err
	}
	return status.Clients, nil
}
