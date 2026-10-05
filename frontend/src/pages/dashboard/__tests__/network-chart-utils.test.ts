import { describe, expect, it } from 'vitest';
import type { StatsDataPoint } from '@/hooks/use-websocket';
import { computeNetworkRates, mergeTrafficSeries } from '@/pages/dashboard/network-chart-utils';
import type { TrafficHistoryPoint } from '@shared/index';

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

describe('mergeTrafficSeries', () => {
  const hist = (t: number, ifname = 'eth0', rx = 0): TrafficHistoryPoint => ({
    t,
    ifname,
    rx_bytes: rx,
    tx_bytes: 0,
  });

  it('keeps server history ahead of the live tail and drops the overlap', () => {
    const merged = mergeTrafficSeries(
      [hist(10, 'eth0', 100), hist(12, 'eth0', 300), hist(14, 'eth0', 900)],
      [point(20_000, 1000, 0), point(22_000, 1500, 0)],
      'eth0',
    );

    // History stops at the newest live point (22s), so all 3 stay ahead of the
    // live tail and the merged series is continuous.
    expect(merged.map((p) => p.timestamp)).toEqual([10_000, 12_000, 14_000, 20_000, 22_000]);
  });

  it('drops history at or after the newest live point so points are not plotted twice', () => {
    const merged = mergeTrafficSeries(
      [hist(10, 'eth0', 100), hist(20, 'eth0', 500), hist(30, 'eth0', 900)],
      [point(20_000, 500, 0)],
      'eth0',
    );

    expect(merged.map((p) => p.timestamp)).toEqual([10_000, 20_000]);
  });

  it('ignores history for other interfaces', () => {
    const merged = mergeTrafficSeries([hist(10, 'br-lan', 1), hist(11, 'eth0', 2)], [], 'eth0');
    expect(merged).toHaveLength(1);
    expect(merged[0]!.rxBytes).toBe(2);
  });
});
