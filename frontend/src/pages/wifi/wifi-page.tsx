import { useWifiConnection } from '@/hooks/use-wifi';
import { CaptivePortalBanner } from '@/components/wifi/captive-portal-banner';
import { WifiModeCard } from '@/components/wifi/wifi-mode-card';
import { WifiRadioHardwareCard } from './wifi-radio-hardware-card';
import { WifiCurrentConnectionCard } from './wifi-current-connection-card';
import { WifiSavedNetworksCard } from './wifi-saved-networks-card';
import { WifiAdvancedSettingsSection } from './wifi-advanced-settings-section';
import { APConfigCard } from './ap-config-card';

export function WifiPage() {
  const { data: connection } = useWifiConnection();
  const currentMode = connection?.mode;
  const isPureAP = currentMode === 'ap';
  const isPureSTA = currentMode === 'client';

  return (
    <div className="space-y-6">
      <CaptivePortalBanner />
      <WifiModeCard />
      <WifiRadioHardwareCard />
      {!isPureAP && <WifiCurrentConnectionCard />}
      {!isPureAP && <WifiSavedNetworksCard />}
      {!isPureSTA && <APConfigCard />}
      <WifiAdvancedSettingsSection />
    </div>
  );
}
