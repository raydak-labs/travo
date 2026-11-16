import { render } from 'svelte/server';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Rate } from '../api/gen';
import { fetchHealth } from '../api/health';
import type { LoadStatus } from '../ui/loader.svelte';
import Dashboard from './Dashboard.svelte';

const { createLoader, dashboardEvents } = vi.hoisted(() => ({
  createLoader: vi.fn(),
  dashboardEvents: {
    connectivity: { status: { state: 'ready' }, data: undefined, retry: vi.fn() },
    resources: { status: { state: 'ready' }, data: undefined, retry: vi.fn() },
    throughput: { status: { state: 'ready' }, data: undefined as unknown, retry: vi.fn() },
  },
}));
vi.mock('../ui/loader.svelte', () => ({ createLoader }));
vi.mock('../dashboard-events.svelte', () => ({ dashboardEvents }));

function dashboard(status: LoadStatus, interfaces?: Rate[]) {
  createLoader.mockReturnValue({
    status: { state: 'ready' },
    data: undefined,
    retry: vi.fn(),
    stop: vi.fn(),
  });
  dashboardEvents.throughput.status = status;
  dashboardEvents.throughput.data = interfaces ? { interfaces } : undefined;
  return render(Dashboard, { props: { events: dashboardEvents as never } }).body;
}

const interfaces = [
  { name: 'br-lan', rx_bytes_per_second: 0, tx_bytes_per_second: null },
  { name: 'phy0-sta0', rx_bytes_per_second: 1024, tx_bytes_per_second: 256 },
] satisfies Rate[];

describe('Dashboard throughput', () => {
  beforeEach(() => {
    vi.stubGlobal('document', { documentElement: { getAttribute: () => null } });
  });
  afterEach(() => vi.unstubAllGlobals());

  it('lists named interfaces and distinguishes unknown rates from zero', () => {
    const body = dashboard({ state: 'ready' }, interfaces);
    expect(body).toContain('br-lan');
    expect(body).toContain('phy0-sta0');
    expect(body).toContain('0 B/s');
    expect(body).toContain('Unknown');
    expect(body).toContain('1.0 KB/s');
    expect(body).toContain('256 B/s');
    expect(createLoader).toHaveBeenCalledWith(fetchHealth);
  });

  it('keeps last values visible when refresh fails', () => {
    const body = dashboard({ state: 'error', message: 'Throughput unavailable' }, interfaces);
    expect(body).toContain('br-lan');
    expect(body).toContain('Last known value');
    expect(body).toContain('Throughput unavailable');
  });

  it('shows an empty successful snapshot without a failure', () => {
    const body = dashboard({ state: 'ready' }, []);
    expect(body).toContain('No interfaces reported');
    expect(body).not.toContain('Last known value');
  });
});
