package ubus

import "testing"

// Compile-time interface check.
var _ Ubus = (*RealUbus)(nil)

func TestValidateUbusArg(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"system", true},
		{"network.interface.wan", true},
		{"iwinfo", true},
		{"system.board", true},
		{"file.stat", true},
		{"radio0", true},
		{"", false},
		{"foo;bar", false},
		{"foo|bar", false},
		{"foo bar", false},
		{"foo&bar", false},
		{"foo`bar", false},
		{"foo$bar", false},
		{"foo'bar", false},
		{"foo\"bar", false},
		{"foo\nbar", false},
	}
	for _, tt := range tests {
		err := validateUbusArg("test", tt.input)
		if tt.valid && err != nil {
			t.Errorf("validateUbusArg(%q): unexpected error: %v", tt.input, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("validateUbusArg(%q): expected error", tt.input)
		}
	}
}

func TestRealUbusCallValidation(t *testing.T) {
	r := NewRealUbus()

	_, err := r.Call("system;rm", "board", nil)
	if err == nil {
		t.Error("expected validation error for path with semicolon")
	}

	_, err = r.Call("system", "board;rm", nil)
	if err == nil {
		t.Error("expected validation error for method with semicolon")
	}

	_, err = r.Call("foo bar", "method", nil)
	if err == nil {
		t.Error("expected validation error for path with space")
	}
}
