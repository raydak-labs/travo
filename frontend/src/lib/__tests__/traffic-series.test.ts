import { describe, expect, it } from 'vitest';
import type { InterfaceDataPoint } from '@/hooks/use-websocket';
import type { TrafficHistoryPoint } from '@shared/index';
import { interfaceNames, mergeInterfaceSeries } from '@/lib/traffic-series';

function hist(t: number, ifname = 'eth0', rx = 0): TrafficHistoryPoint {
  return { t, ifname, rx_bytes: rx, tx_bytes: 0 };
}

function live(timestamp: number, rxBytes: number): InterfaceDataPoint {
  return { timestamp, rxBytes, txBytes: 0 };
}

describe('mergeInterfaceSeries', () => {
  it('keeps server history ahead of the live tail and drops the overlap', () => {
    const merged = mergeInterfaceSeries(
      [hist(10, 'eth0', 100), hist(12, 'eth0', 300), hist(14, 'eth0', 900)],
      [live(20_000, 1000), live(22_000, 1500)],
      'eth0',
    );

    expect(merged.map((p) => p.timestamp)).toEqual([10_000, 12_000, 14_000, 20_000, 22_000]);
    expect(merged[0]!.rxBytes).toBe(100);
  });

  it('drops history at or after the newest live point so points are not plotted twice', () => {
    const merged = mergeInterfaceSeries(
      [hist(10, 'eth0', 100), hist(20, 'eth0', 500), hist(30, 'eth0', 900)],
      [live(20_000, 500)],
      'eth0',
    );

    expect(merged.map((p) => p.timestamp)).toEqual([10_000, 20_000]);
  });

  it('returns history alone when the live buffer has not started yet', () => {
    const merged = mergeInterfaceSeries([hist(10, 'eth0', 100), hist(12, 'eth0', 300)], [], 'eth0');

    expect(merged.map((p) => p.timestamp)).toEqual([10_000, 12_000]);
  });

  it('ignores history belonging to another interface', () => {
    const merged = mergeInterfaceSeries([hist(10, 'wg0', 1), hist(12, 'eth0', 2)], [], 'eth0');

    expect(merged.map((p) => p.rxBytes)).toEqual([2]);
  });
});

describe('interfaceNames', () => {
  it('unions live and history names so a chart is not empty on a fresh load', () => {
    expect(interfaceNames([hist(10, 'eth0')], { wwan0: [] }).sort()).toEqual(['eth0', 'wwan0']);
  });

  it('tolerates missing history', () => {
    expect(interfaceNames(undefined, { eth0: [] })).toEqual(['eth0']);
  });
});
