package models

// NetworkInterface represents a network interface on the router.
type NetworkInterface struct {
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	IPAddress  string   `json:"ip_address"`
	Netmask    string   `json:"netmask"`
	Gateway    string   `json:"gateway"`
	DNSServers []string `json:"dns_servers"`
	MACAddress string   `json:"mac_address"`
	IsUp       bool     `json:"is_up"`
	RxBytes    int64    `json:"rx_bytes"`
	TxBytes    int64    `json:"tx_bytes"`
}

// WanConfig holds WAN connection configuration.
type WanConfig struct {
	Type          string   `json:"type"`
	InterfaceName string   `json:"interface_name"`
	IPAddress     string   `json:"ip_address"`
	Netmask       string   `json:"netmask"`
	Gateway       string   `json:"gateway"`
	DNSServers    []string `json:"dns_servers"`
	MTU           int      `json:"mtu"`
}

// Client represents a connected LAN client.
type Client struct {
	IPAddress      string `json:"ip_address"`
	MACAddress     string `json:"mac_address"`
	Hostname       string `json:"hostname"`
	Alias          string `json:"alias,omitempty"`
	InterfaceName  string `json:"interface_name"`
	RxBytes        int64  `json:"rx_bytes"`
	TxBytes        int64  `json:"tx_bytes"`
	ConnectedSince string `json:"connected_since"`
}

// SetAliasRequest holds the request to set a device alias.
type SetAliasRequest struct {
	MAC   string `json:"mac"`
	Alias string `json:"alias"`
}

// NetworkStatus is the overall network state.
type NetworkStatus struct {
	WAN               *NetworkInterface  `json:"wan"`
	LAN               NetworkInterface   `json:"lan"`
	Interfaces        []NetworkInterface `json:"interfaces"`
	Clients           []Client           `json:"clients"`
	InternetReachable bool               `json:"internet_reachable"`
}

// DHCPConfig holds DHCP server configuration for the LAN.
type DHCPConfig struct {
	Start     int    `json:"start"`
	Limit     int    `json:"limit"`
	LeaseTime string `json:"lease_time"`
}

// DNSConfig holds custom DNS server configuration.
type DNSConfig struct {
	UseCustomDNS bool     `json:"use_custom_dns"`
	Servers      []string `json:"servers"`
}

// MACConfig holds MAC address configuration.
type MACConfig struct {
	Interface  string `json:"interface"`
	CurrentMAC string `json:"current_mac"`
	CustomMAC  string `json:"custom_mac,omitempty"`
}

// SetMACRequest holds the request to set a MAC address.
type SetMACRequest struct {
	MAC string `json:"mac"`
}

// DHCPLease represents an active DHCP lease.
type DHCPLease struct {
	Expiry   int64  `json:"expiry"`
	MAC      string `json:"mac"`
	IP       string `json:"ip"`
	Hostname string `json:"hostname"`
}

// DNSEntry represents a local DNS entry (hostname → IP mapping).
type DNSEntry struct {
	Name    string `json:"name"`
	IP      string `json:"ip"`
	Section string `json:"section,omitempty"`
}

// ClientActionRequest is the request body for kick/block/unblock actions.
type ClientActionRequest struct {
	MAC string `json:"mac"`
}

// WanDetectResult holds the result of WAN connection type auto-detection.
type WanDetectResult struct {
	DetectedType string `json:"detected_type"`
	CurrentType  string `json:"current_type"`
}

// SetInterfaceStateRequest is the request body for bringing an interface up or down.
type SetInterfaceStateRequest struct {
	Up bool `json:"up"`
}

// DHCPReservation represents a static DHCP reservation (IP by MAC).
type DHCPReservation struct {
	Name    string `json:"name"`
	MAC     string `json:"mac"`
	IP      string `json:"ip"`
	Section string `json:"section,omitempty"`
}

// DDNSConfig holds Dynamic DNS provider configuration.
type DDNSConfig struct {
	Enabled    bool   `json:"enabled"`
	Service    string `json:"service"`
	Domain     string `json:"domain"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	LookupHost string `json:"lookup_host"`
}

// DDNSStatus holds the current DDNS service status.
type DDNSStatus struct {
	Running    bool   `json:"running"`
	PublicIP   string `json:"public_ip"`
	LastUpdate string `json:"last_update"`
}

// DataUsagePeriod holds RX/TX byte counts for a period.
type DataUsagePeriod struct {
	RXBytes int64 `json:"rx_bytes"`
	TXBytes int64 `json:"tx_bytes"`
}

// DataUsageInterface holds traffic data for a single network interface.
type DataUsageInterface struct {
	Name  string          `json:"name"`
	Label string          `json:"label"`
	Today DataUsagePeriod `json:"today"`
	Month DataUsagePeriod `json:"month"`
	Total DataUsagePeriod `json:"total"`
}

// DataUsageStatus is the top-level response for the data usage endpoint.
type DataUsageStatus struct {
	// Available is false when vnstat is not installed; interfaces will be empty.
	Available  bool                 `json:"available"`
	Interfaces []DataUsageInterface `json:"interfaces"`
}

// DataBudget holds a monthly usage limit for a single interface.
type DataBudget struct {
	Interface             string  `json:"interface"`
	MonthlyLimitBytes     int64   `json:"monthly_limit_bytes"`
	WarningThresholdPct   float64 `json:"warning_threshold_pct"`
	ResetDay              int     `json:"reset_day"`
}

// DataBudgetConfig holds all configured data budgets.
type DataBudgetConfig struct {
	Budgets []DataBudget `json:"budgets"`
}

// IPv6Status holds the current IPv6 configuration state.
type IPv6Status struct {
	Enabled   bool     `json:"enabled"`
	Addresses []string `json:"addresses"`
}

// FirewallZone holds a summary of a UCI firewall zone.
type FirewallZone struct {
	Name    string   `json:"name"`
	Input   string   `json:"input"`   // ACCEPT / REJECT / DROP
	Output  string   `json:"output"`
	Forward string   `json:"forward"`
	Network []string `json:"network"`
}

// PortForwardRule holds a single DNAT port-forward rule.
type PortForwardRule struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Protocol string `json:"protocol"` // tcp, udp, tcp+udp
	SrcDPort string `json:"src_dport"` // external port/range
	DestIP   string `json:"dest_ip"`
	DestPort string `json:"dest_port"` // internal port (empty = same)
	Enabled  bool   `json:"enabled"`
}

// WoLRequest asks the server to send a magic packet to a specific MAC address.
type WoLRequest struct {
	MAC       string `json:"mac"`
	Interface string `json:"interface"` // optional, e.g. "br-lan"
}

// DoHConfig holds DNS-over-HTTPS settings.
type DoHConfig struct {
	Enabled  bool   `json:"enabled"`
	Provider string `json:"provider"` // "cloudflare", "google", "quad9", "custom"
	URL      string `json:"url"`      // used when provider="custom"
}

// DiagnosticsRequest asks to run a network diagnostic tool.
type DiagnosticsRequest struct {
	Type   string `json:"type"`   // "ping", "traceroute", "dns"
	Target string `json:"target"` // hostname or IP
}

// DiagnosticsResult holds the textual output of a diagnostic command.
type DiagnosticsResult struct {
	Type   string `json:"type"`
	Target string `json:"target"`
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}
