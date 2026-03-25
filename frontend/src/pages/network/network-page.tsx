import { useNetworkStatus, useBlockedClients } from '@/hooks/use-network';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Network } from 'lucide-react';
import { Skeleton } from '@/components/ui/skeleton';
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


export function NetworkPage() {
  const { data: network, isLoading } = useNetworkStatus();
  const { data: blockedClients } = useBlockedClients();

  return (
    <div className="space-y-6">
      <WanStatusCard />

      {/* Interface Traffic Charts */}
      <InterfaceTrafficCharts />

      <WanConfigCard />
      <InterfacesCard />
      <LanConfigCard />
      <DhcpDnsCard />
      <DdnsCard />
      <DnsEntriesCard />
      <DhcpReservationsCard />
      <DhcpLeasesCard />

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
          ) : network?.clients && network.clients.length > 0 ? (
            <ClientsTable clients={network.clients} blockedMacs={blockedClients} />
          ) : (
            <p className="text-sm text-gray-500">No clients connected</p>
          )}
        </CardContent>
      </Card>

      <UptimeLogCard />

      {/* Firewall */}
      <FirewallCard />

      {/* IPv6 */}
      <IPv6Card />

      {/* DNS over HTTPS */}
      <DoHCard />

      {/* Wake-on-LAN */}
      <WoLCard />

      {/* Network Diagnostics */}
      <DiagnosticsCard />

      {/* Speed Test */}
      <SpeedTestCard />

      {/* USB Tethering */}
      <USBTetheringSection />

      {/* Data Usage Tracking */}
      <DataUsageSection />
    </div>
  );
}
