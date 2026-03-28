package services

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/openwrt-travel-gui/backend/internal/ubus"
)

// UCIApplyConfirm stages UCI config changes using rpcd's apply+confirm flow
// (same as LuCI Save & Apply), so a rollback window exists and the device
// can revert if it crashes before confirm. User-driven wireless flows should
// start apply first and confirm only after the browser proves the router is
// still reachable on the new settings.
type UCIApplyConfirm interface {
	// StartApply commits staged UCI by: session login, copy configs to session
	// dir, uci apply (rollback timeout). Returns the rpcd session ID used for
	// later confirm. Configs are names like "wireless", "network", "system".
	StartApply(configs []string) (string, error)
	// Confirm finalizes a previously started apply session.
	Confirm(sessionID string) error
	// ApplyAndConfirm is retained for guarded internal flows that still need a
	// synchronous apply on the router itself.
	ApplyAndConfirm(configs []string) error
}

const (
	uciApplyRollbackTimeout = 30
	etcConfigDir            = "/etc/config"
	rpcdRunDir              = "/var/run/rpcd"
)

// RealUCIApplyConfirm uses ubus session + rpcd uci apply/confirm.
type RealUCIApplyConfirm struct {
	ubus ubus.Ubus
}

// NewRealUCIApplyConfirm returns a real applier that uses the given ubus.
func NewRealUCIApplyConfirm(ub ubus.Ubus) *RealUCIApplyConfirm {
	return &RealUCIApplyConfirm{ubus: ub}
}

// StartApply stages an rpcd rollback apply and returns the session ID.
func (r *RealUCIApplyConfirm) StartApply(configs []string) (string, error) {
	if len(configs) == 0 {
		return "", nil
	}
	sid, err := r.sessionLogin()
	if err != nil || sid == "" {
		return "", fmt.Errorf("uci apply: no session (login failed): %w", err)
	}
	sessionDir := filepath.Join(rpcdRunDir, "uci-"+sid)
	if err := os.MkdirAll(sessionDir, 0700); err != nil {
		return "", fmt.Errorf("uci apply: mkdir session dir: %w", err)
	}
	for _, name := range configs {
		src := filepath.Join(etcConfigDir, name)
		dst := filepath.Join(sessionDir, name)
		if err := copyFile(src, dst); err != nil {
			return "", fmt.Errorf("uci apply: copy %s: %w", name, err)
		}
	}
	applyArgs := map[string]interface{}{
		"ubus_rpc_session": sid,
		"rollback":         true,
		"timeout":          uciApplyRollbackTimeout,
	}
	if _, err := r.ubus.Call("uci", "apply", applyArgs); err != nil {
		return "", fmt.Errorf("uci apply: %w", err)
	}
	return sid, nil
}

// Confirm finalizes a previously started apply session.
func (r *RealUCIApplyConfirm) Confirm(sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("uci confirm: empty session id")
	}
	confirmArgs := map[string]interface{}{
		"ubus_rpc_session": sessionID,
	}
	if _, err := r.ubus.Call("uci", "confirm", confirmArgs); err != nil {
		return fmt.Errorf("uci confirm: %w", err)
	}
	return nil
}

// ApplyAndConfirm implements UCIApplyConfirm.
func (r *RealUCIApplyConfirm) ApplyAndConfirm(configs []string) error {
	sid, err := r.StartApply(configs)
	if err != nil {
		return err
	}
	return r.Confirm(sid)
}

func (r *RealUCIApplyConfirm) sessionLogin() (string, error) {
	args := map[string]interface{}{
		"username": "root",
		"password": "",
	}
	resp, err := r.ubus.Call("session", "login", args)
	if err != nil {
		return "", err
	}
	return extractSessionID(resp), nil
}

// extractSessionID finds ubus_rpc_session in the login response.
// Handles top-level or result-array format.
func extractSessionID(m map[string]interface{}) string {
	if s, _ := m["ubus_rpc_session"].(string); s != "" {
		return s
	}
	arr, ok := m["result"].([]interface{})
	if !ok || len(arr) < 2 {
		return ""
	}
	obj, ok := arr[1].(map[string]interface{})
	if !ok {
		return ""
	}
	s, _ := obj["ubus_rpc_session"].(string)
	return s
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // skip missing config (same as setup script)
		}
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Sync()
}

// NoopUCIApplyConfirm does nothing (for tests).
type NoopUCIApplyConfirm struct{}

// StartApply is a no-op.
func (NoopUCIApplyConfirm) StartApply(_ []string) (string, error) {
	return "", nil
}

// Confirm is a no-op.
func (NoopUCIApplyConfirm) Confirm(_ string) error {
	return nil
}

// ApplyAndConfirm is a no-op.
func (NoopUCIApplyConfirm) ApplyAndConfirm(_ []string) error {
	return nil
}
