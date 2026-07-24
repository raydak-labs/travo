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
      <DNSToolsCard />
      <RepeaterRadioLayoutCard />
      <WifiRadioHardwareCard />
      <GuestNetworkCard />
      <MACAddressCard />
      <MACPolicyCard />
      <BandSwitchingCard />
      <WiFiScheduleCard />
    </div>
  );
}
