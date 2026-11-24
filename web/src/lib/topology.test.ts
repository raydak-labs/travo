import { describe, expect, it } from 'vitest';
import type {
  ClientsOutputBody,
  DashboardConnectivity,
  DashboardUplinks,
  SystemInfoBody,
  TailscaleStatus,
  ThroughputBody,
  TunnelStatus,
} from './api/gen';
import { topologyModel } from './topology';

// Responses of the mock daemon (`scripts/with-mock.sh`), trimmed only where noted.
const uplinks: DashboardUplinks = {
  uplinks: [
    { name: 'wan', configured: true, up: false, device: null, ipv4: null, ssid: null, kind: null },
    {
      name: 'wwan',
      configured: true,
      up: true,
      device: 'phy0-sta0',
      ipv4: '192.168.90.241/24',
      ssid: 'HomeUplink',
      kind: null,
    },
    {
      name: 'usbtether',
      configured: false,
      up: false,
      device: null,
      ipv4: null,
      ssid: null,
      kind: null,
    },
  ],
  failover: { enabled: false, active: false, order: ['wan', 'wwan'] },
};
const connectivity: DashboardConnectivity = {
  active_interface: 'phy0-sta0',
  carrier: true,
  medium: 'wifi',
  internet: 'online',
  portal_url: null,
  captive: {
    dns_bypass: false,
    restore_in_seconds: null,
    pending: false,
    dhcp_renew: 'not_applicable',
  },
};
// Rows trimmed to the uplink devices; non-zero rates to see the conversion.
const throughput: ThroughputBody = {
  interfaces: [
    { name: 'br-lan', rx_bytes_per_second: 0, tx_bytes_per_second: 0 },
    { name: 'phy0-sta0', rx_bytes_per_second: 1250, tx_bytes_per_second: null },
    { name: 'wan', rx_bytes_per_second: 10, tx_bytes_per_second: 20 },
  ],
};
const clients: ClientsOutputBody = {
  clients: [
    {
      mac: '9c:eb:e8:d3:f8:d1',
      ip: '192.168.1.2',
      connection: 'lan',
      reserved: false,
      blocked: false,
    },
    {
      mac: '22:4e:76:6c:2d:62',
      ip: '192.168.1.151',
      hostname: 'REDACTED-HOST',
      connection: 'wifi',
      ap: 'phy1-ap0',
      ssid: 'TravelAP',
      signal: -38,
      lease_expires_in: 0,
      reserved: false,
      blocked: false,
    },
  ],
  partial: false,
};
const resources: SystemInfoBody = {
  hostname: 'OpenWrt',
  model: 'GL.iNet GL-AXT1800',
  board_name: 'glinet,gl-axt1800',
  openwrt: '25.12.6',
  revision: 'r33311-981c978eff',
  target: 'qualcommax/ipq60xx',
  kernel: '6.12.112',
  uptime_seconds: 224,
  load: [0.39, 0.24, 0.1],
  memory: { total_bytes: 428646400, available_bytes: 149295104 },
  storage: { total_bytes: 81616896, available_bytes: 69513216 },
  app: { version: 'dev', build_time: '' },
};
const tailscale: TailscaleStatus = {
  installed: false,
  version: '',
  running: false,
  busy: false,
  backend_state: '',
  auth_url: '',
  self: { id: '', host_name: '', dns_name: '', ips: [], online: false },
  magic_dns: { enabled: false, suffix: '' },
  exit_nodes: [],
  prefs: {
    exit_node: '',
    advertise_routes: [],
    ssh: false,
    accept_dns: false,
    want_running: false,
  },
  health: [],
  settings: { exit_node: '', advertise_lan: false, ssh: false },
  firewall_in_sync: false,
};
const tunnel = (state: TunnelStatus['state']): TunnelStatus => ({
  state,
  handshake_age_seconds: null,
  observed_at: '2026-11-24T18:46:59Z',
  stale: false,
});

const withUplink = (patch: Partial<(typeof uplinks.uplinks)[number]>, name = 'wwan') => ({
  ...uplinks,
  uplinks: uplinks.uplinks.map((u) => (u.name === name ? { ...u, ...patch } : u)),
});
const sourceOf = (model: ReturnType<typeof topologyModel>, name: string) =>
  model.sources.find((s) => s.name === name);

describe('topologyModel', () => {
  it('derives the whole model from the mock daemon responses', () => {
    expect(
      topologyModel({
        uplinks,
        connectivity,
        throughput,
        clients,
        resources,
        tailscale,
        vpn: tunnel('disabled'),
      }),
    ).toEqual({
      sources: [
        { name: 'wan', state: 'down', detail: null, rate: null },
        {
          name: 'wwan',
          state: 'active',
          detail: 'HomeUplink',
          rate: { rxBitsPerSecond: 10000, txBitsPerSecond: null },
        },
        { name: 'usbtether', state: 'not_configured', detail: null, rate: null },
      ],
      failover: false,
      router: { hostname: 'OpenWrt', model: 'GL.iNet GL-AXT1800' },
      clients: { wifi: 1, lan: 1, partial: false },
      internet: 'online',
      vpn: 'off',
      tailscale: 'absent',
    });
  });

  it('marks an up uplink that is not the active interface as standby, with its rate', () => {
    const model = topologyModel({
      uplinks: withUplink({ up: true, device: 'wan', ipv4: '10.0.0.2/24' }, 'wan'),
      connectivity,
      throughput,
    });
    expect(sourceOf(model, 'wan')).toEqual({
      name: 'wan',
      state: 'standby',
      detail: '10.0.0.2',
      rate: { rxBitsPerSecond: 80, txBitsPerSecond: 160 },
    });
    expect(sourceOf(model, 'wwan')?.state).toBe('active');
  });

  it('makes every up uplink standby when the active interface is null or matches none', () => {
    for (const active of [null, 'eth9']) {
      const model = topologyModel({
        uplinks,
        connectivity: { ...connectivity, active_interface: active },
      });
      expect(sourceOf(model, 'wwan')?.state).toBe('standby');
    }
  });

  it('treats an up uplink without a device as standby without a rate', () => {
    const model = topologyModel({
      uplinks: withUplink({ device: null }),
      connectivity,
      throughput,
    });
    expect(sourceOf(model, 'wwan')).toMatchObject({ state: 'standby', rate: null });
  });

  it('treats a configured uplink that is not up as down (carrier without an address)', () => {
    const model = topologyModel({
      uplinks: withUplink({ up: false, ipv4: null }),
      connectivity,
      throughput,
    });
    expect(sourceOf(model, 'wwan')).toMatchObject({ state: 'down', rate: null });
  });

  it('treats an uplink missing from the list as not configured', () => {
    const model = topologyModel({
      uplinks: { ...uplinks, uplinks: uplinks.uplinks.filter((u) => u.name !== 'usbtether') },
    });
    expect(sourceOf(model, 'usbtether')?.state).toBe('not_configured');
    expect(model.sources.map((s) => s.name)).toEqual(['wan', 'wwan', 'usbtether']);
  });

  it('makes every source unknown without uplinks data, and keeps the rest', () => {
    const model = topologyModel({ connectivity, clients, resources });
    expect(model.sources.map((s) => s.state)).toEqual(['unknown', 'unknown', 'unknown']);
    expect(model.failover).toBeNull();
    expect(model.internet).toBe('online');
    expect(model.clients).toEqual({ wifi: 1, lan: 1, partial: false });
    expect(model.router).toEqual({ hostname: 'OpenWrt', model: 'GL.iNet GL-AXT1800' });
  });

  it('attaches a rate only by device name', () => {
    const model = topologyModel({
      uplinks,
      connectivity,
      throughput: {
        interfaces: [{ name: 'wwan', rx_bytes_per_second: 5, tx_bytes_per_second: 5 }],
      },
    });
    expect(sourceOf(model, 'wwan')?.rate).toBeNull();
  });

  it('builds the detail per uplink kind, without the prefix', () => {
    const noSsid = topologyModel({ uplinks: withUplink({ ssid: null }) });
    expect(sourceOf(noSsid, 'wwan')?.detail).toBe('192.168.90.241');
    const tether = topologyModel({
      uplinks: withUplink(
        { configured: true, up: true, device: 'usb0', ipv4: '172.20.10.2/28', kind: 'ipheth' },
        'usbtether',
      ),
    });
    expect(sourceOf(tether, 'usbtether')?.detail).toBe('ipheth');
    const tetherNoKind = topologyModel({
      uplinks: withUplink(
        { configured: true, up: true, device: 'usb0', ipv4: '172.20.10.2/28' },
        'usbtether',
      ),
    });
    expect(sourceOf(tetherNoKind, 'usbtether')?.detail).toBe('172.20.10.2');
  });

  it('reads failover as unknown when its settings cannot be read', () => {
    expect(topologyModel({ uplinks: { ...uplinks, failover: null } }).failover).toBeNull();
    expect(topologyModel({ uplinks: withUplink({}) }).failover).toBe(false);
  });

  it('does not count offline clients and copies partial', () => {
    const model = topologyModel({
      clients: {
        partial: true,
        clients: [
          ...clients.clients,
          { mac: 'aa:bb:cc:dd:ee:ff', connection: 'offline', reserved: true, blocked: false },
        ],
      },
    });
    expect(model.clients).toEqual({ wifi: 1, lan: 1, partial: true });
  });

  it('gives null, never zero, for a failed clients read', () => {
    expect(topologyModel({}).clients).toBeNull();
  });

  it('reads internet as unknown without connectivity data', () => {
    expect(topologyModel({ uplinks }).internet).toBe('unknown');
    expect(topologyModel({ connectivity: { ...connectivity, internet: 'captive' } }).internet).toBe(
      'captive',
    );
  });

  it.each([
    ['connected', 'connected'],
    ['enabled_not_up', 'connecting'],
    ['no_handshake', 'connecting'],
    ['disabled', 'off'],
    ['configured', 'off'],
  ] as const)('maps VPN %s to %s', (state, want) => {
    expect(topologyModel({ vpn: tunnel(state) }).vpn).toBe(want);
  });

  it('reads VPN as unknown without tunnel data', () => {
    expect(topologyModel({}).vpn).toBe('unknown');
  });

  it.each([
    ['not installed', { installed: false, running: false, backend_state: '' }, 'absent'],
    ['running', { installed: true, running: true, backend_state: 'Running' }, 'connected'],
    ['stopped daemon', { installed: true, running: false, backend_state: '' }, 'off'],
    ['needs login', { installed: true, running: true, backend_state: 'NeedsLogin' }, 'off'],
    ['starting', { installed: true, running: true, backend_state: 'Starting' }, 'off'],
  ] as const)('maps Tailscale %s to %s', (_name, patch, want) => {
    expect(topologyModel({ tailscale: { ...tailscale, ...patch } }).tailscale).toBe(want);
  });

  it('reads Tailscale as unknown, never absent, without status data', () => {
    expect(topologyModel({}).tailscale).toBe('unknown');
  });

  it('keeps the router null without resources data', () => {
    expect(topologyModel({ uplinks }).router).toBeNull();
  });
});
