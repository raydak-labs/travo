import type { TrafficHistoryPoint } from '@shared/index';
import type { InterfaceDataPoint } from '@/hooks/use-websocket';

/**
 * Joins the server's retained traffic history with the live WebSocket tail for
 * one interface.
 *
 * Both charts that draw interface traffic need this, and the overlap rule is the
 * part that is easy to get subtly wrong: history is a snapshot fetched at mount
 * while the live buffer has been counting since, so a history point at or after
 * the newest live point would be plotted twice and show a fake spike.
 *
 * One shared copy of the merge exists because "why does this chart show a zero-rate
 * dip" is exactly the question two copies of this rule answer differently.
 */
export function mergeInterfaceSeries(
  history: readonly TrafficHistoryPoint[],
  live: readonly InterfaceDataPoint[],
  ifname: string,
): InterfaceDataPoint[] {
  const newestLive = live.length > 0 ? live[live.length - 1]!.timestamp : null;
  const fromHistory: InterfaceDataPoint[] = [];
  for (const point of history) {
    if (point.ifname !== ifname) continue;
    const timestamp = point.t * 1000;
    // `>=`, not `>`: the live buffer's newest point is very often the same
    // sample the server already retained, and keeping both plots it twice.
    if (newestLive !== null && timestamp >= newestLive) continue;
    fromHistory.push({
      timestamp,
      rxBytes: point.rx_bytes,
      txBytes: point.tx_bytes,
    });
  }
  return [...fromHistory, ...live];
}

/** Interface names present in either source, so a chart is not empty on load. */
export function interfaceNames(
  history: readonly TrafficHistoryPoint[] | undefined,
  live: Readonly<Record<string, unknown>>,
): string[] {
  const names = new Set(Object.keys(live));
  for (const point of history ?? []) names.add(point.ifname);
  return [...names];
}

export interface TransferRate {
  /** Bytes received per second over the newest sampling interval. */
  readonly rx: number;
  /** Bytes sent per second over the newest sampling interval. */
  readonly tx: number;
}

/**
 * Per-second RX/TX rate for the newest sample, or `null` when the series has
 * fewer than two points and no interval can be derived.
 *
 * A counter that went backwards means the interface or the router restarted;
 * the rate is then unknown, not negative. The clamp to 0 matches the charts: a
 * real idle interval also measures 0, and there is no third state to show.
 */
export function latestRate(series: readonly InterfaceDataPoint[]): TransferRate | null {
  const last = series[series.length - 1];
  const previous = series[series.length - 2];
  if (!last || !previous) return null;

  const dtSec = (last.timestamp - previous.timestamp) / 1000;
  if (dtSec <= 0) return null;

  const rxDiff = last.rxBytes - previous.rxBytes;
  const txDiff = last.txBytes - previous.txBytes;
  return {
    rx: rxDiff > 0 ? rxDiff / dtSec : 0,
    tx: txDiff > 0 ? txDiff / dtSec : 0,
  };
}
