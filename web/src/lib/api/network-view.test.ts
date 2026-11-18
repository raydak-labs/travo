import { afterEach, describe, expect, it, vi } from 'vitest';
import { listFirewallZones, listNetworkInterfaces } from './gen';
import { fetchInterfaces, fetchZones, interfaceAddresses, interfaceStatus } from './network-view';

vi.mock('./client', () => ({ client: {} }));
vi.mock('./gen', () => ({ listNetworkInterfaces: vi.fn(), listFirewallZones: vi.fn() }));

afterEach(() => vi.resetAllMocks());

const reply = (data: unknown, status = 200) =>
  ({
    data,
    request: new Request('http://localhost'),
    response: new Response(null, { status }),
  }) as never;

describe('network view', () => {
  it('shows uptime when up; a down uplink warns, another down interface is neutral', () => {
    expect(interfaceStatus({ up: true, uptime_seconds: 90242, uplink: false })).toEqual({
      kind: 'ok',
      label: 'Up 1d 1h',
    });
    expect(interfaceStatus({ up: false, uptime_seconds: 0, uplink: true }).kind).toBe('warn');
    expect(interfaceStatus({ up: false, uptime_seconds: 0, uplink: false }).kind).toBe('neutral');
  });

  it('joins both address families, IPv4 first', () => {
    expect(interfaceAddresses({ ipv4: ['192.168.1.1/24'], ipv6: ['fd06:b820:5389::1/60'] })).toBe(
      '192.168.1.1/24, fd06:b820:5389::1/60',
    );
    expect(interfaceAddresses({ ipv4: [], ipv6: [] })).toBe('');
  });

  it('fails as a load error with the status, never as an empty list', async () => {
    vi.mocked(listNetworkInterfaces).mockResolvedValueOnce(
      reply({ interfaces: [], partial: true }),
    );
    await expect(fetchInterfaces()).resolves.toEqual({ interfaces: [], partial: true });
    vi.mocked(listNetworkInterfaces).mockResolvedValueOnce(reply(undefined, 503));
    await expect(fetchInterfaces()).rejects.toMatchObject({ status: 503 });
    vi.mocked(listFirewallZones).mockResolvedValueOnce(reply(undefined, 503));
    await expect(fetchZones()).rejects.toMatchObject({ status: 503 });
  });
});
