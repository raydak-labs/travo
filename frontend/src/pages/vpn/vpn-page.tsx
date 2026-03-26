import { useState } from 'react';
import { ShieldCheck, ShieldAlert, AlertTriangle, CheckCircle, Loader2, XCircle, Info } from 'lucide-react';
import { WireguardSection } from './wireguard-section';
import { SplitTunnelCard } from './split-tunnel-card';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { useDNSLeakTest, useVerifyWireGuard, useWireguardStatus } from '@/hooks/use-vpn';
import { useAdGuardDNS } from '@/hooks/use-services';
import type { DNSLeakResult, VPNVerifyResult } from '@shared/index';

function DNSLeakTestCard() {
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

        <Button
          size="sm"
          onClick={handleRun}
          disabled={testMutation.isPending}
          className="gap-1.5"
        >
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
            {/* Overall result */}
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

            {/* Current nameservers */}
            <div>
              <p className="text-xs font-medium text-gray-500 uppercase tracking-wide mb-1">
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

            {/* VPN DNS servers */}
            {result.vpn_dns_servers.length > 0 && (
              <div>
                <p className="text-xs font-medium text-gray-500 uppercase tracking-wide mb-1">
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

function StatusRow({ label, ok }: { label: string; ok: boolean }) {
  return (
    <div className="flex items-center justify-between text-sm">
      <span className="text-gray-600 dark:text-gray-400">{label}</span>
      {ok ? (
        <CheckCircle className="h-4 w-4 text-green-500" />
      ) : (
        <XCircle className="h-4 w-4 text-red-500" />
      )}
    </div>
  );
}

function VerifyVPNCard() {
  const verifyMutation = useVerifyWireGuard();
  const [result, setResult] = useState<VPNVerifyResult | null>(null);

  const handleVerify = () => {
    verifyMutation.mutate(undefined, {
      onSuccess: (data) => setResult(data),
    });
  };

  const allOk = result
    ? result.interface_up && result.handshake_ok && result.route_ok && result.firewall_zone_ok && result.forwarding_ok
    : null;

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">Verify VPN</CardTitle>
        <ShieldCheck className="h-4 w-4 text-gray-500" />
      </CardHeader>
      <CardContent className="space-y-3">
        <p className="text-sm text-gray-500">
          Check that the WireGuard tunnel, routes, and firewall rules are correctly configured.
        </p>

        <Button
          size="sm"
          onClick={handleVerify}
          disabled={verifyMutation.isPending}
          className="gap-1.5"
        >
          {verifyMutation.isPending ? (
            <>
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
              Verifying…
            </>
          ) : (
            <>
              <ShieldCheck className="h-3.5 w-3.5" />
              Run Check
            </>
          )}
        </Button>

        {result && (
          <div className="space-y-2 pt-1">
            <div className="flex items-center gap-2">
              {allOk ? (
                <Badge className="bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200">All checks passed</Badge>
              ) : (
                <Badge className="bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200">Issues detected</Badge>
              )}
            </div>
            <div className="space-y-1.5 rounded-md bg-gray-50 dark:bg-gray-800 p-3">
              <StatusRow label="Interface up (wg0)" ok={result.interface_up} />
              <StatusRow label="Recent handshake (< 3 min)" ok={result.handshake_ok} />
              <StatusRow label="Default route via wg0" ok={result.route_ok} />
              <StatusRow label="Firewall zone (wg0)" ok={result.firewall_zone_ok} />
              <StatusRow label="LAN → VPN forwarding" ok={result.forwarding_ok} />
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function AdGuardVPNHint() {
  const { data: wgStatus } = useWireguardStatus();
  const { data: dnsStatus } = useAdGuardDNS();

  const vpnActive = !!wgStatus?.interface;
  const adguardDnsActive = dnsStatus?.enabled === true;

  if (!vpnActive || !adguardDnsActive) return null;

  return (
    <div className="flex gap-3 rounded-md border border-blue-200 bg-blue-50 p-3 text-sm dark:border-blue-800 dark:bg-blue-950">
      <Info className="mt-0.5 h-4 w-4 shrink-0 text-blue-500" />
      <div className="space-y-1">
        <p className="font-medium text-blue-800 dark:text-blue-200">
          WireGuard VPN and AdGuard DNS are both active
        </p>
        <p className="text-blue-700 dark:text-blue-300">
          DNS queries from LAN clients are handled by AdGuard Home locally, then forwarded to your
          configured upstream resolvers over the VPN tunnel. If your WireGuard profile specifies
          custom DNS servers, consider adding them as upstream resolvers in AdGuard to ensure they
          are used.
        </p>
      </div>
    </div>
  );
}

export function VpnPage() {
  return (
    <div className="space-y-6">
      {/* WireGuard Section */}
      <WireguardSection />

      {/* Split Tunneling */}
      <SplitTunnelCard />

      {/* VPN + AdGuard interplay hint */}
      <AdGuardVPNHint />

      {/* Verify VPN */}
      <VerifyVPNCard />

      {/* DNS Leak Test */}
      <DNSLeakTestCard />
    </div>
  );
}
