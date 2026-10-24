package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// captiveWwanBounceGuardPath is the crash guard for the wwan DHCP bounce. It is a
// variable (not a constant) so tests can redirect it; production code must never
// leave it set after a successful bounce (ADR 0003).
var captiveWwanBounceGuardPath = crashGuardDir + "/captive-wwan-bounce-in-progress"

// bounceActiveWwanDHCP renews the wwan DHCP lease, but only when wwan is the
// active uplink. It is a live network mutation, so it writes a crash guard
// before ifdown/ifup and removes it only after the lease came back. Every step
// error is returned; a failed bounce leaves the guard in place because the
// interface may be half down.
func (c *CaptiveService) bounceActiveWwanDHCP() error {
	active, err := c.wwanIsActiveUplink()
	if err != nil {
		return err
	}
	if !active {
		// wwan is not carrying the default route: bouncing it would only
		// disturb an unrelated interface.
		return nil
	}
	if err := writeCaptiveBounceGuard(captiveWwanBounceGuardPath); err != nil {
		return err
	}
	if _, err := c.cmd.Run(openwrtIfdownBin, "wwan"); err != nil {
		return fmt.Errorf("ifdown wwan: %w", err)
	}
	time.Sleep(captiveWwanDownSettle)
	if _, err := c.cmd.Run(openwrtIfupBin, "wwan"); err != nil {
		return fmt.Errorf("ifup wwan: %w", err)
	}
	if err := c.waitForWwanLease(captiveWwanUpSettle); err != nil {
		return err
	}
	_ = os.Remove(captiveWwanBounceGuardPath)
	return nil
}

// wwanIsActiveUplink reports whether netifd currently has wwan up with an IPv4
// default route. Errors are returned so callers do not guess.
func (c *CaptiveService) wwanIsActiveUplink() (bool, error) {
	out, err := c.cmd.Run(captiveUbusBin, "-S", "call", "network.interface", "dump")
	if err != nil {
		return false, fmt.Errorf("ubus network.interface dump: %w", err)
	}
	return ubusInterfaceDumpHasDefaultRoute(out, "wwan"), nil
}

// waitForWwanLease polls the wwan interface status until it has an IPv4 address
// or the timeout expires, so the handler does not sleep for a fixed 8s.
func (c *CaptiveService) waitForWwanLease(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if out, err := c.cmd.Run(captiveUbusBin, "call", "network.interface.wwan", "status"); err == nil {
			if ubusInterfaceStatusHasAddress(out) {
				return nil
			}
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("wwan did not obtain a lease within %s", timeout)
		}
		time.Sleep(captiveWwanPollInterval)
	}
}

// ubusInterfaceDumpHasDefaultRoute reports whether the named netifd interface
// is up and owns an IPv4 default route.
func ubusInterfaceDumpHasDefaultRoute(out []byte, name string) bool {
	var raw map[string]any
	if err := json.Unmarshal(out, &raw); err != nil {
		return false
	}
	ifaces, ok := raw["interface"].([]any)
	if !ok {
		return false
	}
	for _, entry := range ifaces {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if ifaceName, _ := m["interface"].(string); ifaceName != name {
			continue
		}
		up, _ := m["up"].(bool)
		if !up {
			return false
		}
		routes, _ := m["route"].([]any)
		for _, r := range routes {
			rm, ok := r.(map[string]any)
			if !ok {
				continue
			}
			target, _ := rm["target"].(string)
			mask, _ := rm["mask"].(float64)
			if target == "0.0.0.0" && int(mask) == 0 {
				return true
			}
		}
		return false
	}
	return false
}

// ubusInterfaceStatusHasAddress reports whether an interface status reply
// contains an IPv4 address.
func ubusInterfaceStatusHasAddress(out []byte) bool {
	var raw map[string]any
	if err := json.Unmarshal(out, &raw); err != nil {
		return false
	}
	addrs, ok := raw["ipv4-address"].([]any)
	return ok && len(addrs) > 0
}

// writeCaptiveBounceGuard creates the crash guard file for the wwan bounce.
func writeCaptiveBounceGuard(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return fmt.Errorf("create wwan bounce guard dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(time.Now().Format(time.RFC3339Nano)), 0600); err != nil {
		return fmt.Errorf("write wwan bounce guard: %w", err)
	}
	return nil
}
