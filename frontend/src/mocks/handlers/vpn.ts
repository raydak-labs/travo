import { http, HttpResponse } from 'msw/http';
import { API_ROUTES } from '@shared/index';
import {
  mockKillSwitchStatus,
  mockTailscaleStatus,
  mockVpnStatus,
  mockWireguardConfig,
  mockWireguardProfiles,
  mockWireguardStatus,
} from '../data';

export const vpnHandlers = [
  http.get(API_ROUTES.vpn.status, () => {
    return HttpResponse.json([mockVpnStatus]);
  }),

  http.get(API_ROUTES.vpn.wireguard.config, () => {
    return HttpResponse.json(mockWireguardConfig);
  }),

  http.put(API_ROUTES.vpn.wireguard.config, () => {
    return HttpResponse.json({ success: true });
  }),

  http.post(API_ROUTES.vpn.wireguard.toggle, () => {
    return HttpResponse.json({ success: true });
  }),

  http.get(API_ROUTES.vpn.wireguard.status, () => {
    return HttpResponse.json(mockWireguardStatus);
  }),

  http.get(API_ROUTES.vpn.wireguard.profiles, () => {
    return HttpResponse.json(mockWireguardProfiles);
  }),

  http.post(API_ROUTES.vpn.wireguard.profiles, async ({ request }) => {
    const body = (await request.json()) as { name: string; config: string };
    return HttpResponse.json(
      {
        id: `profile-${Date.now()}`,
        name: body.name,
        config: body.config,
        active: false,
        created_at: new Date().toISOString(),
      },
      { status: 201 },
    );
  }),

  http.delete(`${API_ROUTES.vpn.wireguard.profiles}/:id`, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.post(`${API_ROUTES.vpn.wireguard.profiles}/:id/activate`, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.get(API_ROUTES.vpn.killswitch, () => {
    return HttpResponse.json(mockKillSwitchStatus);
  }),

  http.put(API_ROUTES.vpn.killswitch, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.get(API_ROUTES.vpn.tailscale.status, () => {
    return HttpResponse.json(mockTailscaleStatus);
  }),

  http.post(API_ROUTES.vpn.tailscale.toggle, () => {
    return HttpResponse.json({ success: true });
  }),

  http.post(API_ROUTES.vpn.tailscale.auth, () => {
    return HttpResponse.json({ status: 'ok', auth_url: '' });
  }),

  http.post(API_ROUTES.vpn.tailscale.exitNode, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.get(API_ROUTES.vpn.dnsLeakTest, () => {
    return HttpResponse.json({
      nameservers: ['10.0.0.1', '8.8.8.8'],
      vpn_dns_servers: ['10.66.0.1'],
      vpn_active: true,
      potentially_leaking: true,
    });
  }),

  http.get(API_ROUTES.vpn.wireguard.verify, () => {
    return HttpResponse.json({
      interface_up: true,
      handshake_ok: true,
      latest_handshake: Math.floor(Date.now() / 1000) - 60,
      route_ok: true,
      firewall_zone_ok: true,
      forwarding_ok: true,
    });
  }),

  http.get(API_ROUTES.vpn.tailscale.ssh, () => {
    return HttpResponse.json({ enabled: false });
  }),

  http.put(API_ROUTES.vpn.tailscale.ssh, () => {
    return HttpResponse.json({ ok: true });
  }),

  http.get(API_ROUTES.vpn.splitTunnel, () => {
    return HttpResponse.json({ enabled: false, routes: [] });
  }),

  http.put(API_ROUTES.vpn.splitTunnel, () => {
    return HttpResponse.json({ ok: true });
  }),

  http.post(API_ROUTES.vpn.wireguard.import, async ({ request }) => {
    const body = (await request.json()) as { name?: string; config?: string };
    return HttpResponse.json({
      id: 'profile-imported',
      name: body.name ?? 'Imported profile',
      config: body.config ?? '',
      active: false,
      created_at: '2026-03-11T09:00:00Z',
    });
  }),
];
