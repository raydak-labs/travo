import type { NetworkStatus } from '@shared/index';
import { Link } from '@tanstack/react-router';
import { Network } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent, CardFooter } from '@/components/ui/card';
import { EmptyState } from '@/components/ui/empty-state';
import { Skeleton } from '@/components/ui/skeleton';
import { ClientsTable } from '@/pages/network/clients-table';
import { InterfaceTrafficCharts } from '@/pages/network/interface-traffic-charts';
import { UptimeLogCard } from '@/pages/network/uptime-log-card';
import { WanStatusCard } from '@/pages/network/wan-status-card';

const CLIENTS_PREVIEW_LIMIT = 5;

type NetworkPageStatusPanelProps = {
  network: NetworkStatus | undefined;
  isLoading: boolean;
  blockedClients: string[] | undefined;
};

export function NetworkPageStatusPanel({
  network,
  isLoading,
  blockedClients,
}: NetworkPageStatusPanelProps) {
  const hasClients = Boolean(network?.clients && network.clients.length > 0);

  return (
    <div className="space-y-6">
      <WanStatusCard />

      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <div className="space-y-1">
            <CardTitle>Connected Clients</CardTitle>
            <p className="text-xs text-gray-500 dark:text-gray-400">
              Devices on your travel Wi‑Fi. Full list and reservations live under Clients.
            </p>
          </div>
          <Network className="h-4 w-4 text-gray-500 dark:text-gray-400" />
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="space-y-2">
              <Skeleton className="h-8 w-full" />
              <Skeleton className="h-8 w-full" />
              <Skeleton className="h-8 w-full" />
            </div>
          ) : hasClients && network?.clients ? (
            <ClientsTable
              clients={network.clients}
              blockedMacs={blockedClients}
              limit={CLIENTS_PREVIEW_LIMIT}
            />
          ) : (
            <EmptyState message="No clients connected" />
          )}
        </CardContent>
        {hasClients ? (
          <CardFooter className="justify-end">
            <Link
              to="/clients"
              className="text-sm text-blue-600 hover:underline dark:text-blue-400"
            >
              View all
            </Link>
          </CardFooter>
        ) : null}
      </Card>

      <InterfaceTrafficCharts />
      <UptimeLogCard />
    </div>
  );
}
