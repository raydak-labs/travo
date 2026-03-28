import { cn } from '@/lib/cn';
import type { NetworkSectionTab } from '@/pages/network/network-page-types';

type NetworkPageTabBarProps = {
  activeTab: NetworkSectionTab;
  onTabChange: (tab: NetworkSectionTab) => void;
  tabIds: Record<NetworkSectionTab, string>;
  panelIds: Record<NetworkSectionTab, string>;
};

export function NetworkPageTabBar({
  activeTab,
  onTabChange,
  tabIds,
  panelIds,
}: NetworkPageTabBarProps) {
  const tabBtn = (tab: NetworkSectionTab, label: string) => (
    <button
      type="button"
      role="tab"
      id={tabIds[tab]}
      aria-selected={activeTab === tab}
      aria-controls={panelIds[tab]}
      tabIndex={activeTab === tab ? 0 : -1}
      className={cn(
        'rounded-md px-3 py-1.5 text-sm font-medium transition-colors',
        activeTab === tab
          ? 'bg-white text-gray-900 shadow-sm dark:bg-gray-800 dark:text-white'
          : 'text-gray-600 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white',
      )}
      onClick={() => onTabChange(tab)}
    >
      {label}
    </button>
  );

  return (
    <div
      role="tablist"
      aria-label="Network page sections"
      className="flex flex-wrap gap-1 rounded-lg border border-gray-200 bg-gray-50 p-1 dark:border-white/10 dark:bg-gray-900/40"
    >
      {tabBtn('status', 'Status')}
      {tabBtn('configuration', 'Configuration')}
      {tabBtn('advanced', 'Advanced')}
    </div>
  );
}
