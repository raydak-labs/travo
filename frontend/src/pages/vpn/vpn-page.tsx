import { PageSection } from '@/components/ui/page-section';
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
      <PageSection title="Split Tunneling">
        <SplitTunnelCard />
      </PageSection>
      <VpnAdguardHint />
      <PageSection title="Verify VPN">
        <VpnVerifyWireguardCard />
      </PageSection>
      <PageSection title="DNS Leak Test">
        <VpnDnsLeakTestCard />
      </PageSection>
      <PageSection title="VPN Speed Test">
        <VpnSpeedTestCard />
      </PageSection>
    </div>
  );
}
