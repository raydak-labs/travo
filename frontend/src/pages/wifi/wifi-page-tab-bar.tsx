import { cn } from '@/lib/cn';
import type { WifiSectionTab } from '@/pages/wifi/wifi-page-types';

type WifiPageTabBarProps = {
  activeTab: WifiSectionTab;
  onTabChange: (tab: WifiSectionTab) => void;
  tabIds: Record<WifiSectionTab, string>;
  panelIds: Record<WifiSectionTab, string>;
};

export function WifiPageTabBar({ activeTab, onTabChange, tabIds, panelIds }: WifiPageTabBarProps) {
  const tabBtn = (tab: WifiSectionTab, label: string) => (
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
      aria-label="WiFi page sections"
      className="flex flex-wrap gap-1 rounded-lg border border-gray-200 bg-gray-50 p-1 dark:border-white/10 dark:bg-gray-900/40"
    >
      {tabBtn('wireless', 'Wireless')}
      {tabBtn('advanced', 'Advanced')}
    </div>
  );
}
