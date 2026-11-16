import { render } from 'svelte/server';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Status } from '../api/gen';
import { fetchHealth } from '../api/health';
import type { LoadStatus } from '../ui/loader.svelte';
import Dashboard from './Dashboard.svelte';

const { createLoader, dashboardEvents } = vi.hoisted(() => ({
  createLoader: vi.fn(),
  dashboardEvents: {
    connectivity: { status: { state: 'ready' }, data: undefined as unknown, retry: vi.fn() },
    resources: { status: { state: 'ready' }, data: undefined, retry: vi.fn() },
    throughput: { status: { state: 'ready' }, data: undefined, retry: vi.fn() },
  },
}));
vi.mock('../ui/loader.svelte', () => ({ createLoader }));
vi.mock('../dashboard-events.svelte', () => ({ dashboardEvents }));

function dashboard(status: LoadStatus, data: Status | undefined) {
  createLoader.mockReturnValue({ status: { state: 'ready' }, data: undefined, retry: vi.fn() });
  dashboardEvents.connectivity.status = status;
  dashboardEvents.connectivity.data = data;
  return render(Dashboard, { props: { events: dashboardEvents as never } }).body;
}

describe('Dashboard connectivity', () => {
  beforeEach(() => {
    vi.stubGlobal('document', { documentElement: { getAttribute: () => null } });
  });
  afterEach(() => vi.unstubAllGlobals());

  it('shows active uplink and link up even when the internet is offline', () => {
    const body = dashboard(
      { state: 'ready' },
      { active_interface: 'phy0-sta0', carrier: true, internet: 'offline' },
    );
    expect(body).toContain('phy0-sta0');
    expect(body).toContain('Link up');
    expect(body).toContain('Offline');
    expect(createLoader).toHaveBeenCalledWith(fetchHealth);
  });

  it('distinguishes unknown and down carrier', () => {
    const data = { active_interface: null, carrier: null, internet: 'online' } as const;
    const body = dashboard({ state: 'ready' }, data);
    expect(body).toContain('Unknown');
    expect(body).toContain('Online');
    expect(body).not.toContain('Link down');
    expect(dashboard({ state: 'ready' }, { ...data, carrier: false })).toContain('Link down');
  });

  it('retains the last connectivity snapshot on failed refresh', () => {
    const body = dashboard(
      { state: 'error', message: 'Connectivity unavailable' },
      { active_interface: 'phy0-sta0', carrier: true, internet: 'captive' },
    );
    expect(body).toContain('Last known value');
    expect(body).toContain('phy0-sta0');
    expect(body).toContain('Captive portal');
    expect(body).toContain('Connectivity unavailable');
  });

  it('keeps the WiFi button available when connectivity is unavailable', () => {
    const body = dashboard({ state: 'error', message: 'Connectivity unavailable' }, undefined);

    expect(body).toContain('Connect to WiFi');
    expect(body.replace(/<!--.*?-->/g, '')).toMatch(
      /<a[^>]*href="#\/wifi"[^>]*>Connect to WiFi<\/a>/,
    );
  });
});
