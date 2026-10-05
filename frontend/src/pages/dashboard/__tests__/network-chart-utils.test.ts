import { describe, expect, it } from 'vitest';
import type { StatsDataPoint } from '@/hooks/use-websocket';
import { computeNetworkRates } from '@/pages/dashboard/network-chart-utils';

function point(
  timestamp: number,
  rxBytes: number,
  txBytes: number,
  overrides: Partial<StatsDataPoint> = {},
): StatsDataPoint {
  return {
    timestamp,
    cpu: 0,
    memoryUsed: 0,
    memoryTotal: 1,
    rxBytes,
    txBytes,
    ...overrides,
  };
}

describe('computeNetworkRates', () => {
  it('returns empty for fewer than 2 points', () => {
    expect(computeNetworkRates([])).toEqual([]);
    expect(computeNetworkRates([point(1000, 0, 0)])).toEqual([]);
  });

  it('computes positive per-second rates', () => {
    const pts = [point(1000, 0, 0), point(2000, 500, 100)];
    const r = computeNetworkRates(pts);
    expect(r).toHaveLength(1);
    expect(r[0]!.rx).toBe(500);
    expect(r[0]!.tx).toBe(100);
  });

  it('uses zero when counters go backwards', () => {
    const pts = [point(1000, 1000, 0), point(2000, 500, 0)];
    const r = computeNetworkRates(pts);
    expect(r[0]!.rx).toBe(0);
  });

  it('skips segments with non-positive dt', () => {
    const pts = [point(1000, 0, 0), point(1000, 100, 50), point(3000, 200, 80)];
    const r = computeNetworkRates(pts);
    expect(r).toHaveLength(1);
    expect(r[0]!.rx).toBe(50);
    expect(r[0]!.tx).toBe(15);
  });
});
