import { useWifiConnection } from '@/hooks/use-wifi';
import { CaptivePortalCard } from '@/components/wifi/captive-portal-card';
import { WifiHealthBanner } from '@/components/wifi/wifi-health-banner';
import { WifiModeCard } from '@/components/wifi/wifi-mode-card';
import { WifiCurrentConnectionCard } from './wifi-current-connection-card';
import { WifiSavedNetworksCard } from './wifi-saved-networks-card';
import { APConfigCard } from './ap-config-card';

type WifiWirelessPanelProps = {
  panelId: string;
  tabId: string;
  hidden: boolean;
};

export function WifiWirelessPanel({ panelId, tabId, hidden }: WifiWirelessPanelProps) {
  const { data: connection } = useWifiConnection();
  const currentMode = connection?.mode;
  const isPureAP = currentMode === 'ap';
  const isPureSTA = currentMode === 'client';

  return (
    <div id={panelId} role="tabpanel" aria-labelledby={tabId} hidden={hidden} className="space-y-6">
      <WifiHealthBanner />
      {!isPureAP && <CaptivePortalCard />}
      <WifiModeCard />
      {!isPureAP && <WifiCurrentConnectionCard />}
      {!isPureAP && <WifiSavedNetworksCard />}
      {!isPureSTA && <APConfigCard />}
    </div>
  );
}
