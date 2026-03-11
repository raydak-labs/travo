import { useState, useRef, useEffect, useMemo } from 'react';
import { ScrollText, Search, RefreshCw, Filter, Download } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import { useSystemLogs, useKernelLogs } from '@/hooks/use-system';

type LogTab = 'system' | 'kernel';

const SERVICE_FILTERS = [
  { label: 'All', value: '' },
  { label: 'dnsmasq', value: 'dnsmasq' },
  { label: 'AdGuardHome', value: 'AdGuardHome' },
  { label: 'wireguard', value: 'wireguard' },
  { label: 'netifd', value: 'netifd' },
  { label: 'hostapd', value: 'hostapd' },
  { label: 'dropbear', value: 'dropbear' },
] as const;

const LEVEL_FILTERS: { label: string; value: string }[] = [
  { label: 'All Levels', value: '' },
  { label: 'Error & above', value: 'err' },
  { label: 'Warning & above', value: 'warning' },
  { label: 'Notice & above', value: 'notice' },
  { label: 'Info & above', value: 'info' },
  { label: 'Debug (all)', value: 'debug' },
];

const LEVEL_COLORS: Record<string, string> = {
  emerg: 'bg-red-600 text-white',
  alert: 'bg-red-500 text-white',
  crit: 'bg-red-500 text-white',
  err: 'bg-red-400 text-white',
  warning: 'bg-amber-400 text-amber-950',
  notice: 'bg-green-400 text-green-950',
  info: 'bg-blue-400 text-white',
  debug: 'bg-gray-400 text-gray-950',
};

function LevelBadge({ level }: { level: string }) {
  if (!level) return null;
  const color = LEVEL_COLORS[level] ?? 'bg-gray-300 text-gray-800';
  return (
    <span
      className={`mr-2 inline-block rounded px-1.5 py-0.5 text-[10px] font-semibold uppercase leading-none ${color}`}
    >
      {level}
    </span>
  );
}

export function LogsPage() {
  const [activeTab, setActiveTab] = useState<LogTab>('system');
  const [filter, setFilter] = useState('');
  const [serviceFilter, setServiceFilter] = useState('');
  const [levelFilter, setLevelFilter] = useState('');
  const [customService, setCustomService] = useState('');
  const [showCustomInput, setShowCustomInput] = useState(false);
  const logRef = useRef<HTMLPreElement>(null);

  const activeService = showCustomInput ? customService : serviceFilter;

  const {
    data: systemLogs,
    isLoading: systemLoading,
    refetch: refetchSystem,
  } = useSystemLogs(
    activeTab === 'system' ? activeService || undefined : undefined,
    activeTab === 'system' ? levelFilter || undefined : undefined,
  );
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
                onClick={() => {
                  if (filteredLines.length === 0) return;
                  const text = filteredLines.map((e) => e.line).join('\n');
                  const blob = new Blob([text], { type: 'text/plain' });
                  const url = URL.createObjectURL(blob);
                  const a = document.createElement('a');
                  a.href = url;
                  a.download = `logs-${activeTab}-${new Date().toISOString().split('T')[0]}.txt`;
                  a.click();
                  URL.revokeObjectURL(url);
                }}
                disabled={filteredLines.length === 0}
                aria-label="Download logs"
              >
                <Download className="h-4 w-4" />
              </Button>
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

          {/* Service filter (system logs only) */}
          {activeTab === 'system' && (
            <div className="mb-4 flex flex-wrap items-center gap-2">
              <Filter className="h-4 w-4 text-gray-400" />
              {SERVICE_FILTERS.map((sf) => (
                <Button
                  key={sf.value}
                  variant={!showCustomInput && serviceFilter === sf.value ? 'default' : 'outline'}
                  size="sm"
                  className="h-7 text-xs"
                  onClick={() => {
                    setServiceFilter(sf.value);
                    setShowCustomInput(false);
                    setCustomService('');
                  }}
                >
                  {sf.label}
                </Button>
              ))}
              <Button
                variant={showCustomInput ? 'default' : 'outline'}
                size="sm"
                className="h-7 text-xs"
                onClick={() => {
                  setShowCustomInput(true);
                  setServiceFilter('');
                }}
              >
                Custom
              </Button>
              {showCustomInput && (
                <Input
                  placeholder="Service name…"
                  value={customService}
                  onChange={(e) => setCustomService(e.target.value)}
                  className="h-7 w-40 text-xs"
                  autoFocus
                />
              )}
            </div>
          )}

          {/* Level filter (system logs only) */}
          {activeTab === 'system' && (
            <div className="mb-4 flex flex-wrap items-center gap-2">
              <Filter className="h-4 w-4 text-gray-400" />
              <span className="text-xs text-gray-500">Level:</span>
              {LEVEL_FILTERS.map((lf) => (
                <Button
                  key={lf.value}
                  variant={levelFilter === lf.value ? 'default' : 'outline'}
                  size="sm"
                  className="h-7 text-xs"
                  onClick={() => setLevelFilter(lf.value)}
                >
                  {lf.label}
                </Button>
              ))}
            </div>
          )}

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
                  filteredLines.map((entry, i) => (
                    <div key={i}>
                      <LevelBadge level={entry.level} />
                      {entry.line}
                    </div>
                  ))
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
