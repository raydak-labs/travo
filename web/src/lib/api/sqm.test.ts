import { describe, expect, it } from 'vitest';
import { createClient } from './gen/client';
import { fetchSQM, saveSQM, sqmError, sqmInterfaceOptions } from './sqm';

function mockClient(status: number, body: unknown) {
  return createClient({
    baseUrl: 'http://router.test',
    fetch: async () =>
      new Response(JSON.stringify(body), {
        status,
        headers: { 'content-type': 'application/json' },
      }),
  });
}

describe('sqm api', () => {
  it('maps stable error codes without parsing error text', () => {
    expect(sqmError('invalid_request')).toContain('rejected');
    expect(sqmError('sqm_not_installed')).toContain('sqm-scripts');
    expect(sqmError('uci_busy')).toContain('in progress');
    expect(sqmError('transaction_pending')).toBeTruthy();
    expect(sqmError(undefined)).toContain('Could not load or save SQM');
  });

  it('orders uplink devices first, deduplicates devices, and keeps current interface', () => {
    const opts = sqmInterfaceOptions(
      [
        { name: 'lan', device: 'br-lan', uplink: false },
        { name: 'loopback', device: 'lo', uplink: false },
        { name: 'wwan', device: 'phy0-sta0', uplink: true },
        { name: 'wan', device: 'eth1', uplink: true },
        { name: 'wan6', device: 'eth1', uplink: true },
      ],
      'usb0',
    );
    expect(opts).toEqual([
      { value: 'usb0', label: 'usb0' },
      { value: 'eth1', label: 'eth1 (wan, wan6)' },
      { value: 'phy0-sta0', label: 'phy0-sta0 (wwan)' },
      { value: 'br-lan', label: 'br-lan (lan)' },
    ]);
  });

  it('fetches and saves SQM settings', async () => {
    const sample = {
      enabled: true,
      interface: 'eth1',
      download: 85000,
      upload: 10000,
      qdisc: 'cake' as const,
      script: 'piece_of_cake.qos' as const,
      linklayer: 'ethernet' as const,
      overhead: 44,
    };
    const got = await fetchSQM(undefined, mockClient(200, sample));
    expect(got).toEqual(sample);

    const saved = await saveSQM(sample, mockClient(200, sample));
    expect(saved).toEqual({ settings: sample });

    const missing = await saveSQM(
      sample,
      mockClient(503, { error: 'sqm-scripts is not installed', code: 'sqm_not_installed' }),
    );
    expect('error' in missing && missing.error).toContain('sqm-scripts');
  });
});
