import { WireguardSection } from './wireguard-section';
import { SplitTunnelCard } from './split-tunnel-card';
import { VpnSpeedTestCard } from './vpn-speed-test-card';
import { VpnDnsLeakTestCard } from './vpn-dns-leak-test-card';
import { VpnVerifyWireguardCard } from './vpn-verify-wireguard-card';
import { VpnAdguardHint } from './vpn-adguard-hint';

export function VpnPage() {
  return (
    <div className="space-y-6">
      <WireguardSection />
      <SplitTunnelCard />
      <VpnAdguardHint />
      <VpnVerifyWireguardCard />
      <VpnDnsLeakTestCard />
      <VpnSpeedTestCard />
    </div>
  );
}
