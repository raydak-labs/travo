import type { VpnStatus, WireguardConfig, WireGuardStatus } from '@shared/index';
import { WireguardConfigPeersList } from '@/pages/vpn/wireguard-config-peers-list';
import { WireguardConnectionStatsPanels } from '@/pages/vpn/wireguard-connection-stats-panels';
import { WireguardStatusDetailText } from '@/pages/vpn/wireguard-status-detail-text';
import { WireguardStatusToggleRow } from '@/pages/vpn/wireguard-status-toggle-row';

export type WireguardStatusAndConfigPeersProps = {
  wgStatus: VpnStatus | undefined;
  wgLiveStatus: WireGuardStatus | undefined;
  config: WireguardConfig | undefined;
  isToggling: boolean;
  desiredEnabled: boolean | undefined;
  statusDetail: string | undefined;
  toggleMutationPending: boolean;
  onToggleWireguard: () => void;
};

export function WireguardStatusAndConfigPeers({
  wgStatus,
  wgLiveStatus,
  config,
  isToggling,
  desiredEnabled,
  statusDetail,
  toggleMutationPending,
  onToggleWireguard,
}: WireguardStatusAndConfigPeersProps) {
  return (
    <>
      <WireguardStatusToggleRow
        wgStatus={wgStatus}
        isToggling={isToggling}
        desiredEnabled={desiredEnabled}
        toggleMutationPending={toggleMutationPending}
        onToggleWireguard={onToggleWireguard}
      />
      <WireguardStatusDetailText statusDetail={statusDetail} isToggling={isToggling} />
      <WireguardConnectionStatsPanels wgStatus={wgStatus} wgLiveStatus={wgLiveStatus} />
      <WireguardConfigPeersList config={config} />
    </>
  );
}
