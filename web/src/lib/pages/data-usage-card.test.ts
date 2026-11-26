import { render } from 'svelte/server';
import { describe, expect, it, vi } from 'vitest';
import { dataUsageError } from '../api/data-usage';
import type { DataUsageView } from '../api/gen';
import { bytesToGib, dailySamples, gibToBytes, recentEvents, usedPercent } from '../data-usage';
import type { LoadStatus } from '../ui/loader.svelte';
import DataUsageCard from './DataUsageCard.svelte';

const { loader } = vi.hoisted(() => ({
  loader: {
    status: { state: 'ready' } as LoadStatus,
    data: undefined as DataUsageView | undefined,
    retry: vi.fn(),
  },
}));
vi.mock('../ui/loader.svelte', () => ({ createLoader: () => loader }));

const GIB = 1024 ** 3;

describe('data usage card', () => {
  const base: DataUsageView = {
    persistence: 'ok',
    budget: {
      enabled: true,
      limit_bytes: 10 * GIB,
      warn_percent: 80,
      start_day: 5,
      uplinks: ['wwan'],
    },
    budget_state: 'warning',
    budget_used_bytes: 9 * GIB,
    today: { start: '2026-11-19', end: '2026-11-20', uplinks: [] },
    billing_period: {
      start: '2026-11-05',
      end: '2026-12-05',
      uplinks: [{ name: 'wwan', rx_bytes: 8 * GIB, tx_bytes: GIB }],
    },
    days: [
      { date: '2026-11-17', uplinks: [{ name: 'wwan', rx_bytes: 5, tx_bytes: 5 }] },
      { date: '2026-11-19', uplinks: [{ name: 'wwan', rx_bytes: 1, tx_bytes: 1 }] },
    ],
    retention_hours: 168,
    uptime: {
      events: [
        { at: 1763500000, kind: 'boot' },
        { at: 1763510000, kind: 'down', uplink: 'wwan' },
      ],
      availability: [{ name: 'wwan', up_samples: 9, samples: 10 }],
    },
  };

  it('shows the budget state, the billing period and the uptime log', () => {
    loader.data = base;
    const body = render(DataUsageCard).body;
    expect(body).toContain('9.0 GB of 10.0 GB (90%)');
    expect(body).toContain('2026-11-05 to before 2026-12-05');
    expect(body).toContain('9.0 GB (down 8.0 GB, up 1.0 GB)');
    expect(body).toContain('wwan up in 90% of samples');
    expect(body).toContain('wwan down');
    expect(body).toContain('Router started');
    expect(body).toContain('Daily traffic wwan');
  });

  it('offers the reset only for an unreadable budget file', () => {
    loader.data = { ...base, budget_state: 'unreadable' };
    expect(render(DataUsageCard).body).toContain('Reset budget');
    loader.data = base;
    expect(render(DataUsageCard).body).not.toContain('Reset budget');
  });

  it('derives the series and conversions', () => {
    expect(dailySamples(base.days, 'wwan').map((s) => s.value)).toEqual([10, null, 2]);
    expect(dailySamples(base.days, 'wan').map((s) => s.value)).toEqual([0, null, 0]);
    expect(usedPercent(9, 10)).toBe(90);
    expect(usedPercent(5, 0)).toBe(0);
    expect(bytesToGib(gibToBytes(12.5))).toBe(12.5);
    expect(recentEvents(base.uptime.events)[0]?.kind).toBe('down');
    expect(dataUsageError('data_budget_unreadable')).toContain('unreadable');
  });
});
