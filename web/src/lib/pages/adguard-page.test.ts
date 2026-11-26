import { render } from 'svelte/server';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { adguardError, adguardUrl, fetchAdGuard } from '../api/adguard';
import type { AdGuardStatus } from '../api/gen';
import type { LoadStatus } from '../ui/loader.svelte';
import { switchState } from '../ui/switch-html.test-helper';
import AdGuard from './AdGuard.svelte';

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
// The DoH and DNS setup cards have their own tests.
vi.mock('./DoHCard.svelte', () => ({ default: () => {} }));
vi.mock('./AdGuardTopologyCard.svelte', () => ({ default: () => {} }));

const installed: AdGuardStatus = {
  installed: true,
  version: '0.107.79-r1',
  running: true,
  loopback: true,
  busy: false,
  default_dns: true,
  fail_open_since: null,
  topology: 'default',
  migration_failed: false,
};

function page(status: AdGuardStatus) {
  loader.data = status;
  vi.stubGlobal('window', { location: { protocol: 'http:', hostname: '192.168.1.1' } });
  return render(AdGuard).body;
}

describe('AdGuard Home page', () => {
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
    expect(body).toContain('Install AdGuard Home');
    expect(body).not.toContain('Open AdGuard Home');
    expect(createLoader).toHaveBeenCalledWith(fetchAdGuard, 10000);
  });

  it('installed and stopped: the running switch is off, default DNS waits for it', () => {
    const body = page({ ...installed, running: false, default_dns: false });
    expect(body).toContain('Stopped');
    expect(switchState(body, 'adguard-running')).toEqual({ checked: false, disabled: false });
    expect(switchState(body, 'adguard-default-dns')).toEqual({ checked: false, disabled: true });
    expect(body).toContain('Start AdGuard Home first.');
  });

  it('running as default DNS: both switches on and the proxy link', () => {
    const body = page(installed);
    expect(switchState(body, 'adguard-running')).toEqual({ checked: true, disabled: false });
    expect(switchState(body, 'adguard-default-dns')).toEqual({ checked: true, disabled: false });
    expect(body).toContain('href="http://192.168.1.1:3001/"');
    expect(body).not.toContain('role="alert"');
  });

  it('expert topology: running cannot be switched off, no default-DNS switch, proxy link stays', () => {
    const body = page({ ...installed, default_dns: false, topology: 'expert' });
    expect(switchState(body, 'adguard-running')).toEqual({ checked: true, disabled: true });
    expect(switchState(body, 'adguard-default-dns')).toBeNull();
    expect(body).toContain('href="http://192.168.1.1:3001/"');
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

  it('failed loopback migration: says it was undone and when it retries', () => {
    expect(page(installed)).not.toContain('was undone');
    const body = page({ ...installed, loopback: false, migration_failed: true });
    expect(body).toContain('Moving AdGuard Home to loopback failed and was undone');
    expect(body).not.toContain('Open AdGuard Home');
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
