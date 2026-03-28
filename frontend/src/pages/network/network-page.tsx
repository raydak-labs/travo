import { useId, useState } from 'react';
import { useNetworkStatus, useBlockedClients } from '@/hooks/use-network';
import { NetworkPageAdvancedPanel } from '@/pages/network/network-page-advanced-panel';
import { NetworkPageConfigurationPanel } from '@/pages/network/network-page-configuration-panel';
import { NetworkPageStatusPanel } from '@/pages/network/network-page-status-panel';
import { NetworkPageTabBar } from '@/pages/network/network-page-tab-bar';
import type { NetworkSectionTab } from '@/pages/network/network-page-types';

export function NetworkPage() {
  const baseId = useId();
  const tabIds = {
    status: `${baseId}-tab-status`,
    configuration: `${baseId}-tab-configuration`,
    advanced: `${baseId}-tab-advanced`,
  };
  const panelIds = {
    status: `${baseId}-panel-status`,
    configuration: `${baseId}-panel-configuration`,
    advanced: `${baseId}-panel-advanced`,
  };

  const [activeTab, setActiveTab] = useState<NetworkSectionTab>('status');
  const { data: network, isLoading } = useNetworkStatus();
  const { data: blockedClients } = useBlockedClients();

  return (
    <div className="space-y-4">
      <NetworkPageTabBar
        activeTab={activeTab}
        onTabChange={setActiveTab}
        tabIds={tabIds}
        panelIds={panelIds}
      />

      <NetworkPageStatusPanel
        panelId={panelIds.status}
        tabId={tabIds.status}
        hidden={activeTab !== 'status'}
        network={network}
        isLoading={isLoading}
        blockedClients={blockedClients}
      />

      <NetworkPageConfigurationPanel
        panelId={panelIds.configuration}
        tabId={tabIds.configuration}
        hidden={activeTab !== 'configuration'}
      />

      <NetworkPageAdvancedPanel
        panelId={panelIds.advanced}
        tabId={tabIds.advanced}
        hidden={activeTab !== 'advanced'}
      />
    </div>
  );
}
