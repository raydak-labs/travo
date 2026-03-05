import { useQuery } from '@tanstack/react-query';
import { Wifi, Globe, Signal } from 'lucide-react';
import { apiClient } from '@/lib/api-client';
import type { NetworkStatus, WifiConnection } from '@shared/index';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { CaptivePortalBanner } from '@/components/wifi/captive-portal-banner';

export function ConnectionStatusCard() {
  const { data: network, isLoading: networkLoading } = useQuery({
    queryKey: ['network', 'status'],
    queryFn: () => apiClient.get<NetworkStatus>('/api/v1/network/status'),
  });

  const { data: wifi, isLoading: wifiLoading } = useQuery({
    queryKey: ['wifi', 'connection'],
    queryFn: () => apiClient.get<WifiConnection>('/api/v1/wifi/connection'),
  });

  const isLoading = networkLoading || wifiLoading;

  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Connection Status</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <Skeleton className="h-4 w-3/4" />
          <Skeleton className="h-4 w-1/2" />
          <Skeleton className="h-4 w-2/3" />
        </CardContent>
      </Card>
    );
  }

  const isConnected = network?.wan?.is_up ?? false;
  const connectionType = network?.wan?.type ?? 'none';

  const ConnectionIcon = connectionType === 'wifi' ? Wifi : Globe;

  return (
    <div className="space-y-3">
      <CaptivePortalBanner />
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Connection Status</CardTitle>
          <ConnectionIcon className="h-4 w-4 text-gray-500 dark:text-gray-400" />
        </CardHeader>
        <CardContent>
          <div className="flex items-center gap-2">
            <div
              className={`h-2.5 w-2.5 rounded-full ${isConnected ? 'bg-green-500' : 'bg-red-500'}`}
            />
            <Badge variant={isConnected ? 'success' : 'destructive'}>
              {isConnected ? 'Connected' : 'Disconnected'}
            </Badge>
          </div>
          {wifi?.connected && (
            <div className="mt-3 space-y-1 text-sm text-gray-600 dark:text-gray-400">
              <div className="flex items-center gap-2">
                <Wifi className="h-3.5 w-3.5" />
                <span>{wifi.ssid}</span>
              </div>
              <div className="flex items-center gap-2">
                <Signal className="h-3.5 w-3.5" />
                <span>
                  {wifi.signal_percent}% ({wifi.signal_dbm} dBm)
                </span>
              </div>
              <div>IP: {network?.wan?.ip_address}</div>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
