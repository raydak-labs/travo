import { Skeleton } from '@/components/ui/skeleton';
import { EmptyState } from '@/components/ui/empty-state';
import { QueryCard } from '@/components/ui/query-card';
import { ClientRow } from '@/components/clients/client-row';
import type { Client } from '@shared/index';

type ClientsConnectedTableProps = {
  clientsLoading: boolean;
  clientsError: boolean;
  clientsErrorDetail: unknown;
  clientsRefetch: () => void;
  filtered: Client[];
  hasSearch: boolean;
  blockedSet: Set<string>;
  reservedMacs: Set<string>;
  onReserveIP: (client: Client) => void;
};

export function ClientsConnectedTable({
  clientsLoading,
  clientsError,
  clientsErrorDetail,
  clientsRefetch,
  filtered,
  hasSearch,
  blockedSet,
  reservedMacs,
  onReserveIP,
}: ClientsConnectedTableProps) {
  const skeleton = (
    <div className="space-y-2">
      <Skeleton className="h-10 w-full" />
      <Skeleton className="h-10 w-full" />
      <Skeleton className="h-10 w-full" />
    </div>
  );

  // A failed fetch must not read as "no clients connected" — that is a
  // confident claim about the router, and on a flaky link it is wrong.
  return (
    <QueryCard
      isLoading={clientsLoading}
      isError={clientsError}
      error={clientsErrorDetail}
      onRetry={clientsRefetch}
      loading={skeleton}
    >
      {filtered.length === 0 ? (
        <EmptyState
          message={hasSearch ? 'No clients match your search.' : 'No clients connected.'}
        />
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <caption className="sr-only">Currently connected clients</caption>
            <thead>
              <tr className="border-b border-gray-200 text-left dark:border-gray-700">
                <th scope="col" className="pb-2 font-medium text-gray-500 dark:text-gray-400">
                  Device
                </th>
                <th scope="col" className="pb-2 font-medium text-gray-500 dark:text-gray-400">
                  IP Address
                </th>
                <th
                  scope="col"
                  className="hidden pb-2 font-medium text-gray-500 dark:text-gray-400 md:table-cell"
                >
                  Interface
                </th>
                <th
                  scope="col"
                  className="hidden pb-2 font-medium text-gray-500 dark:text-gray-400 lg:table-cell"
                >
                  Connected Since
                </th>
                <th
                  scope="col"
                  className="hidden pb-2 font-medium text-gray-500 dark:text-gray-400 sm:table-cell"
                >
                  Traffic
                </th>
                <th
                  scope="col"
                  className="pb-2 text-right font-medium text-gray-500 dark:text-gray-400"
                >
                  Actions
                </th>
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
      )}
    </QueryCard>
  );
}
