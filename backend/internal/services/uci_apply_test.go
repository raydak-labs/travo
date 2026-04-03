package services

import (
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/auth"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
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

func TestRealUCIApplyConfirm_UsesPasswordFromHolder(t *testing.T) {
	mub := ubus.NewMockUbus()
	pw := auth.NewRootPassword()
	pw.Set("my-secret-password")

	applier := NewRealUCIApplyConfirm(mub, pw)

	// Verify the applier is wired to the password holder.
	// We can't call sessionLogin directly (unexported), but we can verify
	// that the constructor accepts and stores the password holder.
	if applier.password.Get() != "my-secret-password" {
		t.Errorf("expected applier to have 'my-secret-password', got %q", applier.password.Get())
	}
}

func TestRealUCIApplyConfirm_EmptyPasswordWhenNotSet(t *testing.T) {
	mub := ubus.NewMockUbus()
	pw := auth.NewRootPassword()

	applier := NewRealUCIApplyConfirm(mub, pw)

	if applier.password.Get() != "" {
		t.Errorf("expected empty password, got %q", applier.password.Get())
	}
}

func TestRootPassword_GetSet(t *testing.T) {
	pw := auth.NewRootPassword()

	if pw.Get() != "" {
		t.Error("expected empty password initially")
	}

	pw.Set("test123")
	if pw.Get() != "test123" {
		t.Errorf("expected 'test123', got %q", pw.Get())
	}

	pw.Set("newpass")
	if pw.Get() != "newpass" {
		t.Errorf("expected 'newpass', got %q", pw.Get())
	}
}
