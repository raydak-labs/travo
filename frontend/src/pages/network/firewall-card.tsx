import { Shield } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import type { UseQueryResult } from '@tanstack/react-query';
import type { FirewallZone, PortForwardRule } from '@shared/index';
import {
  useFirewallZones,
  usePortForwards,
  useAddPortForward,
  useDeletePortForward,
} from '@/hooks/use-network';
import { FirewallZonesSection } from './firewall-zones-section';
import { FirewallPortForwardSection } from './firewall-port-forward-section';

type ZonesQuery = UseQueryResult<FirewallZone[], Error>;
type RulesQuery = UseQueryResult<PortForwardRule[], Error>;

export function FirewallCard() {
  const zonesQuery: ZonesQuery = useFirewallZones();
  const rulesQuery: RulesQuery = usePortForwards();
  const addRule = useAddPortForward();
  const deleteRule = useDeletePortForward();

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle>Firewall</CardTitle>
        <Shield className="h-4 w-4 text-gray-500 dark:text-gray-400" />
      </CardHeader>
      <CardContent className="space-y-6">
        <FirewallZonesSection query={zonesQuery} />
        <FirewallPortForwardSection query={rulesQuery} addRule={addRule} deleteRule={deleteRule} />
      </CardContent>
    </Card>
  );
}
