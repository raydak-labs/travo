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
