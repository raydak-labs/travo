import { useState } from 'react';
import { ShieldCheck, ShieldAlert, AlertTriangle, CheckCircle, Loader2 } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { useDNSLeakTest } from '@/hooks/use-vpn';
import type { DNSLeakResult } from '@shared/index';

export function VpnDnsLeakTestCard() {
  const testMutation = useDNSLeakTest();
  const [result, setResult] = useState<DNSLeakResult | null>(null);

  const handleRun = () => {
    testMutation.mutate(undefined, {
      onSuccess: (data) => setResult(data),
    });
  };

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">DNS Leak Test</CardTitle>
        <ShieldCheck className="h-4 w-4 text-gray-500" />
      </CardHeader>
      <CardContent className="space-y-3">
        <p className="text-sm text-gray-500">
          Verify that DNS queries are routed through the VPN and not leaking to your ISP.
        </p>

        <Button size="sm" onClick={handleRun} disabled={testMutation.isPending} className="gap-1.5">
          {testMutation.isPending ? (
            <>
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
              Testing…
            </>
          ) : (
            <>
              <ShieldCheck className="h-3.5 w-3.5" />
              Run Test
            </>
          )}
        </Button>

        {result && (
          <div className="space-y-2 rounded-md border p-3 text-sm">
            {result.vpn_active && result.potentially_leaking ? (
              <div className="flex items-center gap-2 text-red-600 dark:text-red-400">
                <ShieldAlert className="h-4 w-4 shrink-0" />
                <span className="font-medium">DNS leak detected</span>
                <Badge variant="destructive">Leaking</Badge>
              </div>
            ) : result.vpn_active ? (
              <div className="flex items-center gap-2 text-green-600 dark:text-green-400">
                <CheckCircle className="h-4 w-4 shrink-0" />
                <span className="font-medium">No DNS leak detected</span>
                <Badge variant="success">OK</Badge>
              </div>
            ) : (
              <div className="flex items-center gap-2 text-yellow-600 dark:text-yellow-400">
                <AlertTriangle className="h-4 w-4 shrink-0" />
                <span className="font-medium">VPN not active — test inconclusive</span>
              </div>
            )}

            <div>
              <p className="mb-1 text-xs font-medium uppercase tracking-wide text-gray-500">
                Active DNS Servers
              </p>
              {result.nameservers.length > 0 ? (
                <ul className="space-y-0.5">
                  {result.nameservers.map((ns) => (
                    <li key={ns} className="font-mono text-xs text-gray-700 dark:text-gray-300">
                      {ns}
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="text-xs text-gray-400">None found in /etc/resolv.conf</p>
              )}
            </div>

            {result.vpn_dns_servers.length > 0 && (
              <div>
                <p className="mb-1 text-xs font-medium uppercase tracking-wide text-gray-500">
                  VPN DNS Servers
                </p>
                <ul className="space-y-0.5">
                  {result.vpn_dns_servers.map((ns) => (
                    <li key={ns} className="font-mono text-xs text-gray-700 dark:text-gray-300">
                      {ns}
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
