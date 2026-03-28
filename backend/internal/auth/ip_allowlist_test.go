package auth

import (
	"net"
	"testing"
)

func TestParseCIDRList_Empty(t *testing.T) {
	nets, err := ParseCIDRList("")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(nets) != 0 {
		t.Fatalf("expected no nets, got %d", len(nets))
	}
}

func TestParseCIDRList_IPWithoutMask(t *testing.T) {
	nets, err := ParseCIDRList("203.0.113.10")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(nets) != 1 {
		t.Fatalf("expected 1 net, got %d", len(nets))
	}
	if !nets[0].Contains(net.ParseIP("203.0.113.10")) {
		t.Fatalf("expected CIDR to contain IP")
	}
}

func TestIPAllowed_LoopbackAlways(t *testing.T) {
	_, n, err := net.ParseCIDR("203.0.113.0/24")
	if err != nil {
		t.Fatal(err)
	}
	if !IPAllowed([]*net.IPNet{n}, net.ParseIP("127.0.0.1")) {
		t.Fatalf("expected loopback allowed")
	}
}

func TestIPAllowed_CIDRMatch(t *testing.T) {
	_, n, err := net.ParseCIDR("192.168.8.0/24")
	if err != nil {
		t.Fatal(err)
	}
	if !IPAllowed([]*net.IPNet{n}, net.ParseIP("192.168.8.1")) {
		t.Fatalf("expected LAN IP allowed")
	}
	if IPAllowed([]*net.IPNet{n}, net.ParseIP("10.0.0.1")) {
		t.Fatalf("expected non-matching IP denied")
	}
}
