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
