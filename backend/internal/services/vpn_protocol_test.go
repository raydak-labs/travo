package services

import "testing"

func TestProtoForConfig_StandardWG(t *testing.T) {
	parsed := &WireguardParsedConfig{
		Interface: WireguardInterface{PrivateKey: "key"},
	}
	if got := protoForConfig(parsed); got != ProtoWireGuard {
		t.Errorf("protoForConfig = %q, want %q", got, ProtoWireGuard)
	}
}

func TestProtoForConfig_AmneziaWG(t *testing.T) {
	parsed := &WireguardParsedConfig{
		Interface: WireguardInterface{PrivateKey: "key", Jc: 5, S1: 10},
	}
	if got := protoForConfig(parsed); got != ProtoAmneziaWG {
		t.Errorf("protoForConfig = %q, want %q", got, ProtoAmneziaWG)
	}
}

func TestProtoForConfig_Nil(t *testing.T) {
	if got := protoForConfig(nil); got != ProtoWireGuard {
		t.Errorf("protoForConfig(nil) = %q, want %q", got, ProtoWireGuard)
	}
}

func TestWgShowCommand(t *testing.T) {
	if got := wgShowCommand(ProtoWireGuard); got != openwrtWgBin {
		t.Errorf("wgShowCommand(WireGuard) = %q, want %q", got, openwrtWgBin)
	}
	if got := wgShowCommand(ProtoAmneziaWG); got != awgBin {
		t.Errorf("wgShowCommand(AmneziaWG) = %q, want %q", got, awgBin)
	}
}

func TestCheckAmneziaWGAvailability_MissingAll(t *testing.T) {
	// On a dev machine without AWG installed, expect not ready
	avail := CheckAmneziaWGAvailability()
	if avail.Ready {
		t.Skip("AWG appears to be installed on this system")
	}
	if avail.Reason == "" {
		t.Error("expected a reason when not ready")
	}
}
