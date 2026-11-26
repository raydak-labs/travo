package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/vpndns"
)

// VPNDNS reads and verifies the `vpn` DNS layer (internal/vpndns).
type VPNDNS interface {
	Status(context.Context) (vpndns.DNSStatus, error)
	Verify(context.Context) (vpndns.DNSVerification, error)
}

type vpnDNSOutput struct {
	Body vpndns.DNSStatus
}

type vpnDNSVerifyOutput struct {
	Body vpndns.DNSVerification
}

func (s *server) registerVPNDNS(a huma.API) {
	register(s, false, op("getVPNDNS", http.MethodGet, "/api/v1/vpn/dns", "VPN DNS layer status", "vpn", 503), a, s.getVPNDNS)
	register(s, false, op("verifyVPNDNS", http.MethodPost, "/api/v1/vpn/dns/verify", "Verify resolution through the VPN resolvers", "vpn", 503), a, s.verifyVPNDNS)
}

func (s *server) getVPNDNS(ctx context.Context, _ *struct{}) (*vpnDNSOutput, error) {
	if s.VPNDNS == nil {
		return nil, vpnDNSUnavailable()
	}
	st, err := s.VPNDNS.Status(ctx)
	if err != nil {
		return nil, vpnDNSUnavailable()
	}
	return &vpnDNSOutput{Body: st}, nil
}

func (s *server) verifyVPNDNS(ctx context.Context, _ *struct{}) (*vpnDNSVerifyOutput, error) {
	if s.VPNDNS == nil {
		return nil, vpnDNSUnavailable()
	}
	v, err := s.VPNDNS.Verify(ctx)
	if err != nil {
		return nil, vpnDNSUnavailable()
	}
	return &vpnDNSVerifyOutput{Body: v}, nil
}

func vpnDNSUnavailable() error {
	return fail(503, "vpn_dns_unavailable", vpndns.ErrUnavailable.Error())
}
