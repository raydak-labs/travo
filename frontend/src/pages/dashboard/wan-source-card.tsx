import { Cable, Wifi, Usb, WifiOff } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { useNetworkStatus } from '@/hooks/use-network';
import type { NetworkInterface } from '@shared/index';

type WanSource = 'ethernet' | 'wifi' | 'usb' | 'none';

interface WanSourceInfo {
  readonly source: WanSource;
  readonly label: string;
  readonly icon: typeof Cable;
  readonly iface?: NetworkInterface;
}

function detectWanSource(
  wan: NetworkInterface | null | undefined,
  interfaces: readonly NetworkInterface[] | undefined,
): WanSourceInfo {
  // Check the primary WAN interface first
  if (wan?.is_up) {
    if (wan.type === 'usb') return { source: 'usb', label: 'USB Tether', icon: Usb, iface: wan };
    if (wan.type === 'wifi') return { source: 'wifi', label: 'WiFi', icon: Wifi, iface: wan };
    return { source: 'ethernet', label: 'Ethernet', icon: Cable, iface: wan };
  }

  // Fallback: scan interfaces for an active upstream
  if (interfaces) {
    for (const iface of interfaces) {
      if (!iface.is_up || iface.type === 'lan' || iface.type === 'vpn') continue;
      if (iface.type === 'usb')
        return { source: 'usb', label: 'USB Tether', icon: Usb, iface };
      if (iface.type === 'wifi') return { source: 'wifi', label: 'WiFi', icon: Wifi, iface };
      if (iface.type === 'wan')
        return { source: 'ethernet', label: 'Ethernet', icon: Cable, iface };
    }
  }

  return { source: 'none', label: 'No Connection', icon: WifiOff };
}

export function WanSourceCard() {
  const { data: network, isLoading } = useNetworkStatus();

  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>WAN Source</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <Skeleton className="h-4 w-3/4" />
          <Skeleton className="h-4 w-1/2" />
        </CardContent>
      </Card>
    );
  }

  const info = detectWanSource(network?.wan, network?.interfaces);
  const Icon = info.icon;
  const isActive = info.source !== 'none';

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">WAN Source</CardTitle>
        <Icon className="h-4 w-4 text-gray-500 dark:text-gray-400" />
      </CardHeader>
      <CardContent>
        <div className="flex items-center gap-2">
          <div
            className={`h-2.5 w-2.5 rounded-full ${isActive ? 'bg-green-500' : 'bg-red-500'}`}
          />
          <Badge variant={isActive ? 'success' : 'destructive'}>{info.label}</Badge>
        </div>
        {info.iface && (
          <div className="mt-3 space-y-1 text-sm text-gray-600 dark:text-gray-400">
            <div>Interface: {info.iface.name}</div>
            {info.iface.ip_address && <div>IP: {info.iface.ip_address}</div>}
            {info.iface.gateway && <div>Gateway: {info.iface.gateway}</div>}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
