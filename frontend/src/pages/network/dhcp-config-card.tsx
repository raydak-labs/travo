import { useState, useEffect } from 'react';
import { Settings } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import { InfoTooltip } from '@/components/ui/info-tooltip';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { useDHCPConfig, useSetDHCPConfig } from '@/hooks/use-network';

export function DhcpConfigCard() {
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
                <label className="flex items-center gap-1 text-xs text-gray-500">
                  Start Offset
                  <InfoTooltip text="First IP offset assigned to clients. Offset 100 on 192.168.1.x means the first assigned address is 192.168.1.100." />
                </label>
                <Input
                  type="number"
                  min={2}
                  max={254}
                  value={dhcpStart}
                  onChange={(e) => setDhcpStart(Number(e.target.value))}
                />
              </div>
              <div className="space-y-1">
                <label className="flex items-center gap-1 text-xs text-gray-500">
                  Pool Size
                  <InfoTooltip text="Maximum number of clients that can receive an IP address. E.g., 50 means up to 50 devices can connect." />
                </label>
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
              <label className="flex items-center gap-1 text-xs text-gray-500">
                Lease Time
                <InfoTooltip text="How long a DHCP lease is valid before renewal. Shorter times reclaim IPs faster; longer times reduce DHCP traffic." />
              </label>
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
  );
}
