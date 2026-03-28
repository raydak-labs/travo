import type { InterfaceDataPoint } from '@/hooks/use-websocket';

export const INTERFACE_LABELS: Record<string, string> = {
  'br-lan': 'LAN',
  eth0: 'WAN (Ethernet)',
  wwan0: 'WWAN (WiFi)',
  wg0: 'WireGuard VPN',
};

/** Stable ordering for dashboard grid */
export const INTERFACE_ORDER = ['eth0', 'br-lan', 'wwan0', 'wg0'];

export function interfaceTrafficLabel(name: string): string {
  return INTERFACE_LABELS[name] ?? name;
}

export function formatTrafficChartTime(timestamp: number): string {
  const date = new Date(timestamp);
  return `${date.getMinutes().toString().padStart(2, '0')}:${date.getSeconds().toString().padStart(2, '0')}`;
}

export type TrafficRatePoint = {
  time: string;
  rx: number;
  tx: number;
};

export function computeTrafficRates(points: InterfaceDataPoint[]): TrafficRatePoint[] {
  const rates: TrafficRatePoint[] = [];
  for (let i = 1; i < points.length; i++) {
    const prev = points[i - 1];
    const curr = points[i];
    const dtSec = (curr.timestamp - prev.timestamp) / 1000;
    if (dtSec <= 0) continue;

    const rxDiff = curr.rxBytes - prev.rxBytes;
    const txDiff = curr.txBytes - prev.txBytes;

    rates.push({
      time: formatTrafficChartTime(curr.timestamp),
      rx: rxDiff > 0 ? rxDiff / dtSec : 0,
      tx: txDiff > 0 ? txDiff / dtSec : 0,
    });
  }
  return rates;
}

export function sortInterfaceNames(names: string[]): string[] {
  return [...names].sort((a, b) => {
    const ai = INTERFACE_ORDER.indexOf(a);
    const bi = INTERFACE_ORDER.indexOf(b);
    return (ai === -1 ? 999 : ai) - (bi === -1 ? 999 : bi);
  });
}
