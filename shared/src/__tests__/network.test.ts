import { describe, it, expect } from 'vitest';
import {
  isNetworkStatus,
  type NetworkInterface,
  type WanConfig,
  type Client,
  type NetworkStatus,
  type WanType,
} from '../api/network';

describe('WanType', () => {
  it('has correct values', () => {
    const validTypes: WanType[] = ['dhcp', 'static', 'pppoe', 'usb_tethering', 'none'];
    expect(validTypes).toHaveLength(5);
  });
});

describe('NetworkStatus', () => {
  const lanInterface: NetworkInterface = {
    name: 'br-lan',
    type: 'lan',
    ip_address: '192.168.1.1',
    netmask: '255.255.255.0',
    gateway: '',
    dns_servers: ['192.168.1.1'],
    mac_address: 'AA:BB:CC:DD:EE:FF',
    is_up: true,
    rx_bytes: 1000000,
    tx_bytes: 2000000,
  };

  const wanInterface: NetworkInterface = {
    name: 'eth0',
    type: 'wan',
    ip_address: '10.0.0.2',
    netmask: '255.255.255.0',
    gateway: '10.0.0.1',
    dns_servers: ['8.8.8.8', '8.8.4.4'],
    mac_address: '11:22:33:44:55:66',
    is_up: true,
    rx_bytes: 5000000,
    tx_bytes: 3000000,
  };

  const client: Client = {
    ip_address: '192.168.1.100',
    mac_address: 'AA:BB:CC:DD:EE:01',
    hostname: 'my-phone',
    interface_name: 'br-lan',
    rx_bytes: 500,
    tx_bytes: 300,
    connected_since: '2025-01-01T00:00:00Z',
  };

  const validStatus: NetworkStatus = {
    wan: wanInterface,
    lan: lanInterface,
    interfaces: [lanInterface, wanInterface],
    clients: [client],
    internet_reachable: true,
  };

  it('validates correct NetworkStatus', () => {
    expect(isNetworkStatus(validStatus)).toBe(true);
  });

  it('accepts null wan', () => {
    const status: NetworkStatus = { ...validStatus, wan: null };
    expect(isNetworkStatus(status)).toBe(true);
  });

  it('rejects invalid data', () => {
    expect(isNetworkStatus(null)).toBe(false);
    expect(isNetworkStatus({})).toBe(false);
    expect(isNetworkStatus({ wan: wanInterface })).toBe(false);
    expect(isNetworkStatus('string')).toBe(false);
  });

  it('validates WanConfig structure', () => {
    const config: WanConfig = {
      type: 'dhcp',
      interface_name: 'eth0',
      ip_address: '10.0.0.2',
      netmask: '255.255.255.0',
      gateway: '10.0.0.1',
      dns_servers: ['8.8.8.8'],
      mtu: 1500,
    };
    expect(config.type).toBe('dhcp');
    expect(config.mtu).toBe(1500);
  });
});
