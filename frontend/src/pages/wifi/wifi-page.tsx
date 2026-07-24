import { useRouterState } from '@tanstack/react-router';
import { WifiRepeaterSameRadioBanner } from '@/components/wifi/wifi-repeater-same-radio-banner';
import { WifiAdvancedPanel } from '@/pages/wifi/wifi-advanced-panel';
import { WifiWirelessPanel } from '@/pages/wifi/wifi-wireless-panel';

export function WifiPage() {
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  const isAdvanced = pathname === '/wifi/advanced';

  return (
    <div className="space-y-6">
      <WifiRepeaterSameRadioBanner />
      {isAdvanced ? <WifiAdvancedPanel /> : <WifiWirelessPanel />}
    </div>
  );
}
