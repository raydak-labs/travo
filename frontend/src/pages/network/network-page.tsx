import { useRouterState } from '@tanstack/react-router';
import { useNetworkStatus, useBlockedClients } from '@/hooks/use-network';
import { NetworkPageAdvancedPanel } from '@/pages/network/network-page-advanced-panel';
import { NetworkPageConfigurationPanel } from '@/pages/network/network-page-configuration-panel';
import { NetworkPageStatusPanel } from '@/pages/network/network-page-status-panel';
import { networkPathnameToTab } from '@/pages/network/network-path-utils';

export function NetworkPage() {
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  const activeTab = networkPathnameToTab(pathname);

  const { data: network, isLoading } = useNetworkStatus();
  const { data: blockedClients } = useBlockedClients();

  return (
    <div className="space-y-6">
      {activeTab === 'status' ? (
        <NetworkPageStatusPanel
          network={network}
          isLoading={isLoading}
          blockedClients={blockedClients}
        />
      ) : null}
      {activeTab === 'configuration' ? <NetworkPageConfigurationPanel /> : null}
      {activeTab === 'advanced' ? <NetworkPageAdvancedPanel /> : null}
    </div>
  );
}
