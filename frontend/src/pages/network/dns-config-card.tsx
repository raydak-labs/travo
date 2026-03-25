import { useState, useEffect } from 'react';
import { Globe } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import { InfoTooltip } from '@/components/ui/info-tooltip';
import { useDNSConfig, useSetDNSConfig } from '@/hooks/use-network';

export function DnsConfigCard() {
  const { data: dnsConfig, isLoading: dnsLoading } = useDNSConfig();
  const setDNS = useSetDNSConfig();
  const [useCustomDNS, setUseCustomDNS] = useState(false);
  const [dnsServer1, setDnsServer1] = useState('');
  const [dnsServer2, setDnsServer2] = useState('');

  useEffect(() => {
    if (dnsConfig) {
      setUseCustomDNS(dnsConfig.use_custom_dns);
      setDnsServer1(dnsConfig.servers?.[0] || '');
      setDnsServer2(dnsConfig.servers?.[1] || '');
    }
  }, [dnsConfig]);

  return (
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
  );
}
