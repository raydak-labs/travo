import { useState } from 'react';
import { Shield } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import { Skeleton } from '@/components/ui/skeleton';
import {
  useWireguardConfig,
  useSetWireguardConfig,
  useToggleWireguard,
  useVpnStatus,
} from '@/hooks/use-vpn';
import { formatBytes } from '@/lib/utils';

export function WireguardSection() {
  const { data: vpnStatuses = [] } = useVpnStatus();
  const { data: config, isLoading } = useWireguardConfig();
  const setConfigMutation = useSetWireguardConfig();
  const toggleMutation = useToggleWireguard();
  const [configText, setConfigText] = useState('');

  const wgStatus = vpnStatuses.find((v) => v.type === 'wireguard');

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">WireGuard</CardTitle>
        <Shield className="h-4 w-4 text-blue-500" />
      </CardHeader>
      <CardContent className="space-y-4">
        {isLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-4 w-3/4" />
            <Skeleton className="h-4 w-1/2" />
          </div>
        ) : (
          <>
            {/* Status */}
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <span className="text-sm text-gray-700 dark:text-gray-300">Status</span>
                <Badge variant={wgStatus?.connected ? 'success' : 'outline'}>
                  {wgStatus?.connected ? 'Connected' : 'Disconnected'}
                </Badge>
              </div>
              <Switch
                id="wireguard-toggle"
                label="Enable"
                checked={wgStatus?.enabled ?? false}
                onChange={() => toggleMutation.mutate(!(wgStatus?.enabled ?? false))}
                disabled={toggleMutation.isPending}
              />
            </div>

            {/* Connection Details */}
            {wgStatus?.connected && (
              <div className="rounded-md bg-gray-50 p-3 text-sm dark:bg-gray-900">
                <div className="grid grid-cols-2 gap-2">
                  <span className="text-gray-500">Endpoint</span>
                  <span className="text-gray-900 dark:text-white">{wgStatus.endpoint}</span>
                  <span className="text-gray-500">RX</span>
                  <span className="text-gray-900 dark:text-white">
                    {formatBytes(wgStatus.rx_bytes)}
                  </span>
                  <span className="text-gray-500">TX</span>
                  <span className="text-gray-900 dark:text-white">
                    {formatBytes(wgStatus.tx_bytes)}
                  </span>
                </div>
              </div>
            )}

            {/* Peers */}
            {config && config.peers.length > 0 && (
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
                          {peer.allowed_ips.join(', ')}
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
            )}

            {/* Import Config */}
            <div>
              <h4 className="mb-2 text-sm font-medium text-gray-700 dark:text-gray-300">
                Import Configuration
              </h4>
              <textarea
                className="w-full rounded-md border border-gray-300 bg-white p-2 text-sm font-mono dark:border-gray-700 dark:bg-gray-900 dark:text-white"
                rows={4}
                placeholder="Paste WireGuard config here..."
                value={configText}
                onChange={(e) => setConfigText(e.target.value)}
              />
              <Button
                size="sm"
                className="mt-2"
                disabled={!configText.trim() || setConfigMutation.isPending}
                onClick={() => {
                  setConfigMutation.mutate(
                    { private_key: configText, address: '', dns: [], peers: [] },
                    { onSuccess: () => setConfigText('') },
                  );
                }}
              >
                {setConfigMutation.isPending ? 'Importing...' : 'Import'}
              </Button>
            </div>
          </>
        )}
      </CardContent>
    </Card>
  );
}
