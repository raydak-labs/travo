import { render } from 'svelte/server';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { adguardError, adguardUrl, fetchAdGuard } from '../api/adguard';
import type { AdGuardStatus } from '../api/gen';
import type { LoadStatus } from '../ui/loader.svelte';
import Services from './Services.svelte';

const { createLoader, loader } = vi.hoisted(() => ({
  createLoader: vi.fn(),
  loader: {
    status: { state: 'ready' } as LoadStatus,
    data: undefined as AdGuardStatus | undefined,
    retry: vi.fn(),
    stop: vi.fn(),
  },
}));
vi.mock('../ui/loader.svelte', () => ({ createLoader }));

const installed: AdGuardStatus = {
  installed: true,
  version: '0.107.79-r1',
  running: true,
  loopback: true,
  busy: false,
  default_dns: true,
  fail_open_since: null,
};

function page(status: AdGuardStatus) {
  loader.data = status;
  vi.stubGlobal('window', { location: { protocol: 'http:', hostname: '192.168.1.1' } });
  return render(Services).body;
}

describe('Services page: AdGuard Home', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    createLoader.mockImplementation(() => loader);
  });

  it('absent: offers the install', () => {
    const body = page({
      ...installed,
      installed: false,
      running: false,
      default_dns: false,
      version: '',
    });
    expect(body).toContain('Not installed');
    expect(body).toContain('Install AdGuard Home');
    expect(body).not.toContain('Open AdGuard Home');
    expect(createLoader).toHaveBeenCalledWith(fetchAdGuard, 10000);
  });

  it('installed and stopped: start, no DNS toggle', () => {
    const body = page({ ...installed, running: false, default_dns: false });
    expect(body).toContain('Stopped');
    expect(body).toContain('Start');
    expect(body).not.toContain('Use as default DNS');
  });

  it('running as default DNS: proxy link and toggle', () => {
    const body = page(installed);
    expect(body).toContain('Running');
    expect(body).toContain('Stop using as default DNS');
    expect(body).toContain('href="http://192.168.1.1:3001/"');
    expect(body).not.toContain('role="alert"');
  });

  it('fail-open: loud banner with re-enable', () => {
    const body = page({
      ...installed,
      default_dns: false,
      fail_open_since: '2026-11-17T08:00:00Z',
    });
    expect(body).toContain('role="alert"');
    expect(body).toContain('AdGuard Home stopped answering');
    expect(body).toContain('Use AdGuard Home again now');
  });

  it('maps error codes, never message text', () => {
    expect(adguardError('adguard_not_answering')).toContain('not resolving');
    expect(adguardError('dns_busy')).toContain('Another change');
    expect(adguardError(undefined)).toContain('failed');
  });

  it('builds the proxy URL for IPv6 hosts', () => {
    expect(adguardUrl({ protocol: 'http:', hostname: 'fd00::1' })).toBe('http://[fd00::1]:3001/');
  });

  it('uses the TLS proxy port for a page loaded over HTTPS', () => {
    expect(adguardUrl({ protocol: 'https:', hostname: '192.168.1.1' })).toBe(
      'https://192.168.1.1:3443/',
    );
  });
});
