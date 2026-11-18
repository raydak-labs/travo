import { render } from 'svelte/server';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { UsbTetherStatus } from '../api/gen';
import type { LoadStatus } from '../ui/loader.svelte';
import { boundLinkMissing, driverState, fetchUsbTether, usbTetherRefusal } from '../usbtether';
import USBTetherCard from './USBTetherCard.svelte';

const { createLoader, usb } = vi.hoisted(() => ({
  createLoader: vi.fn(),
  usb: {
    status: { state: 'ready' } as LoadStatus,
    data: undefined as UsbTetherStatus | undefined,
    retry: vi.fn(),
    stop: vi.fn(),
  },
}));
vi.mock('../ui/loader.svelte', () => ({ createLoader }));

const drivers = (android: string, ios: string): UsbTetherStatus['drivers'] => [
  { package: 'kmod-usb-net-rndis', phone: 'android', state: android as 'installed' },
  { package: 'kmod-usb-net-cdc-ncm', phone: 'android', state: android as 'installed' },
  { package: 'kmod-usb-net-ipheth', phone: 'ios', state: ios as 'installed' },
  { package: 'usbmuxd', phone: 'ios', state: ios as 'installed' },
];

const base: UsbTetherStatus = {
  links: [],
  drivers: drivers('absent', 'absent'),
  configured: false,
  device: '',
  in_wan_zone: false,
  up: false,
  ipv4: '',
  install_blocked: false,
};

function card(data: Partial<UsbTetherStatus>) {
  usb.data = { ...base, ...data };
  return render(USBTetherCard).body.replace(/<!--.*?-->/g, '');
}

describe('USB tethering card', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    createLoader.mockReturnValue(usb);
  });

  it('polls the status and explains how to connect a phone', () => {
    const body = card({});
    expect(createLoader).toHaveBeenCalledWith(fetchUsbTether, 5000);
    expect(body).toContain('No tethered phone detected');
    expect(body).toContain('Not used as an uplink');
    expect(body.match(/>Install</g)?.length).toBe(2);
  });

  it('offers a detected phone as uplink', () => {
    const body = card({
      drivers: drivers('installed', 'absent'),
      links: [{ name: 'usb0', kind: 'rndis', carrier: true }],
    });
    expect(body).toContain('usb0');
    expect(body).toContain('Use as uplink');
    expect(body.match(/>Install</g)?.length).toBe(1);
  });

  it('shows the active uplink and offers to stop it', () => {
    const body = card({
      links: [{ name: 'eth0', kind: 'ipheth', carrier: true }],
      configured: true,
      device: 'eth0',
      up: true,
      ipv4: '172.20.10.2/28',
      in_wan_zone: true,
    });
    expect(body).toContain('Uplink eth0 up, 172.20.10.2/28');
    expect(body).toContain('iPhone');
    expect(body).not.toContain('Use as uplink');
    expect(body).toContain('Stop using the USB uplink');
  });

  it('says when the bound phone is gone and when an install is unfinished', () => {
    const body = card({ configured: true, device: 'usb0', install_blocked: true });
    expect(body).toContain('The phone on usb0 is not connected');
    expect(body).toContain('Driver install unfinished');
  });
});

describe('USB tethering helpers', () => {
  it('reads one driver state per phone; unknown is never absent', () => {
    expect(driverState({ ...base, drivers: drivers('installed', 'absent') }, 'android')).toBe(
      'installed',
    );
    expect(driverState({ ...base, drivers: drivers('installed', 'absent') }, 'ios')).toBe('absent');
    const mixed = drivers('installed', 'absent');
    mixed[1] = { ...mixed[1], state: 'unknown' } as UsbTetherStatus['drivers'][number];
    expect(driverState({ ...base, drivers: mixed }, 'android')).toBe('unknown');
  });

  it('flags a bound link that is not detected', () => {
    expect(boundLinkMissing({ ...base, configured: true, device: 'usb0' })).toBe(true);
    expect(
      boundLinkMissing({
        ...base,
        configured: true,
        device: 'usb0',
        links: [{ name: 'usb0', kind: 'ncm', carrier: true }],
      }),
    ).toBe(false);
    expect(boundLinkMissing(base)).toBe(false);
  });

  it('maps refusals', () => {
    expect(usbTetherRefusal('usbtether_no_device').error).toBe('No tethered phone was detected.');
    expect(usbTetherRefusal('usbtether_no_change').message).toBeDefined();
    expect(usbTetherRefusal('boom').error).toBe('The USB uplink change failed.');
  });
});
