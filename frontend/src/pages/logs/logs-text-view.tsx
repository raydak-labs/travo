import type { RefObject } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { Skeleton } from '@/components/ui/skeleton';
import { QueryCard } from '@/components/ui/query-card';
import { EmptyState } from '@/components/ui/empty-state';
import type { LogEntry, LogResponse } from '@shared/index';
import { LogsLevelBadge } from './logs-level-badge';

type LogsTextViewProps = {
  logRef: RefObject<HTMLPreElement | null>;
  isLoading: boolean;
  /** Explicit failure flag from the owning query; see `loadFailed` below. */
  isError?: boolean;
  error?: unknown;
  onRetry?: () => void;
  filteredLines: readonly LogEntry[];
  lineFilter: string;
  logs: LogResponse | undefined;
};

export function LogsTextView({
  logRef,
  isLoading,
  isError = false,
  error,
  onRetry,
  filteredLines,
  lineFilter,
  logs,
}: LogsTextViewProps) {
  const queryClient = useQueryClient();
  // "No log entries" is only true for a request that succeeded. Both log
  // queries are always enabled, so data stays undefined exactly when the
  // request failed and no placeholder data is in play.
  const loadFailed = isError || (!isLoading && logs === undefined);
  const retry =
    onRetry ?? (() => void queryClient.refetchQueries({ queryKey: ['system', 'logs'] }));

  return (
    <>
      <QueryCard
        isLoading={isLoading}
        isError={loadFailed}
        error={error}
        onRetry={retry}
        loading={
          <div className="space-y-2">
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-5/6" />
            <Skeleton className="h-4 w-4/6" />
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-3/4" />
          </div>
        }
      >
        {filteredLines.length === 0 ? (
          <EmptyState message={lineFilter ? 'No log entries matching filter' : 'No log entries'} />
        ) : (
          <pre
            ref={logRef}
            data-testid="log-content"
            className="max-h-[500px] overflow-y-auto rounded-md bg-gray-950 p-4 font-mono text-xs leading-relaxed text-green-400"
          >
            <code>
              {filteredLines.map((entry, i) => (
                <div key={i}>
                  <LogsLevelBadge level={entry.level} />
                  {entry.line}
                </div>
              ))}
            </code>
          </pre>
        )}
      </QueryCard>

      {!isLoading && !loadFailed && logs && (
        <p className="mt-2 text-xs text-gray-500 dark:text-gray-400">
          {filteredLines.length}
          {lineFilter ? ` / ${logs.total}` : ''} lines
        </p>
      )}
    </>
  );
}
