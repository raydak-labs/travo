import { useState } from 'react';
import { RefreshCw, ChevronDown, ChevronRight } from 'lucide-react';
import type { WifiScanResult, GroupedScanNetwork } from '@shared/index';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { SignalStrengthIcon } from '@/components/wifi/signal-strength-icon';
import { SecurityBadge } from '@/components/wifi/security-badge';
import { formatWifiBandLabel, normalizeWifiBandKey } from '@/lib/wifi-band';
import { groupScanNetworks, wifiScanApTooltip } from './wifi-scan-list-utils';
import { WifiScanListPerBandSignals } from './wifi-scan-list-per-band-signals';

interface WifiScanListProps {
  networks: WifiScanResult[];
  isLoading: boolean;
  onRefresh: () => void;
  onConnect: (group: GroupedScanNetwork) => void;
  /** When set, show "Connected (band)" for the matching SSID */
  connectedSSID?: string | null;
  connectedBand?: string | null;
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
  const groups = groupScanNetworks(networks);

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
            const bandCount = new Set(group.aps.map((ap) => normalizeWifiBandKey(ap.band))).size;
            const isDualBand = bandCount > 1;
            const listKey = `${group.ssid}\t${group.encryption}`;
            const isExpanded = expandedKey === listKey;
            const isConnected = connectedSSID != null && group.ssid === connectedSSID;

            return (
              <li
                key={listKey}
                className="flex flex-col gap-2 py-3"
                title={group.aps.map((ap) => wifiScanApTooltip(ap)).join('\n---\n')}
              >
                <div className="flex items-center justify-between gap-3">
                  <div className="flex items-center gap-3">
                    <SignalStrengthIcon signalPercent={bestSignal} />
                    <div>
                      <p className="text-sm font-medium text-gray-900 dark:text-white">
                        {group.ssid || '(Hidden)'}
                        {isConnected && connectedBand && (
                          <span className="ml-1.5 text-xs font-normal text-green-600 dark:text-green-400">
                            (Connected {formatWifiBandLabel(connectedBand)})
                          </span>
                        )}
                      </p>
                      <div className="mt-0.5 flex flex-wrap items-center gap-1.5">
                        <SecurityBadge encryption={group.encryption} />
                        {isDualBand ? (
                          <Badge variant="outline">Dual-band</Badge>
                        ) : (
                          <Badge variant="outline">{formatWifiBandLabel(group.aps[0].band)}</Badge>
                        )}
                      </div>
                      <WifiScanListPerBandSignals aps={group.aps} />
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
                          <li key={`${ap.bssid}-${ap.channel}`} title={wifiScanApTooltip(ap)}>
                            {formatWifiBandLabel(ap.band)} · Ch {ap.channel} · {ap.signal_dbm} dBm ·{' '}
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
