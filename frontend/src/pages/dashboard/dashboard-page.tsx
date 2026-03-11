import { ConnectionStatusCard } from './connection-status-card';
import { VpnStatusCard } from './vpn-status-card';
import { SystemStatsCard } from './system-stats-card';
import { ClientsCard } from './clients-card';
import { QuickActions } from './quick-actions';
import { BandwidthChart } from './bandwidth-chart';
import { NetworkChart } from './network-chart';
import { TimezoneAlert } from '@/components/timezone-alert';

export function DashboardPage() {
  return (
    <div className="space-y-6">
      <TimezoneAlert />
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <ConnectionStatusCard />
        <VpnStatusCard />
        <SystemStatsCard />
        <ClientsCard />
      </div>
      <div className="grid gap-4 md:grid-cols-2">
        <BandwidthChart />
        <NetworkChart />
      </div>
      <QuickActions />
    </div>
  );
}
