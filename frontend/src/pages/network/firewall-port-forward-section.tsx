import { Skeleton } from '@/components/ui/skeleton';
import type { AddPortForwardRequest, PortForwardRule } from '@shared/index';
import { FirewallPortForwardRulesTable } from './firewall-port-forward-rules-table';
import { FirewallPortForwardAddForm } from './firewall-port-forward-add-form';

type FirewallPortForwardSectionProps = {
  rules: PortForwardRule[] | undefined;
  rulesLoading: boolean;
  addRule: {
    mutate: (payload: AddPortForwardRequest, opts?: { onSuccess?: () => void }) => void;
    isPending: boolean;
  };
  deleteRule: { mutate: (id: string) => void; isPending: boolean };
};

export function FirewallPortForwardSection({
  rules,
  rulesLoading,
  addRule,
  deleteRule,
}: FirewallPortForwardSectionProps) {
  return (
    <div>
      <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-gray-500">
        Port Forwarding
      </h3>
      {rulesLoading ? (
        <div className="space-y-2">
          <Skeleton className="h-8 w-full" />
          <Skeleton className="h-8 w-full" />
        </div>
      ) : (
        <div className="space-y-4">
          {rules && rules.length > 0 && (
            <FirewallPortForwardRulesTable rules={rules} deleteRule={deleteRule} />
          )}
          <FirewallPortForwardAddForm addRule={addRule} />
        </div>
      )}
    </div>
  );
}
