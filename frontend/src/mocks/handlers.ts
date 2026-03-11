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
  mockWireguardStatus,
  mockWireguardProfiles,
  mockWanConfig,
  mockClients,
  mockSystemLogs,
  mockKernelLogs,
  mockDHCPConfig,
  mockDNSConfig,
  mockTimezoneConfig,
  mockAPConfigs,
  mockMACAddresses,
  mockDHCPLeases,
  mockGuestWifi,
  mockDNSEntries,
  mockDHCPReservations,
  mockBlockedClients,
  mockRadios,
  mockKillSwitchStatus,
} from './data';

export const handlers = [
  http.get(API_ROUTES.system.info, () => {
    return HttpResponse.json(mockSystemInfo);
  }),

  http.get(API_ROUTES.system.stats, () => {
    return HttpResponse.json(mockSystemStats);
  }),

  http.get(API_ROUTES.system.logs, ({ request }) => {
    const url = new URL(request.url);
    const service = url.searchParams.get('service');
    const level = url.searchParams.get('level');
    const levelSeverity: Record<string, number> = {
      emerg: 0,
      alert: 1,
      crit: 2,
      err: 3,
      warning: 4,
      notice: 5,
      info: 6,
      debug: 7,
    };
    let lines = [...mockSystemLogs.lines];
    if (service) {
      const lower = service.toLowerCase();
      lines = lines.filter((entry) => entry.line.toLowerCase().includes(lower));
    }
    if (level && level in levelSeverity) {
      const minSev = levelSeverity[level];
      lines = lines.filter((entry) => {
        const entrySev = levelSeverity[entry.level];
        return entrySev !== undefined && entrySev <= minSev;
      });
    }
    return HttpResponse.json({
      source: mockSystemLogs.source,
      lines,
      total: lines.length,
    });
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

  http.get(API_ROUTES.wifi.radios, () => {
    return HttpResponse.json(mockRadios);
  }),

  http.delete(`${API_ROUTES.wifi.deleteSaved}/:section`, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.put(API_ROUTES.wifi.savedPriority, () => {
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

  http.post(API_ROUTES.services.autostart, () => {
    return HttpResponse.json({ status: 'ok' });
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

  http.put(API_ROUTES.network.clientAlias, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.get(API_ROUTES.network.dhcp, () => {
    return HttpResponse.json(mockDHCPConfig);
  }),
  http.put(API_ROUTES.network.dhcp, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.get(API_ROUTES.network.dns, () => {
    return HttpResponse.json(mockDNSConfig);
  }),
  http.put(API_ROUTES.network.dns, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.get(API_ROUTES.network.dnsEntries, () => {
    return HttpResponse.json(mockDNSEntries);
  }),
  http.post(API_ROUTES.network.dnsEntries, () => {
    return HttpResponse.json({ status: 'ok' });
  }),
  http.delete(`${API_ROUTES.network.dnsEntries}/:section`, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.get(API_ROUTES.network.dhcpLeases, () => {
    return HttpResponse.json(mockDHCPLeases);
  }),

  http.get(API_ROUTES.network.dhcpReservations, () => {
    return HttpResponse.json(mockDHCPReservations);
  }),
  http.post(API_ROUTES.network.dhcpReservations, () => {
    return HttpResponse.json({ status: 'ok' });
  }),
  http.delete(`${API_ROUTES.network.dhcpReservations}/:section`, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.post(API_ROUTES.network.clientKick, () => {
    return HttpResponse.json({ status: 'ok' });
  }),
  http.post(API_ROUTES.network.clientBlock, () => {
    return HttpResponse.json({ status: 'ok' });
  }),
  http.post(API_ROUTES.network.clientUnblock, () => {
    return HttpResponse.json({ status: 'ok' });
  }),
  http.get(API_ROUTES.network.clientBlocked, () => {
    return HttpResponse.json(mockBlockedClients);
  }),

  http.post(`${API_ROUTES.network.interfaceState.replace(':name', ':name')}`, () => {
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

  http.post(API_ROUTES.system.factoryReset, () => {
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

  http.get(API_ROUTES.system.timezone, () => {
    return HttpResponse.json(mockTimezoneConfig);
  }),
  http.put(API_ROUTES.system.timezone, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.get(API_ROUTES.system.backup, () => {
    return new HttpResponse(new Blob(['mock-backup-data'], { type: 'application/gzip' }), {
      headers: { 'Content-Disposition': 'attachment; filename=openwrt-backup.tar.gz' },
    });
  }),
  http.post(API_ROUTES.system.restore, () => {
    return HttpResponse.json({
      status: 'ok',
      message: 'Configuration restored. Reboot to apply changes.',
    });
  }),

  http.get(API_ROUTES.wifi.ap, () => {
    return HttpResponse.json(mockAPConfigs);
  }),
  http.put(/\/api\/v1\/wifi\/ap\/.*/, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.get(API_ROUTES.wifi.mac, () => {
    return HttpResponse.json(mockMACAddresses);
  }),
  http.put(API_ROUTES.wifi.mac, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.get(API_ROUTES.wifi.guest, () => {
    return HttpResponse.json(mockGuestWifi);
  }),
  http.put(API_ROUTES.wifi.guest, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.get(API_ROUTES.wifi.radio, () => {
    return HttpResponse.json({ enabled: true });
  }),
  http.put(API_ROUTES.wifi.radio, () => {
    return HttpResponse.json({ status: 'ok' });
  }),
];
