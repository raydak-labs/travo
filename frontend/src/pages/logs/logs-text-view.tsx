import type { RefObject } from 'react';
import { Skeleton } from '@/components/ui/skeleton';
import type { LogEntry, LogResponse } from '@shared/index';
import { LogsLevelBadge } from './logs-level-badge';

type LogsTextViewProps = {
  logRef: RefObject<HTMLPreElement | null>;
  isLoading: boolean;
  filteredLines: readonly LogEntry[];
  lineFilter: string;
  logs: LogResponse | undefined;
};

export function LogsTextView({
  logRef,
  isLoading,
  filteredLines,
  lineFilter,
  logs,
}: LogsTextViewProps) {
  return (
    <>
      {isLoading ? (
        <div className="space-y-2">
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-4 w-5/6" />
          <Skeleton className="h-4 w-4/6" />
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-4 w-3/4" />
        </div>
      ) : (
        <pre
          ref={logRef}
          data-testid="log-content"
          className="max-h-[500px] overflow-y-auto rounded-md bg-gray-950 p-4 font-mono text-xs leading-relaxed text-green-400"
        >
          <code>
            {filteredLines.length > 0 ? (
              filteredLines.map((entry, i) => (
                <div key={i}>
                  <LogsLevelBadge level={entry.level} />
                  {entry.line}
                </div>
              ))
            ) : (
              <span className="text-gray-500">
                No log entries{lineFilter ? ' matching filter' : ''}
              </span>
            )}
          </code>
        </pre>
      )}

      {!isLoading && logs && (
        <p className="mt-2 text-xs text-gray-500">
          {filteredLines.length}
          {lineFilter ? ` / ${logs.total}` : ''} lines
        </p>
      )}
    </>
  );
}
