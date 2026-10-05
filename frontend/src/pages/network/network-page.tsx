import { useRouterState } from '@tanstack/react-router';
import { useNetworkStatus, useBlockedClients } from '@/hooks/use-network';
import { NetworkPageAdvancedPanel } from '@/pages/network/network-page-advanced-panel';
import { NetworkPageConfigurationPanel } from '@/pages/network/network-page-configuration-panel';
import { NetworkPageStatusPanel } from '@/pages/network/network-page-status-panel';
import { networkPathnameToTab } from '@/pages/network/network-path-utils';

export function NetworkPage() {
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  const activeTab = networkPathnameToTab(pathname);

  // The page owns the network-status query and hands the panel its outcome as
  // well as its data, so a failed GET cannot reach a card that would render it
  // as an empty router.
  const { data: network, isLoading, isError, error, refetch } = useNetworkStatus();
  const { data: blockedClients } = useBlockedClients();

  return (
    <div className="space-y-6">
      {activeTab === 'status' ? (
        <NetworkPageStatusPanel
          network={network}
          isLoading={isLoading}
          isError={isError}
          error={error}
          onRetry={() => void refetch()}
          blockedClients={blockedClients}
        />
      ) : null}
      {activeTab === 'configuration' ? <NetworkPageConfigurationPanel /> : null}
      {activeTab === 'advanced' ? <NetworkPageAdvancedPanel /> : null}
    </div>
  );
}
