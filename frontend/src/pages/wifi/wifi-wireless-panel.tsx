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

      {!isPureAP && (
        <div className="grid grid-cols-1 items-stretch gap-4 md:grid-cols-3 md:gap-6 lg:grid-cols-5 xl:grid-cols-6 2xl:grid-cols-7">
          <div className="min-w-0 h-full md:col-span-1 lg:col-span-2 xl:col-span-2 2xl:col-span-2">
            <WifiCurrentConnectionCard />
          </div>
          <div className="min-w-0 h-full md:col-span-2 lg:col-span-3 xl:col-span-4 2xl:col-span-5">
            <WifiSavedNetworksCard />
          </div>
        </div>
      )}

      {!isPureSTA && (
        <div className="mx-auto w-full max-w-screen-2xl 2xl:max-w-[90rem]">
          <APConfigCard />
        </div>
      )}
    </div>
  );
}
