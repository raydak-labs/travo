import { useState, useRef, useEffect, useMemo } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { ScrollText } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { useSystemLogs, useKernelLogs } from '@/hooks/use-system';
import { logsFilterFormSchema, type LogsFilterFormValues } from '@/lib/schemas/logs-forms';
import { defaultLogsFilters, type LogTab } from './logs-constants';
import { LogsToolbar } from './logs-toolbar';
import { LogsTextView } from './logs-text-view';

export function LogsPage() {
  const [activeTab, setActiveTab] = useState<LogTab>('system');
  const logRef = useRef<HTMLPreElement>(null);

  const { register, watch, setValue } = useForm<LogsFilterFormValues>({
    resolver: zodResolver(logsFilterFormSchema),
    defaultValues: defaultLogsFilters,
  });

  const lineFilter = watch('lineFilter');
  const serviceFilter = watch('serviceFilter');
  const levelFilter = watch('levelFilter');
  const customService = watch('customService');
  const showCustomInput = watch('showCustomInput');

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
    if (!lineFilter) return logs.lines;
    const lower = lineFilter.toLowerCase();
    return logs.lines.filter((entry) => entry.line.toLowerCase().includes(lower));
  }, [logs, lineFilter]);

  useEffect(() => {
    if (logRef.current) {
      logRef.current.scrollTop = logRef.current.scrollHeight;
    }
  }, [filteredLines]);

  const handleDownload = () => {
    if (filteredLines.length === 0) return;
    const text = filteredLines.map((e) => e.line).join('\n');
    const blob = new Blob([text], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `logs-${activeTab}-${new Date().toISOString().split('T')[0]}.txt`;
    a.click();
    URL.revokeObjectURL(url);
  };

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">System Logs</CardTitle>
          <ScrollText className="h-4 w-4 text-gray-500" />
        </CardHeader>
        <CardContent>
          <LogsToolbar
            activeTab={activeTab}
            setActiveTab={setActiveTab}
            register={register}
            setValue={setValue}
            serviceFilter={serviceFilter}
            levelFilter={levelFilter}
            showCustomInput={showCustomInput}
            filteredLines={filteredLines}
            onRefresh={() => refetch()}
            onDownload={handleDownload}
          />
          <LogsTextView
            logRef={logRef}
            isLoading={isLoading}
            filteredLines={filteredLines}
            lineFilter={lineFilter}
            logs={logs}
          />
        </CardContent>
      </Card>
    </div>
  );
}
