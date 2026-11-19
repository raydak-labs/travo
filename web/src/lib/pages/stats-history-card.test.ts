import { render } from 'svelte/server';
import { describe, expect, it, vi } from 'vitest';
import type { StatsHistoryBody } from '../api/gen';
import type { LoadStatus } from '../ui/loader.svelte';
import StatsHistoryCard from './StatsHistoryCard.svelte';

const { loader } = vi.hoisted(() => ({
  loader: {
    status: { state: 'ready' } as LoadStatus,
    data: undefined as StatsHistoryBody | undefined,
    retry: vi.fn(),
  },
}));
vi.mock('../ui/loader.svelte', () => ({ createLoader: () => loader }));

const body = (extra: Partial<StatsHistoryBody>): StatsHistoryBody => ({
  interval_seconds: 600,
  retention_hours: 168,
  persistence: 'ok',
  points: [],
  ...extra,
});

describe('stats history card', () => {
  it('draws traffic per uplink, clients and uptime with min/max/last labels', () => {
    loader.data = body({
      points: [
        {
          at: 1_000_000,
          uptime_seconds: 7200,
          clients: 2,
          uplinks: [{ name: 'wwan', rx_bytes: 2048, tx_bytes: 1024 }],
          up: ['wwan'],
        },
        {
          at: 1_000_600,
          uptime_seconds: 60,
          clients: 5,
          uplinks: [{ name: 'wwan', rx_bytes: 1024, tx_bytes: 0 }],
          up: ['wwan'],
        },
      ],
    });
    const html = render(StatsHistoryCard).body;
    expect(html).toContain('aria-pressed="true"');
    expect(html).toContain('Traffic wwan');
    expect(html).toContain('4.0 KB in range');
    expect(html).toContain(
      'aria-label="Traffic wwan per 10 min: min 1.0 KB, max 3.0 KB, last 1.0 KB"',
    );
    expect(html).toContain('aria-label="Clients: min 2, max 5, last 5"');
    expect(html).toContain('aria-label="Uptime: min 1m, max 2h, last 1m"');
    expect(html).toContain('Restarts seen: 1');
    expect(html).not.toContain('Reset history');
  });

  it('empty and degraded: says so and offers the reset', () => {
    loader.data = body({ persistence: 'degraded' });
    const html = render(StatsHistoryCard).body;
    expect(html).toContain('No history yet. The first point appears within 10 minutes.');
    expect(html).toContain('Not saved');
    expect(html).toContain('Reset history');
    expect(html).not.toContain('<svg');
  });
});
