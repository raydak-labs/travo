import type { WireguardConfig } from '@shared/index';

type WireguardConfigPeersListProps = {
  config: WireguardConfig | undefined;
};

export function WireguardConfigPeersList({ config }: WireguardConfigPeersListProps) {
  if (!config?.peers?.length) {
    return null;
  }

  return (
    <div>
      <h4 className="mb-2 text-sm font-medium text-gray-700 dark:text-gray-300">
        Peers ({config.peers.length})
      </h4>
      <ul className="space-y-2" role="list">
        {config.peers.map((peer) => (
          <li
            key={peer.public_key}
            className="rounded-md border border-gray-200 p-3 text-sm dark:border-gray-700"
          >
            <div className="grid grid-cols-2 gap-1">
              <span className="text-gray-500">Endpoint</span>
              <span className="text-gray-900 dark:text-white">{peer.endpoint}</span>
              <span className="text-gray-500">Allowed IPs</span>
              <span className="text-gray-900 dark:text-white">
                {(peer.allowed_ips ?? []).join(', ')}
              </span>
              {peer.last_handshake && (
                <>
                  <span className="text-gray-500">Last Handshake</span>
                  <span className="text-gray-900 dark:text-white">
                    {new Date(peer.last_handshake).toLocaleString()}
                  </span>
                </>
              )}
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}
