import { render } from 'svelte/server';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { LoadStatus } from '../ui/loader.svelte';
import Dashboard from './Dashboard.svelte';
import Internet from './Internet.svelte';
import NetworkDiagnostics from './NetworkDiagnostics.svelte';
import NetworkLAN from './NetworkLAN.svelte';

const { createLoader, dashboardEvents } = vi.hoisted(() => {
  const loading = () => ({
    status: { state: 'loading' } as LoadStatus,
    data: undefined,
    retry: () => {},
  });
  return {
    createLoader: vi.fn(),
    dashboardEvents: {
      connectivity: loading(),
      resources: loading(),
      throughput: loading(),
      tailscale: loading(),
      uplinks: loading(),
      clients: loading(),
      alerts: loading(),
    },
  };
});
vi.mock('../ui/loader.svelte', () => ({ createLoader }));
vi.mock('../dashboard-events.svelte', () => ({ dashboardEvents }));

function headings(body: string) {
  return [...body.matchAll(/<h([1-6])[^>]*>([\s\S]*?)<\/h\1>/g)].map(([, level, text]) => ({
    level: Number(level),
    text: (text ?? '').replace(/<[^>]*>/g, '').trim(),
  }));
}

const pages: [string, () => string, string[]][] = [
  [
    'Dashboard',
    () => render(Dashboard, { props: { events: dashboardEvents as never } }).body,
    ['Connections', 'Status', 'Traffic'],
  ],
  [
    'Internet',
    () => render(Internet).body,
    ['WiFi uplink', 'Other uplinks &amp; failover', 'Data usage &amp; budget'],
  ],
  ['LAN', () => render(NetworkLAN).body, ['Addressing &amp; DHCP', 'Local DNS']],
  ['Diagnostics', () => render(NetworkDiagnostics).body, ['Tools', 'Network state']],
];

describe('page category headings', () => {
  beforeEach(() => {
    vi.stubGlobal('document', { documentElement: { getAttribute: () => null } });
    createLoader.mockReturnValue({
      status: { state: 'loading' },
      data: undefined,
      retry: vi.fn(),
      stop: vi.fn(),
    });
  });
  afterEach(() => vi.unstubAllGlobals());

  it.each(pages)('%s names its category groups as h2, in order', (_, page, groups) => {
    const found = headings(page())
      .filter((h) => h.level === 2 && groups.includes(h.text))
      .map((h) => h.text);
    expect(found).toEqual(groups);
  });

  it.each(pages)('%s never skips or inverts a heading level', (_, page) => {
    const levels = headings(page()).map((h) => h.level);
    expect(levels[0]).toBe(1);
    for (let i = 1; i < levels.length; i++)
      expect(levels[i]).toBeLessThanOrEqual((levels[i - 1] ?? 0) + 1);
  });
});
