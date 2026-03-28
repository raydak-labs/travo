import type { StatsDataPoint } from '@/hooks/use-websocket';

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
