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
  className?: string;
};

/**
 * A compact two-direction link rate. Both directions share one unit when the
 * formatter picks the same one, so the common case reads `25.0 / 2.0 Mbit/s`.
 */
export function TrafficRate({
  rxBytesPerSec,
  txBytesPerSec,
  size = 'sm',
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
  const [downValue, downUnit] = splitRate(down);
  const [upValue, upUnit] = splitRate(up);
  const sharedUnit = downUnit === upUnit;
  const label = `Download ${down}, upload ${up}`;

  return (
    <span
      className={cn(
        'inline-flex min-w-0 flex-wrap items-center font-medium tabular-nums',
        textSize,
        'text-gray-700 dark:text-slate-300',
        gap,
        className,
      )}
      title={label}
    >
      <span className="sr-only">{label}</span>
      <span aria-hidden="true" className={cn('inline-flex items-center', gap)}>
        <ArrowDown className={cn('shrink-0 text-gray-400 dark:text-slate-500', iconSize)} />
        {downValue}
        {sharedUnit ? null : <span className="text-gray-500 dark:text-slate-400">{downUnit}</span>}
      </span>
      <span aria-hidden="true" className="text-gray-300 dark:text-slate-600">
        /
      </span>
      <span aria-hidden="true" className={cn('inline-flex items-center', gap)}>
        <ArrowUp className={cn('shrink-0 text-gray-400 dark:text-slate-500', iconSize)} />
        {upValue}
        <span className="text-gray-500 dark:text-slate-400">{upUnit}</span>
      </span>
    </span>
  );
}

function splitRate(rate: string): [string, string] {
  const [value, unit] = rate.split(' ');
  return [value, unit];
}
