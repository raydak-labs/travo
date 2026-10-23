import { useState } from 'react';
import { WifiOff } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';
import { SectionHeading } from '@/components/ui/section-heading';
import { Skeleton } from '@/components/ui/skeleton';
import { SignalStrengthIcon } from '@/components/wifi/signal-strength-icon';
import { SecurityBadge } from '@/components/wifi/security-badge';
import { WifiScanDialog } from './wifi-scan-dialog';
import { WifiHiddenNetworkDialog } from './wifi-hidden-network-dialog';
import { useWifiConnection, useWifiDisconnect } from '@/hooks/use-wifi';

/**
 * The current station link, without its own card. It is the first section of
 * the merged WiFi uplink card; the saved profiles follow below.
 */
export function WifiCurrentConnectionCard() {
  const { data: connection, isLoading: connectionLoading } = useWifiConnection();
  const disconnectMutation = useWifiDisconnect();
  const [confirming, setConfirming] = useState(false);

  function confirmDisconnect() {
    setConfirming(false);
    disconnectMutation.mutate();
  }

  return (
    <section className="min-w-0" aria-label="Current connection">
      <SectionHeading>Current connection</SectionHeading>
      {connectionLoading ? (
        <div className="space-y-2">
          <Skeleton className="h-4 w-3/4" />
          <Skeleton className="h-4 w-1/2" />
        </div>
      ) : connection?.connected ? (
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div className="flex min-w-0 items-center gap-3">
            <SignalStrengthIcon signalPercent={connection.signal_percent} />
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <span className="truncate text-base font-semibold text-gray-900 dark:text-white">
                  {connection.ssid}
                </span>
                <Badge variant="success">Connected</Badge>
              </div>
              <div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-gray-500 dark:text-gray-400">
                <span>
                  {connection.signal_percent}% ({connection.signal_dbm} dBm)
                </span>
                <span aria-hidden="true" className="text-gray-300 dark:text-gray-600">
                  ·
                </span>
                <span className="font-mono">{connection.ip_address}</span>
                <SecurityBadge encryption={connection.encryption} />
              </div>
            </div>
          </div>
          <div className="flex flex-wrap gap-2">
            {/* One click used to drop the uplink outright. If auto-reconnect
                is on, the link silently returns and the action looks broken;
                if the operator is reaching the router over this WiFi, they
                lose the page entirely. */}
            <Button
              variant="outline"
              size="sm"
              onClick={() => setConfirming(true)}
              disabled={disconnectMutation.isPending}
            >
              {disconnectMutation.isPending ? 'Disconnecting...' : 'Disconnect'}
            </Button>
            <WifiScanDialog />
            <WifiHiddenNetworkDialog />
          </div>
        </div>
      ) : (
        <div className="rounded-lg border border-dashed border-gray-300 p-4 dark:border-white/15">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-300">
              <WifiOff className="h-4 w-4 text-gray-400 dark:text-gray-500" />
              Not connected to any WiFi network
            </div>
            <div className="flex flex-wrap gap-2">
              <WifiScanDialog />
              <WifiHiddenNetworkDialog />
            </div>
          </div>
        </div>
      )}
      <ConfirmDialog
        open={confirming}
        onOpenChange={setConfirming}
        title="Disconnect from this network?"
        description={`Stops using ${connection?.ssid ?? 'this network'} as the Wi-Fi uplink.`}
        warningText="Internet stops working. If you are managing the router over this WiFi connection you will lose access. If auto-reconnect is on, the router will retry this network automatically."
        confirmLabel="Disconnect"
        isPending={disconnectMutation.isPending}
        onConfirm={confirmDisconnect}
      />
    </section>
  );
}
