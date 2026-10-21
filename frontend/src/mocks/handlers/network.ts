import { http, HttpResponse } from 'msw/http';
import { API_ROUTES } from '@shared/index';
import {
  mockBlockedClients,
  mockClients,
  mockDDNSConfigResponse,
  mockDDNSStatus,
  mockDHCPConfig,
  mockDHCPLeases,
  mockDHCPReservations,
  mockDNSConfig,
  mockDNSEntries,
  mockNetworkStatus,
  mockWanConfig,
} from '../data';
import type { ConnectionMethod, FailoverConfig, FailoverEvent } from '@shared/index';

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

export const networkHandlers = [
  http.get(API_ROUTES.network.status, () => {
    return HttpResponse.json(mockNetworkStatus);
  })

  http.get(API_ROUTES.network.trafficHistory, () => {
    // A real server holds ~10 minutes here; the mock keeps one point so the
    // chart path is exercised without inventing a plausible-looking series.
    return HttpResponse.json({ points: [], retained_seconds: 0 });
  })

  http.get(API_ROUTES.network.wan, () => {
    return HttpResponse.json(mockWanConfig);
  })

  http.get(API_ROUTES.network.wanDetect, () => {
    return HttpResponse.json({ detected_type: 'dhcp', current_type: 'dhcp' });
  })

  http.put(API_ROUTES.network.wan, () => {
    return HttpResponse.json({ success: true });
  })

  http.get(API_ROUTES.network.clients, () => {
    return HttpResponse.json(mockClients);
  })

  http.put(API_ROUTES.network.clientAlias, () => {
    return HttpResponse.json({ status: 'ok' });
  })

  http.get(API_ROUTES.network.dhcp, () => {
    return HttpResponse.json(mockDHCPConfig);
  })
  http.put(API_ROUTES.network.dhcp, () => {
    return HttpResponse.json({ status: 'ok' });
  })

  http.get(API_ROUTES.network.dns, () => {
    return HttpResponse.json(mockDNSConfig);
  })
  http.put(API_ROUTES.network.dns, () => {
    return HttpResponse.json({ status: 'ok' });
  })

  http.get(API_ROUTES.network.dnsEntries, () => {
    return HttpResponse.json(mockDNSEntries);
  })
  http.post(API_ROUTES.network.dnsEntries, () => {
    return HttpResponse.json({ status: 'ok' });
  })
  http.delete(`${API_ROUTES.network.dnsEntries}/:section`, () => {
    return HttpResponse.json({ status: 'ok' });
  })

  http.get(API_ROUTES.network.dhcpLeases, () => {
    return HttpResponse.json(mockDHCPLeases);
  })

  http.get(API_ROUTES.network.dhcpReservations, () => {
    return HttpResponse.json(mockDHCPReservations);
  })
  http.post(API_ROUTES.network.dhcpReservations, () => {
    return HttpResponse.json({ status: 'ok' });
  })
  http.delete(`${API_ROUTES.network.dhcpReservations}/:section`, () => {
    return HttpResponse.json({ status: 'ok' });
  })

  http.post(API_ROUTES.network.clientKick, () => {
    return HttpResponse.json({ status: 'ok' });
  })
  http.post(API_ROUTES.network.clientBlock, () => {
    return HttpResponse.json({ status: 'ok' });
  })
  http.post(API_ROUTES.network.clientUnblock, () => {
    return HttpResponse.json({ status: 'ok' });
  })
  http.get(API_ROUTES.network.clientBlocked, () => {
    return HttpResponse.json(mockBlockedClients);
  })

  http.post(`${API_ROUTES.network.interfaceState.replace(':name', ':name')}`, () => {
    return HttpResponse.json({ status: 'ok' });
  })

  http.get(API_ROUTES.network.ddns, () => {
    return HttpResponse.json(mockDDNSConfigResponse);
  })
  http.put(API_ROUTES.network.ddns, () => {
    return HttpResponse.json({ status: 'ok' });
  })
  http.get(API_ROUTES.network.ddnsStatus, () => {
    return HttpResponse.json(mockDDNSStatus);
  })

  http.get(API_ROUTES.network.uptimeLog, () => {
    const now = Date.now();
    return HttpResponse.json([
      { timestamp: now - 1000 * 60 * 2, state: 'connected' },
      { timestamp: now - 1000 * 60 * 35, state: 'disconnected' },
      { timestamp: now - 1000 * 60 * 40, state: 'connected' },
      { timestamp: now - 1000 * 60 * 60 * 3, state: 'disconnected' },
      { timestamp: now - 1000 * 60 * 60 * 3 - 1000 * 60 * 15, state: 'connected' },
    ]);
  })

  http.get(API_ROUTES.network.firewallZones, () => {
    return HttpResponse.json({ zones: [] });
  })

  http.get(API_ROUTES.network.portForwards, () => {
    return HttpResponse.json({ rules: [] });
  })

  http.post(API_ROUTES.network.portForwards, () => {
    return HttpResponse.json({ ok: true });
  })

  http.delete(`${API_ROUTES.network.portForwards}/:id`, () => {
    return HttpResponse.json({ ok: true });
  })

  http.post(API_ROUTES.network.diagnostics, () => {
    return HttpResponse.json({
      type: 'ping',
      target: '8.8.8.8',
      output: 'PING 8.8.8.8: 3 packets',
    });
  })

  http.get(API_ROUTES.network.doh, () => {
    return HttpResponse.json({ enabled: false, provider: 'cloudflare', url: '' });
  })

  http.put(API_ROUTES.network.doh, () => {
    return HttpResponse.json({ ok: true });
  })

  http.get(API_ROUTES.network.ipv6, () => {
    return HttpResponse.json({ enabled: false, mode: 'disabled', address: '', prefix: '' });
  })

  http.put(API_ROUTES.network.ipv6, () => {
    return HttpResponse.json({ ok: true });
  })

  http.post(API_ROUTES.network.wol, () => {
    return HttpResponse.json({ ok: true });
  })

  http.get(API_ROUTES.network.connectionMethod, () => {
    return HttpResponse.json(mockConnectionMethod);
  })

  http.get(API_ROUTES.network.failover, () => {
    return HttpResponse.json(mockFailoverConfig);
  })

  http.put(API_ROUTES.network.failover, () => {
    return HttpResponse.json({ status: 'ok' });
  })

  http.get(API_ROUTES.network.failoverEvents, () => {
    return HttpResponse.json(mockFailoverEvents);
  }),
];
