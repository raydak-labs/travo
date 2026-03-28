import type { UseFormRegister } from 'react-hook-form';
import { Search, RefreshCw, Download } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import type { LogsFilterFormValues } from '@/lib/schemas/logs-forms';
import type { LogEntry } from '@shared/index';
import type { LogTab } from '@/pages/logs/logs-constants';

type LogsToolbarTabsAndSearchProps = {
  activeTab: LogTab;
  setActiveTab: (t: LogTab) => void;
  register: UseFormRegister<LogsFilterFormValues>;
  filteredLines: readonly LogEntry[];
  onRefresh: () => void;
  onDownload: () => void;
};

export function LogsToolbarTabsAndSearch({
  activeTab,
  setActiveTab,
  register,
  filteredLines,
  onRefresh,
  onDownload,
}: LogsToolbarTabsAndSearchProps) {
  return (
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
          <Input placeholder="Filter logs…" className="pl-9" {...register('lineFilter')} />
        </div>
        <Button
          variant="outline"
          size="icon"
          onClick={onDownload}
          disabled={filteredLines.length === 0}
          aria-label="Download logs"
        >
          <Download className="h-4 w-4" />
        </Button>
        <Button variant="outline" size="icon" onClick={onRefresh} aria-label="Refresh logs">
          <RefreshCw className="h-4 w-4" />
        </Button>
      </div>
    </div>
  );
}
