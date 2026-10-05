import { PageSection } from '@/components/ui/page-section';
import { SectionHeading } from '@/components/ui/section-heading';
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

      <div>
        <SectionHeading>Diagnostics</SectionHeading>
        <div className="space-y-3">
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
      </div>
    </div>
  );
}
