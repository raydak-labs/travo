import { Wifi, WifiOff, Signal } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { EmptyState } from '@/components/ui/empty-state';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { SignalStrengthIcon } from '@/components/wifi/signal-strength-icon';
import { SecurityBadge } from '@/components/wifi/security-badge';
import { WifiScanDialog } from './wifi-scan-dialog';
import { WifiHiddenNetworkDialog } from './wifi-hidden-network-dialog';
import { useWifiConnection, useWifiDisconnect } from '@/hooks/use-wifi';

export function WifiCurrentConnectionCard() {
  const { data: connection, isLoading: connectionLoading } = useWifiConnection();
  const disconnectMutation = useWifiDisconnect();

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">Current Connection</CardTitle>
        {connection?.connected ? (
          <Wifi className="h-4 w-4 text-green-500" />
        ) : (
          <WifiOff className="h-4 w-4 text-gray-400" />
        )}
      </CardHeader>
      <CardContent>
        {connectionLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-4 w-3/4" />
            <Skeleton className="h-4 w-1/2" />
          </div>
        ) : connection?.connected ? (
          <div className="space-y-2">
            <div className="flex items-center gap-2">
              <SignalStrengthIcon signalPercent={connection.signal_percent} />
              <span className="font-medium text-gray-900 dark:text-white">{connection.ssid}</span>
              <Badge variant="success">Connected</Badge>
            </div>
            <div className="flex flex-wrap gap-x-4 gap-y-1 text-sm text-gray-600 dark:text-gray-400">
              <div className="flex items-center gap-1">
                <Signal className="h-3.5 w-3.5" />
                <span>
                  {connection.signal_percent}% ({connection.signal_dbm} dBm)
                </span>
              </div>
              <span>Mode: {connection.mode}</span>
              <span>IP: {connection.ip_address}</span>
              <SecurityBadge encryption={connection.encryption} />
            </div>
            <div className="flex gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => disconnectMutation.mutate()}
                disabled={disconnectMutation.isPending}
              >
                {disconnectMutation.isPending ? 'Disconnecting...' : 'Disconnect'}
              </Button>
              <WifiScanDialog />
              <WifiHiddenNetworkDialog />
            </div>
          </div>
        ) : (
          <div className="space-y-3">
            <EmptyState message="Not connected to any WiFi network" />
            <div className="flex gap-2">
              <WifiScanDialog />
              <WifiHiddenNetworkDialog />
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
