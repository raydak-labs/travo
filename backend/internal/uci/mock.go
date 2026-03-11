package uci

import (
	"fmt"
	"sync"
)

// MockUCI implements the UCI interface with an in-memory store.
type MockUCI struct {
	mu   sync.RWMutex
	data map[string]map[string]map[string]string
}

// NewMockUCI creates a MockUCI pre-populated with realistic OpenWRT configuration.
func NewMockUCI() *MockUCI {
	m := &MockUCI{
		data: make(map[string]map[string]map[string]string),
	}
	m.populate()
	return m
}

func (m *MockUCI) populate() {
	m.setInternal("network", "wan", "proto", "dhcp")
	m.setInternal("network", "wan", "ifname", "eth0")
	m.setInternal("network", "wan", "ip4addr", "192.168.1.100")
	m.setInternal("network", "wan", "netmask", "255.255.255.0")
	m.setInternal("network", "wan", "gateway", "192.168.1.1")
	m.setInternal("network", "wan", "dns", "8.8.8.8 8.8.4.4")
	m.setInternal("network", "wan", "mtu", "1500")
	m.setInternal("network", "lan", "proto", "static")
	m.setInternal("network", "lan", "ifname", "br-lan")
	m.setInternal("network", "lan", "ipaddr", "192.168.8.1")
	m.setInternal("network", "lan", "netmask", "255.255.255.0")
	m.setInternal("wireless", "radio0", "type", "mac80211")
	m.setInternal("wireless", "radio0", "channel", "6")
	m.setInternal("wireless", "radio0", "band", "2g")
	m.setInternal("wireless", "radio0", "htmode", "HT20")
	m.setInternal("wireless", "radio0", "disabled", "0")
	m.setInternal("wireless", "radio1", "type", "mac80211")
	m.setInternal("wireless", "radio1", "channel", "36")
	m.setInternal("wireless", "radio1", "band", "5g")
	m.setInternal("wireless", "radio1", "htmode", "VHT80")
	m.setInternal("wireless", "radio1", "disabled", "0")
	m.setInternal("wireless", "default_radio0", "device", "radio0")
	m.setInternal("wireless", "default_radio0", "mode", "ap")
	m.setInternal("wireless", "default_radio0", "ssid", "OpenWrt-Travel")
	m.setInternal("wireless", "default_radio0", "encryption", "psk2")
	m.setInternal("wireless", "default_radio0", "key", "travel12345")
	m.setInternal("wireless", "default_radio1", "device", "radio1")
	m.setInternal("wireless", "default_radio1", "mode", "ap")
	m.setInternal("wireless", "default_radio1", "ssid", "OpenWrt-Travel-5G")
	m.setInternal("wireless", "default_radio1", "encryption", "psk2")
	m.setInternal("wireless", "default_radio1", "key", "travel12345")
	m.setInternal("wireless", "sta0", "device", "radio0")
	m.setInternal("wireless", "sta0", "mode", "sta")
	m.setInternal("wireless", "sta0", "ssid", "Hotel-WiFi")
	m.setInternal("wireless", "sta0", "encryption", "psk2")
	m.setInternal("wireless", "sta0", "key", "hotelpass")
	m.setInternal("wireless", "sta0", "disabled", "0")
	m.setInternal("system", "system", "hostname", "OpenWrt")
	m.setInternal("system", "system", "timezone", "UTC0")
	m.setInternal("system", "system", "zonename", "UTC")
	m.setInternal("dhcp", "lan", "interface", "lan")
	m.setInternal("dhcp", "lan", "start", "100")
	m.setInternal("dhcp", "lan", "limit", "150")
	m.setInternal("dhcp", "lan", "leasetime", "12h")
	m.setInternal("firewall", "defaults", "syn_flood", "1")
	m.setInternal("firewall", "defaults", "input", "ACCEPT")
	m.setInternal("firewall", "defaults", "output", "ACCEPT")
	m.setInternal("firewall", "defaults", "forward", "REJECT")
	m.setInternal("network", "wg0", "proto", "wireguard")
	m.setInternal("network", "wg0", "private_key", "mock_private_key_base64")
	m.setInternal("network", "wg0", "addresses", "10.0.0.2/24")
	m.setInternal("network", "wg0", "dns", "1.1.1.1")
	m.setInternal("network", "wg0", "disabled", "1")
	m.setInternal("network", "wg0_peer0", "public_key", "mock_peer_public_key_base64")
	m.setInternal("network", "wg0_peer0", "endpoint", "vpn.example.com:51820")
	m.setInternal("network", "wg0_peer0", "allowed_ips", "0.0.0.0/0")
}

func (m *MockUCI) setInternal(config, section, option, value string) {
	if m.data[config] == nil {
		m.data[config] = make(map[string]map[string]string)
	}
	if m.data[config][section] == nil {
		m.data[config][section] = make(map[string]string)
	}
	m.data[config][section][option] = value
}

func (m *MockUCI) Get(config, section, option string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if c, ok := m.data[config]; ok {
		if s, ok := c[section]; ok {
			if v, ok := s[option]; ok {
				return v, nil
			}
		}
	}
	return "", fmt.Errorf("uci: entry not found %s.%s.%s", config, section, option)
}

func (m *MockUCI) Set(config, section, option, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data[config] == nil {
		m.data[config] = make(map[string]map[string]string)
	}
	if m.data[config][section] == nil {
		m.data[config][section] = make(map[string]string)
	}
	m.data[config][section][option] = value
	return nil
}

func (m *MockUCI) GetAll(config, section string) (map[string]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if c, ok := m.data[config]; ok {
		if s, ok := c[section]; ok {
			result := make(map[string]string, len(s))
			for k, v := range s {
				result[k] = v
			}
			return result, nil
		}
	}
	return nil, fmt.Errorf("uci: section not found %s.%s", config, section)
}

func (m *MockUCI) Commit(_ string) error {
	return nil
}

func (m *MockUCI) AddSection(config, section, stype string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data[config] == nil {
		m.data[config] = make(map[string]map[string]string)
	}
	if m.data[config][section] != nil {
		return fmt.Errorf("uci: section %s.%s already exists", config, section)
	}
	m.data[config][section] = map[string]string{".type": stype}
	return nil
}

func (m *MockUCI) DeleteSection(config, section string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.data[config]; ok {
		if _, ok := c[section]; ok {
			delete(c, section)
			return nil
		}
	}
	return fmt.Errorf("uci: section not found %s.%s", config, section)
}

func (m *MockUCI) GetSections(config string) (map[string]map[string]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.data[config]
	if !ok {
		return map[string]map[string]string{}, nil
	}
	result := make(map[string]map[string]string, len(c))
	for section, opts := range c {
		result[section] = make(map[string]string, len(opts))
		for k, v := range opts {
			result[section][k] = v
		}
	}
	return result, nil
}
