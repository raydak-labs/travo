package ubus

import (
	"fmt"
	"sync"
)

// MockUbus implements the Ubus interface with pre-registered mock responses.
type MockUbus struct {
	mu        sync.RWMutex
	responses map[string]map[string]interface{}
}

// NewMockUbus creates a MockUbus with realistic pre-registered responses.
func NewMockUbus() *MockUbus {
	m := &MockUbus{
		responses: make(map[string]map[string]interface{}),
	}
	m.populate()
	return m
}

func (m *MockUbus) populate() {
	m.responses["system.board"] = map[string]interface{}{
		"hostname":   "OpenWrt",
		"model":      "GL.iNet GL-MT3000",
		"board_name": "glinet,gl-mt3000",
		"release": map[string]interface{}{
			"distribution": "OpenWrt",
			"version":      "23.05.2",
			"revision":     "r23630-842932a63d",
			"target":       "mediatek/filogic",
		},
		"kernel": "5.15.137",
	}

	m.responses["system.info"] = map[string]interface{}{
		"uptime":    float64(86400),
		"localtime": float64(1700000000),
		"load":      []interface{}{float64(4096), float64(6553), float64(9830)},
		"memory": map[string]interface{}{
			"total":    float64(1073741824),
			"free":     float64(268435456),
			"shared":   float64(67108864),
			"buffered": float64(134217728),
			"cached":   float64(268435456),
		},
		"swap": map[string]interface{}{
			"total": float64(0),
			"free":  float64(0),
		},
	}

	m.responses["network.interface.wan.status"] = map[string]interface{}{
		"up": true, "pending": false, "available": true, "autostart": true,
		"device": "eth0", "l3_device": "eth0", "proto": "dhcp",
		"ipv4-address": []interface{}{
			map[string]interface{}{"address": "192.168.1.100", "mask": float64(24)},
		},
		"route": []interface{}{
			map[string]interface{}{"target": "0.0.0.0", "mask": float64(0), "nexthop": "192.168.1.1"},
		},
		"dns-server": []interface{}{"8.8.8.8", "8.8.4.4"},
	}

	m.responses["network.interface.lan.status"] = map[string]interface{}{
		"up": true, "pending": false, "available": true, "autostart": true,
		"device": "br-lan", "l3_device": "br-lan", "proto": "static",
		"ipv4-address": []interface{}{
			map[string]interface{}{"address": "192.168.8.1", "mask": float64(24)},
		},
	}

	m.responses["iwinfo.scan"] = map[string]interface{}{
		"results": []interface{}{
			map[string]interface{}{
				"ssid": "Hotel-WiFi", "bssid": "AA:BB:CC:DD:EE:01",
				"channel": float64(6), "signal": float64(-45), "quality": float64(70),
				"encryption": map[string]interface{}{"enabled": true, "wpa": []interface{}{float64(2)}, "authentication": []interface{}{"psk"}},
				"band":       "2g",
			},
			map[string]interface{}{
				"ssid": "Airport-Free", "bssid": "AA:BB:CC:DD:EE:02",
				"channel": float64(11), "signal": float64(-65), "quality": float64(50),
				"encryption": map[string]interface{}{"enabled": false},
				"band":       "2g",
			},
			map[string]interface{}{
				"ssid": "CoffeeShop-5G", "bssid": "AA:BB:CC:DD:EE:03",
				"channel": float64(36), "signal": float64(-55), "quality": float64(60),
				"encryption": map[string]interface{}{"enabled": true, "wpa": []interface{}{float64(2)}, "authentication": []interface{}{"psk"}},
				"band":       "5g",
			},
			map[string]interface{}{
				"ssid": "Neighbor-Net", "bssid": "AA:BB:CC:DD:EE:04",
				"channel": float64(1), "signal": float64(-80), "quality": float64(25),
				"encryption": map[string]interface{}{"enabled": true, "wpa": []interface{}{float64(2)}, "authentication": []interface{}{"psk"}},
				"band":       "2g",
			},
		},
	}

	m.responses["iwinfo.info"] = map[string]interface{}{
		"ssid": "Hotel-WiFi", "bssid": "AA:BB:CC:DD:EE:01",
		"mode": "Client", "channel": float64(6),
		"signal": float64(-45), "quality": float64(70),
		"noise": float64(-95), "encryption": "WPA2 PSK (CCMP)", "band": "2g",
	}

	m.responses["dhcp.ipv4leases"] = map[string]interface{}{
		"device": map[string]interface{}{
			"br-lan": map[string]interface{}{
				"leases": []interface{}{
					map[string]interface{}{
						"mac":      "AA:BB:CC:11:22:33",
						"hostname": "laptop",
						"ip":       "192.168.8.100",
						"expires":  float64(43200),
					},
					map[string]interface{}{
						"mac":      "AA:BB:CC:44:55:66",
						"hostname": "phone",
						"ip":       "192.168.8.101",
						"expires":  float64(43200),
					},
				},
			},
		},
	}

	m.responses["file.stat./usr/bin/wireguard"] = map[string]interface{}{
		"path": "/usr/bin/wireguard", "type": "regular", "size": float64(102400),
	}

	m.responses["system.reboot"] = map[string]interface{}{}
}

// Call invokes a mock ubus method.
func (m *MockUbus) Call(path, method string, _ map[string]interface{}) (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := path + "." + method
	if resp, ok := m.responses[key]; ok {
		return resp, nil
	}
	if resp, ok := m.responses[path]; ok {
		return resp, nil
	}
	return nil, fmt.Errorf("ubus: unknown path/method %s %s", path, method)
}

// RegisterResponse allows tests to register custom responses.
func (m *MockUbus) RegisterResponse(key string, response map[string]interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses[key] = response
}
