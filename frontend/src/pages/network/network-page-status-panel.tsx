import type { NetworkStatus } from '@shared/index';
import { Network } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { EmptyState } from '@/components/ui/empty-state';
import { Skeleton } from '@/components/ui/skeleton';
import { ClientsTable } from '@/pages/network/clients-table';
import { InterfaceTrafficCharts } from '@/pages/network/interface-traffic-charts';
import { UptimeLogCard } from '@/pages/network/uptime-log-card';
import { WanStatusCard } from '@/pages/network/wan-status-card';

type NetworkPageStatusPanelProps = {
  panelId: string;
  tabId: string;
  hidden: boolean;
  network: NetworkStatus | undefined;
  isLoading: boolean;
  blockedClients: string[] | undefined;
};

export function NetworkPageStatusPanel({
  panelId,
  tabId,
  hidden,
  network,
  isLoading,
  blockedClients,
}: NetworkPageStatusPanelProps) {
  return (
    <div id={panelId} role="tabpanel" aria-labelledby={tabId} hidden={hidden} className="space-y-6">
      <WanStatusCard />

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
          ) : network?.clients && network.clients.length > 0 ? (
            <ClientsTable clients={network.clients} blockedMacs={blockedClients} />
          ) : (
            <EmptyState message="No clients connected" />
          )}
        </CardContent>
      </Card>

      <InterfaceTrafficCharts />
      <UptimeLogCard />
    </div>
  );
}
