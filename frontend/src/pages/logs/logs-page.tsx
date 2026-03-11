import { useState, useRef, useEffect, useMemo } from 'react';
import { ScrollText, Search, RefreshCw } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import { useSystemLogs, useKernelLogs } from '@/hooks/use-system';

type LogTab = 'system' | 'kernel';

export function LogsPage() {
  const [activeTab, setActiveTab] = useState<LogTab>('system');
  const [filter, setFilter] = useState('');
  const logRef = useRef<HTMLPreElement>(null);

  const { data: systemLogs, isLoading: systemLoading, refetch: refetchSystem } = useSystemLogs();
  const { data: kernelLogs, isLoading: kernelLoading, refetch: refetchKernel } = useKernelLogs();

  const logs = activeTab === 'system' ? systemLogs : kernelLogs;
  const isLoading = activeTab === 'system' ? systemLoading : kernelLoading;
  const refetch = activeTab === 'system' ? refetchSystem : refetchKernel;

  const filteredLines = useMemo(() => {
    if (!logs?.lines) return [];
    if (!filter) return logs.lines;
    const lower = filter.toLowerCase();
    return logs.lines.filter((entry) => entry.line.toLowerCase().includes(lower));
  }, [logs, filter]);

  // Auto-scroll to bottom when logs change
  useEffect(() => {
    if (logRef.current) {
      logRef.current.scrollTop = logRef.current.scrollHeight;
    }
  }, [filteredLines]);

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">System Logs</CardTitle>
          <ScrollText className="h-4 w-4 text-gray-500" />
        </CardHeader>
        <CardContent>
          {/* Tab buttons + controls */}
          <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div className="flex gap-2">
              <Button
                variant={activeTab === 'system' ? 'default' : 'outline'}
                size="sm"
                onClick={() => setActiveTab('system')}
              >
                System Log
              </Button>
              <Button
                variant={activeTab === 'kernel' ? 'default' : 'outline'}
                size="sm"
                onClick={() => setActiveTab('kernel')}
              >
                Kernel Log
              </Button>
            </div>
            <div className="flex gap-2">
              <div className="relative flex-1 sm:w-64">
                <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-gray-400" />
                <Input
                  placeholder="Filter logs…"
                  value={filter}
                  onChange={(e) => setFilter(e.target.value)}
                  className="pl-9"
                />
              </div>
              <Button
                variant="outline"
                size="icon"
                onClick={() => refetch()}
                aria-label="Refresh logs"
              >
                <RefreshCw className="h-4 w-4" />
              </Button>
            </div>
          </div>

          {/* Log display */}
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
              className="max-h-[500px] overflow-y-auto rounded-md bg-gray-950 p-4 text-xs leading-relaxed text-green-400 font-mono"
            >
              <code>
                {filteredLines.length > 0 ? (
                  filteredLines.map((entry, i) => <div key={i}>{entry.line}</div>)
                ) : (
                  <span className="text-gray-500">
                    No log entries{filter ? ' matching filter' : ''}
                  </span>
                )}
              </code>
            </pre>
          )}

          {/* Line count */}
          {!isLoading && logs && (
            <p className="mt-2 text-xs text-gray-500">
              {filteredLines.length}
              {filter ? ` / ${logs.total}` : ''} lines
            </p>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
