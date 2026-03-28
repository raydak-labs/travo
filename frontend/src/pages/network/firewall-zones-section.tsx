import { Skeleton } from '@/components/ui/skeleton';
import { EmptyState } from '@/components/ui/empty-state';
import type { FirewallZone } from '@shared/index';
import { FirewallPolicyBadge } from './firewall-policy-badge';
import { Badge } from '@/components/ui/badge';

type FirewallZonesSectionProps = {
  zones: FirewallZone[] | undefined;
  zonesLoading: boolean;
};

export function FirewallZonesSection({ zones, zonesLoading }: FirewallZonesSectionProps) {
  return (
    <div>
      <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-gray-500">
        Firewall Zones
      </h3>
      {zonesLoading ? (
        <div className="space-y-2">
          <Skeleton className="h-8 w-full" />
          <Skeleton className="h-8 w-full" />
        </div>
      ) : zones && zones.length > 0 ? (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b text-left text-gray-500">
                <th className="pb-2 font-medium">Zone</th>
                <th className="pb-2 font-medium">Networks</th>
                <th className="pb-2 font-medium">Input</th>
                <th className="pb-2 font-medium">Output</th>
                <th className="pb-2 font-medium">Forward</th>
                <th className="pb-2 font-medium">Masq</th>
              </tr>
            </thead>
            <tbody>
              {zones.map((zone) => (
                <tr key={zone.name} className="border-b last:border-0">
                  <td className="py-2 font-medium text-gray-900 dark:text-white">{zone.name}</td>
                  <td className="py-2 text-gray-500">
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
    </div>
  );
}
