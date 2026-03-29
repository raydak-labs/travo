import { Search, Wifi } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { useNetworkStatus, useDetectWanType } from '@/hooks/use-network';
import { formatBytes } from '@/lib/utils';

export function WanConfigCard() {
  const { data: network, isLoading } = useNetworkStatus();
  const detectWanType = useDetectWanType();

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">WAN Configuration</CardTitle>
        <Wifi className="h-4 w-4 text-gray-500" />
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-4 w-3/4" />
            <Skeleton className="h-4 w-1/2" />
          </div>
        ) : network?.wan ? (
          <div className="rounded-md bg-gray-50 p-3 text-sm dark:bg-gray-900">
            <div className="grid grid-cols-2 gap-2">
              <span className="text-gray-500">Type</span>
              <span className="text-gray-900 dark:text-white">{network.wan.type}</span>
              <span className="text-gray-500">IP Address</span>
              <span className="text-gray-900 dark:text-white">{network.wan.ip_address}</span>
              <span className="text-gray-500">Gateway</span>
              <span className="text-gray-900 dark:text-white">{network.wan.gateway}</span>
              <span className="text-gray-500">DNS</span>
              <span className="text-gray-900 dark:text-white">
                {(network.wan.dns_servers ?? []).join(', ') || '—'}
              </span>
              <span className="text-gray-500">MAC</span>
              <span className="text-gray-900 dark:text-white">{network.wan.mac_address}</span>
              <span className="text-gray-500">Traffic</span>
              <span className="text-gray-900 dark:text-white">
                ↓ {formatBytes(network.wan.rx_bytes)} / ↑ {formatBytes(network.wan.tx_bytes)}
              </span>
            </div>
          </div>
        ) : (
          <p className="text-sm text-gray-500">WAN not configured</p>
        )}
        <div className="mt-3 flex items-center gap-3">
          <Button
            variant="outline"
            size="sm"
            onClick={() => detectWanType.mutate()}
            disabled={detectWanType.isPending}
          >
            <Search className="mr-1.5 h-3.5 w-3.5" />
            {detectWanType.isPending ? 'Detecting…' : 'Auto-detect WAN Type'}
          </Button>
          {detectWanType.data && (
            <span className="text-sm text-gray-600 dark:text-gray-400">
              Detected: <strong>{detectWanType.data.detected_type.toUpperCase()}</strong>
              {detectWanType.data.detected_type !== detectWanType.data.current_type && (
                <span className="ml-1 text-amber-600 dark:text-amber-400">
                  (current: {detectWanType.data.current_type})
                </span>
              )}
            </span>
          )}
          {detectWanType.data && network?.wan && <Badge variant="success">WAN Active</Badge>}
        </div>
      </CardContent>
    </Card>
  );
}
