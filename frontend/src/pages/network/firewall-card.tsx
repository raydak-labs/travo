import { Shield } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import {
  useFirewallZones,
  usePortForwards,
  useAddPortForward,
  useDeletePortForward,
} from '@/hooks/use-network';
import { FirewallZonesSection } from './firewall-zones-section';
import { FirewallPortForwardSection } from './firewall-port-forward-section';

export function FirewallCard() {
  const { data: zones, isLoading: zonesLoading } = useFirewallZones();
  const { data: rules, isLoading: rulesLoading } = usePortForwards();
  const addRule = useAddPortForward();
  const deleteRule = useDeletePortForward();

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">Firewall</CardTitle>
        <Shield className="h-4 w-4 text-gray-500" />
      </CardHeader>
      <CardContent className="space-y-6">
        <FirewallZonesSection zones={zones} zonesLoading={zonesLoading} />
        <FirewallPortForwardSection
          rules={rules}
          rulesLoading={rulesLoading}
          addRule={addRule}
          deleteRule={deleteRule}
        />
      </CardContent>
    </Card>
  );
}
