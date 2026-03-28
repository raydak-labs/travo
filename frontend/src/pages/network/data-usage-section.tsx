import { useState } from 'react';
import { BarChart2, RefreshCw, Settings } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { EmptyState } from '@/components/ui/empty-state';
import { Button } from '@/components/ui/button';
import { useDataUsage, useDataBudget, useSetDataBudget } from '@/hooks/use-data-usage';
import type { DataBudget } from '@shared/index';
import { DataUsageInterfaceCard } from './data-usage-interface-card';
import { DataUsageBudgetEditor } from './data-usage-budget-editor';

export function DataUsageSection() {
  const { data: status, isLoading, refetch } = useDataUsage();
  const { data: budgetConfig } = useDataBudget();
  const setBudgetMutation = useSetDataBudget();
  const [editingBudget, setEditingBudget] = useState<string | null>(null);

  if (isLoading) return null;

  if (!status?.available) {
    return (
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Data Usage</CardTitle>
          <BarChart2 className="h-4 w-4 text-gray-500" />
        </CardHeader>
        <CardContent>
          <p className="text-sm text-gray-500">
            Install the <strong>Data Usage (vnstat)</strong> service to track cumulative traffic per
            interface with budget warnings.
          </p>
        </CardContent>
      </Card>
    );
  }

  const findBudget = (name: string) => budgetConfig?.budgets.find((b) => b.interface === name);

  const handleBudgetSave = (budget: DataBudget) => {
    const existing = budgetConfig?.budgets ?? [];
    const updated = existing.filter((b) => b.interface !== budget.interface);
    void setBudgetMutation.mutateAsync({ budgets: [...updated, budget] });
    setEditingBudget(null);
  };

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">Data Usage</CardTitle>
        <div className="flex items-center gap-1">
          <Button variant="ghost" size="sm" className="h-6 w-6 p-0" onClick={() => void refetch()}>
            <RefreshCw className="h-3.5 w-3.5" />
          </Button>
          <BarChart2 className="h-4 w-4 text-gray-500" />
        </div>
      </CardHeader>
      <CardContent className="space-y-3">
        {status.interfaces.length === 0 ? (
          <EmptyState message="No interfaces monitored yet" />
        ) : (
          status.interfaces.map((iface) => (
            <div key={iface.name} className="space-y-2">
              <DataUsageInterfaceCard iface={iface} budget={findBudget(iface.name)} />
              {editingBudget === iface.name ? (
                <DataUsageBudgetEditor
                  ifaceName={iface.name}
                  current={findBudget(iface.name)}
                  onSave={handleBudgetSave}
                />
              ) : (
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-6 gap-1 text-xs text-gray-500"
                  onClick={() => setEditingBudget(iface.name)}
                >
                  <Settings className="h-3 w-3" />
                  {findBudget(iface.name) ? 'Edit budget' : 'Set budget'}
                </Button>
              )}
            </div>
          ))
        )}
      </CardContent>
    </Card>
  );
}
