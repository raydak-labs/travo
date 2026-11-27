import { render } from 'svelte/server';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { clients, connectivity, tailscale, throughput, uplinks } from '../topology.test-inputs';
import type { LoadStatus } from '../ui/loader.svelte';
import { switchState } from '../ui/switch-html.test-helper';
import Dashboard from './Dashboard.svelte';

// Responses of the mock daemon (topology.test-inputs); only the load states change per test.
const { createLoader, dashboardEvents } = vi.hoisted(() => {
  const source = () => ({
    status: { state: 'ready' } as LoadStatus,
    data: undefined as unknown,
    retry: () => {},
  });
  return {
    createLoader: vi.fn(),
    dashboardEvents: {
      connectivity: source(),
      resources: source(),
      throughput: source(),
      tailscale: source(),
      uplinks: source(),
      clients: source(),
      alerts: source(),
    },
  };
});
vi.mock('../ui/loader.svelte', () => ({ createLoader }));
vi.mock('../dashboard-events.svelte', () => ({ dashboardEvents }));

const failed: LoadStatus = { state: 'error', message: 'Source unavailable' };

function dashboard(patch: Partial<Record<keyof typeof dashboardEvents, LoadStatus>> = {}) {
  createLoader.mockReturnValue({
    status: { state: 'ready' },
    data: undefined,
    retry: vi.fn(),
    stop: vi.fn(),
  });
  for (const key of Object.keys(dashboardEvents) as (keyof typeof dashboardEvents)[]) {
    dashboardEvents[key].status = patch[key] ?? { state: 'ready' };
  }
  return render(Dashboard, { props: { events: dashboardEvents as never } }).body.replace(
    /<!--.*?-->/g,
    '',
  );
}

// The Quick status section, cut at the next section so the hidden map does not interfere.
const quick = (body: string) => {
  const start = body.indexOf('Quick status');
  return body.slice(start, body.indexOf('aria-labelledby', start + 1));
};
const wwanCard = (body: string) => {
  const start = body.indexOf('WiFi uplink (wwan)</h2>');
  return body.slice(start, body.indexOf('</dl>', start));
};

describe('Dashboard cards', () => {
  beforeEach(() => {
    vi.stubGlobal('document', { documentElement: { getAttribute: () => null } });
    dashboardEvents.connectivity.data = connectivity;
    dashboardEvents.throughput.data = throughput;
    dashboardEvents.uplinks.data = uplinks;
    dashboardEvents.clients.data = clients;
  });
  afterEach(() => vi.unstubAllGlobals());

  it('shows the internet state in Quick status, so phones see it without the map', () => {
    const part = quick(dashboard());
    expect(part).toContain('Internet');
    expect(part).toContain('Online');
    expect(part).not.toContain('Last known value');
  });

  it('sets Quick status and System side by side in one equal-height card row', () => {
    const body = dashboard();
    const grid = body.lastIndexOf('<div class="grid', body.indexOf('Quick status'));
    const row = body.slice(grid, body.indexOf('>', grid));
    expect(row).toContain('items-stretch');
    expect(row).toContain('lg:grid-cols-2');
    expect(body.slice(grid)).toMatch(/Quick status[\s\S]*Resources/);
  });

  it('carries the Tailscale switch in Quick status once installed', () => {
    dashboardEvents.tailscale.data = {
      ...tailscale,
      installed: true,
      running: true,
      backend_state: 'Running',
    };
    const part = quick(dashboard());
    expect(part).toContain('href="#/vpn/tailscale"');
    expect(switchState(part, 'dashboard-tailscale-switch')).toEqual({
      checked: true,
      disabled: false,
    });
    dashboardEvents.tailscale.data = undefined;
  });

  it('marks partial client counts as incomplete and keeps them', () => {
    dashboardEvents.clients.data = { ...clients, partial: true };
    const part = quick(dashboard());
    expect(part).toContain('1 on WiFi · 1 wired');
    expect(part).toContain('Incomplete');
  });

  it('keeps the last client counts dimmed and marked when the clients read fails', () => {
    const part = quick(dashboard({ clients: failed }));
    expect(part).toContain('1 on WiFi · 1 wired');
    expect(part).toContain('opacity-60');
    expect(part).toContain('Last known value');
  });

  it('dims only the uplink rate when throughput fails', () => {
    const card = wwanCard(dashboard({ throughput: failed }));
    expect(card).toContain('Active');
    expect(card).toMatch(/opacity-60" data-rate/);
    expect(card.slice(0, card.indexOf('<dl'))).not.toContain('Last known value');
  });

  it('marks the uplink state stale when connectivity fails, keeping it', () => {
    const card = wwanCard(dashboard({ connectivity: failed }));
    expect(card.slice(0, card.indexOf('<dl'))).toContain('Last known value');
    expect(card).toContain('Active');
  });
});
