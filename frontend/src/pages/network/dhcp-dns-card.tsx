import { useState, useEffect } from 'react';
import { Settings, Globe } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import { InfoTooltip } from '@/components/ui/info-tooltip';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  useDHCPConfig,
  useSetDHCPConfig,
  useDNSConfig,
  useSetDNSConfig,
} from '@/hooks/use-network';

export function DhcpDnsCard() {
  const { data: dhcpConfig, isLoading: dhcpLoading } = useDHCPConfig();
  const setDHCP = useSetDHCPConfig();
  const { data: dnsConfig, isLoading: dnsLoading } = useDNSConfig();
  const setDNS = useSetDNSConfig();

  const [dhcpStart, setDhcpStart] = useState<number>(100);
  const [dhcpLimit, setDhcpLimit] = useState<number>(150);
  const [dhcpLease, setDhcpLease] = useState<string>('12h');
  const [useCustomDNS, setUseCustomDNS] = useState(false);
  const [dnsServer1, setDnsServer1] = useState('');
  const [dnsServer2, setDnsServer2] = useState('');

  useEffect(() => {
    if (dhcpConfig) {
      setDhcpStart(dhcpConfig.start);
      setDhcpLimit(dhcpConfig.limit);
      setDhcpLease(dhcpConfig.lease_time);
    }
  }, [dhcpConfig]);

  useEffect(() => {
    if (dnsConfig) {
      setUseCustomDNS(dnsConfig.use_custom_dns);
      setDnsServer1(dnsConfig.servers?.[0] || '');
      setDnsServer2(dnsConfig.servers?.[1] || '');
    }
  }, [dnsConfig]);

  return (
    <>
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

      {/* DNS Configuration */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">DNS Configuration</CardTitle>
          <Globe className="h-4 w-4 text-gray-500" />
        </CardHeader>
        <CardContent>
          {dnsLoading ? (
            <Skeleton className="h-4 w-1/2" />
          ) : (
            <div className="space-y-4">
              <div className="flex items-center gap-2">
                <Switch
                  checked={useCustomDNS}
                  onChange={(e) => setUseCustomDNS(e.target.checked)}
                />
                <span className="text-sm">Use custom DNS servers</span>
              </div>
              {useCustomDNS && (
                <>
                  <div className="flex flex-wrap gap-2">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => {
                        setDnsServer1('8.8.8.8');
                        setDnsServer2('8.8.4.4');
                      }}
                    >
                      Google
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => {
                        setDnsServer1('1.1.1.1');
                        setDnsServer2('1.0.0.1');
                      }}
                    >
                      Cloudflare
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => {
                        setDnsServer1('9.9.9.9');
                        setDnsServer2('149.112.112.112');
                      }}
                    >
                      Quad9
                    </Button>
                  </div>
                  <div className="grid grid-cols-2 gap-4">
                    <div className="space-y-1">
                      <label className="flex items-center gap-1 text-xs text-gray-500">
                        Primary DNS
                        <InfoTooltip text="DNS server that resolves domain names to IP addresses for all LAN clients. E.g., 8.8.8.8 (Google), 1.1.1.1 (Cloudflare), 9.9.9.9 (Quad9)." />
                      </label>
                      <Input
                        value={dnsServer1}
                        onChange={(e) => setDnsServer1(e.target.value)}
                        placeholder="8.8.8.8"
                      />
                    </div>
                    <div className="space-y-1">
                      <label className="text-xs text-gray-500">Secondary DNS</label>
                      <Input
                        value={dnsServer2}
                        onChange={(e) => setDnsServer2(e.target.value)}
                        placeholder="8.8.4.4"
                      />
                    </div>
                  </div>
                </>
              )}
              <Button
                size="sm"
                onClick={() => {
                  const servers = [dnsServer1, dnsServer2].filter(Boolean);
                  setDNS.mutate({ use_custom_dns: useCustomDNS, servers });
                }}
                disabled={setDNS.isPending}
              >
                {setDNS.isPending ? 'Saving…' : 'Save DNS Settings'}
              </Button>
            </div>
          )}
        </CardContent>
      </Card>
    </>
  );
}
