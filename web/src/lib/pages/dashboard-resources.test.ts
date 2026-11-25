import { render } from 'svelte/server';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { fetchSystemInfo } from '../api/system';
import type { LoadStatus } from '../ui/loader.svelte';
import Dashboard from './Dashboard.svelte';

type SystemInfo = Awaited<ReturnType<typeof fetchSystemInfo>>;

const fixture = {
  hostname: 'OpenWrt',
  model: 'GL.iNet GL-AXT1800',
  board_name: 'glinet,gl-axt1800',
  openwrt: '25.12.6',
  revision: 'r33311-981c978eff',
  target: 'qualcommax/ipq60xx',
  kernel: '6.12.112',
  uptime_seconds: 224,
  load: [0.39, 0.24, 0.1],
  memory: { total_bytes: 428646400, available_bytes: 149295104 },
  storage: { total_bytes: 81616896, available_bytes: 69513216 },
  app: { version: 'dev', build_time: '' },
} satisfies SystemInfo;

const { createLoader, dashboardEvents } = vi.hoisted(() => ({
  createLoader: vi.fn(),
  dashboardEvents: {
    connectivity: { status: { state: 'ready' }, data: undefined, retry: vi.fn() },
    resources: { status: { state: 'ready' }, data: undefined as unknown, retry: vi.fn() },
    throughput: { status: { state: 'ready' }, data: undefined, retry: vi.fn() },
    tailscale: { status: { state: 'ready' }, data: undefined as unknown, retry: vi.fn() },
    uplinks: { status: { state: 'ready' }, data: undefined, retry: vi.fn() },
    clients: { status: { state: 'ready' }, data: undefined, retry: vi.fn() },
    alerts: { status: { state: 'ready' }, data: undefined, retry: vi.fn() },
  },
}));
vi.mock('../ui/loader.svelte', () => ({ createLoader }));
vi.mock('../dashboard-events.svelte', () => ({ dashboardEvents }));

function dashboard(status: LoadStatus, data?: SystemInfo) {
  createLoader.mockReturnValue({
    status: { state: 'ready' },
    data: undefined,
    retry: vi.fn(),
    stop: vi.fn(),
  });
  dashboardEvents.resources.status = status;
  dashboardEvents.resources.data = data;
  return render(Dashboard, { props: { events: dashboardEvents as never } }).body;
}

describe('Dashboard resources', () => {
  beforeEach(() => {
    vi.stubGlobal('document', { documentElement: { getAttribute: () => null } });
  });
  afterEach(() => vi.unstubAllGlobals());

  it('shows one-minute load, memory and storage with bars, and uptime in one card', () => {
    const body = dashboard({ state: 'ready' }, fixture);
    expect(body).toContain('CPU load (1 min)');
    expect(body).toContain('0.39');
    expect(body).not.toContain('0.24');
    expect(body).toContain('266.4 MB of 408.8 MB');
    expect(body).toContain('3m');
    expect(body).toContain('11.5 MB of 77.8 MB');
    expect(body.match(/data-slot="progress"/g)).toHaveLength(2);
    expect(body).not.toContain('>Refresh<');
    // The version lives in the header menu now.
    expect(body).not.toContain('>dev<');
  });

  it('shows unavailable memory when total memory is zero', () => {
    const body = dashboard(
      { state: 'ready' },
      { ...fixture, memory: { total_bytes: 0, available_bytes: 0 } },
    );
    expect(body).toContain('Unavailable');
    expect(body).not.toContain('0 B of 0 B');
  });

  it('clamps negative used memory to zero', () => {
    const body = dashboard(
      { state: 'ready' },
      { ...fixture, memory: { total_bytes: 1024, available_bytes: 2048 } },
    );
    expect(body).toContain('0 B of 1.0 KB');
  });

  it('shows loading before the first response', () => {
    expect(dashboard({ state: 'loading' })).toContain('Loading');
  });

  it('shows initial failure and retry', () => {
    const body = dashboard({ state: 'error', message: 'Device request failed' });
    expect(body).toContain('Device request failed');
    expect(body).toContain('Retry');
    expect(body).not.toContain('Last known value');
  });

  it('preserves the snapshot and marks it stale after refresh fails', () => {
    const body = dashboard({ state: 'error', message: 'Device request failed' }, fixture);
    expect(body).toContain('0.39');
    expect(body).toContain('266.4 MB of 408.8 MB');
    expect(body).toContain('Last known value');
    expect(body).toContain('Retry');
    // Each resource tile keeps its own dimmed value instead of blanking.
    expect(body).toContain('opacity-60');
    expect(body).toContain('3m');
  });
});
