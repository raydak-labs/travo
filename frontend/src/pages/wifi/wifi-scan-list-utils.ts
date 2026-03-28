import type { WifiScanResult, GroupedScanNetwork } from '@shared/index';
import { formatWifiBandLabel } from '@/lib/wifi-band';

export function wifiScanApTooltip(ap: WifiScanResult): string {
  return [
    `Signal: ${ap.signal_percent}% (${ap.signal_dbm} dBm)`,
    `Channel: ${ap.channel}`,
    `Band: ${formatWifiBandLabel(ap.band)}`,
    `Encryption: ${ap.encryption === 'none' ? 'Open' : ap.encryption.toUpperCase()}`,
    `BSSID: ${ap.bssid}`,
  ].join('\n');
}

export function groupScanNetworks(networks: WifiScanResult[]): GroupedScanNetwork[] {
  const map = new Map<string, WifiScanResult[]>();
  for (const n of networks) {
    const key = `${n.ssid ?? '(hidden)'}\t${n.encryption}`;
    const list = map.get(key) ?? [];
    list.push(n);
    map.set(key, list);
  }
  const groups: GroupedScanNetwork[] = [];
  for (const aps of map.values()) {
    const bySignal = [...aps].sort((a, b) => b.signal_dbm - a.signal_dbm);
    groups.push({
      ssid: bySignal[0].ssid ?? '(Hidden)',
      encryption: bySignal[0].encryption,
      aps: bySignal,
    });
  }
  return groups.sort((a, b) => {
    const bestA = Math.max(...a.aps.map((ap) => ap.signal_dbm));
    const bestB = Math.max(...b.aps.map((ap) => ap.signal_dbm));
    return bestB - bestA;
  });
}
