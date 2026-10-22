import { useWifiConnection } from '@/hooks/use-wifi';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { CaptivePortalCard } from '@/components/wifi/captive-portal-card';
import { WifiHealthBanner } from '@/components/wifi/wifi-health-banner';
import { WifiModeCard } from '@/components/wifi/wifi-mode-card';
import { WifiCurrentConnectionCard } from './wifi-current-connection-card';
import { WifiSummaryBand } from './wifi-summary-band';
import { WifiSavedNetworksCard } from './wifi-saved-networks-card';
import { APConfigCard } from './ap-config-card';

export function WifiWirelessPanel() {
  const { data: connection } = useWifiConnection();
  const currentMode = connection?.mode;
  const isPureAP = currentMode === 'ap';
  const isPureSTA = currentMode === 'client';

  return (
    <div className="space-y-6">
      {/* Actions that unblock connectivity come before the facts they explain:
          a portal login or a down link is the first thing the operator needs. */}
      {!isPureAP && <CaptivePortalCard />}
      <WifiHealthBanner />
      <WifiSummaryBand />
      <WifiModeCard />

      {!isPureAP && (
        <Card>
          <CardHeader className="pb-3">
            <CardTitle>WiFi uplink</CardTitle>
            <CardDescription>
              The network this router uses for internet, and the profiles it can switch to.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <WifiCurrentConnectionCard />
            <div className="border-t border-gray-200 pt-4 dark:border-white/10">
              <WifiSavedNetworksCard />
            </div>
          </CardContent>
        </Card>
      )}

      {!isPureSTA && (
        <div className="mx-auto w-full max-w-screen-2xl 2xl:max-w-[90rem]">
          <APConfigCard />
        </div>
      )}
    </div>
  );
}
