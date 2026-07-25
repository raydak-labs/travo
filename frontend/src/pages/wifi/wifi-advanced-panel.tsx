import { PageSection } from '@/components/ui/page-section';
import { DNSToolsCard } from '@/components/wifi/dns-tools-card';
import { WifiRadioHardwareCard } from './wifi-radio-hardware-card';
import { GuestNetworkCard } from './guest-network-card';
import { MACAddressCard } from './mac-address-card';
import { BandSwitchingCard } from './band-switching-card';
import { WiFiScheduleCard } from './wifi-schedule-card';
import { MACPolicyCard } from './mac-policy-card';
import { RepeaterRadioLayoutCard } from './repeater-radio-layout-card';

export function WifiAdvancedPanel() {
  return (
    <div className="space-y-6">
      <p className="text-sm text-gray-500 dark:text-gray-400">
        Guest network, cloning, and schedule stay visible. Expand only for radio / policy power
        tools.
      </p>

      <GuestNetworkCard />
      <MACAddressCard />
      <BandSwitchingCard />
      <WiFiScheduleCard />
      <DNSToolsCard />

      <div>
        <h2 className="mb-3 text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500">
          Power tools
        </h2>
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
