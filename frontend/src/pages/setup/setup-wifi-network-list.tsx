import { RefreshCw, Signal, Lock } from 'lucide-react';
import type { WifiScanResult } from '@shared/index';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { setupWifiSignalTier } from './setup-wifi-step-utils';

type SetupWifiNetworkListProps = {
  networks: WifiScanResult[] | undefined;
  scanning: boolean;
  selectedSsid: string;
  onRescan: () => void;
  onPickNetwork: (network: WifiScanResult) => void;
};

export function SetupWifiNetworkList({
  networks,
  scanning,
  selectedSsid,
  onRescan,
  onPickNetwork,
}: SetupWifiNetworkListProps) {
  return (
    <>
      <div className="flex items-center justify-between">
        <span className="text-sm font-medium text-gray-700 dark:text-gray-300">
          Available Networks
        </span>
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() => onRescan()}
          disabled={scanning}
        >
          <RefreshCw className={`mr-1 h-3 w-3 ${scanning ? 'animate-spin' : ''}`} />
          Scan
        </Button>
      </div>

      <div className="max-h-64 space-y-2 overflow-y-auto rounded-lg border p-2 dark:border-gray-700">
        {scanning ? (
          Array.from({ length: 4 }, (_, i) => <Skeleton key={i} className="h-12 w-full" />)
        ) : networks && networks.length > 0 ? (
          networks
            .filter((n) => n.ssid)
            .map((network) => {
              const tier = setupWifiSignalTier(network.signal_dbm);
              return (
                <button
                  key={`${network.ssid}-${network.bssid}`}
                  type="button"
                  className={`flex w-full items-center justify-between rounded-lg p-3 text-left transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 ${
                    selectedSsid === network.ssid
                      ? 'bg-blue-50 ring-2 ring-blue-500 dark:bg-blue-950'
                      : 'hover:bg-gray-50 dark:hover:bg-gray-800'
                  }`}
                  onClick={() => onPickNetwork(network)}
                >
                  <div className="flex items-center gap-3">
                    <Signal className="h-4 w-4 text-gray-500" />
                    <div>
                      <span className="text-sm font-medium text-gray-900 dark:text-white">
                        {network.ssid}
                      </span>
                      <div className="flex items-center gap-2">
                        <span className="text-xs text-gray-400">{network.signal_dbm} dBm</span>
                        <span className="text-xs text-gray-400">{network.band}</span>
                      </div>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    {network.encryption !== 'none' && <Lock className="h-3 w-3 text-gray-400" />}
                    <Badge variant={tier >= 3 ? 'default' : 'secondary'}>{tier}/4</Badge>
                  </div>
                </button>
              );
            })
        ) : (
          <p className="p-4 text-center text-sm text-gray-400">No networks found. Try scanning.</p>
        )}
      </div>
    </>
  );
}
