import { PageSection } from '@/components/ui/page-section';
import { DataUsageSection } from '@/pages/network/data-usage-section';
import { DdnsCard } from '@/pages/network/ddns-card';
import { DiagnosticsCard } from '@/pages/network/diagnostics-card';
import { DoHCard } from '@/pages/network/doh-card';
import { FailoverCard } from '@/pages/network/failover-card';
import { FirewallCard } from '@/pages/network/firewall-card';
import { IPv6Card } from '@/pages/network/ipv6-card';
import { SpeedTestCard } from '@/pages/network/speed-test-card';
import { USBTetheringSection } from '@/pages/network/usb-tethering-section';
import { WoLCard } from '@/pages/network/wol-card';

export function NetworkPageAdvancedPanel() {
  return (
    <div className="space-y-6">
      <FailoverCard />
      <USBTetheringSection />
      <DataUsageSection />
      <DiagnosticsCard />
      <SpeedTestCard />

      <div>
        <h2 className="mb-3 text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500">
          Power tools
        </h2>
        <div className="space-y-3">
          <PageSection title="Dynamic DNS">
            <DdnsCard />
          </PageSection>
          <PageSection title="Firewall">
            <FirewallCard />
          </PageSection>
          <PageSection title="IPv6">
            <IPv6Card />
          </PageSection>
          <PageSection title="DNS over HTTPS/TLS">
            <DoHCard />
          </PageSection>
          <PageSection title="Wake-on-LAN">
            <WoLCard />
          </PageSection>
        </div>
      </div>
    </div>
  );
}
