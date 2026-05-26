package services

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// WireguardParsedConfig represents a fully parsed WireGuard .conf file.
type WireguardParsedConfig struct {
	Interface WireguardInterface
	Peers     []WireguardParsedPeer
}

// IsAmnezia returns true if the config contains any AmneziaWG-specific parameters.
func (c *WireguardParsedConfig) IsAmnezia() bool {
	return c.Interface.Jc > 0 || c.Interface.Jmin > 0 || c.Interface.Jmax > 0 ||
		c.Interface.S1 > 0 || c.Interface.S2 > 0 ||
		c.Interface.H1 > 0 || c.Interface.H2 > 0 || c.Interface.H3 > 0 || c.Interface.H4 > 0
}

// WireguardInterface represents the [Interface] section of a WireGuard config.
type WireguardInterface struct {
	PrivateKey string
	Address    string
	DNS        string
	MTU        int
	ListenPort int
	// AmneziaWG obfuscation parameters (zero means not set / standard WireGuard)
	Jc   int
	Jmin int
	Jmax int
	S1   int
	S2   int
	H1   uint32
	H2   uint32
	H3   uint32
	H4   uint32
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
			case "jc":
				jc, err := strconv.Atoi(value)
				if err == nil {
					result.Interface.Jc = jc
				}
			case "jmin":
				jmin, err := strconv.Atoi(value)
				if err == nil {
					result.Interface.Jmin = jmin
				}
			case "jmax":
				jmax, err := strconv.Atoi(value)
				if err == nil {
					result.Interface.Jmax = jmax
				}
			case "s1":
				s1, err := strconv.Atoi(value)
				if err == nil {
					result.Interface.S1 = s1
				}
			case "s2":
				s2, err := strconv.Atoi(value)
				if err == nil {
					result.Interface.S2 = s2
				}
			case "h1":
				h1, err := strconv.ParseUint(value, 10, 32)
				if err == nil {
					result.Interface.H1 = uint32(h1)
				}
			case "h2":
				h2, err := strconv.ParseUint(value, 10, 32)
				if err == nil {
					result.Interface.H2 = uint32(h2)
				}
			case "h3":
				h3, err := strconv.ParseUint(value, 10, 32)
				if err == nil {
					result.Interface.H3 = uint32(h3)
				}
			case "h4":
				h4, err := strconv.ParseUint(value, 10, 32)
				if err == nil {
					result.Interface.H4 = uint32(h4)
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

func SplitWireGuardEndpoint(endpoint string) (host, port string) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", ""
	}
	const defaultPort = "51820"
	if strings.HasPrefix(endpoint, "[") {
		closeIdx := strings.Index(endpoint, "]")
		if closeIdx > 1 {
			host = endpoint[1:closeIdx]
			rest := strings.TrimSpace(endpoint[closeIdx+1:])
			if strings.HasPrefix(rest, ":") {
				port = strings.TrimPrefix(rest, ":")
			}
			if port == "" {
				port = defaultPort
			}
			return host, port
		}
	}
	if h, p, err := net.SplitHostPort(endpoint); err == nil {
		return h, p
	}
	if i := strings.LastIndex(endpoint, ":"); i > 0 {
		candidate := endpoint[i+1:]
		if _, err := strconv.Atoi(candidate); err == nil && candidate != "" {
			return endpoint[:i], candidate
		}
	}
	return endpoint, defaultPort
}
