import { useState } from 'react';
import { ShieldCheck, CheckCircle, XCircle, Loader2 } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { useVerifyWireGuard } from '@/hooks/use-vpn';
import type { VPNVerifyResult } from '@shared/index';

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

export function VpnVerifyWireguardCard() {
  const verifyMutation = useVerifyWireGuard();
  const [result, setResult] = useState<VPNVerifyResult | null>(null);

  const handleVerify = () => {
    verifyMutation.mutate(undefined, {
      onSuccess: (data) => setResult(data),
    });
  };

  const allOk = result
    ? result.interface_up &&
      result.handshake_ok &&
      result.route_ok &&
      result.firewall_zone_ok &&
      result.forwarding_ok
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

        <Button size="sm" onClick={handleVerify} disabled={verifyMutation.isPending} className="gap-1.5">
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
                <Badge className="bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200">
                  All checks passed
                </Badge>
              ) : (
                <Badge className="bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200">
                  Issues detected
                </Badge>
              )}
            </div>
            <div className="space-y-1.5 rounded-md bg-gray-50 p-3 dark:bg-gray-800">
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
