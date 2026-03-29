import { Info } from 'lucide-react';
import { useWireguardStatus } from '@/hooks/use-vpn';
import { useAdGuardDNS } from '@/hooks/use-services';

export function VpnAdguardHint() {
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
