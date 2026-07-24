import { PageSection } from '@/components/ui/page-section';
import { DhcpDnsCard } from '@/pages/network/dhcp-dns-card';
import { DhcpLeasesCard } from '@/pages/network/dhcp-leases-card';
import { DhcpReservationsCard } from '@/pages/network/dhcp-reservations-card';
import { DnsEntriesCard } from '@/pages/network/dns-entries-card';
import { InterfacesCard } from '@/pages/network/interfaces-card';
import { LanConfigCard } from '@/pages/network/lan-config-card';
import { WanConfigCard } from '@/pages/network/wan-config-card';

export function NetworkPageConfigurationPanel() {
  return (
    <div className="space-y-6">
      <WanConfigCard />
      <InterfacesCard />
      <LanConfigCard />
      <PageSection title="DHCP & DNS">
        <div className="space-y-6">
          <DhcpDnsCard />
        </div>
      </PageSection>
      <PageSection title="Local DNS Entries">
        <DnsEntriesCard />
      </PageSection>
      <PageSection title="DHCP Reservations">
        <DhcpReservationsCard />
      </PageSection>
      <PageSection title="DHCP Leases">
        <DhcpLeasesCard />
      </PageSection>
    </div>
  );
}
