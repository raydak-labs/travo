import { Shield } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { useVpnStatus } from '@/hooks/use-vpn';
import { useServices } from '@/hooks/use-services';
import { formatBytes } from '@/lib/utils';
import { WireguardSection } from './wireguard-section';
import { TailscaleSection } from './tailscale-section';

export function VpnPage() {
  const { data: vpnStatuses = [], isLoading } = useVpnStatus();
  const { data: services = [] } = useServices();

  return (
    <div className="space-y-6">
      {/* VPN Overview */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">VPN Overview</CardTitle>
          <Shield className="h-4 w-4 text-gray-500" />
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="space-y-2">
              <Skeleton className="h-4 w-3/4" />
              <Skeleton className="h-4 w-1/2" />
            </div>
          ) : vpnStatuses.length === 0 ? (
            <p className="text-sm text-gray-500">No VPN connections configured</p>
          ) : (
            <ul className="divide-y divide-gray-200 dark:divide-gray-800" role="list">
              {vpnStatuses.map((vpn) => {
                const svc = services.find(
                  (s) => s.id === vpn.type || s.id === vpn.type.toLowerCase(),
                );
                const notInstalled = svc?.state === 'not_installed';
                return (
                  <li key={vpn.type} className="flex items-center justify-between py-3">
                    <div className="flex items-center gap-2">
                      <span
                        className={`font-medium capitalize ${notInstalled ? 'text-gray-400 dark:text-gray-500' : 'text-gray-900 dark:text-white'}`}
                      >
                        {vpn.type}
                      </span>
                      {notInstalled ? (
                        <Badge variant="secondary">Not Installed</Badge>
                      ) : (
                        <Badge variant={vpn.connected ? 'success' : 'outline'}>
                          {vpn.connected ? 'Connected' : 'Disconnected'}
                        </Badge>
                      )}
                    </div>
                    <div className="text-sm text-gray-500">
                      {vpn.connected && !notInstalled && (
                        <span>
                          ↓ {formatBytes(vpn.rx_bytes)} / ↑ {formatBytes(vpn.tx_bytes)}
                        </span>
                      )}
                    </div>
                  </li>
                );
              })}
            </ul>
          )}
        </CardContent>
      </Card>

      {/* WireGuard Section */}
      <WireguardSection />

      {/* Tailscale Section */}
      <TailscaleSection />
    </div>
  );
}
