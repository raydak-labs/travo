import { afterEach, describe, expect, it, vi } from 'vitest';
import { wakeOnLan } from './gen';
import { validWakeMAC, wake, wakeInterfaces } from './wol';

vi.mock('./client', () => ({ client: {} }));
vi.mock('./gen', () => ({ wakeOnLan: vi.fn() }));

afterEach(() => vi.resetAllMocks());

const answer = (data: unknown, error?: unknown) =>
  ({
    data,
    error,
    request: new Request('http://localhost'),
    response: new Response(null, { status: data ? 200 : 400 }),
  }) as never;

describe('wake-on-lan', () => {
  it('accepts unicast MACs only', () => {
    expect(validWakeMAC(' aa:bb:cc:dd:ee:ff ')).toBe(true);
    expect(validWakeMAC('AA-BB-CC-DD-EE-FF')).toBe(true);
    for (const bad of [
      '',
      'aabbccddeeff',
      'aa:bb:cc-dd:ee:ff',
      '01:00:5e:00:00:01',
      'ff:ff:ff:ff:ff:ff',
      '00:00:00:00:00:00',
    ]) {
      expect(validWakeMAC(bad)).toBe(false);
    }
  });

  it('offers up LAN bridges only', () => {
    const row = (
      name: string,
      device: string | undefined,
      up = true,
      uplink = false,
      ipv4 = ['192.168.1.1/24'],
    ) => ({
      name,
      device,
      up,
      uplink,
      ipv4,
    });
    expect(
      wakeInterfaces([
        row('guest', 'br-guest'),
        row('lan', 'br-lan'),
        row('loopback', 'lo'),
        row('wwan', 'phy0-sta0', true, true),
        row('iot', 'br-iot', false),
        row('wan', undefined, false, true, []),
      ]),
    ).toEqual(['guest', 'lan']);
  });

  it('sends the trimmed MAC, the interface only when chosen, and maps error codes', async () => {
    const send = vi.mocked(wakeOnLan);
    const done = { mac: 'aa:bb:cc:dd:ee:ff', interfaces: ['lan'] };
    send.mockResolvedValueOnce(answer(done));
    await expect(wake(' aa:bb:cc:dd:ee:ff ')).resolves.toEqual({ wake: done });
    expect(send.mock.calls[0]?.[0]?.body).toEqual({ mac: 'aa:bb:cc:dd:ee:ff' });
    send.mockResolvedValueOnce(answer(undefined, { code: 'invalid_interface' }));
    await expect(wake('aa:bb:cc:dd:ee:ff', 'guest')).resolves.toEqual({
      error: 'That network is not available for Wake-on-LAN.',
    });
    expect(send.mock.calls[1]?.[0]?.body).toEqual({ mac: 'aa:bb:cc:dd:ee:ff', interface: 'guest' });
    send.mockRejectedValueOnce(new Error('offline'));
    await expect(wake('aa:bb:cc:dd:ee:ff')).resolves.toEqual({
      error: 'Could not send the wake packet.',
    });
  });
});
