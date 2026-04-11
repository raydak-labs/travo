import { useId } from 'react';
import { useNavigate, useRouterState } from '@tanstack/react-router';
import { WifiRepeaterSameRadioBanner } from '@/components/wifi/wifi-repeater-same-radio-banner';
import { WifiPageTabBar } from '@/pages/wifi/wifi-page-tab-bar';
import type { WifiSectionTab } from '@/pages/wifi/wifi-page-types';
import { WifiAdvancedPanel } from '@/pages/wifi/wifi-advanced-panel';
import { WifiWirelessPanel } from '@/pages/wifi/wifi-wireless-panel';

function pathnameToWifiTab(pathname: string): WifiSectionTab {
  return pathname === '/wifi/advanced' ? 'advanced' : 'wireless';
}

export function WifiPage() {
  const baseId = useId();
  const tabIds = {
    wireless: `${baseId}-tab-wireless`,
    advanced: `${baseId}-tab-advanced`,
  };
  const panelIds = {
    wireless: `${baseId}-panel-wireless`,
    advanced: `${baseId}-panel-advanced`,
  };

  const pathname = useRouterState({ select: (s) => s.location.pathname });
  const navigate = useNavigate();
  const activeTab = pathnameToWifiTab(pathname);

  const onTabChange = (tab: WifiSectionTab) => {
    if (tab === 'advanced') {
      void navigate({ to: '/wifi/advanced' });
    } else {
      void navigate({ to: '/wifi' });
    }
  };

  return (
    <div className="space-y-4">
      <WifiRepeaterSameRadioBanner />
      <WifiPageTabBar
        activeTab={activeTab}
        onTabChange={onTabChange}
        tabIds={tabIds}
        panelIds={panelIds}
      />

      <WifiWirelessPanel
        panelId={panelIds.wireless}
        tabId={tabIds.wireless}
        hidden={activeTab !== 'wireless'}
      />

      <WifiAdvancedPanel
        panelId={panelIds.advanced}
        tabId={tabIds.advanced}
        hidden={activeTab !== 'advanced'}
      />
    </div>
  );
}
