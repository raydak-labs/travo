package uci

import "testing"

// Compile-time interface check.
var _ UCI = (*RealUCI)(nil)

func TestValidateIdentifier(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"network", true},
		{"wan", true},
		{"radio0", true},
		{"default_radio0", true},
		{"proto", true},
		{"wg0_peer0", true},
		{"", false},
		{"foo bar", false},
		{"foo;bar", false},
		{"foo|bar", false},
		{"foo&bar", false},
		{"foo`bar", false},
		{"foo$bar", false},
		{"foo'bar", false},
		{"foo\"bar", false},
		{"foo\nbar", false},
		{"foo.bar", false},
		{"foo/bar", false},
		{"../etc/passwd", false},
	}
	for _, tt := range tests {
		err := validateIdentifier("test", tt.input)
		if tt.valid && err != nil {
			t.Errorf("validateIdentifier(%q): unexpected error: %v", tt.input, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("validateIdentifier(%q): expected error", tt.input)
		}
	}
}

func TestParseShowOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name:  "single option",
			input: "network.wan.proto='dhcp'\n",
			expected: map[string]string{
				"proto": "dhcp",
			},
		},
		{
			name:  "multiple options",
			input: "network.wan.proto='dhcp'\nnetwork.wan.ifname='eth0'\nnetwork.wan.mtu='1500'\n",
			expected: map[string]string{
				"proto":  "dhcp",
				"ifname": "eth0",
				"mtu":    "1500",
			},
		},
		{
			name:  "unquoted value",
			input: "network.wan.proto=dhcp\n",
			expected: map[string]string{
				"proto": "dhcp",
			},
		},
		{
			name:     "empty output",
			input:    "",
			expected: map[string]string{},
		},
		{
			name:  "value with spaces",
			input: "network.wan.dns='8.8.8.8 8.8.4.4'\n",
			expected: map[string]string{
				"dns": "8.8.8.8 8.8.4.4",
			},
		},
		{
			name:  "section type line skipped",
			input: "network.wan=interface\nnetwork.wan.proto='dhcp'\n",
			expected: map[string]string{
				"proto": "dhcp",
			},
		},
		{
			name:  "value containing equals",
			input: "network.wan.foo='bar=baz'\n",
			expected: map[string]string{
				"foo": "bar=baz",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseShowOutput(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d entries, got %d: %v", len(tt.expected), len(result), result)
				return
			}
			for k, v := range tt.expected {
				if got := result[k]; got != v {
					t.Errorf("key %q: expected %q, got %q", k, v, got)
				}
			}
		})
	}
}

func TestRealUCIGetValidation(t *testing.T) {
	r := NewRealUCI()

	_, err := r.Get("net;work", "wan", "proto")
	if err == nil {
		t.Error("expected validation error for config with semicolon")
	}

	_, err = r.Get("network", "wan;drop", "proto")
	if err == nil {
		t.Error("expected validation error for section with semicolon")
	}

	_, err = r.Get("network", "wan", "proto|rm")
	if err == nil {
		t.Error("expected validation error for option with pipe")
	}
}

func TestRealUCISetValidation(t *testing.T) {
	r := NewRealUCI()

	err := r.Set("net;work", "wan", "proto", "dhcp")
	if err == nil {
		t.Error("expected validation error for config with semicolon")
	}

	err = r.Set("network", "wan;drop", "proto", "dhcp")
	if err == nil {
		t.Error("expected validation error for section with semicolon")
	}

	err = r.Set("network", "wan", "proto|rm", "dhcp")
	if err == nil {
		t.Error("expected validation error for option with pipe")
	}
}

func TestRealUCIGetAllValidation(t *testing.T) {
	r := NewRealUCI()

	_, err := r.GetAll("net;work", "wan")
	if err == nil {
		t.Error("expected validation error")
	}

	_, err = r.GetAll("network", "wan;drop")
	if err == nil {
		t.Error("expected validation error")
	}
}

func TestRealUCICommitValidation(t *testing.T) {
	r := NewRealUCI()

	err := r.Commit("net;work")
	if err == nil {
		t.Error("expected validation error")
	}
}

func TestRealUCIAddSectionValidation(t *testing.T) {
	r := NewRealUCI()

	err := r.AddSection("net;work", "wan", "interface")
	if err == nil {
		t.Error("expected validation error for config")
	}

	err = r.AddSection("network", "wan;x", "interface")
	if err == nil {
		t.Error("expected validation error for section")
	}

	err = r.AddSection("network", "wan", "inter;face")
	if err == nil {
		t.Error("expected validation error for stype")
	}
}

func TestRealUCIDeleteSectionValidation(t *testing.T) {
	r := NewRealUCI()

	err := r.DeleteSection("net;work", "wan")
	if err == nil {
		t.Error("expected validation error for config")
	}

	err = r.DeleteSection("network", "wan;x")
	if err == nil {
		t.Error("expected validation error for section")
	}
}
