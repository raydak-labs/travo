import { afterEach, describe, expect, it, vi } from 'vitest';
import { listFirewallZones, listNetworkInterfaces, restartNetworkInterface } from './gen';
import {
  fetchInterfaces,
  fetchZones,
  interfaceAddresses,
  interfaceStatus,
  restartable,
  restartUplink,
} from './network-view';

vi.mock('./client', () => ({ client: {} }));
vi.mock('./gen', () => ({
  listNetworkInterfaces: vi.fn(),
  listFirewallZones: vi.fn(),
  restartNetworkInterface: vi.fn(),
}));

afterEach(() => vi.resetAllMocks());

const reply = (data: unknown, status = 200) =>
  ({
    data,
    request: new Request('http://localhost'),
    response: new Response(null, { status }),
  }) as never;

const refused = (code: string) =>
  ({
    data: undefined,
    error: { code },
    request: new Request('http://localhost'),
    response: new Response(null, { status: 409 }),
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

  it('offers restart for uplinks only, never wan6, lan or guest', () => {
    expect(['wan', 'wan6', 'wwan', 'usbtether', 'lan', 'guest'].filter(restartable)).toEqual([
      'wan',
      'wwan',
      'usbtether',
    ]);
  });

  it('restarts with the acknowledgement only when asked and separates a lockout refusal', async () => {
    const restart = vi.mocked(restartNetworkInterface);
    restart.mockResolvedValueOnce(reply({ interface: 'wwan', up: false, elapsed_seconds: 30 }));
    await expect(restartUplink('wwan')).resolves.toEqual({ up: false });
    expect(restart.mock.calls[0]?.[0]?.body).toBeUndefined();
    restart.mockResolvedValueOnce(refused('wifi_lockout_risk'));
    await expect(restartUplink('wwan', true)).resolves.toEqual({ lockout: true });
    expect(restart.mock.calls[1]?.[0]?.body).toEqual({ acknowledge_lockout: true });
    restart.mockResolvedValueOnce(refused('transaction_pending'));
    await expect(restartUplink('wan')).resolves.toEqual({
      error: 'A change is waiting for confirmation; confirm or roll it back first.',
    });
    restart.mockRejectedValueOnce(new Error('offline'));
    await expect(restartUplink('wan')).resolves.toEqual({
      error: 'The interface could not be restarted.',
    });
  });
});
