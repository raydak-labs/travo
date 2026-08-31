package ubus

import (
	"fmt"
	"sync"
)

// MockUbus implements the Ubus interface with pre-registered mock responses.
type MockUbus struct {
	mu        sync.RWMutex
	responses map[string]map[string]any
}

// NewMockUbus creates a MockUbus with realistic pre-registered responses.
func NewMockUbus() *MockUbus {
	m := &MockUbus{
		responses: make(map[string]map[string]any),
	}
	m.populate()
	return m
}

func (m *MockUbus) populate() {
	m.responses["system.board"] = map[string]any{
		"hostname":   "OpenWrt",
		"model":      "GL.iNet GL-MT3000",
		"board_name": "glinet,gl-mt3000",
		"release": map[string]any{
			"distribution": "OpenWrt",
			"version":      "23.05.2",
			"revision":     "r23630-842932a63d",
			"target":       "mediatek/filogic",
		},
		"kernel": "5.15.137",
	}

	m.responses["system.info"] = map[string]any{
		"uptime":    float64(86400),
		"localtime": float64(1700000000),
		"load":      []any{float64(4096), float64(6553), float64(9830)},
		"memory": map[string]any{
			"total":    float64(1073741824),
			"free":     float64(268435456),
			"shared":   float64(67108864),
			"buffered": float64(134217728),
			"cached":   float64(268435456),
		},
		"swap": map[string]any{
			"total": float64(0),
			"free":  float64(0),
		},
	}

	m.responses["network.interface.wan.status"] = map[string]any{
		"up": true, "pending": false, "available": true, "autostart": true,
		"device": "eth0", "l3_device": "eth0", "proto": "dhcp",
		"ipv4-address": []any{
			map[string]any{"address": "192.168.1.100", "mask": float64(24)},
		},
		"route": []any{
			map[string]any{"target": "0.0.0.0", "mask": float64(0), "nexthop": "192.168.1.1"},
		},
		"dns-server": []any{"8.8.8.8", "8.8.4.4"},
	}

	m.responses["network.interface.lan.status"] = map[string]any{
		"up": true, "pending": false, "available": true, "autostart": true,
		"device": "br-lan", "l3_device": "br-lan", "proto": "static",
		"ipv4-address": []any{
			map[string]any{"address": "192.168.8.1", "mask": float64(24)},
		},
	}

	m.responses["iwinfo.scan"] = map[string]any{
		"results": []any{
			map[string]any{
				"ssid": "Hotel-WiFi", "bssid": "AA:BB:CC:DD:EE:01",
				"channel": float64(6), "signal": float64(-45), "quality": float64(70),
				"encryption": map[string]any{"enabled": true, "wpa": []any{float64(2)}, "authentication": []any{"psk"}},
				"band":       "2g",
			},
			map[string]any{
				"ssid": "Airport-Free", "bssid": "AA:BB:CC:DD:EE:02",
				"channel": float64(11), "signal": float64(-65), "quality": float64(50),
				"encryption": map[string]any{"enabled": false},
				"band":       "2g",
			},
			map[string]any{
				"ssid": "CoffeeShop-5G", "bssid": "AA:BB:CC:DD:EE:03",
				"channel": float64(36), "signal": float64(-55), "quality": float64(60),
				"encryption": map[string]any{"enabled": true, "wpa": []any{float64(2)}, "authentication": []any{"psk"}},
				"band":       "5g",
			},
			map[string]any{
				"ssid": "Neighbor-Net", "bssid": "AA:BB:CC:DD:EE:04",
				"channel": float64(1), "signal": float64(-80), "quality": float64(25),
				"encryption": map[string]any{"enabled": true, "wpa": []any{float64(2)}, "authentication": []any{"psk"}},
				"band":       "2g",
			},
		},
	}

	m.responses["iwinfo.info"] = map[string]any{
		"ssid": "Hotel-WiFi", "bssid": "AA:BB:CC:DD:EE:01",
		"mode": "Client", "channel": float64(6),
		"signal": float64(-45), "quality": float64(70),
		"noise": float64(-95), "encryption": "WPA2 PSK (CCMP)", "band": "2g",
	}

	m.responses["dhcp.ipv4leases"] = map[string]any{
		"device": map[string]any{
			"br-lan": map[string]any{
				"leases": []any{
					map[string]any{
						"mac":      "AA:BB:CC:11:22:33",
						"hostname": "laptop",
						"ip":       "192.168.8.100",
						"expires":  float64(43200),
					},
					map[string]any{
						"mac":      "AA:BB:CC:44:55:66",
						"hostname": "phone",
						"ip":       "192.168.8.101",
						"expires":  float64(43200),
					},
				},
			},
		},
	}

	m.responses["file.stat./usr/bin/wireguard"] = map[string]any{
		"path": "/usr/bin/wireguard", "type": "regular", "size": float64(102400),
	}

	m.responses["system.reboot"] = map[string]any{}

	m.responses["network.wireless.status"] = map[string]any{
		"radio0": map[string]any{
			"interfaces": []any{
				map[string]any{
					"ifname":  "phy0-sta0",
					"section": "wifinet2",
					"config": map[string]any{
						"mode":       "sta",
						"ssid":       "Hotel-WiFi",
						"encryption": "psk2",
					},
				},
			},
		},
	}

	m.responses["network.interface.wwan.status"] = map[string]any{
		"up": true,
		"ipv4-address": []any{
			map[string]any{"address": "10.0.0.50", "mask": float64(24)},
		},
	}
}

// Call invokes a mock ubus method.
func (m *MockUbus) Call(path, method string, _ map[string]any) (map[string]any, error) {
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
func (m *MockUbus) RegisterResponse(key string, response map[string]any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses[key] = response
}
