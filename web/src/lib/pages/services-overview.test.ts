import { render } from 'svelte/server';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fetchAdGuard } from '../api/adguard';
import type { LoadStatus } from '../ui/loader.svelte';
import Services from './Services.svelte';

const ready: LoadStatus = { state: 'ready' };
const { createLoader } = vi.hoisted(() => ({ createLoader: vi.fn() }));
vi.mock('../ui/loader.svelte', () => ({ createLoader }));

const loader = (data: unknown) => ({ status: ready, data, retry: vi.fn(), stop: vi.fn() });
const entry = (id: string, installed: boolean) => ({
  id,
  installed,
  built_in: false,
  autostart_supported: true,
  autostart: installed,
  running_supported: false,
  running: false,
  size_bytes: 1048576,
  packages: [id],
  luci_path: installed ? `admin/${id}` : '',
  version: installed ? '1.0' : '',
});

function page() {
  vi.stubGlobal('window', { location: { protocol: 'http:', hostname: '192.168.1.1' } });
  createLoader.mockImplementation((fn: unknown) =>
    fn === fetchAdGuard
      ? loader({ installed: true, running: true, default_dns: true, topology: 'default' })
      : loader({ busy: false, entries: [entry('sqm', true), entry('vnstat', false)] }),
  );
  const body = render(Services).body;
  vi.unstubAllGlobals();
  return body;
}

describe('Services overview', () => {
  beforeEach(() => vi.clearAllMocks());

  it('shows installed services before available ones', () => {
    const body = page();
    const installed = body.indexOf('Installed');
    const available = body.indexOf('Available');
    expect(installed).toBeGreaterThan(-1);
    expect(installed).toBeLessThan(body.indexOf('SQM'));
    expect(body.indexOf('SQM')).toBeLessThan(available);
    expect(available).toBeLessThan(body.indexOf('Install (1 MB)'));
  });

  it('opens AdGuard Home here and other installed services in LuCI', () => {
    const body = page();
    expect(body).toContain('href="#/services/adguard"');
    expect(body).toContain('Configure');
    expect(body).toMatch(/href="[^"]*admin\/sqm"/);
  });
});
