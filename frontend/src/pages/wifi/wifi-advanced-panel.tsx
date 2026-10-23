import { PageSection } from '@/components/ui/page-section';
import { SectionHeading } from '@/components/ui/section-heading';
import { DNSToolsCard } from '@/components/wifi/dns-tools-card';
import { WifiRadioHardwareCard } from './wifi-radio-hardware-card';
import { GuestNetworkCard } from './guest-network-card';
import { MACAddressCard } from './mac-address-card';
import { BandSwitchingCard } from './band-switching-card';
import { WiFiScheduleCard } from './wifi-schedule-card';
import { MACPolicyCard } from './mac-policy-card';
import { RepeaterRadioLayoutCard } from './repeater-radio-layout-card';

/**
 * The settings that are not part of the everyday connect flow.
 *
 * Every entry is the same collapsible `PageSection`, so the page reads as one
 * list instead of five differently-shaped cards. Guest network, cloning and
 * the schedule start expanded because they are the common tasks here; the
 * radio and policy tools stay collapsed.
 */
export function WifiAdvancedPanel() {
  return (
    <div className="space-y-3">
      <PageSection title="Guest network" defaultOpen>
        <GuestNetworkCard />
      </PageSection>
      <PageSection title="MAC address cloning" defaultOpen>
        <MACAddressCard />
      </PageSection>
      <PageSection title="Auto band switching" defaultOpen>
        <BandSwitchingCard />
      </PageSection>
      <PageSection title="WiFi schedule" defaultOpen>
        <WiFiScheduleCard />
      </PageSection>
      <PageSection title="DNS configuration" defaultOpen>
        <DNSToolsCard />
      </PageSection>

      <div className="pt-3">
        <SectionHeading>Power tools</SectionHeading>
        <div className="space-y-3">
          <PageSection title="Radio hardware">
            <WifiRadioHardwareCard />
          </PageSection>
          <PageSection title="Repeater radio layout">
            <RepeaterRadioLayoutCard />
          </PageSection>
          <PageSection title="MAC policy">
            <MACPolicyCard />
          </PageSection>
        </div>
      </div>
    </div>
  );
}
