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
  mockSystemLogs,
  mockKernelLogs,
  mockDHCPConfig,
} from './data';

export const handlers = [
  http.get(API_ROUTES.system.info, () => {
    return HttpResponse.json(mockSystemInfo);
  }),

  http.get(API_ROUTES.system.stats, () => {
    return HttpResponse.json(mockSystemStats);
  }),

  http.get(API_ROUTES.system.logs, () => {
    return HttpResponse.json(mockSystemLogs);
  }),

  http.get(API_ROUTES.system.kernelLogs, () => {
    return HttpResponse.json(mockKernelLogs);
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

  http.delete(`${API_ROUTES.wifi.deleteSaved}/:section`, () => {
    return HttpResponse.json({ status: 'ok' });
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

  http.post(`${API_ROUTES.services.installStream.replace(':id', ':id')}`, ({ params }) => {
    const id = params.id as string;
    const body = [
      JSON.stringify({ type: 'log', data: `Installing package: ${id}` }),
      JSON.stringify({ type: 'log', data: `Fetching ${id}...` }),
      JSON.stringify({ type: 'log', data: `Package ${id} installed successfully` }),
      JSON.stringify({ type: 'done' }),
    ].join('\n');
    return new HttpResponse(body, {
      headers: { 'Content-Type': 'application/x-ndjson' },
    });
  }),

  http.post(API_ROUTES.services.remove, () => {
    return HttpResponse.json({ success: true });
  }),

  http.post(`${API_ROUTES.services.removeStream.replace(':id', ':id')}`, ({ params }) => {
    const id = params.id as string;
    const body = [
      JSON.stringify({ type: 'log', data: `Removing package: ${id}` }),
      JSON.stringify({ type: 'log', data: `Package ${id} removed successfully` }),
      JSON.stringify({ type: 'done' }),
    ].join('\n');
    return new HttpResponse(body, {
      headers: { 'Content-Type': 'application/x-ndjson' },
    });
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

  http.get(API_ROUTES.network.dhcp, () => {
    return HttpResponse.json(mockDHCPConfig);
  }),
  http.put(API_ROUTES.network.dhcp, () => {
    return HttpResponse.json({ status: 'ok' });
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

  http.put(API_ROUTES.auth.password, async ({ request }) => {
    const body = (await request.json()) as { current_password: string; new_password: string };
    if (body.current_password !== 'admin') {
      return HttpResponse.json({ error: 'invalid current password' }, { status: 401 });
    }
    if (body.new_password.length < 6) {
      return HttpResponse.json(
        { error: 'new password must be at least 6 characters' },
        { status: 400 },
      );
    }
    return HttpResponse.json({ status: 'ok' });
  }),

  http.post(API_ROUTES.system.reboot, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.put(API_ROUTES.system.hostname, async ({ request }) => {
    const body = (await request.json()) as { hostname: string };
    if (!body.hostname) {
      return HttpResponse.json({ error: 'hostname is required' }, { status: 400 });
    }
    return HttpResponse.json({ status: 'ok' });
  }),

  http.get(API_ROUTES.system.leds, () => {
    return HttpResponse.json({ stealth_mode: false, led_count: 3 });
  }),

  http.put(API_ROUTES.system.leds, async ({ request }) => {
    const body = (await request.json()) as { stealth_mode: boolean };
    return HttpResponse.json({ stealth_mode: body.stealth_mode, led_count: 3 });
  }),
];
