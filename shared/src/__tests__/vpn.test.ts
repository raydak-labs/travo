import { describe, it, expect } from 'vitest';
import {
  isVpnStatus,
  type VpnType,
  type VpnStatus,
  type WireguardConfig,
  type WireguardPeer,
  type TailscaleStatus,
} from '../api/vpn';

describe('VpnType', () => {
  it('has correct values', () => {
    const types: VpnType[] = ['wireguard', 'openvpn', 'tailscale'];
    expect(types).toHaveLength(3);
  });
});

describe('VpnStatus', () => {
  const validStatus: VpnStatus = {
    type: 'wireguard',
    enabled: true,
    connected: true,
    connected_since: '2025-01-01T00:00:00Z',
    endpoint: '1.2.3.4:51820',
    rx_bytes: 1000000,
    tx_bytes: 2000000,
  };

  it('validates a correct VpnStatus', () => {
    expect(isVpnStatus(validStatus)).toBe(true);
  });

  it('rejects invalid data', () => {
    expect(isVpnStatus(null)).toBe(false);
    expect(isVpnStatus({})).toBe(false);
    expect(isVpnStatus({ type: 'wireguard' })).toBe(false);
    expect(isVpnStatus('string')).toBe(false);
  });

  it('rejects wrong types', () => {
    expect(isVpnStatus({ ...validStatus, enabled: 'yes' })).toBe(false);
    expect(isVpnStatus({ ...validStatus, rx_bytes: '100' })).toBe(false);
  });
});

describe('WireguardConfig', () => {
  it('validates structure', () => {
    const peer: WireguardPeer = {
      public_key: 'abc123=',
      endpoint: '1.2.3.4:51820',
      allowed_ips: ['0.0.0.0/0'],
    };

    const peerWithOptional: WireguardPeer = {
      ...peer,
      preshared_key: 'psk123=',
      last_handshake: '2025-01-01T00:00:00Z',
    };

    const config: WireguardConfig = {
      private_key: 'key123=',
      address: '10.0.0.2/32',
      dns: ['1.1.1.1'],
      peers: [peer, peerWithOptional],
    };
    expect(config.peers).toHaveLength(2);
    expect(config.peers[1].preshared_key).toBe('psk123=');
  });
});

describe('TailscaleStatus', () => {
  it('validates structure', () => {
    const status: TailscaleStatus = {
      installed: true,
      running: true,
      logged_in: true,
      ip_address: '100.64.0.1',
      hostname: 'my-router',
      exit_node_active: false,
    };
    expect(status.exit_node).toBeUndefined();
    expect(status.exit_node_active).toBe(false);
  });

  it('validates with optional exit_node', () => {
    const status: TailscaleStatus = {
      installed: true,
      running: true,
      logged_in: true,
      ip_address: '100.64.0.1',
      hostname: 'my-router',
      exit_node: 'exit-node-1',
      exit_node_active: true,
    };
    expect(status.exit_node).toBe('exit-node-1');
  });
});
