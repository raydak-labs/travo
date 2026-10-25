package wifi

import (
	"testing"
)

func TestParseIwinfoEncryption(t *testing.T) {
	cases := []struct {
		name  string
		input any
		want  string
	}{
		{"nil", nil, "none"},
		{"empty map disabled", map[string]any{"enabled": false}, "none"},
		{"wpa2 psk", map[string]any{
			"enabled":        true,
			"wpa":            []any{float64(2)},
			"authentication": []any{"psk"},
		}, "psk2"},
		{"wpa psk", map[string]any{
			"enabled":        true,
			"wpa":            []any{float64(1)},
			"authentication": []any{"psk"},
		}, "psk"},
		{"wpa3 sae", map[string]any{
			"enabled":        true,
			"wpa":            []any{float64(3)},
			"authentication": []any{"sae"},
		}, "sae"},
		{"wep (no wpa key)", map[string]any{
			"enabled": true,
		}, "wep"},
		{"plain string fallback", "psk2", "psk2"},
		{"empty string fallback", "", "none"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseIwinfoEncryption(tc.input)
			if got != tc.want {
				t.Errorf("parseIwinfoEncryption(%v) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
