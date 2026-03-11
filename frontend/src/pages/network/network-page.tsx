import { useState, useEffect } from 'react';
import { Network, Globe, Wifi, Settings } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { useNetworkStatus, useDHCPConfig, useSetDHCPConfig } from '@/hooks/use-network';
import { formatBytes } from '@/lib/utils';
import { ClientsTable } from './clients-table';

export function NetworkPage() {
  const { data: network, isLoading } = useNetworkStatus();
  const { data: dhcpConfig, isLoading: dhcpLoading } = useDHCPConfig();
  const setDHCP = useSetDHCPConfig();
  const [dhcpStart, setDhcpStart] = useState<number>(100);
  const [dhcpLimit, setDhcpLimit] = useState<number>(150);
  const [dhcpLease, setDhcpLease] = useState<string>('12h');

  useEffect(() => {
    if (dhcpConfig) {
      setDhcpStart(dhcpConfig.start);
      setDhcpLimit(dhcpConfig.limit);
      setDhcpLease(dhcpConfig.lease_time);
    }
  }, [dhcpConfig]);

  return (
    <div className="space-y-6">
      {/* Internet Status */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Internet Connectivity</CardTitle>
          <Globe className="h-4 w-4 text-gray-500" />
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <Skeleton className="h-4 w-1/3" />
          ) : (
            <Badge variant={network?.internet_reachable ? 'success' : 'destructive'}>
              {network?.internet_reachable ? 'Connected' : 'No Internet'}
            </Badge>
          )}
        </CardContent>
      </Card>

      {/* WAN Configuration */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">WAN Configuration</CardTitle>
          <Wifi className="h-4 w-4 text-gray-500" />
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="space-y-2">
              <Skeleton className="h-4 w-3/4" />
              <Skeleton className="h-4 w-1/2" />
            </div>
          ) : network?.wan ? (
            <div className="rounded-md bg-gray-50 p-3 text-sm dark:bg-gray-900">
              <div className="grid grid-cols-2 gap-2">
                <span className="text-gray-500">Type</span>
                <span className="text-gray-900 dark:text-white">{network.wan.type}</span>
                <span className="text-gray-500">IP Address</span>
                <span className="text-gray-900 dark:text-white">{network.wan.ip_address}</span>
                <span className="text-gray-500">Gateway</span>
                <span className="text-gray-900 dark:text-white">{network.wan.gateway}</span>
                <span className="text-gray-500">DNS</span>
                <span className="text-gray-900 dark:text-white">
                  {(network.wan.dns_servers ?? []).join(', ') || '—'}
                </span>
                <span className="text-gray-500">MAC</span>
                <span className="text-gray-900 dark:text-white">{network.wan.mac_address}</span>
                <span className="text-gray-500">Traffic</span>
                <span className="text-gray-900 dark:text-white">
                  ↓ {formatBytes(network.wan.rx_bytes)} / ↑ {formatBytes(network.wan.tx_bytes)}
                </span>
              </div>
            </div>
          ) : (
            <p className="text-sm text-gray-500">WAN not configured</p>
          )}
        </CardContent>
      </Card>

      {/* LAN Configuration */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">LAN Configuration</CardTitle>
          <Network className="h-4 w-4 text-gray-500" />
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="space-y-2">
              <Skeleton className="h-4 w-3/4" />
              <Skeleton className="h-4 w-1/2" />
            </div>
          ) : network ? (
            <div className="rounded-md bg-gray-50 p-3 text-sm dark:bg-gray-900">
              <div className="grid grid-cols-2 gap-2">
                <span className="text-gray-500">IP Address</span>
                <span className="text-gray-900 dark:text-white">{network.lan.ip_address}</span>
                <span className="text-gray-500">Subnet</span>
                <span className="text-gray-900 dark:text-white">{network.lan.netmask}</span>
                <span className="text-gray-500">MAC</span>
                <span className="text-gray-900 dark:text-white">{network.lan.mac_address}</span>
              </div>
            </div>
          ) : null}
        </CardContent>
      </Card>

      {/* DHCP Configuration */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">DHCP Configuration</CardTitle>
          <Settings className="h-4 w-4 text-gray-500" />
        </CardHeader>
        <CardContent>
          {dhcpLoading ? (
            <div className="space-y-2">
              <Skeleton className="h-4 w-3/4" />
              <Skeleton className="h-4 w-1/2" />
            </div>
          ) : (
            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-1">
                  <label className="text-xs text-gray-500">Start Offset</label>
                  <Input
                    type="number"
                    min={2}
                    max={254}
                    value={dhcpStart}
                    onChange={(e) => setDhcpStart(Number(e.target.value))}
                  />
                </div>
                <div className="space-y-1">
                  <label className="text-xs text-gray-500">Pool Size</label>
                  <Input
                    type="number"
                    min={1}
                    max={253}
                    value={dhcpLimit}
                    onChange={(e) => setDhcpLimit(Number(e.target.value))}
                  />
                </div>
              </div>
              <div className="space-y-1">
                <label className="text-xs text-gray-500">Lease Time</label>
                <Select value={dhcpLease} onValueChange={setDhcpLease}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="1h">1 hour</SelectItem>
                    <SelectItem value="2h">2 hours</SelectItem>
                    <SelectItem value="6h">6 hours</SelectItem>
                    <SelectItem value="12h">12 hours</SelectItem>
                    <SelectItem value="24h">24 hours</SelectItem>
                    <SelectItem value="48h">48 hours</SelectItem>
                    <SelectItem value="7d">7 days</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <Button
                onClick={() =>
                  setDHCP.mutate({ start: dhcpStart, limit: dhcpLimit, lease_time: dhcpLease })
                }
                disabled={setDHCP.isPending}
                size="sm"
              >
                {setDHCP.isPending ? 'Saving…' : 'Save DHCP Settings'}
              </Button>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Connected Clients */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Connected Clients</CardTitle>
          <Network className="h-4 w-4 text-gray-500" />
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="space-y-2">
              <Skeleton className="h-8 w-full" />
              <Skeleton className="h-8 w-full" />
              <Skeleton className="h-8 w-full" />
            </div>
          ) : network?.clients && network.clients.length > 0 ? (
            <ClientsTable clients={network.clients} />
          ) : (
            <p className="text-sm text-gray-500">No clients connected</p>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
