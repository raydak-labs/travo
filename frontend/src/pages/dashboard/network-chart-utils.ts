import type { StatsDataPoint } from '@/hooks/use-websocket';
import type { TrafficHistoryPoint } from '@shared/index';

export interface NetworkRatePoint {
  time: string;
  rx: number;
  tx: number;
}

function formatChartAxisTime(timestamp: number): string {
  const date = new Date(timestamp);
  return `${date.getMinutes().toString().padStart(2, '0')}:${date.getSeconds().toString().padStart(2, '0')}`;
}

/** Per-second RX/TX rates from cumulative byte counters (for Recharts). */
export function computeNetworkRates(dataPoints: StatsDataPoint[]): NetworkRatePoint[] {
  const rates: NetworkRatePoint[] = [];
  for (let i = 1; i < dataPoints.length; i++) {
    const prev = dataPoints[i - 1];
    const curr = dataPoints[i];
    const dtSec = (curr.timestamp - prev.timestamp) / 1000;
    if (dtSec <= 0) continue;

    const rxDiff = curr.rxBytes - prev.rxBytes;
    const txDiff = curr.txBytes - prev.txBytes;

    rates.push({
      time: formatChartAxisTime(curr.timestamp),
      rx: rxDiff > 0 ? rxDiff / dtSec : 0,
      tx: txDiff > 0 ? txDiff / dtSec : 0,
    });
  }
  return rates;
}

/**
 * Joins the server's history with the live WebSocket tail into one series.
 *
 * The two sources overlap: history is a snapshot taken at mount and the live
 * buffer has been counting since, so any history point at or after the newest
 * live point is dropped rather than plotted twice.
 */
export function mergeTrafficSeries(
  history: readonly TrafficHistoryPoint[],
  live: readonly StatsDataPoint[],
  ifname: string,
): StatsDataPoint[] {
  const newestLive = live.length > 0 ? live[live.length - 1].timestamp : null;
  const historyPoints: StatsDataPoint[] = [];
  for (const point of history) {
    if (point.ifname !== ifname) continue;
    const timestamp = point.t * 1000;
    if (newestLive !== null && timestamp >= newestLive) continue;
    historyPoints.push({
      timestamp,
      cpu: 0,
      memoryUsed: 0,
      memoryTotal: 0,
      rxBytes: point.rx_bytes,
      txBytes: point.tx_bytes,
    });
  }
  return [...historyPoints, ...live];
}
