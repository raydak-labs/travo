import { describe, it, expect } from 'vitest';
import { networkMedium, type NetworkInterface } from '../api/network';

describe('networkMedium', () => {
  const iface = (name: string, type: string): NetworkInterface => ({
    name,
    type,
    ip_address: '',
    netmask: '',
    gateway: '',
    dns_servers: [],
    mac_address: '',
    is_up: true,
    rx_bytes: 0,
    tx_bytes: 0,
  });

  it('maps every discriminator emitted by the router', () => {
    expect(networkMedium(iface('eth2', 'wan'))).toBe('ethernet');
    expect(networkMedium(iface('br-lan', 'lan'))).toBe('lan');
    expect(networkMedium(iface('phy0-sta0', 'wifi'))).toBe('wifi');
    expect(networkMedium(iface('wg0', 'vpn'))).toBe('vpn');
    expect(networkMedium(iface('usb0', 'usb'))).toBe('usb');
  });

  it('falls back to the interface name for firmware reporting the raw ubus name', () => {
    expect(networkMedium(iface('wwan', 'wwan'))).toBe('wifi');
    expect(networkMedium(iface('wlan-sta0', 'wlan-sta0'))).toBe('wifi');
    expect(networkMedium(iface('usb0', 'usb0'))).toBe('usb');
    expect(networkMedium(iface('wg0', 'wg0'))).toBe('vpn');
    expect(networkMedium(iface('eth0', 'eth0'))).toBe('ethernet');
    expect(networkMedium(iface('tun0', 'tun0'))).toBe('unknown');
  });

  it('reports unknown for a missing interface', () => {
    expect(networkMedium(null)).toBe('unknown');
    expect(networkMedium(undefined)).toBe('unknown');
  });
});
