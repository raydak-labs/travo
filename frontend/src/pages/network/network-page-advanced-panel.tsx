import { DataUsageSection } from '@/pages/network/data-usage-section';
import { DdnsCard } from '@/pages/network/ddns-card';
import { DiagnosticsCard } from '@/pages/network/diagnostics-card';
import { DoHCard } from '@/pages/network/doh-card';
import { FirewallCard } from '@/pages/network/firewall-card';
import { IPv6Card } from '@/pages/network/ipv6-card';
import { SpeedTestCard } from '@/pages/network/speed-test-card';
import { USBTetheringSection } from '@/pages/network/usb-tethering-section';
import { WoLCard } from '@/pages/network/wol-card';

type NetworkPageAdvancedPanelProps = {
  panelId: string;
  tabId: string;
  hidden: boolean;
};

export function NetworkPageAdvancedPanel({
  panelId,
  tabId,
  hidden,
}: NetworkPageAdvancedPanelProps) {
  return (
    <div
      id={panelId}
      role="tabpanel"
      aria-labelledby={tabId}
      hidden={hidden}
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
  );
}
