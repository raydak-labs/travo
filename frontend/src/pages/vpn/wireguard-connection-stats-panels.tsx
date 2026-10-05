import { CardInset } from '@/components/ui/card-inset';
import { formatBytes } from '@/lib/utils';
import type { VpnStatus, WireGuardStatus } from '@shared/index';
import { formatWireguardHandshakeTime } from '@/pages/vpn/wireguard-utils';

type WireguardConnectionStatsPanelsProps = {
  wgStatus: VpnStatus | undefined;
  wgLiveStatus: WireGuardStatus | undefined;
};

export function WireguardConnectionStatsPanels({
  wgStatus,
  wgLiveStatus,
}: WireguardConnectionStatsPanelsProps) {
  if (wgStatus?.connected && wgLiveStatus && (wgLiveStatus.peers?.length ?? 0) > 0) {
    return (
      <CardInset variant="muted">
        <h4 className="mb-2 font-medium text-gray-700 dark:text-gray-300">Connection Status</h4>
        {(wgLiveStatus.peers ?? []).map((peer) => (
          <div key={peer.public_key} className="grid grid-cols-2 gap-2">
            <span className="text-gray-500 dark:text-gray-400">Endpoint</span>
            <span className="text-gray-900 dark:text-white">{peer.endpoint}</span>
            <span className="text-gray-500 dark:text-gray-400">Last Handshake</span>
            <span className="text-gray-900 dark:text-white">
              {formatWireguardHandshakeTime(peer.latest_handshake)}
            </span>
            <span className="text-gray-500 dark:text-gray-400">RX</span>
            <span className="text-gray-900 dark:text-white">{formatBytes(peer.transfer_rx)}</span>
            <span className="text-gray-500 dark:text-gray-400">TX</span>
            <span className="text-gray-900 dark:text-white">{formatBytes(peer.transfer_tx)}</span>
            <span className="text-gray-500 dark:text-gray-400">Allowed IPs</span>
            <span className="text-gray-900 dark:text-white">{peer.allowed_ips}</span>
          </div>
        ))}
      </CardInset>
    );
  }

  if (wgStatus?.connected && !wgLiveStatus) {
    return (
      <CardInset variant="muted">
        <div className="grid grid-cols-2 gap-2">
          <span className="text-gray-500 dark:text-gray-400">Endpoint</span>
          <span className="text-gray-900 dark:text-white">{wgStatus.endpoint}</span>
          <span className="text-gray-500 dark:text-gray-400">RX</span>
          <span className="text-gray-900 dark:text-white">{formatBytes(wgStatus.rx_bytes)}</span>
          <span className="text-gray-500 dark:text-gray-400">TX</span>
          <span className="text-gray-900 dark:text-white">{formatBytes(wgStatus.tx_bytes)}</span>
        </div>
      </CardInset>
    );
  }

  return null;
}
