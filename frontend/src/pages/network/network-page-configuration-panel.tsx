import { DhcpDnsCard } from '@/pages/network/dhcp-dns-card';
import { DhcpLeasesCard } from '@/pages/network/dhcp-leases-card';
import { DhcpReservationsCard } from '@/pages/network/dhcp-reservations-card';
import { DnsEntriesCard } from '@/pages/network/dns-entries-card';
import { InterfacesCard } from '@/pages/network/interfaces-card';
import { LanConfigCard } from '@/pages/network/lan-config-card';
import { WanConfigCard } from '@/pages/network/wan-config-card';

type NetworkPageConfigurationPanelProps = {
  panelId: string;
  tabId: string;
  hidden: boolean;
};

export function NetworkPageConfigurationPanel({
  panelId,
  tabId,
  hidden,
}: NetworkPageConfigurationPanelProps) {
  return (
    <div id={panelId} role="tabpanel" aria-labelledby={tabId} hidden={hidden} className="space-y-6">
      <WanConfigCard />
      <InterfacesCard />
      <LanConfigCard />
      <DhcpDnsCard />
      <DnsEntriesCard />
      <DhcpReservationsCard />
      <DhcpLeasesCard />
    </div>
  );
}
