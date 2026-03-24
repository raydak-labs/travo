import { useState } from 'react';
import { BarChart2, RefreshCw, Settings, Trash2 } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { useDataUsage, useDataBudget, useSetDataBudget, useResetDataUsage } from '@/hooks/use-data-usage';
import type { DataBudget, DataUsageInterface } from '@shared/index';

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
}

function UsageBar({ used, limit, label }: { used: number; limit: number; label: string }) {
  const pct = limit > 0 ? Math.min((used / limit) * 100, 100) : 0;
  const color = pct >= 90 ? 'bg-red-500' : pct >= 80 ? 'bg-yellow-500' : 'bg-blue-500';
  return (
    <div className="space-y-1">
      <div className="flex justify-between text-xs text-gray-500">
        <span>{label}</span>
        <span>{formatBytes(used)} / {formatBytes(limit)} ({pct.toFixed(0)}%)</span>
      </div>
      <div className="h-2 w-full rounded-full bg-gray-200 dark:bg-gray-700">
        <div className={`h-2 rounded-full transition-all ${color}`} style={{ width: `${pct}%` }} />
      </div>
    </div>
  );
}

function InterfaceCard({ iface, budget }: { iface: DataUsageInterface; budget?: DataBudget }) {
  const resetMutation = useResetDataUsage();

  return (
    <div className="rounded-md border p-3 space-y-2">
      <div className="flex items-center justify-between">
        <span className="font-medium text-sm">{iface.label}</span>
        <div className="flex items-center gap-2">
          <Badge variant="outline" className="text-xs font-mono">{iface.name}</Badge>
          <Button
            variant="ghost"
            size="sm"
            className="h-6 w-6 p-0 text-gray-400 hover:text-red-500"
            onClick={() => resetMutation.mutate(iface.name)}
            disabled={resetMutation.isPending}
            title="Reset counters"
          >
            <Trash2 className="h-3.5 w-3.5" />
          </Button>
        </div>
      </div>

      <div className="grid grid-cols-3 gap-3 text-sm">
        <div>
          <p className="text-xs text-gray-500 mb-0.5">Today</p>
          <p className="font-mono">{formatBytes(iface.today.rx_bytes + iface.today.tx_bytes)}</p>
          <p className="text-xs text-gray-400">↓{formatBytes(iface.today.rx_bytes)} ↑{formatBytes(iface.today.tx_bytes)}</p>
        </div>
        <div>
          <p className="text-xs text-gray-500 mb-0.5">This Month</p>
          <p className="font-mono">{formatBytes(iface.month.rx_bytes + iface.month.tx_bytes)}</p>
          <p className="text-xs text-gray-400">↓{formatBytes(iface.month.rx_bytes)} ↑{formatBytes(iface.month.tx_bytes)}</p>
        </div>
        <div>
          <p className="text-xs text-gray-500 mb-0.5">Total</p>
          <p className="font-mono">{formatBytes(iface.total.rx_bytes + iface.total.tx_bytes)}</p>
          <p className="text-xs text-gray-400">↓{formatBytes(iface.total.rx_bytes)} ↑{formatBytes(iface.total.tx_bytes)}</p>
        </div>
      </div>

      {budget && budget.monthly_limit_bytes > 0 && (
        <UsageBar
          used={iface.month.rx_bytes + iface.month.tx_bytes}
          limit={budget.monthly_limit_bytes}
          label="Monthly budget"
        />
      )}
    </div>
  );
}

function BudgetEditor({
  ifaceName,
  current,
  onSave,
}: {
  ifaceName: string;
  current?: DataBudget;
  onSave: (b: DataBudget) => void;
}) {
  const [limitGB, setLimitGB] = useState(
    current ? String(Math.round(current.monthly_limit_bytes / 1e9)) : '',
  );
  const [warnPct, setWarnPct] = useState(current ? String(current.warning_threshold_pct) : '80');

  const handleSave = () => {
    const limitBytes = parseFloat(limitGB) * 1e9;
    const pct = parseFloat(warnPct);
    if (isNaN(limitBytes) || limitBytes <= 0 || isNaN(pct)) return;
    onSave({ interface: ifaceName, monthly_limit_bytes: limitBytes, warning_threshold_pct: pct, reset_day: 1 });
  };

  return (
    <div className="flex items-center gap-2 flex-wrap">
      <Input
        className="h-7 w-24 text-sm"
        placeholder="Limit (GB)"
        value={limitGB}
        onChange={(e) => setLimitGB(e.target.value)}
      />
      <span className="text-xs text-gray-500">GB/month, warn at</span>
      <Input
        className="h-7 w-16 text-sm"
        placeholder="80"
        value={warnPct}
        onChange={(e) => setWarnPct(e.target.value)}
      />
      <span className="text-xs text-gray-500">%</span>
      <Button size="sm" className="h-7 text-xs" onClick={handleSave}>
        Save
      </Button>
    </div>
  );
}

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

  const findBudget = (name: string) =>
    budgetConfig?.budgets.find((b) => b.interface === name);

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
          <p className="text-sm text-gray-500">No interfaces monitored yet.</p>
        ) : (
          status.interfaces.map((iface) => (
            <div key={iface.name} className="space-y-2">
              <InterfaceCard iface={iface} budget={findBudget(iface.name)} />
              {editingBudget === iface.name ? (
                <BudgetEditor
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
