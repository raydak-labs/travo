package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/openwrt-travel-gui/backend/internal/failover"
	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/network"
)

// DDNSConfigResponse reports whether ddns-scripts is installed next to the
// config, so the UI can explain instead of offering a form that can only fail.
type DDNSConfigResponse struct {
	Config    models.DDNSConfig `json:"config"`
	Available bool              `json:"available"`
}

// TrafficHistoryResponse paints the dashboard chart on page load.
type TrafficHistoryResponse struct {
	Points          []network.TrafficHistoryPoint `json:"points"`
	RetainedSeconds int64                         `json:"retained_seconds"`
}

// FirewallZonesResponse lists the firewall zones.
type FirewallZonesResponse struct {
	Zones []models.FirewallZone `json:"zones"`
}

// PortForwardsResponse lists the port-forward rules.
type PortForwardsResponse struct {
	Rules []models.PortForwardRule `json:"rules"`
}

// EnabledRequest toggles a feature.
type EnabledRequest struct {
	Enabled bool `json:"enabled"`
}

// InterfaceRequest names a network interface.
type InterfaceRequest struct {
	Interface string `json:"interface"`
}

type sectionParam struct {
	Section string `path:"section"`
}

type idParam struct {
	ID string `path:"id"`
}

type interfaceStateInput struct {
	Name string `path:"name"`
	Body models.SetInterfaceStateRequest
}

// sqmApplyError is the 500 of POST /sqm/apply: it keeps the apply output.
type sqmApplyError struct {
	models.SQMApplyResult
}

func (e *sqmApplyError) Error() string  { return e.SQMApplyResult.Error }
func (e *sqmApplyError) GetStatus() int { return http.StatusInternalServerError }

func registerNetwork(api *API, d *Dependencies) {
	svc := d.Network

	get(api, "/api/v1/network/status", "GetNetworkStatus", "Interfaces, WAN and internet reachability",
		func(context.Context, *none) (*Body[models.NetworkStatus], error) {
			return replyOrFail(svc.GetNetworkStatus())
		})
	get(api, "/api/v1/network/traffic-history", "GetTrafficHistory", "Recent per-interface traffic samples",
		func(context.Context, *none) (*Body[TrafficHistoryResponse], error) {
			return reply(TrafficHistoryResponse{Points: d.TrafficHistory.History(), RetainedSeconds: d.TrafficHistory.RetainedSeconds()})
		})
	get(api, "/api/v1/network/connection-method", "GetConnectionMethod", "How the caller reaches the router",
		func(ctx context.Context, _ *none) (*Body[*network.ConnectionMethod], error) {
			ip := fiberCtx(ctx).IP()
			if ip == "" {
				return nil, errorf(http.StatusInternalServerError, "could not determine client IP")
			}
			return replyOrFail(svc.GetConnectionMethod(ip))
		})
	get(api, "/api/v1/network/uptime-log", "GetUptimeLog", "Internet up/down events",
		func(context.Context, *none) (*Body[[]network.UptimeEvent], error) {
			return reply(d.UptimeTracker.GetUptimeLog())
		})
	post(api, "/api/v1/network/interfaces/{name}/state", "SetInterfaceState", "Bring an interface up or down",
		func(_ context.Context, in *interfaceStateInput) (*Body[Status], error) {
			if err := svc.SetInterfaceState(in.Name, in.Body.Up); err != nil {
				if errors.Is(err, network.ErrUnknownInterface) {
					return nil, badRequest(ErrInvalidRequestBody + ": " + err.Error())
				}
				return nil, serverError(err)
			}
			return statusOK, nil
		})

	get(api, "/api/v1/network/wan", "GetWanConfig", "WAN configuration",
		func(context.Context, *none) (*Body[models.WanConfig], error) { return replyOrFail(svc.GetWanConfig()) })
	get(api, "/api/v1/network/wan/detect", "DetectWanType", "Detect the WAN type (DHCP or PPPoE)",
		func(context.Context, *none) (*Body[models.WanDetectResult], error) {
			return replyOrFail(svc.DetectWanType())
		})
	put(api, "/api/v1/network/wan", "SetWanConfig", "Set the WAN configuration",
		strict(func(_ context.Context, cfg models.WanConfig) (*Body[Status], error) {
			if err := validateWanConfig(cfg); err != nil {
				return nil, err
			}
			return okOrFail(svc.SetWanConfig(cfg))
		}))

	get(api, "/api/v1/network/clients", "GetClients", "Connected clients",
		func(context.Context, *none) (*Body[[]models.Client], error) { return replyOrFail(svc.GetClients()) })
	put(api, "/api/v1/network/clients/alias", "SetClientAlias", "Name a client by MAC",
		strict(func(_ context.Context, req models.SetAliasRequest) (*Body[Status], error) {
			if err := requireMAC(req.MAC); err != nil {
				return nil, err
			}
			return okOrFail(svc.SetAlias(req.MAC, req.Alias))
		}))
	for _, a := range []struct {
		path, id, summary string
		fn                func(string) error
	}{
		{"kick", "KickClient", "Disconnect a WiFi client", svc.KickClient},
		{"block", "BlockClient", "Block a client by MAC", svc.BlockClient},
		{"unblock", "UnblockClient", "Unblock a client", svc.UnblockClient},
	} {
		post(api, "/api/v1/network/clients/"+a.path, a.id, a.summary,
			func(_ context.Context, in *Body[models.ClientActionRequest]) (*Body[Status], error) {
				if err := requireMAC(in.Body.MAC); err != nil {
					return nil, err
				}
				return okOrFail(a.fn(in.Body.MAC))
			})
	}
	get(api, "/api/v1/network/clients/blocked", "GetBlockedClients", "Blocked client MACs",
		func(context.Context, *none) (*Body[[]string], error) { return replyOrFail(svc.GetBlockedClients()) })

	get(api, "/api/v1/network/dhcp", "GetDHCPConfig", "LAN DHCP pool",
		func(context.Context, *none) (*Body[models.DHCPConfig], error) {
			return replyOrFail(svc.GetDHCPConfig())
		})
	put(api, "/api/v1/network/dhcp", "SetDHCPConfig", "Set the LAN DHCP pool",
		strict(func(_ context.Context, cfg models.DHCPConfig) (*Body[Status], error) {
			switch {
			case cfg.Start < 2 || cfg.Start > 254:
				return nil, badRequest("start must be between 2 and 254")
			case cfg.Limit < 1 || cfg.Limit > 253:
				return nil, badRequest("limit must be between 1 and 253")
			case cfg.Start+cfg.Limit > 255:
				return nil, badRequest("start + limit must not exceed 255")
			case cfg.LeaseTime == "":
				return nil, badRequest("lease_time is required")
			}
			return okOrFail(svc.SetDHCPConfig(cfg))
		}))
	get(api, "/api/v1/network/dhcp/leases", "GetDHCPLeases", "Active DHCP leases",
		func(context.Context, *none) (*Body[[]models.DHCPLease], error) { return reply(svc.GetDHCPLeases()) })
	get(api, "/api/v1/network/dhcp/reservations", "GetDHCPReservations", "Static DHCP leases",
		func(context.Context, *none) (*Body[[]models.DHCPReservation], error) {
			return replyOrFail(svc.GetDHCPReservations())
		})
	post(api, "/api/v1/network/dhcp/reservations", "AddDHCPReservation", "Add a static DHCP lease",
		func(_ context.Context, in *Body[models.DHCPReservation]) (*Body[Status], error) {
			r := in.Body
			switch {
			case r.Name == "":
				return nil, badRequest("name is required")
			case !isValidHostname(r.Name):
				return nil, badRequest("invalid hostname")
			case r.MAC == "":
				return nil, badRequest("mac is required")
			case !isValidMAC(r.MAC):
				return nil, badRequest("invalid MAC address")
			case r.IP == "":
				return nil, badRequest("ip is required")
			case !isValidIPv4(r.IP):
				return nil, badRequest("invalid IP address")
			}
			return okOrFail(svc.AddDHCPReservation(r))
		})
	del(api, "/api/v1/network/dhcp/reservations/{section}", "DeleteDHCPReservation", "Remove a static DHCP lease",
		func(_ context.Context, in *sectionParam) (*Body[Status], error) {
			return okOrFail(svc.DeleteDHCPReservation(in.Section))
		})

	get(api, "/api/v1/network/dns", "GetDNSConfig", "Upstream DNS servers",
		func(context.Context, *none) (*Body[models.DNSConfig], error) { return replyOrFail(svc.GetDNSConfig()) })
	put(api, "/api/v1/network/dns", "SetDNSConfig", "Set the upstream DNS servers",
		strict(func(_ context.Context, cfg models.DNSConfig) (*Body[Status], error) {
			if cfg.UseCustomDNS {
				if len(cfg.Servers) == 0 {
					return nil, badRequest("at least one DNS server is required when using custom DNS")
				}
				for _, s := range cfg.Servers {
					if !isValidIPv4(s) {
						return nil, badRequest(fmt.Sprintf("invalid DNS server IP: %s", s))
					}
				}
			}
			return okOrFail(svc.SetDNSConfig(cfg))
		}))
	get(api, "/api/v1/network/dns/entries", "GetDNSEntries", "Local DNS records",
		func(context.Context, *none) (*Body[[]models.DNSEntry], error) {
			return replyOrFail(svc.GetDNSEntries())
		})
	post(api, "/api/v1/network/dns/entries", "AddDNSEntry", "Add a local DNS record",
		func(_ context.Context, in *Body[models.DNSEntry]) (*Body[Status], error) {
			e := in.Body
			switch {
			case e.Name == "":
				return nil, badRequest("name is required")
			case !isValidHostname(e.Name):
				return nil, badRequest("invalid hostname")
			case e.IP == "":
				return nil, badRequest("ip is required")
			case !isValidIPv4(e.IP):
				return nil, badRequest("invalid IP address")
			}
			return okOrFail(svc.AddDNSEntry(e))
		})
	del(api, "/api/v1/network/dns/entries/{section}", "DeleteDNSEntry", "Remove a local DNS record",
		func(_ context.Context, in *sectionParam) (*Body[Status], error) {
			return okOrFail(svc.DeleteDNSEntry(in.Section))
		})

	get(api, "/api/v1/network/ddns", "GetDDNSConfig", "Dynamic DNS configuration",
		func(context.Context, *none) (*Body[DDNSConfigResponse], error) {
			cfg, err := svc.GetDDNSConfig()
			if err != nil {
				return nil, serverError(err)
			}
			return reply(DDNSConfigResponse{Config: cfg, Available: svc.DDNSAvailable()})
		})
	put(api, "/api/v1/network/ddns", "SetDDNSConfig", "Set the dynamic DNS configuration",
		strict(func(_ context.Context, cfg models.DDNSConfig) (*Body[Status], error) {
			if err := validateDDNSConfig(cfg); err != nil {
				return nil, err
			}
			if err := svc.SetDDNSConfig(cfg); err != nil {
				// A missing ddns-scripts is a capability gap, not a bad request.
				if errors.Is(err, network.ErrDDNSNotAvailable) {
					return nil, errorf(http.StatusServiceUnavailable, err.Error())
				}
				return nil, serverError(err)
			}
			return statusOK, nil
		}))
	get(api, "/api/v1/network/ddns/status", "GetDDNSStatus", "Dynamic DNS update state",
		func(context.Context, *none) (*Body[models.DDNSStatus], error) {
			return replyOrFail(svc.GetDDNSStatus())
		})

	get(api, "/api/v1/network/firewall/zones", "GetFirewallZones", "Firewall zones",
		func(context.Context, *none) (*Body[FirewallZonesResponse], error) {
			zones, err := svc.GetFirewallZones()
			if err != nil {
				return nil, serverError(err)
			}
			return reply(FirewallZonesResponse{Zones: zones})
		})
	get(api, "/api/v1/network/firewall/port-forwards", "GetPortForwards", "Port-forward rules",
		func(context.Context, *none) (*Body[PortForwardsResponse], error) {
			rules, err := svc.GetPortForwards()
			if err != nil {
				return nil, serverError(err)
			}
			return reply(PortForwardsResponse{Rules: rules})
		})
	post(api, "/api/v1/network/firewall/port-forwards", "AddPortForward", "Add a port-forward rule",
		func(_ context.Context, in *Body[models.PortForwardRule]) (*Body[OK], error) {
			return okTrueOrFail(svc.AddPortForward(in.Body))
		}, created)
	del(api, "/api/v1/network/firewall/port-forwards/{id}", "DeletePortForward", "Remove a port-forward rule",
		func(_ context.Context, in *idParam) (*Body[OK], error) {
			return okTrueOrFail(svc.DeletePortForward(in.ID))
		})

	post(api, "/api/v1/network/diagnostics", "RunDiagnostics", "Run ping, traceroute or a DNS lookup",
		func(_ context.Context, in *Body[models.DiagnosticsRequest]) (*Body[models.DiagnosticsResult], error) {
			switch in.Body.Type {
			case "ping", "traceroute", "dns":
			default:
				return nil, badRequest("type must be one of: ping, traceroute, dns")
			}
			// A target the tool would read as an option must be a 400.
			if !isValidDiagnosticsTarget(in.Body.Target) {
				return nil, badRequest("target must be a hostname or IPv4 address, and must not start with '-'")
			}
			return reply(svc.RunDiagnostics(in.Body))
		})
	get(api, "/api/v1/network/doh", "GetDoHConfig", "DNS-over-HTTPS configuration",
		func(context.Context, *none) (*Body[models.DoHConfig], error) { return replyOrFail(svc.GetDoHConfig()) })
	put(api, "/api/v1/network/doh", "SetDoHConfig", "Set the DNS-over-HTTPS configuration",
		strict(func(_ context.Context, cfg models.DoHConfig) (*Body[OK], error) {
			return okTrueOrFail(svc.SetDoHConfig(cfg))
		}))
	get(api, "/api/v1/network/ipv6", "GetIPv6Status", "IPv6 state",
		func(context.Context, *none) (*Body[models.IPv6Status], error) {
			return replyOrFail(svc.GetIPv6Status())
		})
	put(api, "/api/v1/network/ipv6", "SetIPv6Enabled", "Enable or disable IPv6",
		func(_ context.Context, in *Body[EnabledRequest]) (*Body[OK], error) {
			return okTrueOrFail(svc.SetIPv6Enabled(in.Body.Enabled))
		})
	post(api, "/api/v1/network/wol", "SendWoL", "Send a Wake-on-LAN packet",
		func(_ context.Context, in *Body[models.WoLRequest]) (*Body[OK], error) {
			return okTrueOrFail(svc.SendWoL(in.Body.MAC, in.Body.Interface))
		})

	get(api, "/api/v1/sqm/config", "GetSQMConfig", "Smart queue management configuration",
		func(context.Context, *none) (*Body[models.SQMConfig], error) { return replyOrFail(d.SQM.GetConfig()) })
	put(api, "/api/v1/sqm/config", "SetSQMConfig", "Set the smart queue management configuration",
		strict(func(_ context.Context, cfg models.SQMConfig) (*Body[Status], error) {
			if err := d.SQM.SetConfig(cfg); err != nil {
				return nil, badRequest(ErrInvalidRequestBody + ": " + err.Error())
			}
			return statusOK, nil
		}))
	post(api, "/api/v1/sqm/apply", "ApplySQM", "Apply the SQM configuration",
		func(context.Context, *none) (*Body[models.SQMApplyResult], error) {
			out, err := d.SQM.Apply()
			if err != nil {
				return nil, &sqmApplyError{models.SQMApplyResult{OK: false, Output: out, Error: err.Error()}}
			}
			return reply(models.SQMApplyResult{OK: true, Output: out})
		})

	get(api, "/api/v1/network/data-usage", "GetDataUsage", "Traffic counters per interface (vnstat)",
		func(context.Context, *none) (*Body[models.DataUsageStatus], error) {
			return replyOrFail(d.DataUsage.GetStatus())
		})
	post(api, "/api/v1/network/data-usage/reset", "ResetDataUsage", "Reset the counters of one interface",
		func(_ context.Context, in *Body[InterfaceRequest]) (*Body[Status], error) {
			if in.Body.Interface == "" {
				return nil, badRequest("interface is required")
			}
			return okOrFail(d.DataUsage.ResetInterface(in.Body.Interface))
		})
	get(api, "/api/v1/network/data-usage/budget", "GetDataBudget", "Data budgets",
		func(context.Context, *none) (*Body[models.DataBudgetConfig], error) {
			return replyOrFail(d.DataUsage.GetBudget())
		})
	put(api, "/api/v1/network/data-usage/budget", "SetDataBudget", "Set the data budgets",
		strict(func(_ context.Context, cfg models.DataBudgetConfig) (*Body[Status], error) {
			if cfg.Budgets == nil {
				cfg.Budgets = []models.DataBudget{}
			}
			return okOrFail(d.DataUsage.SetBudget(cfg))
		}))

	get(api, "/api/v1/network/usb-tethering", "GetUSBTetherStatus", "Detected USB tethering device",
		func(context.Context, *none) (*Body[network.USBTetherStatus], error) {
			return reply(d.USBTether.GetStatus())
		})
	post(api, "/api/v1/network/usb-tethering/configure", "ConfigureUSBTether", "Use the USB device as a WAN",
		func(_ context.Context, in *Body[InterfaceRequest]) (*Body[Status], error) {
			if in.Body.Interface == "" {
				return nil, badRequest("interface is required")
			}
			return okOrFail(d.USBTether.Configure(in.Body.Interface))
		})
	post(api, "/api/v1/network/usb-tethering/unconfigure", "UnconfigureUSBTether", "Remove the USB WAN",
		func(context.Context, *none) (*Body[Status], error) { return okOrFail(d.USBTether.Unconfigure()) })

	registerFailover(api, d.Failover)
}

func registerFailover(api *API, svc *failover.Service) {
	get(api, "/api/v1/network/failover", "GetFailoverConfig", "Multi-WAN failover configuration",
		func(context.Context, *none) (*Body[models.FailoverConfig], error) {
			return replyOrFail(svc.GetConfig())
		})
	put(api, "/api/v1/network/failover", "SetFailoverConfig", "Set the multi-WAN failover configuration",
		strict(func(_ context.Context, cfg models.FailoverConfig) (*Body[Status], error) {
			if err := svc.SetConfig(cfg); err != nil {
				return nil, badRequest(ErrInvalidRequestBody + ": " + err.Error())
			}
			return statusOK, nil
		}))
	get(api, "/api/v1/network/failover/events", "GetFailoverEvents", "Recent failover events",
		func(context.Context, *none) (*Body[[]models.FailoverEvent], error) { return reply(svc.GetEvents()) })
}

func requireMAC(mac string) error {
	if mac == "" {
		return badRequest("MAC address is required")
	}
	if !isValidMAC(mac) {
		return badRequest("invalid MAC address format")
	}
	return nil
}

func validateWanConfig(cfg models.WanConfig) error {
	if !isValidWanType(cfg.Type) {
		return badRequest("type must be one of: dhcp, static, pppoe")
	}
	if cfg.Type == "static" {
		if cfg.IPAddress != "" && !isValidIPv4(cfg.IPAddress) {
			return badRequest("invalid IP address")
		}
		if cfg.Gateway != "" && !isValidIPv4(cfg.Gateway) {
			return badRequest("invalid gateway address")
		}
		if cfg.Netmask != "" && !isValidNetmask(cfg.Netmask) {
			return badRequest("invalid netmask")
		}
	}
	if cfg.MTU != 0 && !isValidMTU(cfg.MTU) {
		return badRequest(fmt.Sprintf("MTU must be between 68 and 9000, got %d", cfg.MTU))
	}
	for _, dns := range cfg.DNSServers {
		if !isValidIPv4(dns) {
			return badRequest(fmt.Sprintf("invalid DNS server IP: %s", dns))
		}
	}
	return nil
}

func validateDDNSConfig(cfg models.DDNSConfig) error {
	if !cfg.Enabled {
		return nil
	}
	if cfg.Domain == "" {
		return badRequest("domain is required when DDNS is enabled")
	}
	service := strings.TrimSpace(cfg.Service)
	if strings.EqualFold(service, "custom") {
		raw := strings.TrimSpace(cfg.UpdateURL)
		if raw == "" {
			return badRequest("update_url is required for custom DDNS")
		}
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return badRequest("update_url must be a valid http(s) URL")
		}
	} else if service == "" {
		return badRequest("service is required when DDNS is enabled")
	}
	return nil
}
