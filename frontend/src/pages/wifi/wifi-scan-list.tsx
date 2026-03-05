import { RefreshCw } from 'lucide-react';
import type { WifiScanResult } from '@shared/index';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { SignalStrengthIcon } from '@/components/wifi/signal-strength-icon';
import { SecurityBadge } from '@/components/wifi/security-badge';

interface WifiScanListProps {
  networks: WifiScanResult[];
  isLoading: boolean;
  onRefresh: () => void;
  onConnect: (network: WifiScanResult) => void;
}

function bandLabel(band: string): string {
  switch (band) {
    case '2.4ghz':
      return '2.4 GHz';
    case '5ghz':
      return '5 GHz';
    case '6ghz':
      return '6 GHz';
    default:
      return band;
  }
}

export function WifiScanList({ networks, isLoading, onRefresh, onConnect }: WifiScanListProps) {
  const sorted = [...networks].sort((a, b) => b.signal_percent - a.signal_percent);

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
      ) : sorted.length === 0 ? (
        <p className="py-4 text-center text-sm text-gray-500">No networks found</p>
      ) : (
        <ul className="divide-y divide-gray-200 dark:divide-gray-800" role="list">
          {sorted.map((network) => (
            <li key={network.bssid} className="flex items-center justify-between gap-3 py-3">
              <div className="flex items-center gap-3">
                <SignalStrengthIcon signalPercent={network.signal_percent} />
                <div>
                  <p className="text-sm font-medium text-gray-900 dark:text-white">
                    {network.ssid || '(Hidden)'}
                  </p>
                  <div className="mt-0.5 flex items-center gap-1.5">
                    <SecurityBadge encryption={network.encryption} />
                    <Badge variant="outline">{bandLabel(network.band)}</Badge>
                  </div>
                </div>
              </div>
              <Button size="sm" variant="outline" onClick={() => onConnect(network)}>
                Connect
              </Button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
