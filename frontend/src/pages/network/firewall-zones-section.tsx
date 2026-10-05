import type { UseQueryResult } from '@tanstack/react-query';
import { Skeleton } from '@/components/ui/skeleton';
import { QueryCard } from '@/components/ui/query-card';
import { EmptyState } from '@/components/ui/empty-state';
import type { FirewallZone } from '@shared/index';
import { FirewallPolicyBadge } from './firewall-policy-badge';
import { Badge } from '@/components/ui/badge';

type FirewallZonesSectionProps = {
  /** The whole query: an empty zone list may only be claimed by a request that answered. */
  query: UseQueryResult<FirewallZone[], Error>;
};

export function FirewallZonesSection({ query }: FirewallZonesSectionProps) {
  const { data: zones, isLoading, isError, error, refetch } = query;
  return (
    <div>
      <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
        Firewall Zones
      </h3>
      <QueryCard
        isLoading={isLoading}
        isError={isError}
        error={error}
        onRetry={() => void refetch()}
        loading={
          <div className="space-y-2">
            <Skeleton className="h-8 w-full" />
            <Skeleton className="h-8 w-full" />
          </div>
        }
      >
        {zones && zones.length > 0 ? (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <caption className="sr-only">Firewall zones</caption>
              <thead>
                <tr className="border-b text-left text-gray-500 dark:text-gray-400">
                  <th scope="col" className="pb-2 font-medium">
                    Zone
                  </th>
                  <th scope="col" className="pb-2 font-medium">
                    Networks
                  </th>
                  <th scope="col" className="pb-2 font-medium">
                    Input
                  </th>
                  <th scope="col" className="pb-2 font-medium">
                    Output
                  </th>
                  <th scope="col" className="pb-2 font-medium">
                    Forward
                  </th>
                  <th scope="col" className="pb-2 font-medium">
                    Masq
                  </th>
                </tr>
              </thead>
              <tbody>
                {zones.map((zone) => (
                  <tr key={zone.name} className="border-b last:border-0">
                    <td className="py-2 font-medium text-gray-900 dark:text-white">{zone.name}</td>
                    <td className="py-2 text-gray-500 dark:text-gray-400">
                      {zone.network && zone.network.length > 0 ? zone.network.join(', ') : '—'}
                    </td>
                    <td className="py-2">
                      <FirewallPolicyBadge policy={zone.input} />
                    </td>
                    <td className="py-2">
                      <FirewallPolicyBadge policy={zone.output} />
                    </td>
                    <td className="py-2">
                      <FirewallPolicyBadge policy={zone.forward} />
                    </td>
                    <td className="py-2">
                      <Badge variant="secondary">—</Badge>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <EmptyState message="No firewall zones found" />
        )}
      </QueryCard>
    </div>
  );
}
