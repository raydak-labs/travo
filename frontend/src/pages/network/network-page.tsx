import { Network, Globe, Wifi } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { useNetworkStatus } from '@/hooks/use-network';
import { formatBytes } from '@/lib/utils';
import { ClientsTable } from './clients-table';

export function NetworkPage() {
  const { data: network, isLoading } = useNetworkStatus();

  return (
    <div className="space-y-6">
      {/* Internet Status */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Internet Connectivity</CardTitle>
          <Globe className="h-4 w-4 text-gray-500" />
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <Skeleton className="h-4 w-1/3" />
          ) : (
            <Badge variant={network?.internet_reachable ? 'success' : 'destructive'}>
              {network?.internet_reachable ? 'Connected' : 'No Internet'}
            </Badge>
          )}
        </CardContent>
      </Card>

      {/* WAN Configuration */}
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
                  {network.wan.dns_servers.join(', ')}
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
        </CardContent>
      </Card>

      {/* LAN Configuration */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">LAN Configuration</CardTitle>
          <Network className="h-4 w-4 text-gray-500" />
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="space-y-2">
              <Skeleton className="h-4 w-3/4" />
              <Skeleton className="h-4 w-1/2" />
            </div>
          ) : network ? (
            <div className="rounded-md bg-gray-50 p-3 text-sm dark:bg-gray-900">
              <div className="grid grid-cols-2 gap-2">
                <span className="text-gray-500">IP Address</span>
                <span className="text-gray-900 dark:text-white">{network.lan.ip_address}</span>
                <span className="text-gray-500">Subnet</span>
                <span className="text-gray-900 dark:text-white">{network.lan.netmask}</span>
                <span className="text-gray-500">MAC</span>
                <span className="text-gray-900 dark:text-white">{network.lan.mac_address}</span>
              </div>
            </div>
          ) : null}
        </CardContent>
      </Card>

      {/* Connected Clients */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Connected Clients</CardTitle>
          <Network className="h-4 w-4 text-gray-500" />
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="space-y-2">
              <Skeleton className="h-8 w-full" />
              <Skeleton className="h-8 w-full" />
              <Skeleton className="h-8 w-full" />
            </div>
          ) : network && network.clients.length > 0 ? (
            <ClientsTable clients={network.clients} />
          ) : (
            <p className="text-sm text-gray-500">No clients connected</p>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
