package services

import (
	"os"
	"strings"
)

// VpnProtocol identifies a WireGuard-family protocol variant.
type VpnProtocol string

const (
	ProtoWireGuard VpnProtocol = "wireguard"
	ProtoAmneziaWG VpnProtocol = "amneziawg"
)

const (
	awgBin          = "/usr/bin/awg"
	awgProtoHelper  = "/lib/netifd/proto/amneziawg.sh"
	awgKernelModule = "/sys/module/amneziawg"
)

// AmneziaWGAvailability describes the readiness state of AmneziaWG on the device.
type AmneziaWGAvailability struct {
	Ready         bool   `json:"ready"`
	BinaryPresent bool   `json:"binary_present"`
	HelperPresent bool   `json:"helper_present"`
	KernelModule  bool   `json:"kernel_module"`
	Reason        string `json:"reason,omitempty"`
}

// CheckAmneziaWGAvailability probes the device for AWG package readiness.
func CheckAmneziaWGAvailability() AmneziaWGAvailability {
	result := AmneziaWGAvailability{}

	if _, err := os.Stat(awgBin); err == nil {
		result.BinaryPresent = true
	}
	if _, err := os.Stat(awgProtoHelper); err == nil {
		result.HelperPresent = true
	}
	if _, err := os.Stat(awgKernelModule); err == nil {
		result.KernelModule = true
	}

	result.Ready = result.BinaryPresent && result.HelperPresent
	if !result.Ready {
		var missing []string
		if !result.BinaryPresent {
			missing = append(missing, "awg binary")
		}
		if !result.HelperPresent {
			missing = append(missing, "netifd proto helper")
		}
		result.Reason = "missing: " + strings.Join(missing, ", ")
	}
	return result
}

// wgShowCommand returns the correct binary for `wg show` depending on protocol.
func wgShowCommand(proto VpnProtocol) string {
	if proto == ProtoAmneziaWG {
		return awgBin
	}
	return openwrtWgBin
}

// protoForConfig determines the protocol from parsed config.
func protoForConfig(parsed *WireguardParsedConfig) VpnProtocol {
	if parsed != nil && parsed.IsAmnezia() {
		return ProtoAmneziaWG
	}
	return ProtoWireGuard
}
