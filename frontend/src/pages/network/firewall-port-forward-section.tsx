import type { UseQueryResult } from '@tanstack/react-query';
import { Skeleton } from '@/components/ui/skeleton';
import { QueryCard } from '@/components/ui/query-card';
import type { AddPortForwardRequest, PortForwardRule } from '@shared/index';
import { FirewallPortForwardRulesTable } from './firewall-port-forward-rules-table';
import { FirewallPortForwardAddForm } from './firewall-port-forward-add-form';

type FirewallPortForwardSectionProps = {
  /** The whole query: an empty rule list may only be claimed by a request that answered. */
  query: UseQueryResult<PortForwardRule[], Error>;
  addRule: {
    mutate: (payload: AddPortForwardRequest, opts?: { onSuccess?: () => void }) => void;
    isPending: boolean;
  };
  deleteRule: { mutate: (id: string) => void; isPending: boolean };
};

export function FirewallPortForwardSection({
  query,
  addRule,
  deleteRule,
}: FirewallPortForwardSectionProps) {
  const { data: rules, isLoading, isError, error, refetch } = query;
  return (
    <div>
      <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
        Port Forwarding
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
        <div className="space-y-4">
          {rules && rules.length > 0 && (
            <FirewallPortForwardRulesTable rules={rules} deleteRule={deleteRule} />
          )}
          <FirewallPortForwardAddForm addRule={addRule} />
        </div>
      </QueryCard>
    </div>
  );
}
