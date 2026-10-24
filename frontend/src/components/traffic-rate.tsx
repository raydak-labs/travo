import { ArrowDown, ArrowUp } from 'lucide-react';
import { cn } from '@/lib/cn';
import { formatBitRate } from '@/lib/utils';

type TrafficRateProps = {
  /** Receive rate in bytes per second, or `null` until two samples exist. */
  rxBytesPerSec: number | null;
  /** Transmit rate in bytes per second, or `null` until two samples exist. */
  txBytesPerSec: number | null;
  /** `xs` is for the topology diagram; `sm` is for the dashboard quick status. */
  size?: 'sm' | 'xs';
  /** Inline reads as one sentence; stacked keeps the topology hint narrow. */
  layout?: 'inline' | 'stacked';
  className?: string;
};

/**
 * A compact two-direction link rate. Each direction keeps its own unit so the
 * value never reads as if only the second number carries the unit.
 */
export function TrafficRate({
  rxBytesPerSec,
  txBytesPerSec,
  size = 'sm',
  layout = 'inline',
  className,
}: TrafficRateProps) {
  const textSize = size === 'sm' ? 'text-sm' : 'text-[10px]';
  const iconSize = size === 'sm' ? 'h-3.5 w-3.5' : 'h-2.5 w-2.5';
  const gap = size === 'sm' ? 'gap-1' : 'gap-0.5';

  if (rxBytesPerSec === null || txBytesPerSec === null) {
    return (
      <span
        className={cn('text-gray-400 dark:text-gray-500', textSize, className)}
        aria-label="Traffic rate unknown"
      >
        —
      </span>
    );
  }

  const down = formatBitRate(rxBytesPerSec * 8);
  const up = formatBitRate(txBytesPerSec * 8);
  const label = `Download ${down}, upload ${up}`;

  return (
    <span
      className={cn(
        'inline-flex min-w-0 font-medium tabular-nums',
        layout === 'inline' ? 'flex-wrap items-center gap-1' : 'flex-col items-start gap-0.5',
        textSize,
        'text-gray-700 dark:text-slate-300',
        className,
      )}
      title={label}
    >
      <span className="sr-only">{label}</span>
      <Direction icon={ArrowDown} rate={down} gap={gap} iconSize={iconSize} />
      {layout === 'inline' ? (
        <span aria-hidden="true" className="text-gray-300 dark:text-slate-600">
          /
        </span>
      ) : null}
      <Direction icon={ArrowUp} rate={up} gap={gap} iconSize={iconSize} />
    </span>
  );
}

function Direction({
  icon: Icon,
  rate,
  gap,
  iconSize,
}: {
  icon: typeof ArrowDown;
  rate: string;
  gap: string;
  iconSize: string;
}) {
  const [value, unit] = rate.split(' ');
  return (
    <span aria-hidden="true" className={cn('inline-flex items-center', gap)}>
      <Icon className={cn('shrink-0 text-gray-400 dark:text-slate-500', iconSize)} />
      {value}
      <span className="text-gray-500 dark:text-slate-400">{unit}</span>
    </span>
  );
}
