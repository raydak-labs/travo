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
    <div className="flex gap-3 rounded-md border border-[var(--status-info-border)] bg-[var(--status-info-surface)] p-3 text-sm">
      <Info className="mt-0.5 h-4 w-4 shrink-0 text-[var(--status-info-text)]" />
      <div className="space-y-1">
        <p className="font-medium text-[var(--status-info-text)]">
          WireGuard VPN and AdGuard DNS are both active
        </p>
        <p>
          DNS queries from LAN clients are handled by AdGuard Home locally, then forwarded to your
          configured upstream resolvers over the VPN tunnel. If your WireGuard profile specifies
          custom DNS servers, consider adding them as upstream resolvers in AdGuard to ensure they
          are used.
        </p>
      </div>
    </div>
  );
}
