import { describe, expect, it } from 'vitest';
import type { StatsHistoryPoint } from './api/gen';
import { historyView, sparklinePath, summarize } from './stats-history';

const point = (at: number, uptime: number, extra: Partial<StatsHistoryPoint> = {}) => ({
  at,
  uptime_seconds: uptime,
  clients: 3,
  uplinks: [],
  up: null,
  ...extra,
});

describe('historyView', () => {
  it('sums rx+tx per uplink, keeps missing buckets as null and counts uptime resets', () => {
    const view = historyView([
      point(600, 6000, { uplinks: [{ name: 'wwan', rx_bytes: 100, tx_bytes: 20 }] }),
      point(1200, 6600, {
        clients: null,
        uplinks: [
          { name: 'wwan', rx_bytes: 50, tx_bytes: 5 },
          { name: 'usbtether', rx_bytes: 7, tx_bytes: 1 },
        ],
      }),
      point(1800, 300),
      point(2400, 900, { uplinks: [{ name: 'wwan', rx_bytes: 1, tx_bytes: 0 }] }),
    ]);
    expect(view.traffic.map((t) => [t.name, t.total])).toEqual([
      ['wwan', 176],
      ['usbtether', 8],
    ]);
    expect(view.traffic[0]?.samples.map((s) => s.value)).toEqual([120, 55, null, 1]);
    expect(view.traffic[1]?.samples.map((s) => s.value)).toEqual([null, 8, null, null]);
    expect(view.clients.map((s) => s.value)).toEqual([3, null, 3, 3]);
    expect(view.uptime.map((s) => s.value)).toEqual([6000, 6600, 300, 900]);
    expect(view.reboots).toBe(1);
  });

  it('is empty for no points', () => {
    expect(historyView([])).toEqual({ traffic: [], clients: [], uptime: [], reboots: 0 });
  });
});

describe('summarize', () => {
  it('ignores nulls and reports the last recorded value', () => {
    expect(
      summarize([
        { at: 1, value: 4 },
        { at: 2, value: 9 },
        { at: 3, value: 2 },
        { at: 4, value: null },
      ]),
    ).toEqual({ min: 2, max: 9, last: 2 });
    expect(summarize([{ at: 1, value: null }])).toBeNull();
  });
});

describe('sparklinePath', () => {
  const frame = { width: 100, height: 10, start: 0, end: 1000, intervalSeconds: 100 };

  it('maps time to x and value to y with zero at the bottom', () => {
    expect(
      sparklinePath(
        [
          { at: 0, value: 0 },
          { at: 100, value: 5 },
          { at: 200, value: 10 },
        ],
        frame,
      ),
    ).toBe('M0 10L0 10L10 5L20 0');
  });

  it('breaks the line at nulls and at missing buckets', () => {
    expect(
      sparklinePath(
        [
          { at: 0, value: 10 },
          { at: 100, value: null },
          { at: 200, value: 10 },
          { at: 500, value: 10 },
          { at: 600, value: 10 },
        ],
        frame,
      ),
    ).toBe('M0 0L0 0M20 0L20 0M50 0L50 0L60 0');
  });

  it('draws nothing without values and keeps an all-zero series on the baseline', () => {
    expect(sparklinePath([{ at: 0, value: null }], frame)).toBe('');
    expect(
      sparklinePath(
        [
          { at: 0, value: 0 },
          { at: 100, value: 0 },
        ],
        frame,
      ),
    ).toBe('M0 10L0 10L10 10');
  });
});
