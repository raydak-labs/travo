package services

import (
	"fmt"
	"strconv"
	"strings"
)

// WireguardParsedConfig represents a fully parsed WireGuard .conf file.
type WireguardParsedConfig struct {
	Interface WireguardInterface
	Peers     []WireguardParsedPeer
}

// WireguardInterface represents the [Interface] section of a WireGuard config.
type WireguardInterface struct {
	PrivateKey string
	Address    string
	DNS        string
	MTU        int
	ListenPort int
}

// WireguardParsedPeer represents a [Peer] section of a WireGuard config.
type WireguardParsedPeer struct {
	PublicKey           string
	AllowedIPs          string
	Endpoint            string
	PersistentKeepalive int
	PresharedKey        string
}

// ParseWireguardConfig parses a WireGuard .conf file content into a structured config.
func ParseWireguardConfig(confContent string) (*WireguardParsedConfig, error) {
	confContent = strings.TrimSpace(confContent)
	if confContent == "" {
		return nil, fmt.Errorf("empty configuration")
	}

	result := &WireguardParsedConfig{}
	var currentSection string
	var currentPeer *WireguardParsedPeer
	foundInterface := false

	lines := strings.Split(confContent, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Check for section headers
		upper := strings.ToUpper(line)
		if strings.HasPrefix(upper, "[INTERFACE]") {
			currentSection = "interface"
			foundInterface = true
			// Finish previous peer if any
			if currentPeer != nil {
				result.Peers = append(result.Peers, *currentPeer)
				currentPeer = nil
			}
			continue
		}
		if strings.HasPrefix(upper, "[PEER]") {
			currentSection = "peer"
			// Finish previous peer if any
			if currentPeer != nil {
				result.Peers = append(result.Peers, *currentPeer)
			}
			currentPeer = &WireguardParsedPeer{}
			continue
		}

		// Parse key = value
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch currentSection {
		case "interface":
			switch strings.ToLower(key) {
			case "privatekey":
				result.Interface.PrivateKey = value
			case "address":
				result.Interface.Address = value
			case "dns":
				result.Interface.DNS = value
			case "mtu":
				mtu, err := strconv.Atoi(value)
				if err == nil {
					result.Interface.MTU = mtu
				}
			case "listenport":
				port, err := strconv.Atoi(value)
				if err == nil {
					result.Interface.ListenPort = port
				}
			}
		case "peer":
			if currentPeer == nil {
				continue
			}
			switch strings.ToLower(key) {
			case "publickey":
				currentPeer.PublicKey = value
			case "allowedips":
				currentPeer.AllowedIPs = value
			case "endpoint":
				currentPeer.Endpoint = value
			case "persistentkeepalive":
				ka, err := strconv.Atoi(value)
				if err == nil {
					currentPeer.PersistentKeepalive = ka
				}
			case "presharedkey":
				currentPeer.PresharedKey = value
			}
		}
	}

	// Finish last peer
	if currentPeer != nil {
		result.Peers = append(result.Peers, *currentPeer)
	}

	// Validate required fields
	if !foundInterface {
		return nil, fmt.Errorf("missing [Interface] section")
	}
	if result.Interface.PrivateKey == "" {
		return nil, fmt.Errorf("missing required field: PrivateKey in [Interface]")
	}

	for i, peer := range result.Peers {
		if peer.PublicKey == "" {
			return nil, fmt.Errorf("missing required field: PublicKey in [Peer] %d", i)
		}
	}

	return result, nil
}
