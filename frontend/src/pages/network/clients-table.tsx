import { Zap, Ban, ShieldOff } from 'lucide-react';
import type { Client } from '@shared/index';
import { formatBytes } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { ClientAliasCell } from '@/components/clients/client-alias-cell';
import {
  useKickClient,
  useBlockClient,
  useUnblockClient,
} from '@/hooks/use-network';

interface ClientsTableProps {
  clients: readonly Client[];
  blockedMacs?: readonly string[];
}

export function ClientsTable({ clients, blockedMacs = [] }: ClientsTableProps) {
  const sorted = [...clients].sort((a, b) => {
    const nameA = a.alias || a.hostname || '';
    const nameB = b.alias || b.hostname || '';
    return nameA.localeCompare(nameB);
  });
  const kick = useKickClient();
  const block = useBlockClient();
  const unblock = useUnblockClient();

  const blockedSet = new Set(blockedMacs.map((m) => m.toUpperCase()));

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-gray-200 dark:border-gray-700">
            <th className="pb-2 text-left font-medium text-gray-500">Name</th>
            <th className="pb-2 text-left font-medium text-gray-500">IP</th>
            <th className="hidden pb-2 text-left font-medium text-gray-500 md:table-cell">MAC</th>
            <th className="hidden pb-2 text-left font-medium text-gray-500 lg:table-cell">
              Interface
            </th>
            <th className="hidden pb-2 text-left font-medium text-gray-500 md:table-cell">
              Connected Since
            </th>
            <th className="pb-2 text-right font-medium text-gray-500">Traffic</th>
            <th className="pb-2 text-right font-medium text-gray-500">Actions</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-gray-100 dark:divide-gray-800">
          {sorted.map((client) => {
            const isBlocked = blockedSet.has(client.mac_address.toUpperCase());
            return (
              <tr key={client.mac_address} className="group">
                <td className="py-2">
                  <ClientAliasCell client={client} />
                </td>
                <td className="py-2 text-gray-600 dark:text-gray-400">{client.ip_address}</td>
                <td className="hidden py-2 text-gray-600 dark:text-gray-400 md:table-cell">
                  {client.mac_address}
                </td>
                <td className="hidden py-2 text-gray-600 dark:text-gray-400 lg:table-cell">
                  {client.interface_name}
                </td>
                <td className="hidden py-2 text-gray-600 dark:text-gray-400 md:table-cell">
                  {client.connected_since && !isNaN(new Date(client.connected_since).getTime())
                    ? new Date(client.connected_since).toLocaleString()
                    : 'Unknown'}
                </td>
                <td className="py-2 text-right text-gray-600 dark:text-gray-400">
                  ↓ {formatBytes(client.rx_bytes)} / ↑ {formatBytes(client.tx_bytes)}
                </td>
                <td className="py-2 text-right">
                  <div className="flex items-center justify-end gap-1">
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
          })}
        </tbody>
      </table>
    </div>
  );
}
