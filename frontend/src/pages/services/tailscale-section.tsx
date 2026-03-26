import { useState } from 'react';
import { Globe, ExternalLink, Users, Wifi } from 'lucide-react';
import { Link } from '@tanstack/react-router';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { Skeleton } from '@/components/ui/skeleton';
import { useServices } from '@/hooks/use-services';
import {
  useTailscaleStatus,
  useToggleTailscale,
  useTailscaleAuth,
  useSetTailscaleExitNode,
  useTailscaleSSH,
  useSetTailscaleSSH,
} from '@/hooks/use-vpn';
import type { TailscalePeer } from '@shared/index';

function PeerRow({
  peer,
  onSetExitNode,
  isPending,
}: {
  peer: TailscalePeer;
  onSetExitNode: (ip: string) => void;
  isPending: boolean;
}) {
  return (
    <div className="flex items-center justify-between py-1.5">
      <div className="flex items-center gap-2 min-w-0">
        <span
          className={`h-2 w-2 shrink-0 rounded-full ${peer.online ? 'bg-green-500' : 'bg-gray-300'}`}
        />
        <div className="min-w-0">
          <span className="text-sm font-medium truncate">{peer.hostname}</span>
          <span className="ml-2 text-xs text-gray-500 font-mono">{peer.tailscale_ip}</span>
        </div>
        {peer.exit_node && (
          <Badge className="bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200 text-xs shrink-0">
            Exit Node
          </Badge>
        )}
      </div>
      {peer.exit_node_option && !peer.exit_node && (
        <Button
          variant="outline"
          size="sm"
          onClick={() => onSetExitNode(peer.tailscale_ip)}
          disabled={isPending || !peer.online}
          className="shrink-0 ml-2"
        >
          Use as exit
        </Button>
      )}
    </div>
  );
}

function AuthSection() {
  const [authKey, setAuthKey] = useState('');
  const authMutation = useTailscaleAuth();

  return (
    <div className="space-y-3">
      <p className="text-sm text-gray-600 dark:text-gray-400">
        Not authenticated. Enter a pre-auth key or start interactive login.
      </p>
      <div className="flex gap-2">
        <Input
          placeholder="tskey-auth-... (optional)"
          value={authKey}
          onChange={(e) => setAuthKey(e.target.value)}
          className="text-sm font-mono"
        />
        <Button size="sm" onClick={() => authMutation.mutate(authKey || undefined)} disabled={authMutation.isPending}>
          Authenticate
        </Button>
      </div>
      {authMutation.data?.auth_url && (
        <a
          href={authMutation.data.auth_url}
          target="_blank"
          rel="noopener noreferrer"
          className="flex items-center gap-1 text-sm text-blue-600 hover:underline dark:text-blue-400"
        >
          Open auth URL <ExternalLink className="h-3 w-3" />
        </a>
      )}
    </div>
  );
}

export function TailscaleSection() {
  const { data: status, isLoading } = useTailscaleStatus();
  const { data: services = [] } = useServices();
  const toggleMutation = useToggleTailscale();
  const exitNodeMutation = useSetTailscaleExitNode();
  const { data: sshStatus } = useTailscaleSSH();
  const sshMutation = useSetTailscaleSSH();

  const tsService = services.find((s) => s.id === 'tailscale');
  const isInstalled = tsService ? tsService.state !== 'not_installed' : !!status;

  return (
    <Card className={!isInstalled ? 'opacity-60' : undefined}>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">Tailscale</CardTitle>
        <Globe className="h-4 w-4 text-blue-500" />
      </CardHeader>
      <CardContent className="space-y-4">
        {!isInstalled ? (
          <div className="text-center py-4">
            <p className="text-sm text-gray-500 mb-2">Tailscale is not installed</p>
            <Link to="/services" className="text-sm text-blue-600 hover:underline dark:text-blue-400">
              Install via Services →
            </Link>
          </div>
        ) : isLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-4 w-3/4" />
            <Skeleton className="h-4 w-1/2" />
          </div>
        ) : status ? (
          <>
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <span className="text-sm text-gray-700 dark:text-gray-300">Status</span>
                <Badge variant={status.logged_in ? 'success' : 'outline'}>
                  {status.logged_in ? 'Logged In' : 'Logged Out'}
                </Badge>
                {status.running && <Badge variant="success">Running</Badge>}
              </div>
              <Switch
                id="tailscale-toggle"
                label="Enable"
                checked={status.running}
                onChange={() => toggleMutation.mutate(!status.running)}
                disabled={toggleMutation.isPending}
              />
            </div>

            {!status.logged_in && status.running && (
              status.auth_url ? (
                <a
                  href={status.auth_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="flex items-center gap-1 text-sm text-blue-600 hover:underline dark:text-blue-400"
                >
                  Complete login at Tailscale <ExternalLink className="h-3 w-3" />
                </a>
              ) : (
                <AuthSection />
              )
            )}

            {status.logged_in && (
              <div className="rounded-md bg-gray-50 p-3 text-sm dark:bg-gray-900 space-y-1">
                <div className="grid grid-cols-2 gap-x-4 gap-y-1">
                  <span className="text-gray-500">IP Address</span>
                  <span className="font-mono text-gray-900 dark:text-white">{status.ip_address}</span>
                  <span className="text-gray-500">Hostname</span>
                  <span className="text-gray-900 dark:text-white">{status.hostname}</span>
                </div>
                {status.exit_node && (
                  <div className="flex items-center gap-2 mt-2 pt-2 border-t border-gray-200 dark:border-gray-700">
                    <Wifi className="h-3 w-3 text-blue-500" />
                    <span className="text-gray-500 text-xs">Exit node:</span>
                    <span className="font-mono text-xs text-gray-900 dark:text-white">{status.exit_node}</span>
                    {status.exit_node_active && (
                      <Badge className="bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200 text-xs">
                        Active
                      </Badge>
                    )}
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-5 px-1 text-xs text-gray-500"
                      onClick={() => exitNodeMutation.mutate('')}
                      disabled={exitNodeMutation.isPending}
                    >
                      Clear
                    </Button>
                  </div>
                )}
              </div>
            )}

            {status.logged_in && status.running && (
              <div className="flex items-center justify-between py-1 border-t border-gray-100 dark:border-gray-800">
                <div>
                  <span className="text-sm text-gray-700 dark:text-gray-300">Allow Tailscale SSH</span>
                  <p className="text-xs text-gray-500">Let Tailscale manage SSH access from trusted devices</p>
                </div>
                <Switch
                  id="tailscale-ssh"
                  label="SSH"
                  checked={sshStatus?.enabled ?? false}
                  onChange={() => sshMutation.mutate(!(sshStatus?.enabled ?? false))}
                  disabled={sshMutation.isPending}
                />
              </div>
            )}

            {status.logged_in && status.peers && status.peers.length > 0 && (
              <div className="space-y-1">
                <div className="flex items-center gap-1 text-xs text-gray-500 mb-1">
                  <Users className="h-3 w-3" />
                  <span>
                    {status.peers.length} peer{status.peers.length !== 1 ? 's' : ''}
                  </span>
                </div>
                <div className="divide-y divide-gray-100 dark:divide-gray-800">
                  {status.peers.map((peer) => (
                    <PeerRow
                      key={peer.tailscale_ip}
                      peer={peer}
                      onSetExitNode={(ip) => exitNodeMutation.mutate(ip)}
                      isPending={exitNodeMutation.isPending}
                    />
                  ))}
                </div>
              </div>
            )}
          </>
        ) : null}
      </CardContent>
    </Card>
  );
}

