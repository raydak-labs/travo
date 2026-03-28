import { useState } from 'react';
import { Shield, ShieldAlert, Trash2, Play, Plus, Upload, Loader2 } from 'lucide-react';
import { Link } from '@tanstack/react-router';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import { Skeleton } from '@/components/ui/skeleton';
import { useServices } from '@/hooks/use-services';
import {
  useWireguardConfig,
  useToggleWireguard,
  useVpnStatus,
  useWireguardStatus,
  useWireguardProfiles,
  useAddWireguardProfile,
  useDeleteWireguardProfile,
  useActivateWireguardProfile,
  useKillSwitch,
  useSetKillSwitch,
} from '@/hooks/use-vpn';
import { formatBytes } from '@/lib/utils';

function formatHandshakeTime(epoch: number): string {
  if (epoch === 0) return 'Never';
  const now = Math.floor(Date.now() / 1000);
  const diff = now - epoch;
  if (diff < 60) return `${diff} seconds ago`;
  if (diff < 3600) return `${Math.floor(diff / 60)} minutes ago`;
  if (diff < 86400) return `${Math.floor(diff / 3600)} hours ago`;
  return `${Math.floor(diff / 86400)} days ago`;
}

export function WireguardSection() {
  const { data: vpnStatuses = [] } = useVpnStatus();
  const { data: services = [] } = useServices();
  const { data: config, isLoading } = useWireguardConfig();
  const { data: wgLiveStatus } = useWireguardStatus();
  const { data: profiles = [] } = useWireguardProfiles();
  const toggleMutation = useToggleWireguard();
  const addProfileMutation = useAddWireguardProfile();
  const deleteProfileMutation = useDeleteWireguardProfile();
  const activateProfileMutation = useActivateWireguardProfile();
  const { data: killSwitch } = useKillSwitch();
  const killSwitchMutation = useSetKillSwitch();
  const [configText, setConfigText] = useState('');
  const [profileName, setProfileName] = useState('');

  const wgStatus = vpnStatuses.find((v) => v.type === 'wireguard');
  const wgService = services.find((s) => s.id === 'wireguard');
  const isInstalled = wgService ? wgService.state !== 'not_installed' : !!wgStatus;
  const isToggling = toggleMutation.isPending;
  const desiredEnabled = isToggling ? toggleMutation.variables : undefined;

  const statusDetail = wgStatus?.status_detail;

  const handleAddProfile = () => {
    if (!profileName.trim() || !configText.trim()) return;
    addProfileMutation.mutate(
      { name: profileName.trim(), config: configText.trim() },
      {
        onSuccess: () => {
          setConfigText('');
          setProfileName('');
        },
      },
    );
  };

  return (
    <Card className={!isInstalled ? 'opacity-60' : undefined}>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">WireGuard</CardTitle>
        <Shield className="h-4 w-4 text-blue-500" />
      </CardHeader>
      <CardContent className="space-y-4">
        {!isInstalled ? (
          <div className="text-center py-4">
            <p className="text-sm text-gray-500 mb-2">WireGuard is not installed</p>
            <Link
              to="/services"
              className="text-sm text-blue-600 hover:underline dark:text-blue-400"
            >
              Install via Services →
            </Link>
          </div>
        ) : isLoading ? (
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
                  {isToggling
                    ? desiredEnabled
                      ? 'Enabling…'
                      : 'Disabling…'
                    : wgStatus?.connected
                      ? 'Connected'
                      : 'Disconnected'}
                </Badge>
                {isToggling && (
                  <span className="inline-flex items-center gap-1 text-xs text-gray-500">
                    <Loader2 className="h-3 w-3 animate-spin" />
                    Applying changes…
                  </span>
                )}
              </div>
              <Switch
                id="wireguard-toggle"
                label="Enable"
                checked={wgStatus?.enabled ?? false}
                onChange={() => toggleMutation.mutate(!(wgStatus?.enabled ?? false))}
                disabled={toggleMutation.isPending}
                aria-busy={isToggling ? 'true' : undefined}
              />
            </div>

            {/* Fine-grained state */}
            {!!statusDetail && !isToggling && (
              <div className="text-xs text-gray-500">
                {statusDetail === 'disabled' && 'WireGuard is disabled.'}
                {statusDetail === 'configured' && 'WireGuard is configured but not connected yet.'}
                {statusDetail === 'enabled_not_up' &&
                  'WireGuard is enabled but the interface is not up yet.'}
                {statusDetail === 'up_no_handshake' &&
                  'WireGuard interface is up, but handshake has not completed yet.'}
              </div>
            )}

            {/* Connection Details — live stats from wg show */}
            {wgStatus?.connected && wgLiveStatus && (wgLiveStatus.peers?.length ?? 0) > 0 && (
              <div className="rounded-md bg-gray-50 p-3 text-sm dark:bg-gray-900">
                <h4 className="mb-2 font-medium text-gray-700 dark:text-gray-300">
                  Connection Status
                </h4>
                {(wgLiveStatus.peers ?? []).map((peer) => (
                  <div key={peer.public_key} className="grid grid-cols-2 gap-2">
                    <span className="text-gray-500">Endpoint</span>
                    <span className="text-gray-900 dark:text-white">{peer.endpoint}</span>
                    <span className="text-gray-500">Last Handshake</span>
                    <span className="text-gray-900 dark:text-white">
                      {formatHandshakeTime(peer.latest_handshake)}
                    </span>
                    <span className="text-gray-500">RX</span>
                    <span className="text-gray-900 dark:text-white">
                      {formatBytes(peer.transfer_rx)}
                    </span>
                    <span className="text-gray-500">TX</span>
                    <span className="text-gray-900 dark:text-white">
                      {formatBytes(peer.transfer_tx)}
                    </span>
                    <span className="text-gray-500">Allowed IPs</span>
                    <span className="text-gray-900 dark:text-white">{peer.allowed_ips}</span>
                  </div>
                ))}
              </div>
            )}
            {wgStatus?.connected && !wgLiveStatus && (
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
            {config && config.peers && config.peers.length > 0 && (
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
            )}

            {/* Profiles */}
            {profiles.length > 0 && (
              <div>
                <h4 className="mb-2 text-sm font-medium text-gray-700 dark:text-gray-300">
                  Profiles ({profiles.length})
                </h4>
                <ul className="space-y-2" role="list" aria-label="WireGuard profiles">
                  {profiles.map((profile) => (
                    <li
                      key={profile.id}
                      className={`flex items-center justify-between rounded-md border p-3 text-sm ${
                        profile.active
                          ? 'border-blue-500 bg-blue-50 dark:border-blue-400 dark:bg-blue-950'
                          : 'border-gray-200 dark:border-gray-700'
                      }`}
                    >
                      <div className="flex items-center gap-2">
                        <span className="font-medium text-gray-900 dark:text-white">
                          {profile.name}
                        </span>
                        {profile.active && <Badge variant="success">Active</Badge>}
                      </div>
                      <div className="flex items-center gap-1">
                        {!profile.active && (
                          <Button
                            size="sm"
                            variant="ghost"
                            onClick={() => activateProfileMutation.mutate(profile.id)}
                            disabled={activateProfileMutation.isPending}
                            title="Activate profile"
                          >
                            <Play className="h-4 w-4" />
                          </Button>
                        )}
                        <Button
                          size="sm"
                          variant="ghost"
                          onClick={() => deleteProfileMutation.mutate(profile.id)}
                          disabled={deleteProfileMutation.isPending}
                          title="Delete profile"
                        >
                          <Trash2 className="h-4 w-4 text-red-500" />
                        </Button>
                      </div>
                    </li>
                  ))}
                </ul>
              </div>
            )}

            {/* Kill Switch */}
            <div className="rounded-md border border-gray-200 p-3 dark:border-gray-700">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <ShieldAlert className="h-4 w-4 text-orange-500" />
                  <span className="text-sm font-medium text-gray-700 dark:text-gray-300">
                    Kill Switch
                  </span>
                </div>
                <Switch
                  id="killswitch-toggle"
                  checked={killSwitch?.enabled ?? false}
                  onChange={() => killSwitchMutation.mutate(!(killSwitch?.enabled ?? false))}
                  disabled={killSwitchMutation.isPending}
                />
              </div>
              <p className="mt-1 text-xs text-gray-500">
                {killSwitch?.enabled
                  ? 'All traffic is blocked if VPN disconnects. Disable to allow direct internet access.'
                  : 'When enabled, blocks all internet traffic if the VPN connection drops to prevent IP leaks.'}
              </p>
            </div>

            {/* Import Config as Profile */}
            <div>
              <h4 className="mb-2 text-sm font-medium text-gray-700 dark:text-gray-300">
                <div className="flex items-center gap-1">
                  <Upload className="h-4 w-4" />
                  Import Profile
                </div>
              </h4>
              <input
                type="text"
                className="w-full rounded-md border border-gray-300 bg-white p-2 text-sm dark:border-gray-700 dark:bg-gray-900 dark:text-white mb-2"
                placeholder="Profile name (e.g. Home VPN, Travel, Work)"
                value={profileName}
                onChange={(e) => setProfileName(e.target.value)}
              />
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
                disabled={!configText.trim() || !profileName.trim() || addProfileMutation.isPending}
                onClick={handleAddProfile}
              >
                <Plus className="h-4 w-4 mr-1" />
                {addProfileMutation.isPending ? 'Saving...' : 'Save Profile'}
              </Button>
            </div>
          </>
        )}
      </CardContent>
    </Card>
  );
}
