package network

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

// RunDiagnostics runs ping, traceroute, or DNS lookup and returns the output.
//
// The target becomes the final argv entry of a root-run command, so it is
// validated as a hostname or IP literal first: an unvalidated target of "-f"
// turned `ping` into a root flood of the whole internet, and "-w1"/"-s" style
// values rewrite its behaviour.
func (n *Service) RunDiagnostics(req models.DiagnosticsRequest) models.DiagnosticsResult {
	result := models.DiagnosticsResult{Type: req.Type, Target: req.Target}
	if err := validateDiagnosticTarget(req.Target); err != nil {
		result.Error = err.Error()
		return result
	}
	var out []byte
	var err error
	switch req.Type {
	case "ping":
		out, err = n.cmd.Run("ping", "-c", "4", "-W", "3", req.Target)
	case "traceroute":
		out, err = n.cmd.Run("traceroute", "-w", "3", "-q", "1", "-m", "20", req.Target)
	case "dns":
		out, err = n.cmd.Run("nslookup", req.Target)
	default:
		result.Error = "unknown diagnostic type"
		return result
	}
	if err != nil {
		result.Error = err.Error()
	}
	result.Output = string(out)
	return result
}

// validateDiagnosticTarget accepts a hostname or an IP literal and nothing else.
func validateDiagnosticTarget(target string) error {
	trimmed := strings.TrimSpace(target)
	if trimmed == "" {
		return errors.New("target is required")
	}
	if trimmed != target || strings.HasPrefix(target, "-") {
		return fmt.Errorf("invalid diagnostic target %q: must be a hostname or IP address", target)
	}
	if _, err := netip.ParseAddr(target); err == nil {
		return nil
	}
	for _, label := range strings.Split(target, ".") {
		if label == "" {
			return fmt.Errorf("invalid diagnostic target %q: must be a hostname or IP address", target)
		}
		for _, r := range label {
			isLetter := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
			isDigit := r >= '0' && r <= '9'
			if !isLetter && !isDigit && r != '-' && r != '_' {
				return fmt.Errorf("invalid diagnostic target %q: must be a hostname or IP address", target)
			}
		}
	}
	return nil
}

// SendWoL sends a Wake-on-LAN magic packet to the given MAC address.
func (n *Service) SendWoL(mac, iface string) error {
	args := []string{mac}
	if iface != "" {
		args = append([]string{"-i", iface}, args...)
	}
	_, err := n.cmd.Run("etherwake", args...)
	if err != nil {
		// fallback to ether-wake
		_, err = n.cmd.Run("ether-wake", args...)
	}
	return err
}
