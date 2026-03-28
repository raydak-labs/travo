import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import type { TailscalePeer } from '@shared/index';

export type TailscalePeerRowProps = {
  peer: TailscalePeer;
  onSetExitNode: (ip: string) => void;
  isPending: boolean;
};

export function TailscalePeerRow({ peer, onSetExitNode, isPending }: TailscalePeerRowProps) {
  return (
    <div className="flex items-center justify-between py-1.5">
      <div className="flex min-w-0 items-center gap-2">
        <span
          className={`h-2 w-2 shrink-0 rounded-full ${peer.online ? 'bg-green-500' : 'bg-gray-300'}`}
        />
        <div className="min-w-0">
          <span className="truncate text-sm font-medium">{peer.hostname}</span>
          <span className="ml-2 font-mono text-xs text-gray-500">{peer.tailscale_ip}</span>
        </div>
        {peer.exit_node && (
          <Badge className="shrink-0 bg-blue-100 text-xs text-blue-800 dark:bg-blue-900 dark:text-blue-200">
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
          className="ml-2 shrink-0"
        >
          Use as exit
        </Button>
      )}
    </div>
  );
}
