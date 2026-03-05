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
	InterfaceName  string `json:"interface_name"`
	RxBytes        int64  `json:"rx_bytes"`
	TxBytes        int64  `json:"tx_bytes"`
	ConnectedSince string `json:"connected_since"`
}

// NetworkStatus is the overall network state.
type NetworkStatus struct {
	WAN               *NetworkInterface  `json:"wan"`
	LAN               NetworkInterface   `json:"lan"`
	Interfaces        []NetworkInterface `json:"interfaces"`
	Clients           []Client           `json:"clients"`
	InternetReachable bool               `json:"internet_reachable"`
}
