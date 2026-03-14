package services

import (
	"testing"
)

func TestExtractSessionID(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]interface{}
		want string
	}{
		{"top-level", map[string]interface{}{"ubus_rpc_session": "abc123"}, "abc123"},
		{"empty", map[string]interface{}{}, ""},
		{"result-array", map[string]interface{}{
			"result": []interface{}{0, map[string]interface{}{"ubus_rpc_session": "sid456"}},
		}, "sid456"},
		{"result-array-no-second", map[string]interface{}{
			"result": []interface{}{0},
		}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSessionID(tt.m)
			if got != tt.want {
				t.Errorf("extractSessionID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNoopUCIApplyConfirm_ApplyAndConfirm(t *testing.T) {
	var n NoopUCIApplyConfirm
	if err := n.ApplyAndConfirm([]string{"wireless", "network"}); err != nil {
		t.Errorf("NoopUCIApplyConfirm.ApplyAndConfirm() error = %v", err)
	}
}
