import { Network } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { QueryCard } from '@/components/ui/query-card';
import { useNetworkStatus } from '@/hooks/use-network';

export function LanConfigCard() {
  const { data: network, isLoading, isError, error, refetch } = useNetworkStatus();

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle>LAN Configuration</CardTitle>
        <Network className="h-4 w-4 text-gray-500 dark:text-gray-400" />
      </CardHeader>
      <CardContent>
        <QueryCard
          isLoading={isLoading}
          isError={isError}
          error={error}
          onRetry={() => void refetch()}
        >
          {network ? (
            <div className="rounded-md bg-gray-50 p-3 text-sm dark:bg-gray-900">
              <div className="grid grid-cols-2 gap-2">
                <span className="text-gray-500 dark:text-gray-400">IP Address</span>
                <span className="text-gray-900 dark:text-white">{network.lan.ip_address}</span>
                <span className="text-gray-500 dark:text-gray-400">Subnet</span>
                <span className="text-gray-900 dark:text-white">{network.lan.netmask}</span>
                <span className="text-gray-500 dark:text-gray-400">MAC</span>
                <span className="text-gray-900 dark:text-white">{network.lan.mac_address}</span>
              </div>
            </div>
          ) : null}
        </QueryCard>
      </CardContent>
    </Card>
  );
}
