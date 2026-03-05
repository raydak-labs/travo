package ubus

import "testing"

func TestMockUbusCallSystemBoard(t *testing.T) {
	m := NewMockUbus()
	resp, err := m.Call("system", "board", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp["hostname"] != "OpenWrt" {
		t.Errorf("expected hostname 'OpenWrt', got %v", resp["hostname"])
	}
	if resp["model"] != "GL.iNet GL-MT3000" {
		t.Errorf("expected model, got %v", resp["model"])
	}
}

func TestMockUbusCallSystemInfo(t *testing.T) {
	m := NewMockUbus()
	resp, err := m.Call("system", "info", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	uptime, ok := resp["uptime"].(float64)
	if !ok || uptime != 86400 {
		t.Errorf("expected uptime 86400, got %v", resp["uptime"])
	}
	mem, ok := resp["memory"].(map[string]interface{})
	if !ok {
		t.Fatal("expected memory map")
	}
	total, ok := mem["total"].(float64)
	if !ok || total != 1073741824 {
		t.Errorf("expected total 1073741824, got %v", mem["total"])
	}
}

func TestMockUbusCallNetworkWan(t *testing.T) {
	m := NewMockUbus()
	resp, err := m.Call("network.interface.wan", "status", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	up, ok := resp["up"].(bool)
	if !ok || !up {
		t.Errorf("expected wan up=true, got %v", resp["up"])
	}
}

func TestMockUbusCallIwinfoScan(t *testing.T) {
	m := NewMockUbus()
	resp, err := m.Call("iwinfo", "scan", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	results, ok := resp["results"].([]interface{})
	if !ok {
		t.Fatal("expected results array")
	}
	if len(results) < 3 {
		t.Errorf("expected at least 3 results, got %d", len(results))
	}
}

func TestMockUbusCallIwinfoInfo(t *testing.T) {
	m := NewMockUbus()
	resp, err := m.Call("iwinfo", "info", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp["ssid"] != "Hotel-WiFi" {
		t.Errorf("expected ssid 'Hotel-WiFi', got %v", resp["ssid"])
	}
}

func TestMockUbusCallUnknownPath(t *testing.T) {
	m := NewMockUbus()
	_, err := m.Call("nonexistent", "method", nil)
	if err == nil {
		t.Error("expected error for unknown path")
	}
}

func TestMockUbusRegisterResponse(t *testing.T) {
	m := NewMockUbus()
	m.RegisterResponse("custom.call", map[string]interface{}{"result": "ok"})
	resp, err := m.Call("custom", "call", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp["result"] != "ok" {
		t.Errorf("expected result 'ok', got %v", resp["result"])
	}
}
