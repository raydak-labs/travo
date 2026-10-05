import { formatBytes } from '@/lib/utils';

type UsageBarProps = {
  used: number;
  limit: number;
  label: string;
};

export function DataUsageUsageBar({ used, limit, label }: UsageBarProps) {
  // The bar is clamped so it stays inside its track, but the *number* is not:
  // at 4x the budget a clamped "100%" made 101% and 400% indistinguishable,
  // which is exactly the information a metered traveler needs.
  const barPct = limit > 0 ? Math.min((used / limit) * 100, 100) : 0;
  const over = limit > 0 && used > limit;
  // Budget pressure is a status, so it wears the status tokens: danger past the
  // limit, warn approaching it, info below.
  const color =
    over || barPct >= 90
      ? 'bg-[var(--status-danger-border)]'
      : barPct >= 80
        ? 'bg-[var(--status-warn-border)]'
        : 'bg-[var(--status-info-border)]';
  return (
    <div className="space-y-1">
      <div className="flex justify-between text-xs text-gray-500 dark:text-gray-400">
        <span>{label}</span>
        <span>
          {formatBytes(used)} / {formatBytes(limit)} ({barPct.toFixed(0)}%)
        </span>
      </div>
      <div className="h-2 w-full rounded-full bg-gray-200 dark:bg-gray-700">
        <div
          className={`h-2 rounded-full transition-all ${color}`}
          style={{ width: `${barPct}%` }}
        />
      </div>
      {over && (
        <p className="text-xs font-medium text-[var(--status-danger-text)]">
          Over budget by {formatBytes(used - limit)}
        </p>
      )}
    </div>
  );
}
