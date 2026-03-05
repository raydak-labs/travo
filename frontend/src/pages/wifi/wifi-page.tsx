import { useState } from 'react';
import { Wifi, WifiOff, Signal } from 'lucide-react';
import type { WifiScanResult } from '@shared/index';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import { Skeleton } from '@/components/ui/skeleton';
import { CaptivePortalBanner } from '@/components/wifi/captive-portal-banner';
import { SignalStrengthIcon } from '@/components/wifi/signal-strength-icon';
import { SecurityBadge } from '@/components/wifi/security-badge';
import { WifiScanList } from './wifi-scan-list';
import { WifiConnectDialog } from './wifi-connect-dialog';
import { WifiModeSelector } from './wifi-mode-selector';
import {
  useWifiConnection,
  useWifiScan,
  useWifiConnect,
  useWifiDisconnect,
  useWifiMode,
  useSavedNetworks,
} from '@/hooks/use-wifi';

export function WifiPage() {
  const { data: connection, isLoading: connectionLoading } = useWifiConnection();
  const { data: scanResults = [], isLoading: scanLoading, refetch: refetchScan } = useWifiScan();
  const { data: savedNetworks = [], isLoading: savedLoading } = useSavedNetworks();
  const connectMutation = useWifiConnect();
  const disconnectMutation = useWifiDisconnect();
  const modeMutation = useWifiMode();

  const [selectedNetwork, setSelectedNetwork] = useState<WifiScanResult | null>(null);

  function handleConnect(ssid: string, password: string) {
    connectMutation.mutate({ ssid, password }, { onSuccess: () => setSelectedNetwork(null) });
  }

  return (
    <div className="space-y-6">
      <CaptivePortalBanner />

      {/* Current Connection */}
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
              <Button
                variant="outline"
                size="sm"
                onClick={() => disconnectMutation.mutate()}
                disabled={disconnectMutation.isPending}
              >
                {disconnectMutation.isPending ? 'Disconnecting...' : 'Disconnect'}
              </Button>
            </div>
          ) : (
            <p className="text-sm text-gray-500">Not connected to any WiFi network</p>
          )}
        </CardContent>
      </Card>

      {/* Available Networks */}
      <Card>
        <CardContent className="pt-6">
          <WifiScanList
            networks={scanResults}
            isLoading={scanLoading}
            onRefresh={() => void refetchScan()}
            onConnect={setSelectedNetwork}
          />
        </CardContent>
      </Card>

      {/* Saved Networks */}
      <Card>
        <CardHeader>
          <CardTitle className="text-sm font-medium">Saved Networks</CardTitle>
        </CardHeader>
        <CardContent>
          {savedLoading ? (
            <div className="space-y-2">
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
            </div>
          ) : savedNetworks.length === 0 ? (
            <p className="text-sm text-gray-500">No saved networks</p>
          ) : (
            <ul className="divide-y divide-gray-200 dark:divide-gray-800" role="list">
              {savedNetworks.map((network) => (
                <li key={network.ssid} className="flex items-center justify-between py-3">
                  <div className="flex items-center gap-3">
                    <Wifi className="h-4 w-4 text-gray-400" />
                    <div>
                      <p className="text-sm font-medium text-gray-900 dark:text-white">
                        {network.ssid}
                      </p>
                      <SecurityBadge encryption={network.encryption} />
                    </div>
                  </div>
                  <Switch
                    defaultChecked={network.auto_connect}
                    aria-label={`Auto-connect ${network.ssid}`}
                  />
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>

      {/* WiFi Mode Selector */}
      <Card>
        <CardContent className="pt-6">
          <WifiModeSelector
            currentMode={connection?.mode ?? 'client'}
            onModeChange={(mode) => modeMutation.mutate(mode)}
            isChanging={modeMutation.isPending}
          />
        </CardContent>
      </Card>

      {/* Connect Dialog */}
      {selectedNetwork && (
        <WifiConnectDialog
          network={selectedNetwork}
          isConnecting={connectMutation.isPending}
          error={connectMutation.error?.message ?? null}
          onConnect={handleConnect}
          onCancel={() => setSelectedNetwork(null)}
        />
      )}
    </div>
  );
}
