package api

import "testing"

func TestIsValidIPv4(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"192.168.1.1", true},
		{"10.0.0.1", true},
		{"255.255.255.255", true},
		{"0.0.0.0", true},
		{"8.8.8.8", true},
		{"", false},
		{"not-an-ip", false},
		{"256.1.1.1", false},
		{"192.168.1", false},
		{"192.168.1.1.1", false},
		{"192.168.1.999", false},
		{"::1", false},        // IPv6 not valid IPv4
		{"1.2.3.4/24", false}, // CIDR not valid as plain IP
		{"192.168.1.1:80", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isValidIPv4(tt.input)
			if got != tt.want {
				t.Errorf("isValidIPv4(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsValidCIDR(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"10.0.0.0/24", true},
		{"0.0.0.0/0", true},
		{"192.168.1.0/32", true},
		{"10.0.0.0/8", true},
		{"fd00::/64", true}, // IPv6 CIDR is also valid CIDR
		{"", false},
		{"10.0.0.0", false}, // missing mask
		{"10.0.0.0/33", false},
		{"not-cidr/24", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isValidCIDR(tt.input)
			if got != tt.want {
				t.Errorf("isValidCIDR(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsValidMTU(t *testing.T) {
	tests := []struct {
		input int
		want  bool
	}{
		{1500, true},
		{68, true},
		{9000, true},
		{576, true},
		{67, false},
		{9001, false},
		{0, false},
		{-1, false},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := isValidMTU(tt.input)
			if got != tt.want {
				t.Errorf("isValidMTU(%d) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsValidBase64Key(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid 44-char key", "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY=", true},
		{"empty", "", false},
		{"too short", "YWJj", false},
		{"not base64", "!!!notbase64notbase64notbase64notbase64no!=", false},
		{"43 chars", "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NT=", false},
		{"45 chars", "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY0=", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidBase64Key(tt.input)
			if got != tt.want {
				t.Errorf("isValidBase64Key(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsValidEndpoint(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"vpn.example.com:51820", true},
		{"1.2.3.4:51820", true},
		{"192.168.1.1:1", true},
		{"my-host.example.com:65535", true},
		{"", false},
		{"host-only", false},
		{"host:", false},
		{":51820", false},
		{"host:0", false},
		{"host:65536", false},
		{"host:notanumber", false},
		{"host:-1", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isValidEndpoint(tt.input)
			if got != tt.want {
				t.Errorf("isValidEndpoint(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsValidPort(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"80", true},
		{"1", true},
		{"65535", true},
		{"51820", true},
		{"0", false},
		{"65536", false},
		{"", false},
		{"abc", false},
		{"-1", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isValidPort(tt.input)
			if got != tt.want {
				t.Errorf("isValidPort(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
