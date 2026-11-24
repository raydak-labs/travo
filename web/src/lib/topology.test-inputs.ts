import type {
  ClientsOutputBody,
  DashboardConnectivity,
  DashboardUplinks,
  SystemInfoBody,
  TailscaleStatus,
  ThroughputBody,
  TunnelStatus,
} from './api/gen';

// Responses of the mock daemon (`scripts/with-mock.sh`), trimmed only where noted.
export const uplinks: DashboardUplinks = {
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
export const connectivity: DashboardConnectivity = {
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
export const throughput: ThroughputBody = {
  interfaces: [
    { name: 'br-lan', rx_bytes_per_second: 0, tx_bytes_per_second: 0 },
    { name: 'phy0-sta0', rx_bytes_per_second: 1250, tx_bytes_per_second: null },
    { name: 'wan', rx_bytes_per_second: 10, tx_bytes_per_second: 20 },
  ],
};
export const clients: ClientsOutputBody = {
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
export const resources: SystemInfoBody = {
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
export const tailscale: TailscaleStatus = {
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
export const tunnel = (state: TunnelStatus['state']): TunnelStatus => ({
  state,
  handshake_age_seconds: null,
  observed_at: '2026-11-24T18:46:59Z',
  stale: false,
});
