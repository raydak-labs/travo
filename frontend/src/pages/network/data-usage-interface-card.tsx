import { Trash2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { useResetDataUsage } from '@/hooks/use-data-usage';
import type { DataBudget, DataUsageInterface } from '@shared/index';
import { formatBytes } from '@/lib/utils';
import { DataUsageUsageBar } from './data-usage-usage-bar';

type DataUsageInterfaceCardProps = {
  iface: DataUsageInterface;
  budget?: DataBudget;
};

export function DataUsageInterfaceCard({ iface, budget }: DataUsageInterfaceCardProps) {
  const resetMutation = useResetDataUsage();

  return (
    <div className="space-y-2 rounded-md border p-3">
      <div className="flex items-center justify-between">
        <span className="text-sm font-medium">{iface.label}</span>
        <div className="flex items-center gap-2">
          <Badge variant="outline" className="font-mono text-xs">
            {iface.name}
          </Badge>
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
          <p className="mb-0.5 text-xs text-gray-500">Today</p>
          <p className="font-mono">{formatBytes(iface.today.rx_bytes + iface.today.tx_bytes)}</p>
          <p className="text-xs text-gray-400">
            ↓{formatBytes(iface.today.rx_bytes)} ↑{formatBytes(iface.today.tx_bytes)}
          </p>
        </div>
        <div>
          <p className="mb-0.5 text-xs text-gray-500">This Month</p>
          <p className="font-mono">{formatBytes(iface.month.rx_bytes + iface.month.tx_bytes)}</p>
          <p className="text-xs text-gray-400">
            ↓{formatBytes(iface.month.rx_bytes)} ↑{formatBytes(iface.month.tx_bytes)}
          </p>
        </div>
        <div>
          <p className="mb-0.5 text-xs text-gray-500">Total</p>
          <p className="font-mono">{formatBytes(iface.total.rx_bytes + iface.total.tx_bytes)}</p>
          <p className="text-xs text-gray-400">
            ↓{formatBytes(iface.total.rx_bytes)} ↑{formatBytes(iface.total.tx_bytes)}
          </p>
        </div>
      </div>

      {budget && budget.monthly_limit_bytes > 0 && (
        <DataUsageUsageBar
          used={iface.month.rx_bytes + iface.month.tx_bytes}
          limit={budget.monthly_limit_bytes}
          label="Monthly budget"
        />
      )}
    </div>
  );
}
