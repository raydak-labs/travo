import { http, HttpResponse } from 'msw/http';
import { API_ROUTES, MIN_PASSWORD_LENGTH } from '@shared/index';
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
  mockNTPConfig,
  mockAPConfigs,
  mockMACAddresses,
  mockDHCPLeases,
  mockGuestWifi,
  mockDNSEntries,
  mockDHCPReservations,
  mockBlockedClients,
  mockRadios,
  mockKillSwitchStatus,
  mockDDNSConfigResponse,
  mockDDNSStatus,
  mockSQMConfig,
} from './data';
import type {
  AdGuardConfig,
  ConnectionMethod,
  FailoverConfig,
  FailoverEvent,
  SpeedTestResult,
  SpeedtestServiceStatus,
} from '@shared/index';

// Fixtures for routes that had no mock until now. They live here rather than in
// mocks/data.ts because that file belongs to another lane's change set.
const mockSpeedtestServiceStatus: SpeedtestServiceStatus = {
  installed: true,
  supported: true,
  architecture: 'aarch64_cortex-a53',
  version: '1.1.0',
  package_name: 'wget-ssp-speedtest',
  storage_size_mb: 2,
};

const mockSpeedTestResult: SpeedTestResult = {
  download_mbps: 84.2,
  upload_mbps: 21.5,
  ping_ms: 18,
  server: 'Mock ISP, Frankfurt',
};

const mockConnectionMethod: ConnectionMethod = {
  method: 'wifi-client',
  interface: 'wwan0',
  ip_address: '192.168.1.105',
};

const mockFailoverConfig: FailoverConfig = {
  available: true,
  service_installed: true,
  enabled: false,
  active_interface: 'wwan0',
  candidates: [
    {
      id: 'wan',
      label: 'WAN (Ethernet)',
      interface_name: 'wan',
      kind: 'ethernet',
      available: true,
      enabled: true,
      priority: 1,
      tracking_state: 'online',
      is_up: true,
    },
    {
      id: 'wwan',
      label: 'WWAN (WiFi Client)',
      interface_name: 'wwan0',
      kind: 'wifi',
      available: true,
      enabled: true,
      priority: 2,
      tracking_state: 'online',
      is_up: true,
    },
  ],
  health: {
    track_ips: ['1.1.1.1', '8.8.8.8'],
    reliability: 2,
    count: 3,
    timeout: 3,
    interval: 60,
    failure_interval: 10,
    recovery_interval: 60,
    down: 3,
    up: 2,
  },
};

const mockFailoverEvents: FailoverEvent[] = [
  {
    from_interface: 'eth2',
    to_interface: 'wwan0',
    timestamp: 1772000000,
    reason: 'wan link down',
  },
];

const mockAdGuardConfig: AdGuardConfig = {
  content: 'dns:\n  bind_hosts:\n    - 0.0.0.0\n  port: 53\nfilters: []\n',
};

export const handlers = [
  http.get(API_ROUTES.system.info, () => {
    return HttpResponse.json(mockSystemInfo);
  }),

  http.get(API_ROUTES.system.stats, () => {
    return HttpResponse.json(mockSystemStats);
  }),

  http.get(API_ROUTES.system.statsHistory, () => {
    // Generate 20 mock data points over the last 10 minutes
    const now = Math.floor(Date.now() / 1000);
    const points = Array.from({ length: 20 }, (_, i) => ({
      time: now - (19 - i) * 30,
      cpu: 20 + Math.random() * 40,
      memory: 45 + Math.random() * 15,
      rx_bytes: 1000000 * (i + 1),
      tx_bytes: 500000 * (i + 1),
    }));
    return HttpResponse.json(points);
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

  http.get(API_ROUTES.network.trafficHistory, () => {
    // A real server holds ~10 minutes here; the mock keeps one point so the
    // chart path is exercised without inventing a plausible-looking series.
    return HttpResponse.json({ points: [], retained_seconds: 0 });
  }),

  http.get(API_ROUTES.sqm.config, () => {
    return HttpResponse.json(mockSQMConfig);
  }),

  http.put(API_ROUTES.sqm.config, async ({ request }) => {
    const body = (await request.json()) as typeof mockSQMConfig;
    return HttpResponse.json({ status: 'ok', saved: body });
  }),

  http.post(API_ROUTES.sqm.apply, () => {
    return HttpResponse.json({ ok: true, output: 'restarted' });
  }),

  http.get(API_ROUTES.wifi.scan, () => {
    return HttpResponse.json(mockWifiScanResults);
  }),

  http.get(API_ROUTES.wifi.connection, () => {
    return HttpResponse.json(mockWifiConnection);
  }),

  http.get(API_ROUTES.wifi.health, () => {
    return HttpResponse.json({ status: 'ok', issues: [] });
  }),

  http.post(API_ROUTES.wifi.connect, async ({ request }) => {
    const body = (await request.json()) as { ssid: string; password: string };
    return HttpResponse.json({
      status: 'ok',
      apply: { pending: true, token: `apply-${body.ssid}`, rollback_timeout_seconds: 30 },
    });
  }),

  http.post(API_ROUTES.wifi.disconnect, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.put(API_ROUTES.wifi.mode, async ({ request }) => {
    const body = (await request.json()) as { mode: string };
    return HttpResponse.json({
      status: 'ok',
      apply: { pending: true, token: `apply-mode-${body.mode}`, rollback_timeout_seconds: 30 },
    });
  }),

  http.post(API_ROUTES.wifi.applyConfirm, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.get(API_ROUTES.wifi.saved, () => {
    return HttpResponse.json(mockSavedNetworks);
  }),

  http.get(API_ROUTES.wifi.radios, () => {
    return HttpResponse.json(mockRadios);
  }),

  http.put(`${API_ROUTES.wifi.radios}/:name/role`, () => {
    return HttpResponse.json({ status: 'ok', pending: false });
  }),

  http.get(API_ROUTES.wifi.bandSwitching, () => {
    return HttpResponse.json({
      config: {
        enabled: false,
        preferred_band: '5g',
        check_interval_sec: 10,
        down_switch_threshold_dbm: -70,
        down_switch_delay_sec: 30,
        up_switch_threshold_dbm: -60,
        up_switch_delay_sec: 60,
        min_viable_signal_dbm: -80,
      },
      status: {
        state: 'inactive',
        current_band: '',
        signal_dbm: 0,
        weak_signal_secs: 0,
        cooldown_sec: 0,
      },
    });
  }),

  http.put(API_ROUTES.wifi.bandSwitching, () => {
    return HttpResponse.json({ status: 'ok' });
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

  http.post(API_ROUTES.captive.autoAccept, () => {
    return HttpResponse.json({
      ok: true,
      message: 'mock auto-accept',
      detected: false,
      can_reach_internet: true,
    });
  }),

  http.post(API_ROUTES.captive.dnsBypass, () => {
    return HttpResponse.json({ ok: true, message: 'DNS switched to upstream' });
  }),

  http.post(API_ROUTES.captive.dnsRestore, () => {
    return HttpResponse.json({ ok: true, message: 'DNS restored' });
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

  http.get(API_ROUTES.network.wanDetect, () => {
    return HttpResponse.json({ detected_type: 'dhcp', current_type: 'dhcp' });
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

  http.post(API_ROUTES.system.timeSync, () => {
    return HttpResponse.json({ ok: true, skew_seconds: 0 });
  }),

  http.post(API_ROUTES.auth.login, async ({ request }) => {
    const body = (await request.json()) as { password: string };
    if (body.password === 'admin') {
      return HttpResponse.json({
        token: 'mock-jwt-token-abc123',
        expires_at: '2026-03-05T00:00:00Z',
        expires_in: 86400,
      });
    }
    return HttpResponse.json({ error: 'Invalid password' }, { status: 401 });
  }),

  http.post(API_ROUTES.auth.logout, () => {
    return HttpResponse.json({ success: true });
  }),

  http.get(API_ROUTES.auth.session, () => {
    return HttpResponse.json({ valid: true, expires_in: 86400 });
  }),

  http.put(API_ROUTES.auth.password, async ({ request }) => {
    const body = (await request.json()) as { current_password: string; new_password: string };
    if (body.current_password !== 'admin') {
      return HttpResponse.json({ error: 'invalid current password' }, { status: 401 });
    }
    // Must match the backend: a change revokes every session (the caller's
    // included) and returns a replacement token, which the client MUST store.
    // A response without `token` makes the client persist "undefined" as its
    // session token. See shared/src/api/auth.ts ChangePasswordResponse.
    if (body.new_password.length < MIN_PASSWORD_LENGTH) {
      return HttpResponse.json(
        { error: `new password must be at least ${MIN_PASSWORD_LENGTH} characters` },
        { status: 400 },
      );
    }
    return HttpResponse.json({
      status: 'ok',
      token: 'mock-rotation-token',
      expires_at: new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString(),
      expires_in: 24 * 60 * 60,
      revoked_sessions: 1,
    });
  }),

  http.post(API_ROUTES.system.reboot, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.post(API_ROUTES.system.shutdown, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.post(API_ROUTES.system.ntpSync, () => {
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

  http.get(API_ROUTES.system.ledsSchedule, () => {
    return HttpResponse.json({ enabled: false, on_time: '08:00', off_time: '22:00' });
  }),

  http.put(API_ROUTES.system.ledsSchedule, () => {
    return HttpResponse.json({ status: 'ok' });
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

  http.get(API_ROUTES.system.ntp, () => {
    return HttpResponse.json(mockNTPConfig);
  }),
  http.put(API_ROUTES.system.ntp, () => {
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

  http.post(API_ROUTES.system.firmwareUpgrade, () => {
    return HttpResponse.json({
      status: 'ok',
      message: 'Firmware upgrade initiated. Device will reboot.',
    });
  }),

  http.get(API_ROUTES.wifi.ap, () => {
    return HttpResponse.json(mockAPConfigs);
  }),
  http.get(API_ROUTES.wifi.repeaterOptions, () => {
    return HttpResponse.json({ allow_ap_on_sta_radio: false });
  }),
  http.put(API_ROUTES.wifi.repeaterOptions, async ({ request }) => {
    const body = (await request.json()) as { allow_ap_on_sta_radio: boolean };
    return HttpResponse.json({
      status: 'ok',
      allow_ap_on_sta_radio: body.allow_ap_on_sta_radio,
    });
  }),
  http.post(API_ROUTES.wifi.repeaterReconcile, () => {
    return HttpResponse.json({
      status: 'ok',
      apply: { pending: true, token: 'apply-repeater-reconcile', rollback_timeout_seconds: 30 },
    });
  }),
  http.put(`${API_ROUTES.wifi.ap}/:section`, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.get(API_ROUTES.wifi.mac, () => {
    return HttpResponse.json(mockMACAddresses);
  }),
  http.put(API_ROUTES.wifi.mac, () => {
    return HttpResponse.json({ status: 'ok' });
  }),
  http.post(API_ROUTES.wifi.macRandomize, () => {
    return HttpResponse.json({ status: 'ok', mac: '02:ab:cd:ef:12:34' });
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

  http.get(API_ROUTES.wifi.autoreconnect, () => {
    return HttpResponse.json({ enabled: false });
  }),
  http.put(API_ROUTES.wifi.autoreconnect, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.get(API_ROUTES.system.setupComplete, () => {
    return HttpResponse.json({ complete: false });
  }),
  http.post(API_ROUTES.system.setupComplete, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.get(API_ROUTES.adguard.dns, () => {
    return HttpResponse.json({ enabled: false, dns_port: 5353 });
  }),
  http.put(API_ROUTES.adguard.dns, async ({ request }) => {
    const body = (await request.json()) as { enabled: boolean };
    return HttpResponse.json({ status: 'ok', enabled: body.enabled });
  }),

  http.get(API_ROUTES.adguard.dnsMode, () => {
    return HttpResponse.json({
      mode: 'adguard-forwarding',
      description: 'dnsmasq forwards DNS to AdGuard Home (port 5353)',
      adguard_running: true,
      dns_bypassed: false,
    });
  }),

  http.get(API_ROUTES.system.alerts, () => {
    return HttpResponse.json({ alerts: [] });
  }),

  http.get(API_ROUTES.network.ddns, () => {
    return HttpResponse.json(mockDDNSConfigResponse);
  }),
  http.put(API_ROUTES.network.ddns, () => {
    return HttpResponse.json({ status: 'ok' });
  }),
  http.get(API_ROUTES.network.ddnsStatus, () => {
    return HttpResponse.json(mockDDNSStatus);
  }),

  http.get(API_ROUTES.system.buttons, () => {
    return HttpResponse.json([
      { name: 'reset', action: 'none' },
      { name: 'wps', action: 'none' },
    ]);
  }),

  http.put(API_ROUTES.system.buttonActions, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.get(API_ROUTES.network.usbTethering, () => {
    return HttpResponse.json({
      detected: false,
      device_type: '',
      interface: '',
      is_up: false,
      ip_address: '',
      configured: false,
    });
  }),

  http.post(API_ROUTES.network.usbTetheringConfigure, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.post(API_ROUTES.network.usbTetheringUnconfigure, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.get(API_ROUTES.network.dataUsage, () => {
    return HttpResponse.json({
      available: true,
      interfaces: [
        {
          name: 'eth0',
          label: 'Ethernet WAN',
          today: { rx_bytes: 52428800, tx_bytes: 15728640 },
          month: { rx_bytes: 2147483648, tx_bytes: 536870912 },
          total: { rx_bytes: 10737418240, tx_bytes: 2684354560 },
        },
        {
          name: 'wwan0',
          label: 'WiFi Uplink',
          today: { rx_bytes: 10485760, tx_bytes: 3145728 },
          month: { rx_bytes: 8589934592, tx_bytes: 1073741824 },
          total: { rx_bytes: 21474836480, tx_bytes: 5368709120 },
        },
      ],
    });
  }),

  http.get(API_ROUTES.network.dataUsageBudget, () => {
    return HttpResponse.json({
      budgets: [
        {
          interface: 'wwan0',
          monthly_limit_bytes: 10737418240,
          warning_threshold_pct: 80,
          reset_day: 1,
        },
      ],
    });
  }),

  http.put(API_ROUTES.network.dataUsageBudget, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.post(API_ROUTES.network.dataUsageReset, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.get(API_ROUTES.network.uptimeLog, () => {
    const now = Date.now();
    return HttpResponse.json([
      { timestamp: now - 1000 * 60 * 2, state: 'connected' },
      { timestamp: now - 1000 * 60 * 35, state: 'disconnected' },
      { timestamp: now - 1000 * 60 * 40, state: 'connected' },
      { timestamp: now - 1000 * 60 * 60 * 3, state: 'disconnected' },
      { timestamp: now - 1000 * 60 * 60 * 3 - 1000 * 60 * 15, state: 'connected' },
    ]);
  }),

  http.get(API_ROUTES.network.firewallZones, () => {
    return HttpResponse.json({ zones: [] });
  }),

  http.get(API_ROUTES.network.portForwards, () => {
    return HttpResponse.json({ rules: [] });
  }),

  http.post(API_ROUTES.network.portForwards, () => {
    return HttpResponse.json({ ok: true });
  }),

  http.delete(`${API_ROUTES.network.portForwards}/:id`, () => {
    return HttpResponse.json({ ok: true });
  }),

  http.post(API_ROUTES.network.diagnostics, () => {
    return HttpResponse.json({
      type: 'ping',
      target: '8.8.8.8',
      output: 'PING 8.8.8.8: 3 packets',
    });
  }),

  http.get(API_ROUTES.network.doh, () => {
    return HttpResponse.json({ enabled: false, provider: 'cloudflare', url: '' });
  }),

  http.put(API_ROUTES.network.doh, () => {
    return HttpResponse.json({ ok: true });
  }),

  http.get(API_ROUTES.network.ipv6, () => {
    return HttpResponse.json({ enabled: false, mode: 'disabled', address: '', prefix: '' });
  }),

  http.put(API_ROUTES.network.ipv6, () => {
    return HttpResponse.json({ ok: true });
  }),

  http.post(API_ROUTES.network.wol, () => {
    return HttpResponse.json({ ok: true });
  }),

  http.get(API_ROUTES.system.alertThresholds, () => {
    return HttpResponse.json({ storage_percent: 90, cpu_percent: 90, memory_percent: 90 });
  }),

  http.put(API_ROUTES.system.alertThresholds, () => {
    return HttpResponse.json({ ok: true });
  }),

  http.get(API_ROUTES.system.sshKeys, () => {
    return HttpResponse.json({ keys: [] });
  }),

  http.post(API_ROUTES.system.sshKeys, () => {
    return HttpResponse.json({ ok: true });
  }),

  http.delete(`${API_ROUTES.system.sshKeys}/:index`, () => {
    return HttpResponse.json({ ok: true });
  }),

  http.post(API_ROUTES.system.speedTest, () => {
    return HttpResponse.json({
      download_mbps: 50.0,
      upload_mbps: 20.0,
      ping_ms: 15.0,
      server: 'test',
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

  http.get(API_ROUTES.wifi.schedule, () => {
    return HttpResponse.json({ enabled: false, on_time: '07:00', off_time: '23:00', days: [] });
  }),

  http.put(API_ROUTES.wifi.schedule, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.get(API_ROUTES.wifi.macPolicies, () => {
    return HttpResponse.json({ policies: [] });
  }),

  http.put(API_ROUTES.wifi.macPolicies, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  // Routes below had no handler at all. Without them the dev server hit the
  // network for real (main.tsx starts MSW with onUnhandledFrame "bypass") and
  // every test asserted against whatever the machine answered.

  http.get(API_ROUTES.system.speedtestService, () => {
    return HttpResponse.json(mockSpeedtestServiceStatus);
  }),

  http.post(API_ROUTES.system.speedtestServiceInstall, () => {
    return HttpResponse.json({ ok: true });
  }),

  http.post(API_ROUTES.system.speedtestServiceUninstall, () => {
    return HttpResponse.json({ ok: true });
  }),

  http.post(API_ROUTES.system.speedtestServiceRun, () => {
    return HttpResponse.json(mockSpeedTestResult);
  }),

  http.get(API_ROUTES.network.connectionMethod, () => {
    return HttpResponse.json(mockConnectionMethod);
  }),

  http.get(API_ROUTES.network.failover, () => {
    return HttpResponse.json(mockFailoverConfig);
  }),

  http.put(API_ROUTES.network.failover, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.get(API_ROUTES.network.failoverEvents, () => {
    return HttpResponse.json(mockFailoverEvents);
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

  http.post(API_ROUTES.vpn.speedTest, () => {
    return HttpResponse.json(mockSpeedTestResult);
  }),

  http.get(API_ROUTES.adguard.config, () => {
    return HttpResponse.json(mockAdGuardConfig);
  }),

  http.put(API_ROUTES.adguard.config, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.put(API_ROUTES.adguard.password, () => {
    return HttpResponse.json({ status: 'ok' });
  }),
];
