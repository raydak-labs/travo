import { Wifi, Users } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import type { TailscaleStatus } from '@shared/index';
import { TailscalePeerRow } from './tailscale-peer-row';

type TailscaleLoggedInPanelProps = {
  status: TailscaleStatus;
  wireguardEnabled: boolean;
  sshEnabled: boolean;
  sshPending: boolean;
  onSshToggle: () => void;
  onClearExitNode: () => void;
  onSetExitNode: (ip: string) => void;
  exitNodePending: boolean;
};

export function TailscaleLoggedInPanel({
  status,
  wireguardEnabled,
  sshEnabled,
  sshPending,
  onSshToggle,
  onClearExitNode,
  onSetExitNode,
  exitNodePending,
}: TailscaleLoggedInPanelProps) {
  return (
    <>
      <div className="space-y-1 rounded-md bg-gray-50 p-3 text-sm dark:bg-gray-900">
        <div className="grid grid-cols-2 gap-x-4 gap-y-1">
          <span className="text-gray-500">IP Address</span>
          <span className="font-mono text-gray-900 dark:text-white">{status.ip_address}</span>
          <span className="text-gray-500">Hostname</span>
          <span className="text-gray-900 dark:text-white">{status.hostname}</span>
        </div>
        {status.exit_node && (
          <div className="mt-2 flex items-center gap-2 border-t border-gray-200 pt-2 dark:border-gray-700">
            <Wifi className="h-3 w-3 text-blue-500" />
            <span className="text-xs text-gray-500">Exit node:</span>
            <span className="font-mono text-xs text-gray-900 dark:text-white">
              {status.exit_node}
            </span>
            {status.exit_node_active && (
              <Badge className="bg-green-100 text-xs text-green-800 dark:bg-green-900 dark:text-green-200">
                Active
              </Badge>
            )}
            <Button
              variant="ghost"
              size="sm"
              className="h-5 px-1 text-xs text-gray-500"
              onClick={onClearExitNode}
              disabled={exitNodePending}
            >
              Clear
            </Button>
          </div>
        )}
      </div>

      {status.running && (
        <div className="flex items-center justify-between border-t border-gray-100 py-1 dark:border-white/[0.08]">
          <div>
            <span className="text-sm text-gray-700 dark:text-gray-300">Allow Tailscale SSH</span>
            <p className="text-xs text-gray-500">
              Let Tailscale manage SSH access from trusted devices
            </p>
          </div>
          <Switch
            id="tailscale-ssh"
            label="SSH"
            checked={sshEnabled}
            onChange={onSshToggle}
            disabled={sshPending}
          />
        </div>
      )}

      {status.peers && status.peers.length > 0 && (
        <div className="space-y-1">
          {wireguardEnabled && (
            <p className="rounded-md border border-amber-200 bg-amber-50/90 px-3 py-2 text-sm dark:border-amber-800 dark:bg-amber-950/50">
              WireGuard is enabled. Using a Tailscale exit node turns WireGuard off first so only
              one full-tunnel VPN path runs at a time.
            </p>
          )}
          <div className="mb-1 flex items-center gap-1 text-xs text-gray-500">
            <Users className="h-3 w-3" />
            <span>
              {status.peers.length} peer{status.peers.length !== 1 ? 's' : ''}
            </span>
          </div>
          <div className="divide-y divide-gray-100 dark:divide-gray-800">
            {status.peers.map((peer) => (
              <TailscalePeerRow
                key={peer.tailscale_ip}
                peer={peer}
                onSetExitNode={onSetExitNode}
                isPending={exitNodePending}
              />
            ))}
          </div>
        </div>
      )}
    </>
  );
}
