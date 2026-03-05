import { http, HttpResponse } from 'msw';
import { API_ROUTES } from '@shared/index';
import {
  mockSystemInfo,
  mockSystemStats,
  mockNetworkStatus,
  mockWifiConnection,
  mockWifiScanResults,
  mockSavedNetworks,
  mockVpnStatus,
  mockServices,
  mockCaptivePortalStatus,
  mockWireguardConfig,
  mockTailscaleStatus,
  mockWanConfig,
  mockClients,
} from './data';

export const handlers = [
  http.get(API_ROUTES.system.info, () => {
    return HttpResponse.json(mockSystemInfo);
  }),

  http.get(API_ROUTES.system.stats, () => {
    return HttpResponse.json(mockSystemStats);
  }),

  http.get(API_ROUTES.network.status, () => {
    return HttpResponse.json(mockNetworkStatus);
  }),

  http.get(API_ROUTES.wifi.scan, () => {
    return HttpResponse.json(mockWifiScanResults);
  }),

  http.get(API_ROUTES.wifi.connection, () => {
    return HttpResponse.json(mockWifiConnection);
  }),

  http.post(API_ROUTES.wifi.connect, async ({ request }) => {
    const body = (await request.json()) as { ssid: string; password: string };
    return HttpResponse.json({ success: true, ssid: body.ssid });
  }),

  http.post(API_ROUTES.wifi.disconnect, () => {
    return HttpResponse.json({ success: true });
  }),

  http.put(API_ROUTES.wifi.mode, async ({ request }) => {
    const body = (await request.json()) as { mode: string };
    return HttpResponse.json({ success: true, mode: body.mode });
  }),

  http.get(API_ROUTES.wifi.saved, () => {
    return HttpResponse.json(mockSavedNetworks);
  }),

  http.get(API_ROUTES.vpn.status, () => {
    return HttpResponse.json([mockVpnStatus]);
  }),

  http.get(API_ROUTES.services.list, () => {
    return HttpResponse.json(mockServices);
  }),

  http.get(API_ROUTES.captive.status, () => {
    return HttpResponse.json(mockCaptivePortalStatus);
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

  http.get(API_ROUTES.vpn.tailscale.status, () => {
    return HttpResponse.json(mockTailscaleStatus);
  }),

  http.post(API_ROUTES.vpn.tailscale.toggle, () => {
    return HttpResponse.json({ success: true });
  }),

  http.post(API_ROUTES.services.install, () => {
    return HttpResponse.json({ success: true });
  }),

  http.post(API_ROUTES.services.remove, () => {
    return HttpResponse.json({ success: true });
  }),

  http.post(API_ROUTES.services.start, () => {
    return HttpResponse.json({ success: true });
  }),

  http.post(API_ROUTES.services.stop, () => {
    return HttpResponse.json({ success: true });
  }),

  http.get(API_ROUTES.network.wan, () => {
    return HttpResponse.json(mockWanConfig);
  }),

  http.put(API_ROUTES.network.wan, () => {
    return HttpResponse.json({ success: true });
  }),

  http.get(API_ROUTES.network.clients, () => {
    return HttpResponse.json(mockClients);
  }),

  http.post(API_ROUTES.auth.login, async ({ request }) => {
    const body = (await request.json()) as { password: string };
    if (body.password === 'admin') {
      return HttpResponse.json({
        token: 'mock-jwt-token-abc123',
        expires_at: '2026-03-05T00:00:00Z',
      });
    }
    return HttpResponse.json({ error: 'Invalid password' }, { status: 401 });
  }),

  http.post(API_ROUTES.auth.logout, () => {
    return HttpResponse.json({ success: true });
  }),

  http.get(API_ROUTES.auth.session, () => {
    return HttpResponse.json({ valid: true });
  }),

  http.post(API_ROUTES.system.reboot, () => {
    return HttpResponse.json({ status: 'ok' });
  }),
];
