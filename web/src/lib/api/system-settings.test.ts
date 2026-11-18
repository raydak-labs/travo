import { afterEach, describe, expect, it, vi } from 'vitest';
import { getSystemSettings, getTimezones, type SystemSettingsBody, setSystemSettings } from './gen';
import {
  fetchDeviceSettings,
  parseServers,
  saveDeviceSettings,
  settingsError,
  validHostname,
} from './system-settings';

vi.mock('./client', () => ({ client: {} }));
vi.mock('./gen', () => ({
  getSystemSettings: vi.fn(),
  getTimezones: vi.fn(),
  setSystemSettings: vi.fn(),
}));

afterEach(() => vi.resetAllMocks());

// The shape `uci export system` gives on the AXT1800 (testdata uci-export-system.txt).
const current: SystemSettingsBody = {
  hostname: 'OpenWrt',
  zonename: 'UTC',
  timezone: 'GMT0',
  ntp: { enabled: true, servers: ['0.openwrt.pool.ntp.org', '1.openwrt.pool.ntp.org'] },
};

function reply<T>(data: T | undefined, error?: { code?: string }, status = 200) {
  return {
    data,
    error,
    request: new Request('http://localhost'),
    response: new Response(null, { status }),
  } as never;
}

describe('device settings helpers', () => {
  it('splits servers on lines, commas and spaces, keeping order', () => {
    expect(parseServers(' a.org\n\nb.org, 10.0.0.1  c.org ')).toEqual([
      'a.org',
      'b.org',
      '10.0.0.1',
      'c.org',
    ]);
    expect(parseServers('   ')).toEqual([]);
  });

  it('accepts one DNS label as hostname', () => {
    expect(validHostname('OpenWrt')).toBe(true);
    expect(validHostname('travo-1')).toBe(true);
    expect(validHostname('a'.repeat(63))).toBe(true);
    expect(validHostname('a'.repeat(64))).toBe(false);
    expect(validHostname('-a')).toBe(false);
    expect(validHostname('a.b')).toBe(false);
    expect(validHostname('')).toBe(false);
  });

  it('maps codes to messages', () => {
    expect(settingsError('uci_busy')).toContain('in progress');
    expect(settingsError('invalid_request')).toContain('rejected');
    expect(settingsError(undefined)).toContain('Could not save');
  });

  it('loads settings and zones together, failing as one', async () => {
    vi.mocked(getSystemSettings).mockResolvedValue(reply(current));
    vi.mocked(getTimezones).mockResolvedValue(reply({ zonenames: ['UTC', 'Europe/Berlin'] }));
    expect(await fetchDeviceSettings()).toEqual({
      settings: current,
      zonenames: ['UTC', 'Europe/Berlin'],
    });
    vi.mocked(getSystemSettings).mockResolvedValue(reply(undefined, undefined, 503));
    await expect(fetchDeviceSettings()).rejects.toMatchObject({ status: 503 });
  });

  it('sends the stored timezone back for an unchanged zone', async () => {
    vi.mocked(setSystemSettings).mockResolvedValue(reply(current));
    await saveDeviceSettings(current, {
      hostname: ' travo ',
      zonename: 'UTC',
      ntpEnabled: true,
      servers: 'pool.ntp.org',
    });
    expect(setSystemSettings).toHaveBeenCalledWith({
      body: {
        hostname: 'travo',
        zonename: 'UTC',
        timezone: 'GMT0',
        ntp: { enabled: true, servers: ['pool.ntp.org'] },
      },
    });
  });

  it('lets the daemon derive the timezone for a new zone', async () => {
    vi.mocked(setSystemSettings).mockResolvedValue(reply(undefined, { code: 'uci_busy' }, 409));
    const result = await saveDeviceSettings(current, {
      hostname: 'OpenWrt',
      zonename: 'Europe/Berlin',
      ntpEnabled: false,
      servers: '',
    });
    expect(vi.mocked(setSystemSettings).mock.calls[0]?.[0].body.timezone).toBe('');
    expect(result).toEqual({ error: settingsError('uci_busy') });
  });
});
