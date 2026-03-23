import { useState } from 'react';
import { RefreshCw, ChevronDown, ChevronRight } from 'lucide-react';
import type { WifiScanResult, GroupedScanNetwork } from '@shared/index';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { SignalStrengthIcon } from '@/components/wifi/signal-strength-icon';
import { SecurityBadge } from '@/components/wifi/security-badge';

interface WifiScanListProps {
  networks: WifiScanResult[];
  isLoading: boolean;
  onRefresh: () => void;
  onConnect: (group: GroupedScanNetwork) => void;
  /** When set, show "Connected (band)" for the matching SSID */
  connectedSSID?: string | null;
  connectedBand?: string | null;
}

/** Normalize band string from API (e.g. "5GHz", "2.4GHz") or WifiBand to display label */
function bandLabel(band: string): string {
  const b = band.toLowerCase();
  if (b === '2.4ghz' || b === '2.4g') return '2.4 GHz';
  if (b === '5ghz' || b === '5g') return '5 GHz';
  if (b === '6ghz' || b === '6g') return '6 GHz';
  return band;
}

/** Normalize band to a key for grouping (2.4 vs 5 vs 6) */
function bandKey(band: string): string {
  const b = band.toLowerCase();
  if (b === '2.4ghz' || b === '2.4g') return '2.4ghz';
  if (b === '5ghz' || b === '5g') return '5ghz';
  if (b === '6ghz' || b === '6g') return '6ghz';
  return band;
}

function scanTooltip(ap: WifiScanResult): string {
  return [
    `Signal: ${ap.signal_percent}% (${ap.signal_dbm} dBm)`,
    `Channel: ${ap.channel}`,
    `Band: ${bandLabel(ap.band)}`,
    `Encryption: ${ap.encryption === 'none' ? 'Open' : ap.encryption.toUpperCase()}`,
    `BSSID: ${ap.bssid}`,
  ].join('\n');
}

function groupNetworks(networks: WifiScanResult[]): GroupedScanNetwork[] {
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

function PerBandSignals({ aps }: { aps: readonly WifiScanResult[] }) {
  const byBand = new Map<string, WifiScanResult>();
  for (const ap of aps) {
    const k = bandKey(ap.band);
    const existing = byBand.get(k);
    if (!existing || ap.signal_dbm > existing.signal_dbm) byBand.set(k, ap);
  }
  const bands = Array.from(byBand.entries()).sort(([a], [b]) => a.localeCompare(b));
  return (
    <div className="mt-0.5 flex flex-wrap items-center gap-x-3 gap-y-0.5 text-xs text-gray-600 dark:text-gray-400">
      {bands.map(([k, ap]) => (
        <span key={k}>
          {bandLabel(ap.band)}{' '}
          <SignalStrengthIcon
            signalPercent={ap.signal_percent}
            className="inline-block h-3.5 w-3.5"
          />{' '}
          {ap.signal_dbm} dBm
        </span>
      ))}
    </div>
  );
}

export function WifiScanList({
  networks,
  isLoading,
  onRefresh,
  onConnect,
  connectedSSID,
  connectedBand,
}: WifiScanListProps) {
  const [expandedKey, setExpandedKey] = useState<string | null>(null);
  const groups = groupNetworks(networks);

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Available Networks</h3>
        <Button variant="ghost" size="sm" onClick={onRefresh} disabled={isLoading}>
          <RefreshCw className={`mr-1.5 h-3.5 w-3.5 ${isLoading ? 'animate-spin' : ''}`} />
          Scan
        </Button>
      </div>

      {isLoading ? (
        <div className="space-y-2" data-testid="scan-loading">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-14 w-full" />
          ))}
        </div>
      ) : groups.length === 0 ? (
        <p className="py-4 text-center text-sm text-gray-500">No networks found</p>
      ) : (
        <ul className="divide-y divide-gray-200 dark:divide-gray-800" role="list">
          {groups.map((group) => {
            const bestSignal = Math.max(...group.aps.map((ap) => ap.signal_percent));
            const bandCount = new Set(group.aps.map((ap) => bandKey(ap.band))).size;
            const isDualBand = bandCount > 1;
            const listKey = `${group.ssid}\t${group.encryption}`;
            const isExpanded = expandedKey === listKey;
            const isConnected = connectedSSID != null && group.ssid === connectedSSID;

            return (
              <li
                key={listKey}
                className="flex flex-col gap-2 py-3"
                title={group.aps.map((ap) => scanTooltip(ap)).join('\n---\n')}
              >
                <div className="flex items-center justify-between gap-3">
                  <div className="flex items-center gap-3">
                    <SignalStrengthIcon signalPercent={bestSignal} />
                    <div>
                      <p className="text-sm font-medium text-gray-900 dark:text-white">
                        {group.ssid || '(Hidden)'}
                        {isConnected && connectedBand && (
                          <span className="ml-1.5 text-xs font-normal text-green-600 dark:text-green-400">
                            (Connected {bandLabel(connectedBand)})
                          </span>
                        )}
                      </p>
                      <div className="mt-0.5 flex items-center gap-1.5 flex-wrap">
                        <SecurityBadge encryption={group.encryption} />
                        {isDualBand ? (
                          <Badge variant="outline">Dual-band</Badge>
                        ) : (
                          <Badge variant="outline">{bandLabel(group.aps[0].band)}</Badge>
                        )}
                      </div>
                      <PerBandSignals aps={group.aps} />
                    </div>
                  </div>
                  <Button size="sm" variant="outline" onClick={() => onConnect(group)}>
                    Connect
                  </Button>
                </div>
                {group.aps.length > 1 && (
                  <>
                    <button
                      type="button"
                      className="flex items-center gap-1 text-xs text-gray-500 hover:text-gray-700 dark:hover:text-gray-300"
                      onClick={() => setExpandedKey(isExpanded ? null : listKey)}
                      aria-expanded={isExpanded}
                    >
                      {isExpanded ? (
                        <ChevronDown className="h-3.5 w-3.5" />
                      ) : (
                        <ChevronRight className="h-3.5 w-3.5" />
                      )}
                      {isExpanded ? 'Hide' : 'Show'} individual APs ({group.aps.length})
                    </button>
                    {isExpanded && (
                      <ul className="ml-6 list-disc space-y-0.5 text-xs text-gray-600 dark:text-gray-400">
                        {group.aps.map((ap) => (
                          <li key={`${ap.bssid}-${ap.channel}`} title={scanTooltip(ap)}>
                            {bandLabel(ap.band)} · Ch {ap.channel} · {ap.signal_dbm} dBm ·{' '}
                            {ap.bssid}
                          </li>
                        ))}
                      </ul>
                    )}
                  </>
                )}
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
