import { useQuery } from '@tanstack/react-query';
import { Shield } from 'lucide-react';
import { apiClient } from '@/lib/api-client';
import type { VpnStatus } from '@shared/index';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Switch } from '@/components/ui/switch';
import { Skeleton } from '@/components/ui/skeleton';
import { formatBytes } from '@/lib/utils';

export function VpnStatusCard() {
  const { data, isLoading } = useQuery({
    queryKey: ['vpn', 'status'],
    queryFn: () => apiClient.get<VpnStatus[]>('/api/v1/vpn/status'),
  });

  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>VPN Status</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <Skeleton className="h-4 w-3/4" />
          <Skeleton className="h-4 w-1/2" />
        </CardContent>
      </Card>
    );
  }

  const vpn = data?.find((v) => v.enabled || v.connected);

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">VPN Status</CardTitle>
        <Shield className="h-4 w-4 text-gray-500 dark:text-gray-400" />
      </CardHeader>
      <CardContent>
        {vpn ? (
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Badge variant={vpn.connected ? 'success' : 'destructive'}>
                  {vpn.connected ? 'Connected' : 'Disconnected'}
                </Badge>
                <span className="text-xs uppercase text-gray-500 dark:text-gray-400">
                  {vpn.type}
                </span>
              </div>
              <Switch checked={vpn.enabled} readOnly aria-label="Toggle VPN" />
            </div>
            {vpn.connected && (
              <>
                <p className="text-sm text-gray-600 dark:text-gray-400">{vpn.endpoint}</p>
                {(vpn.rx_bytes > 0 || vpn.tx_bytes > 0) && (
                  <div className="flex gap-4 text-xs text-gray-500 dark:text-gray-400">
                    <span>↓ {formatBytes(vpn.rx_bytes)}</span>
                    <span>↑ {formatBytes(vpn.tx_bytes)}</span>
                  </div>
                )}
              </>
            )}
          </div>
        ) : (
          <p className="text-sm text-gray-500 dark:text-gray-400">No VPN configured</p>
        )}
      </CardContent>
    </Card>
  );
}
