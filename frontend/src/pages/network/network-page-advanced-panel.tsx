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
      <DdnsCard />
      <FailoverCard />
      <FirewallCard />
      <IPv6Card />
      <DoHCard />
      <WoLCard />
      <DiagnosticsCard />
      <SpeedTestCard />
      <USBTetheringSection />
      <DataUsageSection />
    </div>
  );
}
