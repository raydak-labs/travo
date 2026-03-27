import { useState, useEffect } from 'react';
import { RefreshCw } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/cn';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { useDDNSConfig, useDDNSStatus, useSetDDNSConfig } from '@/hooks/use-network';

export function DdnsCard() {
  const { data: ddnsConfig, isLoading: ddnsConfigLoading } = useDDNSConfig();
  const { data: ddnsStatus } = useDDNSStatus();
  const setDDNS = useSetDDNSConfig();

  const [ddnsEnabled, setDdnsEnabled] = useState(false);
  const [ddnsService, setDdnsService] = useState('');
  const [ddnsDomain, setDdnsDomain] = useState('');
  const [ddnsUsername, setDdnsUsername] = useState('');
  const [ddnsPassword, setDdnsPassword] = useState('');
  const [ddnsLookupHost, setDdnsLookupHost] = useState('');
  const [ddnsUpdateUrl, setDdnsUpdateUrl] = useState('');

  useEffect(() => {
    if (ddnsConfig) {
      setDdnsEnabled(ddnsConfig.enabled);
      setDdnsService(ddnsConfig.service);
      setDdnsDomain(ddnsConfig.domain);
      setDdnsUsername(ddnsConfig.username);
      setDdnsPassword(ddnsConfig.password);
      setDdnsLookupHost(ddnsConfig.lookup_host);
      setDdnsUpdateUrl(ddnsConfig.update_url ?? '');
    }
  }, [ddnsConfig]);

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">Dynamic DNS (DDNS)</CardTitle>
        <RefreshCw className="h-4 w-4 text-gray-500" />
      </CardHeader>
      <CardContent>
        {ddnsConfigLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-4 w-1/2" />
            <Skeleton className="h-4 w-3/4" />
          </div>
        ) : (
          <div className="space-y-4">
            {ddnsStatus && (ddnsStatus.running || ddnsStatus.public_ip) && (
              <div className="flex items-center gap-3 rounded-md bg-gray-50 p-3 dark:bg-gray-900">
                <span
                  className={`inline-block h-2.5 w-2.5 rounded-full ${
                    ddnsStatus.running
                      ? 'bg-green-500 shadow-[0_0_6px_rgba(34,197,94,0.6)]'
                      : 'bg-gray-300 dark:bg-gray-600'
                  }`}
                />
                <div className="flex-1 text-sm">
                  <span className="font-medium text-gray-900 dark:text-white">
                    {ddnsStatus.running ? 'Running' : 'Stopped'}
                  </span>
                  {ddnsStatus.public_ip && (
                    <span className="ml-2 text-gray-500">IP: {ddnsStatus.public_ip}</span>
                  )}
                  {ddnsStatus.last_update && (
                    <span className="ml-2 text-xs text-gray-400">
                      Updated: {ddnsStatus.last_update}
                    </span>
                  )}
                </div>
              </div>
            )}
            <div className="flex items-center gap-2">
              <Switch checked={ddnsEnabled} onChange={(e) => setDdnsEnabled(e.target.checked)} />
              <span className="text-sm">Enable Dynamic DNS</span>
            </div>
            {ddnsEnabled && (
              <>
                <div className="space-y-1">
                  <label className="text-xs text-gray-500">Provider</label>
                  <Select
                    value={ddnsService}
                    onValueChange={(v) => {
                      setDdnsService(v);
                      if (v !== 'custom') {
                        setDdnsUpdateUrl('');
                      }
                    }}
                  >
                    <SelectTrigger>
                      <SelectValue placeholder="Select a DDNS provider" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="duckdns.org">DuckDNS</SelectItem>
                      <SelectItem value="no-ip.com">No-IP</SelectItem>
                      <SelectItem value="cloudflare.com-v4">Cloudflare</SelectItem>
                      <SelectItem value="freedns.afraid.org">FreeDNS</SelectItem>
                      <SelectItem value="dynu.com">Dynu</SelectItem>
                      <SelectItem value="desec.io">deSEC</SelectItem>
                      <SelectItem value="custom">Custom (update URL)</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                {ddnsService === 'custom' && (
                  <div className="space-y-1">
                    <label className="text-xs text-gray-500 dark:text-gray-400">Update URL</label>
                    <textarea
                      value={ddnsUpdateUrl}
                      onChange={(e) => setDdnsUpdateUrl(e.target.value)}
                      placeholder="https://example.com/update?hostname=[DOMAIN]&myip=[IP]"
                      rows={3}
                      className={cn(
                        'flex w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 placeholder:text-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent disabled:cursor-not-allowed disabled:opacity-50 dark:border-gray-700 dark:bg-gray-900 dark:text-white dark:placeholder:text-gray-500',
                      )}
                    />
                    <p className="text-xs text-gray-500 dark:text-gray-400">
                      Use ddns-scripts placeholders such as [IP], [DOMAIN], [USERNAME], [PASSWORD] as required by your
                      provider.
                    </p>
                  </div>
                )}
                <div className="space-y-1">
                  <label className="text-xs text-gray-500">Domain</label>
                  <Input
                    value={ddnsDomain}
                    onChange={(e) => setDdnsDomain(e.target.value)}
                    placeholder="myrouter.duckdns.org"
                  />
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-1">
                    <label className="text-xs text-gray-500">Username / Token</label>
                    <Input
                      value={ddnsUsername}
                      onChange={(e) => setDdnsUsername(e.target.value)}
                      placeholder="username or token"
                    />
                  </div>
                  <div className="space-y-1">
                    <label className="text-xs text-gray-500">Password</label>
                    <Input
                      type="password"
                      value={ddnsPassword}
                      onChange={(e) => setDdnsPassword(e.target.value)}
                      placeholder="password"
                    />
                  </div>
                </div>
                <div className="space-y-1">
                  <label className="text-xs text-gray-500">Lookup Host</label>
                  <Input
                    value={ddnsLookupHost}
                    onChange={(e) => setDdnsLookupHost(e.target.value)}
                    placeholder="myrouter.duckdns.org"
                  />
                </div>
              </>
            )}
            <Button
              size="sm"
              onClick={() =>
                setDDNS.mutate({
                  enabled: ddnsEnabled,
                  service: ddnsService,
                  domain: ddnsDomain,
                  username: ddnsUsername,
                  password: ddnsPassword,
                  lookup_host: ddnsLookupHost,
                  update_url: ddnsService === 'custom' ? ddnsUpdateUrl : '',
                })
              }
              disabled={setDDNS.isPending}
            >
              {setDDNS.isPending ? 'Saving…' : 'Save DDNS Settings'}
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
