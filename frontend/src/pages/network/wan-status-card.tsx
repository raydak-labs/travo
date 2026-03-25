import { Info, Cable, CheckCircle, XCircle } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { useNetworkStatus } from '@/hooks/use-network';
import type { NetworkInterface } from '@shared/index';

const WAN_SOURCES = [
  {
    key: 'wan',
    label: 'WAN (Ethernet)',
    match: (iface: NetworkInterface) =>
      iface.type === 'wan' || (iface.name === 'wan' && iface.type !== 'wifi'),
    description: 'Wired ethernet uplink via the WAN port.',
  },
  {
    key: 'wwan',
    label: 'WWAN (WiFi Client)',
    match: (iface: NetworkInterface) =>
      iface.type === 'wifi' && (iface.name.startsWith('wlan-sta') || iface.name.startsWith('wwan')),
    description: 'Wireless uplink connected to an upstream WiFi network.',
  },
] as const;

function WanInterplay({ interfaces }: { interfaces: readonly NetworkInterface[] }) {
  const sources = WAN_SOURCES.map((src) => {
    const iface = interfaces.find(src.match);
    const active = iface != null && iface.is_up && iface.ip_address !== '';
    return { ...src, iface, active };
  });

  const anyActive = sources.some((s) => s.active);

  return (
    <div className="space-y-3">
      <div className="space-y-2">
        {sources.map((src) => (
          <div
            key={src.key}
            className="flex items-center gap-3 rounded-md bg-gray-50 p-3 dark:bg-gray-900"
          >
            <span
              className={`inline-block h-2.5 w-2.5 rounded-full ${
                src.active
                  ? 'bg-green-500 shadow-[0_0_6px_rgba(34,197,94,0.6)]'
                  : 'bg-gray-300 dark:bg-gray-600'
              }`}
              aria-label={src.active ? 'Active' : 'Inactive'}
            />
            <div className="flex-1">
              <span className="text-sm font-medium text-gray-900 dark:text-white">{src.label}</span>
              {src.iface && (
                <span className="ml-2 text-xs text-gray-500">
                  {src.active ? src.iface.ip_address : 'down'}
                </span>
              )}
              {!src.iface && <span className="ml-2 text-xs text-gray-400">not configured</span>}
            </div>
            <Badge variant={src.active ? 'success' : 'secondary'}>
              {src.active ? 'Active' : 'Inactive'}
            </Badge>
          </div>
        ))}
      </div>

      <div
        className={`flex items-start gap-2 rounded-md border p-3 text-xs ${
          anyActive
            ? 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-900 dark:bg-blue-950 dark:text-blue-300'
            : 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-300'
        }`}
      >
        <Info className="mt-0.5 h-3.5 w-3.5 shrink-0" />
        <span>
          {anyActive
            ? 'OpenWrt uses the highest-priority active source for internet. When both WAN (ethernet) and WWAN (WiFi) are connected, ethernet takes priority. If the wired link drops, traffic automatically fails over to WiFi.'
            : 'No WAN source is active. Connect an ethernet cable or join an upstream WiFi network to get internet access.'}
        </span>
      </div>
    </div>
  );
}

export function WanStatusCard() {
  const { data: network, isLoading } = useNetworkStatus();

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">WAN Status</CardTitle>
        <Cable className="h-4 w-4 text-gray-500" />
      </CardHeader>
      <CardContent className="space-y-4">
        {/* Internet connectivity row */}
        <div className="flex items-center gap-2">
          {isLoading ? (
            <Skeleton className="h-4 w-1/3" />
          ) : network?.internet_reachable ? (
            <>
              <CheckCircle className="h-4 w-4 text-green-500" />
              <span className="text-sm font-medium text-gray-900 dark:text-white">
                Internet Connected
              </span>
              <Badge variant="success">Online</Badge>
            </>
          ) : (
            <>
              <XCircle className="h-4 w-4 text-red-500" />
              <span className="text-sm font-medium text-gray-900 dark:text-white">
                No Internet
              </span>
              <Badge variant="destructive">Offline</Badge>
            </>
          )}
        </div>

        {/* WAN source interplay */}
        {isLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-8 w-full" />
            <Skeleton className="h-8 w-full" />
          </div>
        ) : (
          <WanInterplay interfaces={network?.interfaces ?? []} />
        )}
      </CardContent>
    </Card>
  );
}
