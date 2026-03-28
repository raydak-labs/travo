import { Zap, Ban, ShieldOff, BookmarkPlus } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { ClientAliasCell } from './client-alias-cell';
import {
  useKickClient,
  useBlockClient,
  useUnblockClient,
} from '@/hooks/use-network';
import { formatBytes } from '@/lib/utils';
import type { Client } from '@shared/index';

export type ClientRowProps = {
  client: Client;
  isBlocked: boolean;
  hasReservation: boolean;
  onReserveIP: (client: Client) => void;
};

export function ClientRow({ client, isBlocked, hasReservation, onReserveIP }: ClientRowProps) {
  const kick = useKickClient();
  const block = useBlockClient();
  const unblock = useUnblockClient();

  const connectedSince =
    client.connected_since && !isNaN(new Date(client.connected_since).getTime())
      ? new Date(client.connected_since).toLocaleString()
      : null;

  return (
    <tr className="group border-b border-gray-100 dark:border-white/[0.08] last:border-0">
      <td className="py-3 pr-4">
        <div className="group/alias">
          <ClientAliasCell
            client={client}
            inputClassName="h-7 w-36 text-sm"
            placeholder="Device alias"
            displayNameClassName="font-medium text-gray-900 dark:text-white"
            editButtonClassName="opacity-0 group-hover/alias:opacity-100"
          />
        </div>
        <div className="mt-0.5 text-xs text-gray-400">{client.mac_address}</div>
      </td>
      <td className="py-3 pr-4 text-sm text-gray-700 dark:text-gray-300">
        <div className="flex items-center gap-1.5">
          {client.ip_address}
          {hasReservation && (
            <Badge variant="outline" className="px-1 py-0 text-xs">
              static
            </Badge>
          )}
        </div>
      </td>
      <td className="hidden py-3 pr-4 text-sm text-gray-500 md:table-cell">
        {client.interface_name}
      </td>
      <td className="hidden py-3 pr-4 text-sm text-gray-500 lg:table-cell">
        {connectedSince ?? '—'}
      </td>
      <td className="py-3 pr-4 text-sm text-gray-500">
        <div className="text-xs">
          <span className="text-blue-600 dark:text-blue-400">↓ {formatBytes(client.rx_bytes)}</span>
          {' / '}
          <span className="text-orange-600 dark:text-orange-400">↑ {formatBytes(client.tx_bytes)}</span>
        </div>
      </td>
      <td className="py-3 text-right">
        <div className="flex items-center justify-end gap-1">
          {isBlocked && (
            <Badge variant="destructive" className="mr-1 text-xs">
              Blocked
            </Badge>
          )}
          {!hasReservation && (
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 text-blue-600"
              title="Reserve this IP (static DHCP)"
              onClick={() => onReserveIP(client)}
            >
              <BookmarkPlus className="h-3.5 w-3.5" />
            </Button>
          )}
          <Button
            variant="ghost"
            size="icon"
            className="h-7 w-7"
            title="Kick (disconnect)"
            onClick={() => kick.mutate(client.mac_address)}
            disabled={kick.isPending}
          >
            <Zap className="h-3.5 w-3.5" />
          </Button>
          {isBlocked ? (
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 text-green-600"
              title="Unblock"
              onClick={() => unblock.mutate(client.mac_address)}
              disabled={unblock.isPending}
            >
              <ShieldOff className="h-3.5 w-3.5" />
            </Button>
          ) : (
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 text-red-600"
              title="Block"
              onClick={() => block.mutate(client.mac_address)}
              disabled={block.isPending}
            >
              <Ban className="h-3.5 w-3.5" />
            </Button>
          )}
        </div>
      </td>
    </tr>
  );
}
