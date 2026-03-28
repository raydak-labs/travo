import { Skeleton } from '@/components/ui/skeleton';
import { ClientRow } from '@/components/clients/client-row';
import type { Client } from '@shared/index';

type ClientsConnectedTableProps = {
  clientsLoading: boolean;
  filtered: Client[];
  hasSearch: boolean;
  blockedSet: Set<string>;
  reservedMacs: Set<string>;
  onReserveIP: (client: Client) => void;
};

export function ClientsConnectedTable({
  clientsLoading,
  filtered,
  hasSearch,
  blockedSet,
  reservedMacs,
  onReserveIP,
}: ClientsConnectedTableProps) {
  if (clientsLoading) {
    return (
      <div className="space-y-2">
        <Skeleton className="h-10 w-full" />
        <Skeleton className="h-10 w-full" />
        <Skeleton className="h-10 w-full" />
      </div>
    );
  }

  if (filtered.length === 0) {
    return (
      <p className="py-4 text-center text-sm text-gray-500">
        {hasSearch ? 'No clients match your search.' : 'No clients connected.'}
      </p>
    );
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-gray-200 text-left dark:border-gray-700">
            <th className="pb-2 font-medium text-gray-500">Device</th>
            <th className="pb-2 font-medium text-gray-500">IP Address</th>
            <th className="hidden pb-2 font-medium text-gray-500 md:table-cell">Interface</th>
            <th className="hidden pb-2 font-medium text-gray-500 lg:table-cell">Connected Since</th>
            <th className="pb-2 font-medium text-gray-500">Traffic</th>
            <th className="pb-2 text-right font-medium text-gray-500">Actions</th>
          </tr>
        </thead>
        <tbody>
          {filtered.map((client) => (
            <ClientRow
              key={client.mac_address}
              client={client}
              isBlocked={blockedSet.has(client.mac_address.toUpperCase())}
              hasReservation={reservedMacs.has(client.mac_address.toUpperCase())}
              onReserveIP={onReserveIP}
            />
          ))}
        </tbody>
      </table>
    </div>
  );
}
