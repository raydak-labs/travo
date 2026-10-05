import { describe, expect, it } from 'vitest';
import {
  describeVpnStatus,
  isUplinkActive,
  resolveUplinks,
  uplinkStatsInterface,
} from '@/lib/uplink';
import type { NetworkInterface, NetworkInterfaceStats, VpnStatus } from '@shared/index';

function iface(name: string, type: string, up: boolean, ip = ''): NetworkInterface {
  return {
    name,
    type,
    is_up: up,
    ip_address: ip,
    gateway: '',
    netmask: '',
    mac_address: '',
    rx_bytes: 0,
    tx_bytes: 0,
  } as NetworkInterface;
}

describe('isUplinkActive', () => {
  it('requires an address, not just carrier', () => {
    expect(isUplinkActive(iface('wan', 'wan', true))).toBe(false);
    expect(isUplinkActive(iface('wan', 'wan', true, '10.0.0.2'))).toBe(true);
    expect(isUplinkActive(iface('wan', 'wan', false, '10.0.0.2'))).toBe(false);
    expect(isUplinkActive(undefined)).toBe(false);
  });
});

describe('resolveUplinks', () => {
  it('reports a phone-tethered router as having a live uplink', () => {
    // The regression this guards: the backend omitted network.usbtether from
    // status.Interfaces, so a phone-only uplink read as "No Internet".
    const uplinks = resolveUplinks([
      iface('br-lan', 'lan', true, '192.168.1.1'),
      iface('usbtether', 'usb', true, '10.42.0.2'),
    ]);
    const usb = uplinks.find((u) => u.key === 'usbtether');
    expect(usb?.active).toBe(true);
    expect(uplinks.some((u) => u.active)).toBe(true);
  });

  it('marks a present-but-down uplink active=false and missing as not configured', () => {
    const uplinks = resolveUplinks([iface('eth0', 'wan', false)]);
    const wan = uplinks.find((u) => u.key === 'wan');
    expect(wan?.iface).toBeDefined();
    expect(wan?.active).toBe(false);

    const wwan = uplinks.find((u) => u.key === 'wwan');
    expect(wwan?.notConfiguredCopy).not.toBe(wwan?.inactiveCopy);
  });

  it('gives every uplink the same label on every page', () => {
    const uplinks = resolveUplinks([]);
    expect(uplinks.map((u) => u.key)).toEqual(['wan', 'wwan', 'usbtether']);
    expect(new Set(uplinks.map((u) => u.label)).size).toBe(3);
  });
});

describe('uplinkStatsInterface', () => {
  const stats: NetworkInterfaceStats[] = [
    { interface: 'br-lan', rx_bytes: 1, tx_bytes: 1 },
    { interface: 'wwan0', rx_bytes: 2, tx_bytes: 2 },
    { interface: 'eth0', rx_bytes: 3, tx_bytes: 3 },
  ];

  it('plots the named uplink rather than the first series', () => {
    // br-lan was always index 0, so the "Network Throughput" chart showed LAN.
    expect(uplinkStatsInterface(stats, 'wwan0')?.interface).toBe('wwan0');
    expect(uplinkStatsInterface(stats, 'eth0')?.interface).toBe('eth0');
  });

  it('never silently falls back to the LAN bridge', () => {
    const lanOnly: NetworkInterfaceStats[] = [{ interface: 'br-lan', rx_bytes: 1, tx_bytes: 1 }];
    // No uplink series exists, so there is nothing to plot.
    expect(uplinkStatsInterface(lanOnly, 'eth0')).toBeUndefined();
  });

  it('handles an empty payload', () => {
    expect(uplinkStatsInterface([], 'eth0')).toBeUndefined();
    expect(uplinkStatsInterface(undefined, 'eth0')).toBeUndefined();
  });
});

describe('describeVpnStatus', () => {
  const status = (over: Partial<VpnStatus>): VpnStatus =>
    ({ type: 'wireguard', enabled: false, connected: false, ...over }) as VpnStatus;

  it('reports a connected tunnel as healthy', () => {
    const d = describeVpnStatus([status({ enabled: true, connected: true })]);
    expect(d).toMatchObject({ label: 'On', active: true, healthy: true });
  });

  it('distinguishes enabled-but-not-connected from off', () => {
    // Previously this rendered "Off" directly above a "Disable VPN" button.
    const d = describeVpnStatus([status({ enabled: true, status_detail: 'enabled_not_up' })]);
    expect(d.label).toBe('Enabled, not connected');
    expect(d.active).toBe(true);
    expect(d.healthy).toBe(false);
  });

  it('names a missing handshake', () => {
    expect(
      describeVpnStatus([status({ enabled: true, status_detail: 'up_no_handshake' })]).label,
    ).toBe('Enabled, no handshake');
  });

  it('reports a fully disabled tunnel as off', () => {
    expect(describeVpnStatus([status({})])).toMatchObject({ label: 'Off', active: false });
    expect(describeVpnStatus(undefined).label).toBe('Off');
  });
});
