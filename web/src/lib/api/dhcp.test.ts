import { afterEach, describe, expect, it, vi } from 'vitest';
import { addDNSName, dhcpError, removeDNSName, saveDHCP, turnsOff, validLeaseTime } from './dhcp';
import { addDnsName, deleteDnsName, setDhcpSettings } from './gen';

vi.mock('./client', () => ({ client: {} }));
vi.mock('./gen', () => ({ addDnsName: vi.fn(), deleteDnsName: vi.fn(), setDhcpSettings: vi.fn() }));

afterEach(() => vi.resetAllMocks());

const answer = (data: unknown, error?: unknown) =>
  ({
    data,
    error,
    request: new Request('http://localhost'),
    response: new Response(null, { status: data ? 200 : 409 }),
  }) as never;

describe('dhcp', () => {
  it('accepts the lease times the daemon accepts', () => {
    for (const ok of ['2m', '45m', '12h', '7d', 'infinite', ' 1h '])
      expect(validLeaseTime(ok)).toBe(true);
    for (const bad of ['', '1m', '0h', '90s', '1w', '12', 'forever', '01h'])
      expect(validLeaseTime(bad)).toBe(false);
  });

  it('flags only an on-to-off save', () => {
    expect(turnsOff({ enabled: true }, { enabled: false })).toBe(true);
    expect(turnsOff({ enabled: false }, { enabled: false })).toBe(false);
    expect(turnsOff({ enabled: true }, { enabled: true })).toBe(false);
  });

  it('maps stable codes, never error text', () => {
    expect(dhcpError('uci_busy')).toBe('Another change is staged; try again.');
    expect(dhcpError('dns_name_exists')).toBe('That name already has a local DNS entry.');
    expect(dhcpError('whatever')).toBe('Could not change the DHCP settings.');
  });

  it('saves the whole config and maps refusals', async () => {
    const view = { enabled: true, start: 10, limit: 20, leasetime: '1h', callerLease: false };
    vi.mocked(setDhcpSettings).mockResolvedValueOnce(answer(view));
    const body = { enabled: true, start: 10, limit: 20, leasetime: '1h' };
    expect(await saveDHCP(body)).toEqual({ view });
    expect(vi.mocked(setDhcpSettings).mock.calls[0]?.[0]?.body).toEqual(body);
    vi.mocked(setDhcpSettings).mockResolvedValueOnce(
      answer(undefined, { code: 'dhcp_foreign_config' }),
    );
    expect(await saveDHCP(body)).toEqual({ error: expect.stringContaining('LuCI') });
    vi.mocked(setDhcpSettings).mockRejectedValueOnce(new Error('offline'));
    expect(await saveDHCP(body)).toEqual({ error: 'Could not change the DHCP settings.' });
  });

  it('adds trimmed names and treats a gone entry as removed', async () => {
    const entry = { id: 'travo_dns_1', names: ['nas'], ip: '192.168.1.20', owned: true };
    vi.mocked(addDnsName).mockResolvedValueOnce(answer(entry));
    expect(await addDNSName(' nas ', ' 192.168.1.20 ')).toEqual({ entry });
    expect(vi.mocked(addDnsName).mock.calls[0]?.[0]?.body).toEqual({
      name: 'nas',
      ip: '192.168.1.20',
    });
    vi.mocked(deleteDnsName).mockResolvedValueOnce(answer(undefined, { code: 'not_found' }));
    expect(await removeDNSName('travo_dns_1')).toEqual({ ok: true });
    vi.mocked(deleteDnsName).mockResolvedValueOnce(answer(undefined, { code: 'dns_name_foreign' }));
    expect(await removeDNSName('cfg01')).toEqual({
      error: 'That entry was set in LuCI; edit it there.',
    });
  });
});
