package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/netview"
)

// NetworkView is the read-only interface and firewall zone view (internal/netview).
type NetworkView interface {
	Interfaces(context.Context) (netview.Listing, error)
	Zones(context.Context) ([]netview.Zone, error)
}

type networkInterfacesOutput struct {
	Body struct {
		Interfaces []netview.Interface `json:"interfaces" nullable:"false"`
		Partial    bool                `json:"partial" doc:"The firewall config could not be read; zones are missing"`
	}
}

type firewallZonesOutput struct {
	Body struct {
		Zones []netview.Zone `json:"zones" nullable:"false"`
	}
}

func (s *server) registerNetworkView(a huma.API) {
	register(s, false, op("listNetworkInterfaces", http.MethodGet, "/api/v1/network/interfaces", "netifd interfaces with uptime, addresses and firewall zone", "network", 503), a, s.listNetworkInterfaces)
	register(s, false, op("listFirewallZones", http.MethodGet, "/api/v1/network/firewall/zones", "Firewall zones with policies, networks and forwardings (read-only)", "network", 503), a, s.listFirewallZones)
}

func networkUnavailable(err error) error {
	if err != nil {
		slog.Warn("network view", "err", err)
	}
	return fail(http.StatusServiceUnavailable, "network_unavailable", "network data is unavailable")
}

func (s *server) listNetworkInterfaces(ctx context.Context, _ *struct{}) (*networkInterfacesOutput, error) {
	if s.NetworkView == nil {
		return nil, networkUnavailable(nil)
	}
	list, err := s.NetworkView.Interfaces(ctx)
	if err != nil {
		return nil, networkUnavailable(err)
	}
	out := &networkInterfacesOutput{}
	out.Body.Interfaces, out.Body.Partial = list.Interfaces, list.Partial
	if out.Body.Interfaces == nil {
		out.Body.Interfaces = []netview.Interface{}
	}
	return out, nil
}

func (s *server) listFirewallZones(ctx context.Context, _ *struct{}) (*firewallZonesOutput, error) {
	if s.NetworkView == nil {
		return nil, networkUnavailable(nil)
	}
	zones, err := s.NetworkView.Zones(ctx)
	if err != nil {
		return nil, networkUnavailable(err)
	}
	out := &firewallZonesOutput{}
	out.Body.Zones = zones
	if out.Body.Zones == nil {
		out.Body.Zones = []netview.Zone{}
	}
	return out, nil
}
