import { DhcpPoolSettingsCard } from './dhcp-pool-settings-card';
import { LanDnsSettingsCard } from './lan-dns-settings-card';

export function DhcpDnsCard() {
  return (
    <>
      <DhcpPoolSettingsCard />
      <LanDnsSettingsCard />
    </>
  );
}
