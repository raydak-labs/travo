import type { ReactNode } from 'react';
import { cn } from '@/lib/cn';
import { QueryCard } from '@/components/ui/query-card';
import { StatusPill, type StatusPillProps } from '@/components/ui/status-pill';

type SummaryBandProps = {
  /** Accessible name for the band; not visually rendered. */
  label: string;
  children: ReactNode;
  className?: string;
};

/**
 * Always-visible row of the most important facts on a page. Not collapsible:
 * its whole job is to answer "is this thing working" before the user scrolls.
 */
export function SummaryBand({ label, children, className }: SummaryBandProps) {
  return (
    <section
      aria-label={label}
      className={cn(
        'grid gap-px overflow-hidden rounded-lg border border-gray-200 bg-gray-200 shadow-sm',
        'sm:grid-cols-2 lg:grid-cols-5 dark:border-white/10 dark:bg-white/10',
        className,
      )}
    >
      {children}
    </section>
  );
}

type SummaryTileProps = {
  title: string;
  tone?: StatusPillProps['tone'];
  stale?: boolean;
  isLoading?: boolean;
  isError?: boolean;
  error?: unknown;
  onRetry?: () => void;
  children: ReactNode;
  className?: string;
};

/**
 * One cell of the band.
 *
 * Each tile degrades on its own: a tile whose query failed shows the shared
 * error with retry, and a tile still holding values from before a dropped
 * connection keeps them, dimmed and marked stale. Collapsing the whole band into
 * one error would throw away facts that were true seconds ago.
 */
export function SummaryTile({
  title,
  tone,
  stale = false,
  isLoading = false,
  isError = false,
  error,
  onRetry,
  children,
  className,
}: SummaryTileProps) {
  return (
    <div
      className={cn(
        'flex min-w-0 flex-col gap-2 bg-white p-4 dark:bg-gray-950',
        stale && 'opacity-70',
        className,
      )}
    >
      <div className="flex items-center gap-2">
        <span className="text-xs font-medium text-gray-500 dark:text-gray-400">{title}</span>
        {tone && !isLoading && !isError ? (
          <StatusPill tone={tone} withDot>
            {toneLabel(tone)}
          </StatusPill>
        ) : null}
      </div>
      <QueryCard
        isLoading={isLoading}
        isError={isError}
        error={error}
        onRetry={onRetry}
        loading={<div className="h-8 w-full animate-pulse rounded bg-gray-200 dark:bg-gray-800" />}
      >
        {children}
      </QueryCard>
    </div>
  );
}

function toneLabel(tone: NonNullable<StatusPillProps['tone']>): string {
  switch (tone) {
    case 'ok':
      return 'OK';
    case 'warn':
      return 'Warning';
    case 'danger':
      return 'Problem';
    case 'info':
      return 'Info';
    case 'stale':
      return 'Stale';
    default:
      return 'Inactive';
  }
}
