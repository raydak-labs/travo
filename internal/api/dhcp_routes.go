package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/netip"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/dhcpserver"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

// DHCPServer is the lan DHCP server and the local DNS names (internal/dhcpserver).
type DHCPServer interface {
	Settings(ctx context.Context, caller netip.Addr) (dhcpserver.DHCPView, error)
	SetSettings(ctx context.Context, in dhcpserver.DHCPSettings, caller netip.Addr) (dhcpserver.DHCPView, error)
	DNSNames(context.Context) ([]dhcpserver.DNSName, error)
	AddDNSName(ctx context.Context, name, ip string) (dhcpserver.DNSName, error)
	RemoveDNSName(ctx context.Context, id string) error
}

type dhcpOutput struct{ Body dhcpserver.DHCPView }

type dhcpInput struct{ Body dhcpserver.DHCPSettings }

type dnsNamesOutput struct {
	Body struct {
		Names []dhcpserver.DNSName `json:"names" nullable:"false"`
	}
}

type dnsNameInput struct {
	Body struct {
		Name string `json:"name" minLength:"1" maxLength:"253" doc:"Host name, e.g. nas or nas.home"`
		IP   string `json:"ip" minLength:"2" maxLength:"45" doc:"IPv4 or IPv6 address"`
	}
}

type dnsNameOutput struct{ Body dhcpserver.DNSName }

type dnsNamePath struct {
	ID string `path:"id" pattern:"^[A-Za-z0-9_]{1,64}$" doc:"UCI section of the entry"`
}

func (s *server) registerDHCP(a huma.API) {
	register(s, false, op("getDhcpSettings", http.MethodGet, "/api/v1/network/dhcp",
		"LAN DHCP server settings (dhcp.lan)", "network", 409, 503), a, s.getDHCP)
	put := op("setDhcpSettings", http.MethodPut, "/api/v1/network/dhcp",
		"Replace the LAN DHCP server settings (applies to new leases and renewals)", "network", 400, 409, 503)
	put.MaxBodyBytes = 512
	register(s, false, put, a, s.setDHCP)
	register(s, false, op("listDnsNames", http.MethodGet, "/api/v1/network/dns-names",
		"Local DNS names dnsmasq answers (dhcp domain sections)", "network", 503), a, s.listDNSNames)
	add := op("addDnsName", http.MethodPost, "/api/v1/network/dns-names",
		"Add a local DNS name", "network", 400, 409, 503)
	add.MaxBodyBytes = 512
	register(s, false, add, a, s.addDNSName)
	register(s, false, op("deleteDnsName", http.MethodDelete, "/api/v1/network/dns-names/{id}",
		"Remove a local DNS name Travo created", "network", 404, 409, 503), a, s.deleteDNSName)
}

func dhcpUnavailable() error {
	return fail(http.StatusServiceUnavailable, "dhcp_unavailable", "dhcp settings are unavailable")
}

// dhcpFailure maps the expected failures; anything else is logged and 503.
func dhcpFailure(err error) error {
	var fe *dhcpserver.FieldError
	switch {
	case errors.As(err, &fe):
		return fail(http.StatusBadRequest, "invalid_request", fe.Error())
	case errors.Is(err, dhcpserver.ErrForeignValue), errors.Is(err, dhcpserver.ErrNoSection):
		return fail(http.StatusConflict, "dhcp_foreign_config", "dhcp.lan is configured in a way Travo does not edit; use LuCI")
	case errors.Is(err, dhcpserver.ErrNameExists):
		return fail(http.StatusConflict, "dns_name_exists", "that name already has a local DNS entry")
	case errors.Is(err, dhcpserver.ErrNamesFull):
		return fail(http.StatusConflict, "dns_names_full", "too many local DNS names")
	case errors.Is(err, dhcpserver.ErrNameForeign):
		return fail(http.StatusConflict, "dns_name_foreign", "the entry was not created by Travo; edit it in LuCI")
	case errors.Is(err, dhcpserver.ErrNameNotFound):
		return fail(http.StatusNotFound, "not_found", "no local DNS name with that id")
	case errors.Is(err, txn.ErrForeignStaged):
		return fail(http.StatusConflict, "uci_busy", "another change is staged; try again")
	}
	slog.Warn("dhcp", "err", err)
	return dhcpUnavailable()
}

func (s *server) getDHCP(ctx context.Context, _ *struct{}) (*dhcpOutput, error) {
	if s.DHCP == nil {
		return nil, dhcpUnavailable()
	}
	caller, _ := netip.ParseAddr(infoOf(ctx).ip)
	v, err := s.DHCP.Settings(ctx, caller)
	if err != nil {
		return nil, dhcpFailure(err)
	}
	return &dhcpOutput{Body: v}, nil
}

func (s *server) setDHCP(ctx context.Context, in *dhcpInput) (*dhcpOutput, error) {
	if s.DHCP == nil {
		return nil, dhcpUnavailable()
	}
	caller, _ := netip.ParseAddr(infoOf(ctx).ip)
	v, err := s.DHCP.SetSettings(ctx, in.Body, caller)
	if err != nil {
		return nil, dhcpFailure(err)
	}
	return &dhcpOutput{Body: v}, nil
}

func (s *server) listDNSNames(ctx context.Context, _ *struct{}) (*dnsNamesOutput, error) {
	if s.DHCP == nil {
		return nil, dhcpUnavailable()
	}
	list, err := s.DHCP.DNSNames(ctx)
	if err != nil {
		return nil, dhcpFailure(err)
	}
	out := &dnsNamesOutput{}
	out.Body.Names = list
	if list == nil {
		out.Body.Names = []dhcpserver.DNSName{}
	}
	return out, nil
}

func (s *server) addDNSName(ctx context.Context, in *dnsNameInput) (*dnsNameOutput, error) {
	if s.DHCP == nil {
		return nil, dhcpUnavailable()
	}
	n, err := s.DHCP.AddDNSName(ctx, in.Body.Name, in.Body.IP)
	if err != nil {
		return nil, dhcpFailure(err)
	}
	return &dnsNameOutput{Body: n}, nil
}

func (s *server) deleteDNSName(ctx context.Context, in *dnsNamePath) (*struct{ Body okBody }, error) {
	if s.DHCP == nil {
		return nil, dhcpUnavailable()
	}
	if err := s.DHCP.RemoveDNSName(ctx, in.ID); err != nil {
		return nil, dhcpFailure(err)
	}
	return okResponse(), nil
}
