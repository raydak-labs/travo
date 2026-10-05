import type { ReactNode } from 'react';
import { cn } from '@/lib/cn';
import { StatusPill } from '@/components/ui/status-pill';

type StatValueProps = {
  label: string;
  /**
   * `null` means the value is genuinely unknown, which is different from a
   * falsy-but-known value such as `0` or an empty string. Callers must not
   * collapse the two: "0 bytes" and "we could not read it" are different
   * claims.
   */
  value: ReactNode | null;
  /** Extra line under the value, e.g. "Channel 36 · 80 MHz". */
  hint?: ReactNode;
  /** Marks a value last seen before a dropped connection; rendered dimmed. */
  stale?: boolean;
  className?: string;
};

/** A single labelled fact. The building block every status tile and card row needs. */
export function StatValue({ label, value, hint, stale = false, className }: StatValueProps) {
  const unknown = value === null || value === undefined;

  return (
    <div className={cn('min-w-0 space-y-1', className)}>
      <div className="flex items-center gap-2">
        <span className="text-xs font-medium text-gray-500 dark:text-gray-400">{label}</span>
        {stale && !unknown ? <StatusPill tone="stale">stale</StatusPill> : null}
      </div>
      {unknown ? (
        <p className="text-sm text-gray-400 dark:text-gray-500">No data</p>
      ) : (
        <p
          className={cn(
            'truncate text-sm font-medium text-gray-900 dark:text-white',
            stale && 'text-gray-400 dark:text-gray-500',
          )}
        >
          {value}
        </p>
      )}
      {hint ? <p className="text-xs text-gray-500 dark:text-gray-400">{hint}</p> : null}
    </div>
  );
}