import { formatBytes } from '@/lib/utils';

type UsageBarProps = {
  used: number;
  limit: number;
  label: string;
};

export function DataUsageUsageBar({ used, limit, label }: UsageBarProps) {
  const pct = limit > 0 ? Math.min((used / limit) * 100, 100) : 0;
  const color = pct >= 90 ? 'bg-red-500' : pct >= 80 ? 'bg-yellow-500' : 'bg-blue-500';
  return (
    <div className="space-y-1">
      <div className="flex justify-between text-xs text-gray-500">
        <span>{label}</span>
        <span>
          {formatBytes(used)} / {formatBytes(limit)} ({pct.toFixed(0)}%)
        </span>
      </div>
      <div className="h-2 w-full rounded-full bg-gray-200 dark:bg-gray-700">
        <div className={`h-2 rounded-full transition-all ${color}`} style={{ width: `${pct}%` }} />
      </div>
    </div>
  );
}
