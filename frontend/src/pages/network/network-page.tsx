import { useId, useState } from 'react';
import { useNetworkStatus, useBlockedClients } from '@/hooks/use-network';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { EmptyState } from '@/components/ui/empty-state';
import { Network } from 'lucide-react';
import { Skeleton } from '@/components/ui/skeleton';
import { cn } from '@/lib/cn';
import { ClientsTable } from './clients-table';
import { DataUsageSection } from './data-usage-section';
import { USBTetheringSection } from './usb-tethering-section';
import { InterfaceTrafficCharts } from './interface-traffic-charts';
import { FirewallCard } from './firewall-card';
import { DiagnosticsCard } from './diagnostics-card';
import { IPv6Card } from './ipv6-card';
import { WoLCard } from './wol-card';
import { DoHCard } from './doh-card';
import { SpeedTestCard } from './speed-test-card';
import { WanStatusCard } from './wan-status-card';
import { WanConfigCard } from './wan-config-card';
import { LanConfigCard } from './lan-config-card';
import { InterfacesCard } from './interfaces-card';
import { DhcpDnsCard } from './dhcp-dns-card';
import { DdnsCard } from './ddns-card';
import { DnsEntriesCard } from './dns-entries-card';
import { DhcpReservationsCard } from './dhcp-reservations-card';
import { DhcpLeasesCard } from './dhcp-leases-card';
import { UptimeLogCard } from './uptime-log-card';

type NetworkSectionTab = 'status' | 'configuration' | 'advanced';

export function NetworkPage() {
  const baseId = useId();
  const tabIds = {
    status: `${baseId}-tab-status`,
    configuration: `${baseId}-tab-configuration`,
    advanced: `${baseId}-tab-advanced`,
  };
  const panelIds = {
    status: `${baseId}-panel-status`,
    configuration: `${baseId}-panel-configuration`,
    advanced: `${baseId}-panel-advanced`,
  };

  const [activeTab, setActiveTab] = useState<NetworkSectionTab>('status');
  const { data: network, isLoading } = useNetworkStatus();
  const { data: blockedClients } = useBlockedClients();

  const tabBtn = (tab: NetworkSectionTab, label: string) => (
    <button
      type="button"
      role="tab"
      id={tabIds[tab]}
      aria-selected={activeTab === tab}
      aria-controls={panelIds[tab]}
      tabIndex={activeTab === tab ? 0 : -1}
      className={cn(
        'rounded-md px-3 py-1.5 text-sm font-medium transition-colors',
        activeTab === tab
          ? 'bg-white text-gray-900 shadow-sm dark:bg-gray-800 dark:text-white'
          : 'text-gray-600 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white',
      )}
      onClick={() => setActiveTab(tab)}
    >
      {label}
    </button>
  );

  return (
    <div className="space-y-4">
      <div
        role="tablist"
        aria-label="Network page sections"
        className="flex flex-wrap gap-1 rounded-lg border border-gray-200 bg-gray-50 p-1 dark:border-gray-800 dark:bg-gray-900/40"
      >
        {tabBtn('status', 'Status')}
        {tabBtn('configuration', 'Configuration')}
        {tabBtn('advanced', 'Advanced')}
      </div>

      <div
        id={panelIds.status}
        role="tabpanel"
        aria-labelledby={tabIds.status}
        hidden={activeTab !== 'status'}
        className="space-y-6"
      >
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

      <div
        id={panelIds.configuration}
        role="tabpanel"
        aria-labelledby={tabIds.configuration}
        hidden={activeTab !== 'configuration'}
        className="space-y-6"
      >
        <WanConfigCard />
        <InterfacesCard />
        <LanConfigCard />
        <DhcpDnsCard />
        <DnsEntriesCard />
        <DhcpReservationsCard />
        <DhcpLeasesCard />
      </div>

      <div
        id={panelIds.advanced}
        role="tabpanel"
        aria-labelledby={tabIds.advanced}
        hidden={activeTab !== 'advanced'}
        className="space-y-6"
      >
        <DdnsCard />
        <FirewallCard />
        <IPv6Card />
        <DoHCard />
        <WoLCard />
        <DiagnosticsCard />
        <SpeedTestCard />
        <USBTetheringSection />
        <DataUsageSection />
      </div>
    </div>
  );
}
