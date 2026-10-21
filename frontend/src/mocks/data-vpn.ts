/**
 * Mock fixtures for the vpn routes.
 */
import type {
  VpnStatus,
  WireguardConfig,
  TailscaleStatus,
  WireGuardStatus,
  WireGuardProfile,
  KillSwitchStatus,
} from '@shared/index';

export const mockVpnStatus: VpnStatus = {
  type: 'wireguard',
  enabled: true,
  connected: true,
  connected_since: '2026-03-04T06:00:00Z',
  endpoint: 'vpn.example.com:51820',
  rx_bytes: 104857600,
  tx_bytes: 52428800,
};
export const mockWireguardConfig: WireguardConfig = {
  private_key: 'cGVlcnByaXZhdGVrZXk=',
  address: '10.0.0.2/32',
  dns: ['1.1.1.1', '8.8.8.8'],
  peers: [
    {
      public_key: 'cGVlcnB1YmxpY2tleTE=',
      endpoint: 'vpn.example.com:51820',
      allowed_ips: ['0.0.0.0/0'],
      last_handshake: '2026-03-04T11:55:00Z',
    },
    {
      public_key: 'cGVlcnB1YmxpY2tleXR3bw==',
      endpoint: 'vpn2.example.com:51820',
      allowed_ips: ['10.0.0.0/24', '192.168.1.0/24'],
      preshared_key: 'cHJlc2hhcmVka2V5',
      last_handshake: '2026-03-04T10:30:00Z',
    },
  ],
};
export const mockTailscaleStatus: TailscaleStatus = {
  installed: true,
  running: true,
  logged_in: true,
  ip_address: '100.100.1.42',
  hostname: 'gl-mt3000',
  exit_node: '100.100.1.10',
  exit_node_active: true,
  peers: [
    {
      hostname: 'laptop-home',
      tailscale_ip: '100.100.1.10',
      os: 'linux',
      online: true,
      exit_node: true,
      exit_node_option: true,
      last_seen: new Date().toISOString(),
    },
    {
      hostname: 'phone-pixel',
      tailscale_ip: '100.100.1.11',
      os: 'android',
      online: false,
      exit_node: false,
      exit_node_option: false,
      last_seen: new Date(Date.now() - 3600000).toISOString(),
    },
  ],
};
export const mockWireguardStatus: WireGuardStatus = {
  interface: 'wg0',
  public_key: 'cHVibGlja2V5aWZhY2U=',
  listen_port: 51820,
  peers: [
    {
      public_key: 'cGVlcnB1YmxpY2tleTE=',
      endpoint: '1.2.3.4:51820',
      latest_handshake: Math.floor(Date.now() / 1000) - 92,
      transfer_rx: 129536789,
      transfer_tx: 71234567,
      allowed_ips: '0.0.0.0/0',
    },
  ],
};
export const mockWireguardProfiles: WireGuardProfile[] = [
  {
    id: 'profile-001',
    name: 'Home VPN',
    config:
      '[Interface]\nPrivateKey = cGVlcnByaXZhdGVrZXk=\nAddress = 10.0.0.2/32\nDNS = 1.1.1.1\n\n[Peer]\nPublicKey = cGVlcnB1YmxpY2tleTE=\nEndpoint = vpn.example.com:51820\nAllowedIPs = 0.0.0.0/0\n',
    active: true,
    created_at: '2026-03-01T10:00:00Z',
  },
  {
    id: 'profile-002',
    name: 'Travel VPN',
    config:
      '[Interface]\nPrivateKey = dHJhdmVscHJpdmF0ZWtleQ==\nAddress = 10.0.1.2/32\nDNS = 8.8.8.8\n\n[Peer]\nPublicKey = dHJhdmVscHVibGlja2V5\nEndpoint = travel.vpn.example.com:51820\nAllowedIPs = 0.0.0.0/0\n',
    active: false,
    created_at: '2026-03-05T14:30:00Z',
  },
];
export const mockKillSwitchStatus: KillSwitchStatus = {
  enabled: false,
};
