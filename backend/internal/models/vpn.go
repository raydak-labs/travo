package models

// VpnStatus represents the status of a VPN connection.
type VpnStatus struct {
	Type           string `json:"type"`
	Enabled        bool   `json:"enabled"`
	Connected      bool   `json:"connected"`
	ConnectedSince string `json:"connected_since"`
	Endpoint       string `json:"endpoint"`
	RxBytes        int64  `json:"rx_bytes"`
	TxBytes        int64  `json:"tx_bytes"`
	// StatusDetail provides fine-grained tunnel state:
	// "disabled", "configured", "enabled_not_up", "up_no_handshake", "connected"
	StatusDetail string `json:"status_detail,omitempty"`
}

// WireguardPeer represents a WireGuard peer.
type WireguardPeer struct {
	PublicKey     string   `json:"public_key"`
	Endpoint      string   `json:"endpoint"`
	AllowedIPs    []string `json:"allowed_ips"`
	PresharedKey  *string  `json:"preshared_key,omitempty"`
	LastHandshake *string  `json:"last_handshake,omitempty"`
}

// WireguardConfig holds WireGuard tunnel configuration.
type WireguardConfig struct {
	PrivateKey string          `json:"private_key"`
	Address    string          `json:"address"`
	DNS        []string        `json:"dns"`
	Peers      []WireguardPeer `json:"peers"`
}

// WireGuardPeerStatus holds live status for a single WireGuard peer.
type WireGuardPeerStatus struct {
	PublicKey       string `json:"public_key"`
	Endpoint        string `json:"endpoint"`
	LatestHandshake int64  `json:"latest_handshake"` // unix epoch seconds, 0 = never
	TransferRx      int64  `json:"transfer_rx"`      // bytes received
	TransferTx      int64  `json:"transfer_tx"`      // bytes sent
	AllowedIPs      string `json:"allowed_ips"`
}

// WireGuardStatus holds live WireGuard interface status from `wg show`.
type WireGuardStatus struct {
	Interface  string                `json:"interface"`
	PublicKey  string                `json:"public_key"`
	ListenPort int                   `json:"listen_port"`
	Peers      []WireGuardPeerStatus `json:"peers"`
}

// WireGuardProfile represents a saved WireGuard configuration profile.
type WireGuardProfile struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Config    string `json:"config"` // Raw WireGuard .conf content
	Active    bool   `json:"active"` // Is this the currently loaded profile?
	CreatedAt string `json:"created_at"`
}

// KillSwitchStatus represents the VPN kill switch state.
type KillSwitchStatus struct {
	Enabled bool `json:"enabled"`
}

// DNSLeakResult holds the result of a DNS leak test.
type DNSLeakResult struct {
	// Nameservers currently in /etc/resolv.conf (what the system resolves with).
	Nameservers []string `json:"nameservers"`
	// VPNDNSServers are the DNS servers configured in the active WireGuard profile.
	VPNDNSServers []string `json:"vpn_dns_servers"`
	// VPNActive is true when a VPN tunnel is enabled.
	VPNActive bool `json:"vpn_active"`
	// PotentiallyLeaking is true when VPN is active but none of the current
	// nameservers match the VPN-configured DNS servers.
	PotentiallyLeaking bool `json:"potentially_leaking"`
}

// VPNVerifyResult contains the result of verifying the WireGuard tunnel health.
type VPNVerifyResult struct {
	// InterfaceUp is true when wg0 exists and is in UP state.
	InterfaceUp bool `json:"interface_up"`
	// HandshakeOk is true when the most recent peer handshake is < 3 minutes old.
	HandshakeOk bool `json:"handshake_ok"`
	// LatestHandshake is the unix epoch of the most recent handshake (0 = never).
	LatestHandshake int64 `json:"latest_handshake"`
	// RouteOk is true when a default route via wg0 exists.
	RouteOk bool `json:"route_ok"`
	// FirewallZoneOk is true when the wg0 firewall zone exists in UCI.
	FirewallZoneOk bool `json:"firewall_zone_ok"`
	// ForwardingOk is true when a lan→wg0 firewall forwarding rule exists.
	ForwardingOk bool `json:"forwarding_ok"`
}

// TailscaleStatus represents Tailscale connection status.
type TailscaleStatus struct {
	Installed      bool    `json:"installed"`
	Running        bool    `json:"running"`
	LoggedIn       bool    `json:"logged_in"`
	IPAddress      string  `json:"ip_address"`
	Hostname       string  `json:"hostname"`
	ExitNode       *string `json:"exit_node,omitempty"`
	ExitNodeActive bool    `json:"exit_node_active"`
}
