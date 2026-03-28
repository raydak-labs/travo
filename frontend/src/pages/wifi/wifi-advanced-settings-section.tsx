import { useState } from 'react';
import { ChevronDown } from 'lucide-react';
import { GuestNetworkCard } from './guest-network-card';
import { MACAddressCard } from './mac-address-card';
import { BandSwitchingCard } from './band-switching-card';
import { WiFiScheduleCard } from './wifi-schedule-card';
import { MACPolicyCard } from './mac-policy-card';

export function WifiAdvancedSettingsSection() {
  const [advancedOpen, setAdvancedOpen] = useState(false);

  return (
    <div>
      <button
        type="button"
        className="flex w-full items-center justify-between rounded-lg border border-gray-200 bg-gray-50 px-4 py-3 text-sm font-medium text-gray-700 hover:bg-gray-100 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-300 dark:hover:bg-gray-800"
        onClick={() => setAdvancedOpen((o) => !o)}
        aria-expanded={advancedOpen}
      >
        <span>Advanced Settings</span>
        <ChevronDown
          className={`h-4 w-4 text-gray-500 transition-transform ${advancedOpen ? 'rotate-180' : ''}`}
        />
      </button>
      {advancedOpen && (
        <div className="mt-4 space-y-4">
          <GuestNetworkCard />
          <MACAddressCard />
          <MACPolicyCard />
          <BandSwitchingCard />
          <WiFiScheduleCard />
        </div>
      )}
    </div>
  );
}
