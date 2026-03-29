import { GuestNetworkCard } from './guest-network-card';
import { MACAddressCard } from './mac-address-card';
import { BandSwitchingCard } from './band-switching-card';
import { WiFiScheduleCard } from './wifi-schedule-card';
import { MACPolicyCard } from './mac-policy-card';

type WifiAdvancedPanelProps = {
  panelId: string;
  tabId: string;
  hidden: boolean;
};

export function WifiAdvancedPanel({ panelId, tabId, hidden }: WifiAdvancedPanelProps) {
  return (
    <div id={panelId} role="tabpanel" aria-labelledby={tabId} hidden={hidden} className="space-y-6">
      <GuestNetworkCard />
      <MACAddressCard />
      <MACPolicyCard />
      <BandSwitchingCard />
      <WiFiScheduleCard />
    </div>
  );
}
