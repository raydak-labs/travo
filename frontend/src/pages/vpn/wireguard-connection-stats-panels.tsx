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
      <div className="rounded-md bg-gray-50 p-3 text-sm dark:bg-gray-900">
        <h4 className="mb-2 font-medium text-gray-700 dark:text-gray-300">Connection Status</h4>
        {(wgLiveStatus.peers ?? []).map((peer) => (
          <div key={peer.public_key} className="grid grid-cols-2 gap-2">
            <span className="text-gray-500">Endpoint</span>
            <span className="text-gray-900 dark:text-white">{peer.endpoint}</span>
            <span className="text-gray-500">Last Handshake</span>
            <span className="text-gray-900 dark:text-white">
              {formatWireguardHandshakeTime(peer.latest_handshake)}
            </span>
            <span className="text-gray-500">RX</span>
            <span className="text-gray-900 dark:text-white">{formatBytes(peer.transfer_rx)}</span>
            <span className="text-gray-500">TX</span>
            <span className="text-gray-900 dark:text-white">{formatBytes(peer.transfer_tx)}</span>
            <span className="text-gray-500">Allowed IPs</span>
            <span className="text-gray-900 dark:text-white">{peer.allowed_ips}</span>
          </div>
        ))}
      </div>
    );
  }

  if (wgStatus?.connected && !wgLiveStatus) {
    return (
      <div className="rounded-md bg-gray-50 p-3 text-sm dark:bg-gray-900">
        <div className="grid grid-cols-2 gap-2">
          <span className="text-gray-500">Endpoint</span>
          <span className="text-gray-900 dark:text-white">{wgStatus.endpoint}</span>
          <span className="text-gray-500">RX</span>
          <span className="text-gray-900 dark:text-white">{formatBytes(wgStatus.rx_bytes)}</span>
          <span className="text-gray-500">TX</span>
          <span className="text-gray-900 dark:text-white">{formatBytes(wgStatus.tx_bytes)}</span>
        </div>
      </div>
    );
  }

  return null;
}
