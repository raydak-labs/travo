package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/uplink"
)

// WireGuardImportRequest carries a wg-quick .conf file.
type WireGuardImportRequest struct {
	Config string `json:"config"`
}

// WireGuardProfileRequest names a .conf file to keep as a profile.
type WireGuardProfileRequest struct {
	Name   string `json:"name"`
	Config string `json:"config"`
}

// TailscaleAuthRequest optionally carries a pre-auth key.
type TailscaleAuthRequest struct {
	AuthKey string `json:"auth_key,omitempty"`
}

// TailscaleAuthResponse is the login URL when Tailscale needs one.
type TailscaleAuthResponse struct {
	AuthURL string `json:"auth_url"`
}

// ExitNodeRequest selects a Tailscale exit node; empty clears it.
type ExitNodeRequest struct {
	NodeIP string `json:"node_ip"`
}

// PortalRequest names a captive portal; empty means "the detected one".
type PortalRequest struct {
	PortalURL string `json:"portal_url,omitempty"`
}

// OKMessage is an acknowledgement with a human-readable message.
type OKMessage struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// AdGuardConfigBody is the raw AdGuard Home YAML.
type AdGuardConfigBody struct {
	Content string `json:"content"`
}

// AdGuardPasswordRequest sets the AdGuard web UI login.
type AdGuardPasswordRequest struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password"`
}

type toggleInput struct {
	Body EnabledRequest
}

type tailscaleAuthInput struct {
	Body *TailscaleAuthRequest
}

type portalInput struct {
	Body *PortalRequest
}

// vpnErr maps a VPN error to a status: a missing optional package is a
// capability gap (503 with the reason), not a server fault.
func vpnErr(err error) error {
	if errors.Is(err, uplink.ErrTailscaleNotInstalled) {
		return errorf(http.StatusServiceUnavailable, err.Error())
	}
	return serverError(err)
}

func vpnOK(err error) (*Body[Status], error) {
	if err != nil {
		return nil, vpnErr(err)
	}
	return statusOK, nil
}

func registerVPN(api *API, svc *uplink.VpnService) {
	get(api, "/api/v1/vpn/status", "GetVpnStatus", "WireGuard and Tailscale state",
		func(context.Context, *none) (*Body[[]models.VpnStatus], error) {
			return replyOrFail(svc.GetVpnStatus())
		})

	get(api, "/api/v1/vpn/wireguard", "GetWireguard", "WireGuard configuration",
		func(context.Context, *none) (*Body[models.WireguardConfig], error) {
			return replyOrFail(svc.GetWireguardConfig())
		})
	put(api, "/api/v1/vpn/wireguard", "SetWireguard", "Set the WireGuard configuration",
		strict(func(_ context.Context, cfg models.WireguardConfig) (*Body[Status], error) {
			if cfg.PrivateKey != "" && !isValidBase64Key(cfg.PrivateKey) {
				return nil, badRequest("private key must be a valid base64-encoded 32-byte key")
			}
			for i, peer := range cfg.Peers {
				if peer.Endpoint != "" && !isValidEndpoint(peer.Endpoint) {
					return nil, badRequest(fmt.Sprintf("peer %d: endpoint must be in host:port format", i))
				}
				for _, cidr := range peer.AllowedIPs {
					if !isValidCIDR(cidr) {
						return nil, badRequest(fmt.Sprintf("peer %d: invalid CIDR in allowed_ips: %s", i, cidr))
					}
				}
			}
			return okOrFail(svc.SetWireguardConfig(cfg))
		}))
	post(api, "/api/v1/vpn/wireguard/toggle", "ToggleWireguard", "Enable or disable WireGuard",
		func(_ context.Context, in *toggleInput) (*Body[Status], error) {
			return okOrFail(svc.ToggleWireguard(in.Body.Enabled))
		})
	post(api, "/api/v1/vpn/wireguard/import", "ImportWireguard", "Import a wg-quick .conf",
		func(_ context.Context, in *Body[WireGuardImportRequest]) (*Body[Status], error) {
			if in.Body.Config == "" {
				return nil, badRequest("config field is required")
			}
			if err := svc.ImportWireguardConfig(in.Body.Config); err != nil {
				return nil, badRequest(ErrInvalidRequestBody + ": " + err.Error())
			}
			return statusOK, nil
		})
	get(api, "/api/v1/vpn/wireguard/status", "GetWireguardStatus", "WireGuard peers and handshakes",
		func(context.Context, *none) (*Body[*models.WireGuardStatus], error) {
			return replyOrFail(svc.GetWireGuardStatus())
		})
	get(api, "/api/v1/vpn/wireguard/verify", "VerifyWireguard", "Interface, handshake, route and firewall checks",
		func(context.Context, *none) (*Body[models.VPNVerifyResult], error) {
			return reply(svc.VerifyWireGuard())
		})
	get(api, "/api/v1/vpn/wireguard/profiles", "GetWireguardProfiles", "Saved WireGuard profiles",
		func(context.Context, *none) (*Body[[]models.WireGuardProfile], error) {
			return replyOrFail(svc.GetProfiles())
		})
	post(api, "/api/v1/vpn/wireguard/profiles", "AddWireguardProfile", "Save a WireGuard profile",
		func(_ context.Context, in *Body[WireGuardProfileRequest]) (*Body[*models.WireGuardProfile], error) {
			if in.Body.Name == "" {
				return nil, badRequest("name is required")
			}
			if in.Body.Config == "" {
				return nil, badRequest("config is required")
			}
			profile, err := svc.AddProfile(in.Body.Name, in.Body.Config)
			if err != nil {
				return nil, badRequest(ErrInvalidRequestBody + ": " + err.Error())
			}
			return reply(profile)
		}, created)
	del(api, "/api/v1/vpn/wireguard/profiles/{id}", "DeleteWireguardProfile", "Delete a WireGuard profile",
		func(_ context.Context, in *idParam) (*Body[Status], error) {
			if err := svc.DeleteProfile(in.ID); err != nil {
				return nil, errorf(http.StatusNotFound, err.Error())
			}
			return statusOK, nil
		})
	post(api, "/api/v1/vpn/wireguard/profiles/{id}/activate", "ActivateWireguardProfile", "Activate a WireGuard profile",
		func(_ context.Context, in *idParam) (*Body[Status], error) {
			if err := svc.ActivateProfile(in.ID); err != nil {
				return nil, badRequest(ErrInvalidRequestBody + ": " + err.Error())
			}
			return statusOK, nil
		})

	get(api, "/api/v1/vpn/killswitch", "GetKillSwitch", "VPN kill switch state",
		func(context.Context, *none) (*Body[models.KillSwitchStatus], error) {
			return replyOrFail(svc.GetKillSwitch())
		})
	put(api, "/api/v1/vpn/killswitch", "SetKillSwitch", "Enable or disable the VPN kill switch",
		func(_ context.Context, in *Body[EnabledRequest]) (*Body[Status], error) {
			return okOrFail(svc.SetKillSwitch(in.Body.Enabled))
		})
	get(api, "/api/v1/vpn/split-tunnel", "GetSplitTunnel", "Split tunnel routes",
		func(context.Context, *none) (*Body[models.SplitTunnelConfig], error) {
			return replyOrFail(svc.GetSplitTunnel())
		})
	put(api, "/api/v1/vpn/split-tunnel", "SetSplitTunnel", "Set the split tunnel routes",
		strict(func(_ context.Context, cfg models.SplitTunnelConfig) (*Body[Status], error) {
			if cfg.Mode != "all" && cfg.Mode != "custom" {
				return nil, badRequest("mode must be 'all' or 'custom'")
			}
			return okOrFail(svc.SetSplitTunnel(cfg))
		}))
	get(api, "/api/v1/vpn/dns-leak-test", "DNSLeakTest", "Which resolvers answer through the tunnel",
		func(context.Context, *none) (*Body[models.DNSLeakResult], error) { return reply(svc.RunDNSLeakTest()) })
	post(api, "/api/v1/vpn/speed-test", "RunWireGuardSpeedTest", "Measure throughput through the tunnel",
		func(context.Context, *none) (*Body[models.SpeedTestResult], error) {
			result, err := svc.RunWireGuardSpeedTest()
			if err != nil {
				// Every error is a precondition: the tunnel is not usable for the test.
				return nil, badRequest(ErrInvalidRequestBody + ": " + err.Error())
			}
			return reply(result)
		})

	get(api, "/api/v1/vpn/tailscale", "GetTailscale", "Tailscale state",
		func(context.Context, *none) (*Body[models.TailscaleStatus], error) {
			return replyOrFail(svc.GetTailscaleStatus())
		})
	post(api, "/api/v1/vpn/tailscale/toggle", "ToggleTailscale", "Enable or disable Tailscale",
		func(_ context.Context, in *toggleInput) (*Body[Status], error) {
			return vpnOK(svc.ToggleTailscale(in.Body.Enabled))
		})
	post(api, "/api/v1/vpn/tailscale/auth", "TailscaleAuth", "Start Tailscale login",
		func(_ context.Context, in *tailscaleAuthInput) (*Body[TailscaleAuthResponse], error) {
			key := ""
			if in.Body != nil {
				key = in.Body.AuthKey
			}
			url, err := svc.StartTailscaleAuth(key)
			if err != nil {
				return nil, vpnErr(err)
			}
			return reply(TailscaleAuthResponse{AuthURL: url})
		})
	post(api, "/api/v1/vpn/tailscale/exit-node", "SetTailscaleExitNode", "Select or clear the exit node",
		func(_ context.Context, in *Body[ExitNodeRequest]) (*Body[Status], error) {
			return vpnOK(svc.SetTailscaleExitNode(in.Body.NodeIP))
		})
	get(api, "/api/v1/vpn/tailscale/ssh", "GetTailscaleSSH", "Whether Tailscale SSH is enabled",
		func(context.Context, *none) (*Body[EnabledResponse], error) {
			enabled, err := svc.GetTailscaleSSHEnabled()
			return replyOrFail(EnabledResponse{Enabled: enabled}, err)
		})
	put(api, "/api/v1/vpn/tailscale/ssh", "SetTailscaleSSH", "Enable or disable Tailscale SSH",
		func(_ context.Context, in *Body[EnabledRequest]) (*Body[OK], error) {
			if err := svc.SetTailscaleSSHEnabled(in.Body.Enabled); err != nil {
				return nil, vpnErr(err)
			}
			return okTrue, nil
		})
}

func registerAdGuard(api *API, svc *uplink.AdGuardService, captive *uplink.CaptiveService) {
	get(api, "/api/v1/adguard/dns", "GetAdGuardDNS", "Whether dnsmasq forwards to AdGuard",
		func(context.Context, *none) (*Body[models.AdGuardDNSStatus], error) {
			return replyOrFail(svc.GetDNSStatus())
		})
	put(api, "/api/v1/adguard/dns", "SetAdGuardDNS", "Forward LAN DNS to AdGuard or stop",
		func(_ context.Context, in *Body[EnabledRequest]) (*Body[Status], error) {
			return okOrFail(svc.SetDNS(in.Body.Enabled))
		})
	get(api, "/api/v1/adguard/dns-mode", "GetAdGuardDNSMode", "Effective LAN DNS mode",
		func(context.Context, *none) (*Body[models.DNSMode], error) {
			return reply(svc.GetDNSMode(captive.IsDNSBypassed()))
		})
	get(api, "/api/v1/adguard/config", "GetAdGuardConfig", "AdGuard Home YAML",
		func(context.Context, *none) (*Body[AdGuardConfigBody], error) {
			content, err := svc.GetConfig()
			if errors.Is(err, os.ErrNotExist) {
				return nil, errorf(http.StatusNotFound, "AdGuard config file not found")
			}
			return replyOrFail(AdGuardConfigBody{Content: content}, err)
		})
	put(api, "/api/v1/adguard/config", "SetAdGuardConfig", "Replace the AdGuard Home YAML",
		func(_ context.Context, in *Body[AdGuardConfigBody]) (*Body[Status], error) {
			if in.Body.Content == "" {
				return nil, badRequest("content is required")
			}
			return okOrFail(svc.SetConfig(in.Body.Content))
		}, describe("The YAML is validated, backed up and rolled back if AdGuard does not come up."))
	put(api, "/api/v1/adguard/password", "SetAdGuardPassword", "Set the AdGuard web UI login",
		func(_ context.Context, in *Body[AdGuardPasswordRequest]) (*Body[Status], error) {
			if in.Body.Password == "" {
				return nil, badRequest("password is required")
			}
			user := in.Body.Username
			if user == "" {
				user = "admin"
			}
			return okOrFail(svc.SetPassword(user, in.Body.Password))
		})
}

func registerCaptive(api *API, svc *uplink.CaptiveService) {
	get(api, "/api/v1/captive/status", "GetCaptiveStatus", "Captive portal detection",
		func(context.Context, *none) (*Body[models.CaptivePortalStatus], error) {
			status, err := svc.CheckCaptivePortal()
			if err != nil {
				return nil, serverError(err)
			}
			status.DNSBypassed = svc.IsDNSBypassed()
			status.DNSBypassNeeded = !status.DNSBypassed && status.Detected && svc.CheckDNSBypassNeeded()
			status.STAConnected = svc.IsUpstreamConnected()
			// No side effects: the dashboard polls this. Recovery from a stale
			// bypass is POST /captive/dns-restore or the startup auto-restore.
			return reply(status)
		})
	post(api, "/api/v1/captive/auto-accept", "CaptiveAutoAccept", "Accept the portal's terms automatically",
		func(_ context.Context, in *portalInput) (*Body[models.CaptiveAutoAcceptResult], error) {
			portal := ""
			if in.Body != nil {
				portal = strings.TrimSpace(in.Body.PortalURL)
			}
			if portal == "" {
				st, err := svc.CheckCaptivePortal()
				if err != nil {
					return nil, serverError(err)
				}
				if st.PortalURL != nil {
					portal = strings.TrimSpace(*st.PortalURL)
				}
			}
			if portal == "" {
				return nil, badRequest("no portal URL available")
			}
			return replyOrFail(svc.AutoAcceptCaptivePortal(portal))
		})
	post(api, "/api/v1/captive/dns-bypass", "CaptiveDNSBypass", "Point DNS at the upstream network for the portal",
		func(context.Context, *none) (*Body[OKMessage], error) {
			if err := svc.BypassDNS(); err != nil {
				return nil, serverError(err)
			}
			return reply(OKMessage{OK: true, Message: "DNS switched to upstream for captive portal access"})
		})
	post(api, "/api/v1/captive/dns-restore", "CaptiveDNSRestore", "Restore DNS after the portal",
		func(context.Context, *none) (*Body[OKMessage], error) {
			if err := svc.RestoreDNS(); err != nil {
				return nil, serverError(err)
			}
			return reply(OKMessage{OK: true, Message: "DNS restored to original configuration"})
		})
}
